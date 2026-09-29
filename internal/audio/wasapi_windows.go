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

type wasapiCapture struct {
	mu      sync.Mutex
	client  *wca.IAudioClient
	capture *wca.IAudioCaptureClient
	device  *wca.IMMDevice
	event   uintptr
	running bool
	cfg     CaptureConfig
	sink    FrameSink
	stop    chan struct{}
	wg      sync.WaitGroup
	format  *wca.WAVEFORMATEX
}

func (c *wasapiCapture) Open(ctx context.Context, cfg CaptureConfig, sink FrameSink) error {
	if cfg.SampleRate <= 0 {
		cfg.SampleRate = 48000
	}
	if cfg.Channels <= 0 {
		cfg.Channels = 1
	}
	if cfg.BufferMS <= 0 {
		cfg.BufferMS = 20
	}
	if cfg.ChannelMode == "" {
		cfg.ChannelMode = "Mono"
	}
	c.cfg = cfg
	c.sink = sink
	if err := initializeCOM(); err != nil {
		return err
	}
	var enumerator *wca.IMMDeviceEnumerator
	if err := wca.CoCreateInstance(wca.CLSID_MMDeviceEnumerator, 0, wca.CLSCTX_INPROC_SERVER, wca.IID_IMMDeviceEnumerator, &enumerator); err != nil {
		return err
	}
	defer enumerator.Release()
	if cfg.DeviceID == "" {
		var dev *wca.IMMDevice
		if err := enumerator.GetDefaultAudioEndpoint(wca.ECapture, wca.EConsole, &dev); err != nil {
			return err
		}
		c.device = dev
	} else {
		var dev *wca.IMMDevice
		if err := enumerator.GetDevice(cfg.DeviceID, &dev); err != nil {
			return err
		}
		c.device = dev
	}
	if err := c.device.Activate(wca.IID_IAudioClient, wca.CLSCTX_INPROC_SERVER, 0, &c.client); err != nil {
		return err
	}
	if err := c.client.GetMixFormat(&c.format); err != nil {
		return err
	}
	frames := uint32(cfg.SampleRate * cfg.BufferMS / 1000)
	if frames < 128 {
		frames = 128
	}
	duration := wca.REFERENCE_TIME(int64(cfg.BufferMS) * 10000)
	flags := uint32(wca.AUDCLNT_STREAMFLAGS_EVENTCALLBACK | wca.AUDCLNT_STREAMFLAGS_AUTOCONVERTPCM | wca.AUDCLNT_STREAMFLAGS_SRC_DEFAULT_QUALITY)
	if err := c.client.Initialize(wca.AUDCLNT_SHAREMODE_SHARED, flags, duration, 0, c.format, nil); err != nil {
		return err
	}
	_ = frames // shared-mode engine chooses packet sizing; event callback governs wakeups.
	if err := c.client.GetService(wca.IID_IAudioCaptureClient, &c.capture); err != nil {
		return err
	}
	var err error
	c.event, err = createAudioEvent()
	if err != nil {
		return fmt.Errorf("create audio event: %w", err)
	}
	if err := c.client.SetEventHandle(c.event); err != nil {
		return err
	}
	return nil
}

func (c *wasapiCapture) Start(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.running {
		return nil
	}
	if c.client == nil || c.capture == nil {
		return fmt.Errorf("capture not open")
	}
	if err := c.client.Start(); err != nil {
		return err
	}
	c.running = true
	c.stop = make(chan struct{})
	c.wg.Add(1)
	go c.loop()
	return nil
}

func (c *wasapiCapture) Stop() error {
	c.mu.Lock()
	if !c.running {
		c.mu.Unlock()
		return nil
	}
	c.running = false
	close(c.stop)
	c.mu.Unlock()
	c.wg.Wait()
	if c.client != nil {
		return c.client.Stop()
	}
	return nil
}

func (c *wasapiCapture) Close() error {
	_ = c.Stop()
	if c.capture != nil {
		c.capture.Release()
		c.capture = nil
	}
	if c.client != nil {
		c.client.Release()
		c.client = nil
	}
	if c.device != nil {
		c.device.Release()
		c.device = nil
	}
	if c.event != 0 {
		wca.CloseHandle(c.event)
		c.event = 0
	}
	if c.format != nil {
		ole.CoTaskMemFree(uintptr(unsafe.Pointer(c.format)))
		c.format = nil
	}
	ole.CoUninitialize()
	return nil
}

func (c *wasapiCapture) loop() {
	defer c.wg.Done()
	firstUTC := time.Now().UTC()
	var framesRead uint64
	for {
		select {
		case <-c.stop:
			return
		default:
		}
		wca.WaitForSingleObject(c.event, 100)
		for {
			var packet uint32
			if err := c.capture.GetNextPacketSize(&packet); err != nil {
				return
			}
			if packet == 0 {
				break
			}
			var data *byte
			var flags uint32
			var pos, qpc uint64
			var n uint32
			if err := c.capture.GetBuffer(&data, &n, &flags, &pos, &qpc); err != nil {
				return
			}
			var samples []float32
			if flags&wca.AUDCLNT_BUFFERFLAGS_SILENT != 0 {
				samples = make([]float32, int(n))
			} else if data != nil && n > 0 {
				samples = decodePCM(data, int(n), c.format, c.cfg.ChannelMode)
			}
			if c.sink != nil && len(samples) > 0 {
				inputRate := int(c.format.NSamplesPerSec)
				outputRate := c.cfg.SampleRate
				if outputRate <= 0 {
					outputRate = 48000
				}
				outputSamples := samples
				if inputRate > 0 && inputRate != outputRate {
					outputSamples = resampleLinear(samples, inputRate, outputRate)
				}
				ts := firstUTC.Add(time.Duration(framesRead) * time.Second / time.Duration(maxInt(outputRate, 1)))
				c.sink.OnPCMFrame(PCMFrame{Timestamp: ts, Samples: outputSamples, SampleRate: outputRate, Channels: 1})
				framesRead += uint64(len(outputSamples))
			} else {
				framesRead += uint64(n)
			}
			_ = pos
			_ = qpc
			_ = c.capture.ReleaseBuffer(n)
		}
	}
}

func isIEEEFloatFormat(wfx *wca.WAVEFORMATEX) bool {
	if wfx == nil {
		return false
	}
	if wfx.WFormatTag == 3 {
		return true
	}
	// WAVE_FORMAT_EXTENSIBLE (0xFFFE) stores the actual subtype GUID after
	// WAVEFORMATEX. IEEE float starts with Data1 = 3.
	if wfx.WFormatTag != 0xFFFE || wfx.CbSize < 22 {
		return false
	}
	base := uintptr(unsafe.Pointer(wfx))
	guid := unsafe.Slice((*byte)(unsafe.Pointer(base+24)), 16)
	return guid[0] == 3 && guid[1] == 0 && guid[2] == 0 && guid[3] == 0
}

func decodePCM(ptr *byte, frames int, wfx *wca.WAVEFORMATEX, mode string) []float32 {
	channels := int(wfx.NChannels)
	if channels < 1 {
		channels = 1
	}
	bits := int(wfx.WBitsPerSample)
	bytesPerSample := (bits + 7) / 8
	if bytesPerSample < 1 {
		return nil
	}
	data := unsafe.Slice(ptr, frames*channels*bytesPerSample)
	out := make([]float32, frames)
	floatFormat := isIEEEFloatFormat(wfx)
	for i := 0; i < frames; i++ {
		var sum float64
		count := 0
		startCh, endCh := 0, channels
		switch mode {
		case "Left":
			endCh = 1
		case "Right":
			startCh, endCh = 1, 2
		case "Both", "Mono":
		default:
		}
		if channels == 1 {
			startCh, endCh = 0, 1
		}
		if startCh >= channels {
			startCh = 0
			endCh = 1
		}
		if endCh > channels {
			endCh = channels
		}
		for ch := startCh; ch < endCh; ch++ {
			off := (i*channels + ch) * bytesPerSample
			var v float64
			if floatFormat && bits == 32 {
				v = float64(*(*float32)(unsafe.Pointer(&data[off])))
			} else if bits == 16 {
				v = float64(int16(data[off])|int16(data[off+1])<<8) / 32768
			} else if bits == 24 {
				x := int32(data[off]) | int32(data[off+1])<<8 | int32(data[off+2])<<16
				if x&0x800000 != 0 {
					x |= ^0xffffff
				}
				v = float64(x) / 8388608
			} else if bits == 32 {
				x := int32(data[off]) | int32(data[off+1])<<8 | int32(data[off+2])<<16 | int32(data[off+3])<<24
				v = float64(x) / 2147483648
			} else {
				v = 0
			}
			sum += v
			count++
		}
		if count == 0 {
			count = 1
		}
		out[i] = float32(math.Max(-1, math.Min(1, sum/float64(count))))
	}
	return out
}
