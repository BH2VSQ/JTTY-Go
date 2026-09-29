package detector

import (
	"math"
	"sort"
	"time"

	"github.com/BH2VSQ/jtty-go/internal/dsp"
)

type ScannerConfig struct {
	SampleRate          float64
	MinFrequencyHz      float64
	MaxFrequencyHz      float64
	FFTSize             int
	ThresholdDb         float64
	MinimumSeparationHz float64
	NoisePercentile     float64
}

type ScanResult struct {
	Candidates []Candidate
	Power      []float32
	MinHz      float64
	BinHz      float64
}

type Scanner struct {
	cfg       ScannerConfig
	real      []float64
	imag      []float64
	window    []float64
	noiseWork []float64
	fftHz     float64
}

func NewScanner(cfg ScannerConfig) *Scanner {
	if cfg.SampleRate <= 0 {
		cfg.SampleRate = 48000
	}
	if cfg.MinFrequencyHz < 0 {
		cfg.MinFrequencyHz = 0
	}
	if cfg.MaxFrequencyHz <= cfg.MinFrequencyHz {
		cfg.MaxFrequencyHz = 3000
	}
	if cfg.FFTSize < 256 || cfg.FFTSize&(cfg.FFTSize-1) != 0 {
		cfg.FFTSize = 4096
	}
	if cfg.ThresholdDb <= 0 {
		cfg.ThresholdDb = 8
	}
	if cfg.MinimumSeparationHz <= 0 {
		cfg.MinimumSeparationHz = 35
	}
	if cfg.NoisePercentile <= 0 {
		cfg.NoisePercentile = 20
	}
	window := make([]float64, cfg.FFTSize)
	for i := range window {
		window[i] = 0.5 - 0.5*math.Cos(2*math.Pi*float64(i)/float64(cfg.FFTSize-1))
	}
	return &Scanner{cfg: cfg, real: make([]float64, cfg.FFTSize), imag: make([]float64, cfg.FFTSize), window: window, noiseWork: make([]float64, cfg.FFTSize/2+1), fftHz: cfg.SampleRate / float64(cfg.FFTSize)}
}

// FFTSize returns the configured transform length.
func (s *Scanner) FFTSize() int { return s.cfg.FFTSize }

// SetThresholdDB changes the candidate threshold without rebuilding the FFT buffers.
func (s *Scanner) SetThresholdDB(db float64) {
	if db < 0 {
		db = 0
	}
	if db > 60 {
		db = 60
	}
	s.cfg.ThresholdDb = db
}

// SetFrequencyRange changes the displayed and candidate-search range without
// rebuilding the FFT buffers. The range is intentionally bounded by the
// capture Nyquist limit.
func (s *Scanner) SetFrequencyRange(minHz, maxHz float64) {
	if minHz < 0 {
		minHz = 0
	}
	nyquist := s.cfg.SampleRate / 2
	if maxHz > nyquist {
		maxHz = nyquist
	}
	if maxHz <= minHz {
		maxHz = minHz + s.fftHz
	}
	s.cfg.MinFrequencyHz = minHz
	s.cfg.MaxFrequencyHz = maxHz
}

func (s *Scanner) FrequencyRange() (float64, float64) {
	return s.cfg.MinFrequencyHz, s.cfg.MaxFrequencyHz
}

// Scan converts one FFT window into candidate signals.
func (s *Scanner) Scan(samples []float32, at time.Time) []Candidate {
	return s.ScanFrame(samples, at).Candidates
}

func (s *Scanner) ScanFrame(samples []float32, at time.Time) ScanResult {
	result := ScanResult{MinHz: s.cfg.MinFrequencyHz, BinHz: s.fftHz}
	if len(samples) != s.cfg.FFTSize {
		return result
	}
	power := dsp.PowerSpectrumWithWindow(samples, s.window, s.real, s.imag)
	noise := dsp.NoiseFloorInto(power, s.noiseWork, s.cfg.NoisePercentile)
	if noise <= 0 {
		return result
	}

	displayMinBin := int(math.Max(0, math.Floor(s.cfg.MinFrequencyHz/s.fftHz)))
	displayMaxBin := int(math.Min(float64(len(power)-1), math.Ceil(s.cfg.MaxFrequencyHz/s.fftHz)))
	if displayMaxBin < displayMinBin {
		return result
	}
	result.Power = make([]float32, displayMaxBin-displayMinBin+1)
	for i := displayMinBin; i <= displayMaxBin; i++ {
		result.Power[i-displayMinBin] = float32(10 * math.Log10(math.Max(power[i], 1e-24)))
	}

	minBin := displayMinBin
	if minBin < 1 {
		minBin = 1
	}
	maxBin := int(math.Min(float64(len(power)-2), math.Ceil(s.cfg.MaxFrequencyHz/s.fftHz)))
	if maxBin < minBin {
		return result
	}

	cands := make([]Candidate, 0, 16)
	for i := minBin; i <= maxBin; i++ {
		p := power[i]
		if p <= power[i-1] || p < power[i+1] {
			continue
		}
		snr := dsp.PowerToDB(p, noise)
		if snr < s.cfg.ThresholdDb {
			continue
		}
		y1 := math.Log(math.Max(power[i-1], 1e-24))
		y2 := math.Log(math.Max(power[i], 1e-24))
		y3 := math.Log(math.Max(power[i+1], 1e-24))
		den := y1 - 2*y2 + y3
		offset := 0.0
		if math.Abs(den) > 1e-12 {
			offset = 0.5 * (y1 - y3) / den
		}
		if offset > 0.5 {
			offset = 0.5
		}
		if offset < -0.5 {
			offset = -0.5
		}
		freq := (float64(i) + offset) * s.fftHz
		cands = append(cands, Candidate{FrequencyHz: freq, SNR: snr, FirstSeen: at, LastSeen: at, State: StateSearching})
	}
	sort.Slice(cands, func(i, j int) bool { return cands[i].SNR > cands[j].SNR })
	selected := cands[:0]
	for _, c := range cands {
		tooClose := false
		for _, keep := range selected {
			if math.Abs(keep.FrequencyHz-c.FrequencyHz) < s.cfg.MinimumSeparationHz {
				tooClose = true
				break
			}
		}
		if !tooClose {
			selected = append(selected, c)
		}
	}
	result.Candidates = selected
	return result
}
