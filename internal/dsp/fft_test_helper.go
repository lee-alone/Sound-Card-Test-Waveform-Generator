package dsp

import (
	"math"
	"math/cmplx"
)

// In-place Cooley-Tukey Radix-2 FFT for complex128 data (len must be a power of 2)
func fftRadix2(a []complex128) {
	n := len(a)
	if n <= 1 {
		return
	}

	// Bit-reversal permutation
	j := 0
	for i := 0; i < n-1; i++ {
		if i < j {
			a[i], a[j] = a[j], a[i]
		}
		k := n / 2
		for k <= j {
			j -= k
			k /= 2
		}
		j += k
	}

	// Cooley-Tukey butterfly computations
	for l := 2; l <= n; l <<= 1 {
		halfL := l / 2
		angle := -2.0 * math.Pi / float64(l)
		wStep := cmplx.Rect(1.0, angle)
		for i := 0; i < n; i += l {
			w := complex(1.0, 0.0)
			for m := 0; m < halfL; m++ {
				u := a[i+m]
				v := a[i+m+halfL] * w
				a[i+m] = u + v
				a[i+m+halfL] = u - v
				w *= wStep
			}
		}
	}
}

// ComputePowerSpectrum returns normalized power spectrum for the first N/2 bins.
func ComputePowerSpectrum(samples []float64) []float64 {
	n := len(samples)
	buf := make([]complex128, n)
	for i, s := range samples {
		buf[i] = complex(s, 0.0)
	}

	fftRadix2(buf)

	half := n / 2
	power := make([]float64, half)
	for i := 0; i < half; i++ {
		mag := cmplx.Abs(buf[i])
		power[i] = mag * mag
	}
	return power
}
