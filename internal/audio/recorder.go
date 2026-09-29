package audio

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Recorder struct {
	mu          sync.Mutex
	baseDir     string
	file        *os.File
	currentDate string
	sampleRate  int
	channels    int
	dataBytes   uint32
	activePath  string
	jobs        chan PCMFrame
	stop        chan struct{}
	wg          sync.WaitGroup
	closed      bool
}

func NewRecorder() *Recorder { return &Recorder{} }

func (r *Recorder) Start(dir string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if dir == "" {
		return fmt.Errorf("empty record directory")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if r.jobs != nil && r.baseDir == dir {
		return nil
	}
	if r.jobs != nil {
		r.stopLocked()
	}
	r.baseDir = dir
	r.currentDate = ""
	r.sampleRate = 0
	r.channels = 0
	r.dataBytes = 0
	r.activePath = ""
	r.jobs = make(chan PCMFrame, 32)
	r.stop = make(chan struct{})
	r.closed = false
	r.wg.Add(1)
	go r.worker()
	return nil
}

func (r *Recorder) Stop() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.stopLocked()
}

func (r *Recorder) stopLocked() error {
	if r.jobs != nil {
		jobs, stop := r.jobs, r.stop
		r.jobs = nil
		r.stop = nil
		close(jobs)
		if stop != nil {
			close(stop)
		}
		r.mu.Unlock()
		r.wg.Wait()
		r.mu.Lock()
	}
	return r.closeFileLocked()
}

func (r *Recorder) worker() {
	defer r.wg.Done()
	for frame := range r.jobs {
		_ = r.Write(frame)
	}
}

func (r *Recorder) WriteAsync(frame PCMFrame) error {
	if len(frame.Samples) == 0 {
		return nil
	}
	frame.Samples = append([]float32(nil), frame.Samples...)
	r.mu.Lock()
	jobs := r.jobs
	r.mu.Unlock()
	if jobs == nil {
		return fmt.Errorf("recorder is not running")
	}
	select {
	case jobs <- frame:
		return nil
	default:
		return fmt.Errorf("record queue full")
	}
}

func (r *Recorder) closeFileLocked() error {
	if r.file == nil {
		r.activePath = ""
		return nil
	}
	if err := writeWAVHeader(r.file, r.sampleRate, r.channels, r.dataBytes); err != nil {
		_ = r.file.Close()
		r.file = nil
		r.activePath = ""
		return err
	}
	err := r.file.Close()
	r.file = nil
	r.activePath = ""
	return err
}

func (r *Recorder) IsActive() bool      { r.mu.Lock(); defer r.mu.Unlock(); return r.file != nil }
func (r *Recorder) CurrentPath() string { r.mu.Lock(); defer r.mu.Unlock(); return r.activePath }

func (r *Recorder) Write(frame PCMFrame) error {
	if len(frame.Samples) == 0 {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.baseDir == "" {
		return fmt.Errorf("recorder is not configured")
	}
	rate := frame.SampleRate
	if rate <= 0 {
		rate = 48000
	}
	channels := frame.Channels
	if channels <= 0 {
		channels = 1
	}
	ts := frame.Timestamp.UTC()
	if ts.IsZero() {
		ts = time.Now().UTC()
	}
	date := ts.Format("20060102")
	if r.file == nil || r.currentDate != date || r.sampleRate != rate || r.channels != channels {
		if r.file != nil {
			if err := r.closeFileLocked(); err != nil {
				return err
			}
		}
		path := filepath.Join(r.baseDir, date+".wav")
		f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
		if err != nil {
			return err
		}
		info, err := f.Stat()
		if err != nil {
			_ = f.Close()
			return err
		}
		if info.Size() == 0 {
			if _, err := f.Write(make([]byte, 44)); err != nil {
				_ = f.Close()
				return err
			}
			r.dataBytes = 0
		} else {
			if info.Size() < 44 {
				_ = f.Close()
				return fmt.Errorf("invalid WAV file %s", path)
			}
			r.dataBytes = uint32(info.Size() - 44)
		}
		r.file = f
		r.currentDate = date
		r.sampleRate = rate
		r.channels = channels
		r.activePath = path
		if _, err := f.Seek(0, io.SeekEnd); err != nil {
			_ = r.closeFileLocked()
			return err
		}
		if err := writeWAVHeader(f, rate, channels, r.dataBytes); err != nil {
			_ = r.closeFileLocked()
			return err
		}
	}
	buf := make([]byte, len(frame.Samples)*2)
	for i, sample := range frame.Samples {
		if sample > 1 {
			sample = 1
		} else if sample < -1 {
			sample = -1
		}
		v := int16(sample * 32767)
		binary.LittleEndian.PutUint16(buf[i*2:], uint16(v))
	}
	if _, err := r.file.Write(buf); err != nil {
		return err
	}
	r.dataBytes += uint32(len(buf))
	return writeWAVHeader(r.file, r.sampleRate, r.channels, r.dataBytes)
}

func writeWAVHeader(w io.WriteSeeker, sampleRate, channels int, dataBytes uint32) error {
	if sampleRate <= 0 {
		sampleRate = 48000
	}
	if channels <= 0 {
		channels = 1
	}
	byteRate := uint32(sampleRate * channels * 2)
	blockAlign := uint16(channels * 2)
	if _, err := w.Seek(0, io.SeekStart); err != nil {
		return err
	}
	header := make([]byte, 44)
	copy(header[0:4], []byte("RIFF"))
	binary.LittleEndian.PutUint32(header[4:8], 36+dataBytes)
	copy(header[8:12], []byte("WAVE"))
	copy(header[12:16], []byte("fmt "))
	binary.LittleEndian.PutUint32(header[16:20], 16)
	binary.LittleEndian.PutUint16(header[20:22], 1)
	binary.LittleEndian.PutUint16(header[22:24], uint16(channels))
	binary.LittleEndian.PutUint32(header[24:28], uint32(sampleRate))
	binary.LittleEndian.PutUint32(header[28:32], byteRate)
	binary.LittleEndian.PutUint16(header[32:34], blockAlign)
	binary.LittleEndian.PutUint16(header[34:36], 16)
	copy(header[36:40], []byte("data"))
	binary.LittleEndian.PutUint32(header[40:44], dataBytes)
	if _, err := w.Write(header); err != nil {
		return err
	}
	_, err := w.Seek(0, io.SeekEnd)
	return err
}
