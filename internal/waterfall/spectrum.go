package waterfall

import (
	"math"
	"sync"
	"time"

	"github.com/BH2VSQ/jtty-go/internal/dsp"
)

type Frame struct {
	Timestamp time.Time `json:"timestamp"`
	StartHz   float64   `json:"startHz"`
	BinHz     float64   `json:"binHz"`
	Power     []float32 `json:"power"`
}

type Viewport struct {
	StartHz int `json:"startHz"`
	EndHz   int `json:"endHz"`
}

func FromPower(at time.Time, startHz, binHz float64, power []float32) Frame {
	return Frame{Timestamp: at, StartHz: startHz, BinHz: binHz, Power: append([]float32(nil), power...)}
}

// Analyzer is the display-only FFT path. It is intentionally independent of
// the detector scanner so changing display resolution never changes decoder
// timing or detector search parameters.
type Analyzer struct {
	mu          sync.Mutex
	sampleRate  int
	minHz       float64
	maxHz       float64
	fftSize     int
	hop         int
	window      []float64
	real        []float64
	imag        []float64
	buffer      []float32
	baseUTC     time.Time
	baseSamples uint64
}

func NewAnalyzer(sampleRate int, minHz, maxHz float64, fftSize, hop int) *Analyzer {
	if sampleRate <= 0 {
		sampleRate = 48000
	}
	if fftSize < 1024 || fftSize&(fftSize-1) != 0 {
		fftSize = 8192
	}
	if hop <= 0 || hop > fftSize {
		hop = fftSize / 4
	}
	if minHz < 0 {
		minHz = 0
	}
	if maxHz <= minHz || maxHz > float64(sampleRate/2) {
		maxHz = 2700
	}
	w := make([]float64, fftSize)
	for i := range w {
		w[i] = 0.5 - 0.5*math.Cos(2*math.Pi*float64(i)/float64(fftSize-1))
	}
	return &Analyzer{sampleRate: sampleRate, minHz: minHz, maxHz: maxHz, fftSize: fftSize, hop: hop, window: w, real: make([]float64, fftSize), imag: make([]float64, fftSize), buffer: make([]float32, 0, fftSize*2)}
}

func (a *Analyzer) SetFrequencyRange(minHz, maxHz float64) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if minHz < 0 {
		minHz = 0
	}
	if maxHz > float64(a.sampleRate/2) {
		maxHz = float64(a.sampleRate / 2)
	}
	if maxHz <= minHz {
		maxHz = minHz + float64(a.sampleRate)/float64(a.fftSize)
	}
	a.minHz, a.maxHz = minHz, maxHz
}

func (a *Analyzer) Process(samples []float32, timestamp time.Time) []Frame {
	if len(samples) == 0 {
		return nil
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.baseUTC.IsZero() {
		a.baseUTC = timestamp.UTC()
	}
	a.buffer = append(a.buffer, samples...)
	frames := make([]Frame, 0, 2)
	binHz := float64(a.sampleRate) / float64(a.fftSize)
	minBin := int(math.Max(0, math.Floor(a.minHz/binHz)))
	maxBin := int(math.Min(float64(a.fftSize/2), math.Ceil(a.maxHz/binHz)))
	if maxBin < minBin {
		maxBin = minBin
	}
	for len(a.buffer) >= a.fftSize {
		power := dsp.PowerSpectrumWithWindow(a.buffer[:a.fftSize], a.window, a.real, a.imag)
		if len(power) > 0 {
			end := maxBin
			if end >= len(power) {
				end = len(power) - 1
			}
			start := minBin
			if start >= len(power) {
				start = len(power) - 1
			}
			out := make([]float32, end-start+1)
			for i := start; i <= end; i++ {
				out[i-start] = float32(10 * math.Log10(math.Max(power[i], 1e-24)))
			}
			at := a.baseUTC.Add(time.Duration(a.baseSamples) * time.Second / time.Duration(a.sampleRate))
			frames = append(frames, Frame{Timestamp: at, StartHz: float64(start) * binHz, BinHz: binHz, Power: out})
		}
		shift := a.hop
		if shift > len(a.buffer) {
			shift = len(a.buffer)
		}
		copy(a.buffer, a.buffer[shift:])
		a.buffer = a.buffer[:len(a.buffer)-shift]
		a.baseSamples += uint64(shift)
	}
	return frames
}
