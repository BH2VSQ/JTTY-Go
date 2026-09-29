package dsp

import "testing"

func BenchmarkNoiseFloorInto(b *testing.B) {
	power := make([]float64, 2049)
	scratch := make([]float64, len(power))
	b.ReportAllocs()
	for i := range power {
		power[i] = float64(i + 1)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = NoiseFloorInto(power, scratch, 20)
	}
}
