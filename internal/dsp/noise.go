package dsp

import (
	"math"
	"sort"
)

// NoiseFloor returns a robust noise estimate using the lower percentile of power.
// A low percentile is less sensitive to sparse narrowband JTTY signals.
func NoiseFloor(power []float64, percentile float64) float64 {
	return NoiseFloorInto(power, nil, percentile)
}

// NoiseFloorInto estimates the lower-percentile noise floor using caller-owned
// scratch storage. Realtime scanner paths pass a reusable buffer here so the
// 11+ Hz detector loop does not allocate and copy the complete FFT spectrum on
// every scan.
func NoiseFloorInto(power, scratch []float64, percentile float64) float64 {
	if len(power) == 0 {
		return 0
	}
	if percentile <= 0 {
		percentile = 20
	}
	if percentile >= 100 {
		percentile = 50
	}
	if len(scratch) < len(power) {
		scratch = make([]float64, len(power))
	}
	scratch = scratch[:len(power)]
	copy(scratch, power)
	sort.Float64s(scratch)
	idx := int(math.Round((percentile / 100) * float64(len(scratch)-1)))
	return scratch[idx]
}

func PowerToDB(power, reference float64) float64 {
	if power <= 0 {
		power = 1e-24
	}
	if reference <= 0 {
		reference = 1
	}
	return 10 * math.Log10(power/reference)
}
