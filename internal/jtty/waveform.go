package jtty

import "math"

func GFSKPulse(bt, t float64) float64 {
	c := math.Pi * math.Sqrt(2.0/math.Ln2)
	return 0.5 * (math.Erf(c*bt*(t+0.5)) - math.Erf(c*bt*(t-0.5)))
}

func jttyPhaseIncrements(tones []int, samplesPerSymbol int, bt float64) []float64 {
	if len(tones) == 0 || samplesPerSymbol <= 0 {
		return nil
	}
	out := make([]float64, (len(tones)+2)*samplesPerSymbol)
	pulse := make([]float64, 3*samplesPerSymbol)
	jttyPhaseIncrementsInto(out, pulse, tones, samplesPerSymbol, bt)
	return out
}

func jttyPhaseIncrementsInto(dphi, pulse []float64, tones []int, samplesPerSymbol int, bt float64) {
	if len(tones) == 0 || samplesPerSymbol <= 0 || cap(dphi) < (len(tones)+2)*samplesPerSymbol || cap(pulse) < 3*samplesPerSymbol {
		return
	}
	dphi = dphi[:(len(tones)+2)*samplesPerSymbol]
	pulse = pulse[:3*samplesPerSymbol]
	clear(dphi)
	for i := range pulse {
		tt := (float64(i) + 1 - 1.5*float64(samplesPerSymbol)) / float64(samplesPerSymbol)
		pulse[i] = GFSKPulse(bt, tt)
	}
	nsym := len(tones)
	peak := 2 * math.Pi / float64(samplesPerSymbol)
	for j, tone := range tones {
		start := j * samplesPerSymbol
		for i := 0; i < 3*samplesPerSymbol && start+i < len(dphi); i++ {
			dphi[start+i] += peak * pulse[i] * float64(tone)
		}
	}
	firstTone := tones[0]
	for i := 0; i < 2*samplesPerSymbol; i++ {
		dphi[i] += peak * float64(firstTone) * pulse[samplesPerSymbol+i]
	}
	lastTone := tones[len(tones)-1]
	start := nsym * samplesPerSymbol
	for i := 0; i < 2*samplesPerSymbol; i++ {
		dphi[start+i] += peak * float64(lastTone) * pulse[i]
	}
}

func generateJTTYComplexWaveformInto(out []complex128, dphi, pulse []float64, tones []int, samplesPerSymbol int, bt, sampleRate, f0 float64) {
	if len(tones) == 0 || samplesPerSymbol <= 0 || sampleRate <= 0 || len(out) < len(tones)*samplesPerSymbol {
		return
	}
	out = out[:len(tones)*samplesPerSymbol]
	needDphi := (len(tones) + 2) * samplesPerSymbol
	if cap(dphi) < needDphi || cap(pulse) < 3*samplesPerSymbol {
		return
	}
	jttyPhaseIncrementsInto(dphi, pulse, tones, samplesPerSymbol, bt)
	dphi = dphi[:needDphi]
	dt := 1 / sampleRate
	for i := range dphi {
		dphi[i] += 2 * math.Pi * f0 * dt
	}
	phi := 0.0
	k := 0
	for i := samplesPerSymbol; i < samplesPerSymbol+len(out) && k < len(out); i++ {
		out[k] = complex(math.Cos(phi), math.Sin(phi))
		phi += dphi[i]
		phi = math.Mod(phi, 2*math.Pi)
		if phi < 0 {
			phi += 2 * math.Pi
		}
		k++
	}
	nramp := int(math.Round(float64(samplesPerSymbol) / 8))
	if nramp > 0 && nramp*2 <= len(out) {
		for i := 0; i < nramp; i++ {
			x := 0.5 * (1 - math.Cos(2*math.Pi*float64(i)/(2*float64(nramp))))
			out[i] *= complex(x, 0)
			j := len(out) - nramp + i
			y := 0.5 * (1 + math.Cos(2*math.Pi*float64(i)/(2*float64(nramp))))
			out[j] *= complex(y, 0)
		}
	}
}

func GenerateJTTYComplexWaveform(tones []int, samplesPerSymbol int, bt, sampleRate, f0 float64) []complex128 {
	if len(tones) == 0 || samplesPerSymbol <= 0 || sampleRate <= 0 {
		return nil
	}
	dphi := jttyPhaseIncrements(tones, samplesPerSymbol, bt)
	out := make([]complex128, len(tones)*samplesPerSymbol)
	dt := 1 / sampleRate
	for i := range dphi {
		dphi[i] += 2 * math.Pi * f0 * dt
	}
	phi := 0.0
	k := 0
	for i := samplesPerSymbol; i < samplesPerSymbol+len(out) && k < len(out); i++ {
		out[k] = complex(math.Cos(phi), math.Sin(phi))
		phi += dphi[i]
		phi = math.Mod(phi, 2*math.Pi)
		if phi < 0 {
			phi += 2 * math.Pi
		}
		k++
	}
	nramp := int(math.Round(float64(samplesPerSymbol) / 8))
	if nramp > 0 && nramp*2 <= len(out) {
		for i := 0; i < nramp; i++ {
			x := 0.5 * (1 - math.Cos(2*math.Pi*float64(i)/(2*float64(nramp))))
			out[i] *= complex(x, 0)
			j := len(out) - nramp + i
			y := 0.5 * (1 + math.Cos(2*math.Pi*float64(i)/(2*float64(nramp))))
			out[j] *= complex(y, 0)
		}
	}
	return out
}

// GenerateJTTYWaveformInto writes a JTTY waveform into caller-owned storage.
// It is allocation-free after the supplied scratch buffers have reached their
// steady-state capacity and is used by the TX path to avoid per-frame GC churn.
func GenerateJTTYWaveformInto(out []float32, dphi, pulse []float64, tones []int, samplesPerSymbol int, bt, sampleRate, f0 float64) {
	if len(tones) == 0 || samplesPerSymbol <= 0 || sampleRate <= 0 || len(out) < len(tones)*samplesPerSymbol {
		return
	}
	out = out[:len(tones)*samplesPerSymbol]
	needDphi := (len(tones) + 2) * samplesPerSymbol
	if cap(dphi) < needDphi || cap(pulse) < 3*samplesPerSymbol {
		return
	}
	jttyPhaseIncrementsInto(dphi, pulse, tones, samplesPerSymbol, bt)
	dphi = dphi[:needDphi]
	dt := 1 / sampleRate
	for i := range dphi {
		dphi[i] += 2 * math.Pi * f0 * dt
	}
	phi := 0.0
	k := 0
	for i := samplesPerSymbol; i < samplesPerSymbol+len(out) && k < len(out); i++ {
		out[k] = float32(math.Sin(phi))
		phi += dphi[i]
		phi = math.Mod(phi, 2*math.Pi)
		if phi < 0 {
			phi += 2 * math.Pi
		}
		k++
	}
	nramp := int(math.Round(float64(samplesPerSymbol) / 8))
	if nramp > 0 && nramp*2 <= len(out) {
		for i := 0; i < nramp; i++ {
			x := 0.5 * (1 - math.Cos(2*math.Pi*float64(i)/(2*float64(nramp))))
			out[i] *= float32(x)
			j := len(out) - nramp + i
			y := 0.5 * (1 + math.Cos(2*math.Pi*float64(i)/(2*float64(nramp))))
			out[j] *= float32(y)
		}
	}
}

// GenerateJTTYWaveform follows WSJT-X gen_jttywave.f90. For JTTY the
// nominal sample rate is 12000 Hz and the default Gaussian BT is 2.0.
func GenerateJTTYWaveform(tones []int, samplesPerSymbol int, bt, sampleRate, f0 float64) []float32 {
	if len(tones) == 0 || samplesPerSymbol <= 0 || sampleRate <= 0 {
		return nil
	}
	dphi := jttyPhaseIncrements(tones, samplesPerSymbol, bt)
	out := make([]float32, len(tones)*samplesPerSymbol)
	dt := 1 / sampleRate
	for i := range dphi {
		dphi[i] += 2 * math.Pi * f0 * dt
	}
	phi := 0.0
	k := 0
	for i := samplesPerSymbol; i < samplesPerSymbol+len(out) && k < len(out); i++ {
		out[k] = float32(math.Sin(phi))
		phi += dphi[i]
		phi = math.Mod(phi, 2*math.Pi)
		if phi < 0 {
			phi += 2 * math.Pi
		}
		k++
	}
	nramp := int(math.Round(float64(samplesPerSymbol) / 8))
	if nramp > 0 && nramp*2 <= len(out) {
		for i := 0; i < nramp; i++ {
			x := 0.5 * (1 - math.Cos(2*math.Pi*float64(i)/(2*float64(nramp))))
			out[i] = float32(float64(out[i]) * x)
			j := len(out) - nramp + i
			y := 0.5 * (1 + math.Cos(2*math.Pi*float64(i)/(2*float64(nramp))))
			out[j] = float32(float64(out[j]) * y)
		}
	}
	return out
}

func GenerateSyncWaveform(samplesPerSymbol int) []complex128 {
	out := make([]complex128, 13*samplesPerSymbol)
	phase := 0.0
	for i, tone := range SyncTones {
		step := 2 * math.Pi * float64(tone) / float64(samplesPerSymbol)
		for j := 0; j < samplesPerSymbol; j++ {
			out[i*samplesPerSymbol+j] = complex(math.Cos(phase), math.Sin(phase))
			phase += step
		}
	}
	return out
}
