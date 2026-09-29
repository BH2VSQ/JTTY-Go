package dsp

import (
	"math"
)

// FFT computes an in-place radix-2 complex FFT. len(re) must equal len(im)
// and be a positive power of two. Forward transform uses the -2*pi sign.
func FFT(re, im []float64) {
	n := len(re)
	if n == 0 || n != len(im) || n&(n-1) != 0 {
		panic("dsp.FFT: length must be a non-zero power of two and match imag length")
	}

	for i, j := 1, 0; i < n; i++ {
		bit := n >> 1
		for ; j&bit != 0; bit >>= 1 {
			j ^= bit
		}
		j ^= bit
		if i < j {
			re[i], re[j] = re[j], re[i]
			im[i], im[j] = im[j], im[i]
		}
	}

	for length := 2; length <= n; length <<= 1 {
		half := length >> 1
		angle := -2 * math.Pi / float64(length)
		wpr, wpi := math.Cos(angle), math.Sin(angle)
		for i := 0; i < n; i += length {
			wr, wi := 1.0, 0.0
			for j := 0; j < half; j++ {
				even := i + j
				odd := even + half
				tr := wr*re[odd] - wi*im[odd]
				ti := wr*im[odd] + wi*re[odd]
				uR, uI := re[even], im[even]
				re[even], im[even] = uR+tr, uI+ti
				re[odd], im[odd] = uR-tr, uI-ti
				wr, wi = wr*wpr-wi*wpi, wr*wpi+wi*wpr
			}
		}
	}
}

// PowerSpectrum returns one-sided power bins for a real-valued input.
// A Hann window is applied in-place to a reusable work buffer supplied by the caller.
func PowerSpectrum(samples []float32, realWork, imagWork []float64) []float64 {
	n := len(samples)
	window := make([]float64, n)
	if n > 1 {
		for i := range window {
			window[i] = 0.5 - 0.5*math.Cos(2*math.Pi*float64(i)/float64(n-1))
		}
	}
	return PowerSpectrumWithWindow(samples, window, realWork, imagWork)
}

// PowerSpectrumWithWindow computes a one-sided power spectrum using a caller-
// supplied window. The window is reused by the detector so steady-state scans
// do not repeatedly evaluate trigonometric functions.
func PowerSpectrumWithWindow(samples []float32, window, realWork, imagWork []float64) []float64 {
	n := len(samples)
	if len(realWork) != n || len(imagWork) != n || len(window) != n {
		panic("dsp.PowerSpectrumWithWindow: buffers must match sample length")
	}
	if n == 0 || n&(n-1) != 0 {
		panic("dsp.PowerSpectrumWithWindow: sample length must be a non-zero power of two")
	}
	for i, x := range samples {
		realWork[i] = float64(x) * window[i]
		imagWork[i] = 0
	}
	FFT(realWork, imagWork)
	out := realWork[:n/2+1]
	for i := range out {
		p := realWork[i]*realWork[i] + imagWork[i]*imagWork[i]
		out[i] = math.Max(p/float64(n*n), 1e-24)
	}
	return out
}
