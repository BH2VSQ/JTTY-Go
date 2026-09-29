//go:build windows

package audio

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"
	"unsafe"

	"github.com/degubites/go-wca/pkg/wca"
	"github.com/go-ole/go-ole"
)

// wasapiOutput is a shared-mode WASAPI renderer. The device mix format is
// used as the hardware contract; JTTY's 12 kHz waveform is linearly resampled
// here so the UI and decoder remain independent of sound-card rates.
type wasapiOutput struct {
	mu           sync.Mutex
	client       *wca.IAudioClient
	render       *wca.IAudioRenderClient
	device       *wca.IMMDevice
	format       *wca.WAVEFORMATEX
	config       OutputConfig
	bufferFrames uint32
	running      bool
}

func (o *wasapiOutput) Open(ctx context.Context, cfg OutputConfig) error {
	_ = ctx
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.client != nil {
		_ = o.closeLocked()
	}
	if cfg.SampleRate <= 0 {
		cfg.SampleRate = 12000
	}
	if cfg.Level < 0 {
		cfg.Level = 0
	}
	if cfg.Level > 100 {
		cfg.Level = 100
	}
	if cfg.Channels <= 0 {
		cfg.Channels = 1
	}
	o.config = cfg
	if err := initializeCOM(); err != nil {
		return err
	}
	var enum *wca.IMMDeviceEnumerator
	if err := wca.CoCreateInstance(wca.CLSID_MMDeviceEnumerator, 0, wca.CLSCTX_INPROC_SERVER, wca.IID_IMMDeviceEnumerator, &enum); err != nil {
		return err
	}
	defer enum.Release()
	var dev *wca.IMMDevice
	if cfg.DeviceID == "" {
		if err := enum.GetDefaultAudioEndpoint(wca.ERender, wca.EConsole, &dev); err != nil {
			return err
		}
	} else if err := enum.GetDevice(cfg.DeviceID, &dev); err != nil {
		return err
	}
	o.device = dev
	if err := o.device.Activate(wca.IID_IAudioClient, wca.CLSCTX_INPROC_SERVER, 0, &o.client); err != nil {
		return err
	}
	if err := o.client.GetMixFormat(&o.format); err != nil {
		return err
	}
	flags := uint32(wca.AUDCLNT_STREAMFLAGS_AUTOCONVERTPCM | wca.AUDCLNT_STREAMFLAGS_SRC_DEFAULT_QUALITY)
	duration := wca.REFERENCE_TIME(int64(maxInt(cfg.BufferMS, 20)) * 10000)
	if err := o.client.Initialize(wca.AUDCLNT_SHAREMODE_SHARED, flags, duration, 0, o.format, nil); err != nil {
		return err
	}
	if err := o.client.GetBufferSize(&o.bufferFrames); err != nil {
		return err
	}
	if err := o.client.GetService(wca.IID_IAudioRenderClient, &o.render); err != nil {
		return err
	}
	return nil
}

func (o *wasapiOutput) Play(ctx context.Context, samples []float32, sampleRate int) error {
	if len(samples) == 0 {
		return nil
	}
	o.mu.Lock()
	client, render, format := o.client, o.render, o.format
	bufferFrames := o.bufferFrames
	level := o.config.Level
	o.mu.Unlock()
	if client == nil || render == nil || format == nil {
		return fmt.Errorf("audio output not open")
	}
	if sampleRate <= 0 {
		sampleRate = 12000
	}
	x := samples
	if int(format.NSamplesPerSec) != sampleRate {
		x = resampleLinear(x, sampleRate, int(format.NSamplesPerSec))
	}
	gain := float32(level) / 100
	if gain != 1 {
		// TX waveform buffers are caller-owned and generated solely for this
		// Play call, so scale in place instead of allocating a second PCM slice.
		for i := range x {
			x[i] *= gain
		}
	}
	channels := int(format.NChannels)
	if channels <= 0 {
		channels = 1
	}
	bytesPerSample := (int(format.WBitsPerSample) + 7) / 8
	if bytesPerSample < 2 {
		return fmt.Errorf("unsupported render format: %d bits", format.WBitsPerSample)
	}
	o.mu.Lock()
	if !o.running {
		if err := client.Start(); err != nil {
			o.mu.Unlock()
			return err
		}
		o.running = true
	}
	o.mu.Unlock()
	pos := 0
	for pos < len(x) {
		select {
		case <-ctx.Done():
			_ = o.Stop()
			return ctx.Err()
		default:
		}
		var padding uint32
		if err := client.GetCurrentPadding(&padding); err != nil {
			return err
		}
		avail := bufferFrames - minU32(bufferFrames, padding)
		if avail == 0 {
			time.Sleep(1 * time.Millisecond)
			continue
		}
		frames := int(avail)
		if frames > len(x)-pos {
			frames = len(x) - pos
		}
		var data *byte
		if err := render.GetBuffer(uint32(frames), &data); err != nil {
			return err
		}
		if err := writeRenderPCM(data, x[pos:pos+frames], frames, channels, int(format.WBitsPerSample), format); err != nil {
			_ = render.ReleaseBuffer(uint32(frames), 0)
			return err
		}
		if err := render.ReleaseBuffer(uint32(frames), 0); err != nil {
			return err
		}
		pos += frames
	}
	// Wait until all submitted audio is consumed so PTT is not dropped early.
	for {
		select {
		case <-ctx.Done():
			_ = o.Stop()
			return ctx.Err()
		default:
		}
		var padding uint32
		if err := client.GetCurrentPadding(&padding); err != nil {
			return err
		}
		if padding == 0 {
			break
		}
		time.Sleep(2 * time.Millisecond)
	}
	return nil
}

func (o *wasapiOutput) Stop() error {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.client == nil || !o.running {
		return nil
	}
	o.running = false
	return o.client.Stop()
}
func (o *wasapiOutput) Close() error { o.mu.Lock(); defer o.mu.Unlock(); return o.closeLocked() }
func (o *wasapiOutput) closeLocked() error {
	if o.client != nil {
		_ = o.client.Stop()
		o.running = false
	}
	if o.render != nil {
		o.render.Release()
		o.render = nil
	}
	if o.client != nil {
		o.client.Release()
		o.client = nil
	}
	if o.device != nil {
		o.device.Release()
		o.device = nil
	}
	if o.format != nil {
		ole.CoTaskMemFree(uintptr(unsafe.Pointer(o.format)))
		o.format = nil
	}
	o.bufferFrames = 0
	return nil
}

func scalePCM(in []float32, gain float32) []float32 {
	out := make([]float32, len(in))
	for i, v := range in {
		out[i] = v * gain
	}
	return out
}
func resampleLinear(in []float32, from, to int) []float32 {
	if len(in) == 0 || from <= 0 || to <= 0 || from == to {
		return append([]float32(nil), in...)
	}
	n := int(math.Round(float64(len(in)) * float64(to) / float64(from)))
	if n < 1 {
		n = 1
	}
	out := make([]float32, n)
	for i := 0; i < n; i++ {
		src := float64(i) * float64(from) / float64(to)
		j := int(src)
		frac := float32(src - float64(j))
		if j >= len(in)-1 {
			out[i] = in[len(in)-1]
		} else {
			out[i] = in[j]*(1-frac) + in[j+1]*frac
		}
	}
	return out
}
func renderFormatIsFloat(wfx *wca.WAVEFORMATEX) bool {
	if wfx == nil {
		return false
	}
	if wfx.WFormatTag == 3 {
		return true
	}
	if wfx.WFormatTag != 0xFFFE || wfx.CbSize < 22 {
		return false
	}
	base := uintptr(unsafe.Pointer(wfx))
	guid := unsafe.Slice((*byte)(unsafe.Pointer(base+24)), 16)
	return guid[0] == 3 && guid[1] == 0 && guid[2] == 0 && guid[3] == 0
}

func writeRenderPCM(ptr *byte, samples []float32, frames, channels, bits int, format *wca.WAVEFORMATEX) error {
	bps := (bits + 7) / 8
	if bps < 2 {
		return fmt.Errorf("unsupported PCM sample width %d", bits)
	}
	data := unsafe.Slice(ptr, frames*channels*bps)
	floatFmt := bits == 32 && renderFormatIsFloat(format)
	for i := 0; i < frames; i++ {
		v := float64(samples[i])
		if v > 1 {
			v = 1
		}
		if v < -1 {
			v = -1
		}
		for ch := 0; ch < channels; ch++ {
			off := (i*channels + ch) * bps
			if floatFmt && bits == 32 {
				*(*float32)(unsafe.Pointer(&data[off])) = float32(v)
				continue
			}
			switch bits {
			case 16:
				q := int16(math.Round(v * 32767))
				data[off] = byte(q)
				data[off+1] = byte(q >> 8)
			case 24:
				q := int32(math.Round(v * 8388607))
				data[off] = byte(q)
				data[off+1] = byte(q >> 8)
				data[off+2] = byte(q >> 16)
			case 32:
				q := int32(math.Round(v * 2147483647))
				data[off] = byte(q)
				data[off+1] = byte(q >> 8)
				data[off+2] = byte(q >> 16)
				data[off+3] = byte(q >> 24)
			default:
				return fmt.Errorf("unsupported PCM bit depth %d", bits)
			}
		}
	}
	return nil
}
func minU32(a, b uint32) uint32 {
	if a < b {
		return a
	}
	return b
}
func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
