package waterfall

import (
	"math"
	"testing"
	"time"
)

func TestAnalyzerHighResolutionFFT(t *testing.T) {
	const (
		rate = 48000
		fft  = 16384
		hop  = 4096
	)
	a := NewAnalyzer(rate, 0, 2700, fft, hop)
	samples := make([]float32, fft)
	for i := range samples {
		samples[i] = float32(0.8 * math.Sin(2*math.Pi*1000*float64(i)/rate))
	}
	frames := a.Process(samples, time.Unix(0, 0).UTC())
	if len(frames) != 1 {
		t.Fatalf("got %d waterfall frames, want 1", len(frames))
	}
	f := frames[0]
	if f.BinHz <= 2.9 || f.BinHz >= 3.0 {
		t.Fatalf("unexpected bin width %.6f Hz", f.BinHz)
	}
	if len(f.Power) < 900 {
		t.Fatalf("got only %d display bins", len(f.Power))
	}
	if math.Abs(f.StartHz) > 0.01 {
		t.Fatalf("unexpected start frequency %.3f Hz", f.StartHz)
	}
}

func TestAnalyzerRetainsStreamingWindowAcrossChunks(t *testing.T) {
	a := NewAnalyzer(48000, 0, 2700, 16384, 4096)
	first := make([]float32, 8192)
	second := make([]float32, 8192)
	if got := a.Process(first, time.Unix(1, 0).UTC()); len(got) != 0 {
		t.Fatalf("first half unexpectedly produced %d frames", len(got))
	}
	if got := a.Process(second, time.Unix(1, 500000000).UTC()); len(got) != 1 {
		t.Fatalf("combined chunks produced %d frames, want 1", len(got))
	}
}
