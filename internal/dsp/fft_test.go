package dsp

import (
	"math"
	"testing"
)

func TestFFTSingleTone(t *testing.T) {
	const n = 1024
	const k = 37
	re := make([]float64, n)
	im := make([]float64, n)
	for i := 0; i < n; i++ {
		re[i] = math.Sin(2 * math.Pi * k * float64(i) / n)
	}
	p := PowerSpectrum(func() []float32 {
		x := make([]float32, n)
		for i := range x {
			x[i] = float32(re[i])
		}
		return x
	}(), re, im)
	peak := 0
	for i := 1; i < len(p); i++ {
		if p[i] > p[peak] {
			peak = i
		}
	}
	if peak != k && peak != n-k {
		t.Fatalf("peak=%d want=%d", peak, k)
	}
}
