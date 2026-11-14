package dsp

import (
	"math"
	"testing"
)

func TestSineNumericalCorrectness(t *testing.T) {
	gen := NewSineGenerator()
	fs := 96000
	freq := 1000.0
	amp := 0.8
	dc := 0.05
	phase0 := math.Pi / 6.0

	cfg := Config{
		SampleRate:   fs,
		Frequency:    freq,
		Amplitude:    amp,
		InitialPhase: phase0,
		DCOffset:     dc,
	}

	if err := gen.Prepare(cfg); err != nil {
		t.Fatalf("Prepare failed: %v", err)
	}

	n := 4096
	samples := make([]float64, n)
	gen.Render(samples)

	// Analytical verification
	step := (2.0 * math.Pi * freq) / float64(fs)
	var maxAbsErr float64
	var sumSqErr float64
	var sumSamples float64

	for i := 0; i < n; i++ {
		expectedPhase := phase0 + float64(i)*step
		expected := amp*math.Sin(expectedPhase) + dc
		err := math.Abs(samples[i] - expected)
		if err > maxAbsErr {
			maxAbsErr = err
		}
		sumSqErr += err * err
		sumSamples += samples[i]
	}

	rmsErr := math.Sqrt(sumSqErr / float64(n))
	meanDC := sumSamples / float64(n)

	t.Logf("Numerical Correctness: MaxAbsErr = %e, RMSErr = %e, MeanDC = %f", maxAbsErr, rmsErr, meanDC)

	// In double precision float64 arithmetic, error against direct formula should be tiny (< 1e-12)
	if maxAbsErr > 1e-12 {
		t.Errorf("maxAbsErr = %e exceeds limit 1e-12", maxAbsErr)
	}
	if rmsErr > 1e-12 {
		t.Errorf("rmsErr = %e exceeds limit 1e-12", rmsErr)
	}

	// Frequency analytical verification
	derivedFreq := (gen.PhaseStep() * float64(fs)) / (2.0 * math.Pi)
	freqErr := math.Abs(derivedFreq - freq)
	t.Logf("Frequency Analytical Error = %e Hz", freqErr)
	if freqErr > 1e-12 {
		t.Errorf("frequency analytical error = %e exceeds 1e-12 Hz", freqErr)
	}
}

func TestSineSpectralPurityCoherent(t *testing.T) {
	// Quantified criteria from Schedule T1.4:
	// Fs = 192kHz, N = 131072, coherent sampling condition where fundamental falls on integer FFT bin.
	// HD2 ~ HD9 must be below -130 dBc (with -140 dBc target).
	fs := 192000
	n := 131072 // 2^17

	// Choose prime bin k for coherent sampling:
	k := 683
	freq := (float64(k) * float64(fs)) / float64(n) // ~1000.48828125 Hz
	t.Logf("Coherent bin k=%d, Freq=%.8f Hz, Fs=%d, N=%d", k, freq, fs, n)

	gen := NewSineGenerator()
	cfg := Config{
		SampleRate:   fs,
		Frequency:    freq,
		Amplitude:    1.0,
		InitialPhase: 0.0,
		DCOffset:     0.0,
	}

	if err := gen.Prepare(cfg); err != nil {
		t.Fatalf("Prepare failed: %v", err)
	}

	samples := make([]float64, n)
	gen.Render(samples)

	// Compute power spectrum
	power := ComputePowerSpectrum(samples)
	pCarrier := power[k]
	if pCarrier <= 0 {
		t.Fatalf("carrier power at bin %d is zero", k)
	}

	t.Logf("Carrier Power at bin %d: %e", k, pCarrier)

	// Check HD2 through HD9
	for h := 2; h <= 9; h++ {
		hBin := h * k
		if hBin >= len(power) {
			t.Fatalf("harmonic %d bin %d exceeds spectrum length %d", h, hBin, len(power))
		}
		pHarmonic := power[hBin]
		dBc := -300.0 // noise floor fallback
		if pHarmonic > 0 {
			dBc = 10.0 * math.Log10(pHarmonic/pCarrier)
		}
		t.Logf("  HD%d at bin %d: %.2f dBc", h, hBin, dBc)

		if dBc > -130.0 {
			t.Errorf("HD%d = %.2f dBc exceeds required limit of -130 dBc", h, dBc)
		}
		if dBc > -140.0 {
			t.Logf("Note: HD%d = %.2f dBc did not reach optional optimization target of -140 dBc", h, dBc)
		}
	}
}

func TestSineZeroAllocations(t *testing.T) {
	gen := NewSineGenerator()
	cfg := Config{
		SampleRate: 192000,
		Frequency:  1000.0,
		Amplitude:  0.9,
	}
	if err := gen.Prepare(cfg); err != nil {
		t.Fatalf("Prepare failed: %v", err)
	}

	buf := make([]float64, 1024)
	allocs := testing.AllocsPerRun(1000, func() {
		gen.Render(buf)
	})

	if allocs != 0 {
		t.Errorf("Sine.Render allocs = %v, want 0", allocs)
	}
}
