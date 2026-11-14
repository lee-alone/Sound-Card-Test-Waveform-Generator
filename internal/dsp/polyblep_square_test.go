package dsp

import (
	"math"
	"testing"
)

func TestPolyBLEPSquareZeroAllocations(t *testing.T) {
	gen := NewPolyBLEPSquareGenerator()
	err := gen.Prepare(Config{
		SampleRate:   48000,
		Frequency:    1000.0,
		Amplitude:    0.8,
		InitialPhase: 0.0,
		DCOffset:     0.0,
	})
	if err != nil {
		t.Fatalf("Prepare failed: %v", err)
	}

	dst := make([]float64, 512)

	allocs := testing.AllocsPerRun(1000, func() {
		gen.Render(dst)
	})

	if allocs != 0 {
		t.Errorf("expected 0 allocations in Render, got %v", allocs)
	}
}

func TestPolyBLEPSquareDCOffset(t *testing.T) {
	// For 1kHz at 48kHz, period is 48 samples.
	// Render an exact multiple of periods to measure steady-state DC.
	gen := NewPolyBLEPSquareGenerator()
	err := gen.Prepare(Config{
		SampleRate:   48000,
		Frequency:    1000.0,
		Amplitude:    1.0,
		InitialPhase: 0.0,
		DCOffset:     0.0,
	})
	if err != nil {
		t.Fatalf("Prepare failed: %v", err)
	}

	samples := make([]float64, 48000) // 1000 full periods
	gen.Render(samples)

	sum := 0.0
	for _, s := range samples {
		sum += s
	}
	meanDC := sum / float64(len(samples))

	t.Logf("1kHz Square Mean DC: %e", meanDC)
	if math.Abs(meanDC) >= 1e-6 {
		t.Errorf("expected DC offset < 1e-6, got %e", meanDC)
	}
}

func TestPolyBLEPSquareAliasingSuppressionMatrix(t *testing.T) {
	testMatrix := []struct {
		rate int
		freq float64
		desc string
	}{
		{rate: 48000, freq: 1000.0, desc: "1kHz @ 48kHz (f/Fs=0.021)"},
		{rate: 48000, freq: 5000.0, desc: "5kHz @ 48kHz (f/Fs=0.104)"},
		{rate: 48000, freq: 10000.0, desc: "10kHz @ 48kHz (f/Fs=0.208)"},
		{rate: 48000, freq: 19200.0, desc: "19.2kHz @ 48kHz (f/Fs=0.400)"},
		{rate: 96000, freq: 38400.0, desc: "38.4kHz @ 96kHz (f/Fs=0.400)"},
		{rate: 192000, freq: 76800.0, desc: "76.8kHz @ 192kHz (f/Fs=0.400)"},
	}

	const N = 131072

	for _, tc := range testMatrix {
		t.Run(tc.desc, func(t *testing.T) {
			// Find coherent bin closest to target frequency
			kFund := int(math.Round(tc.freq * float64(N) / float64(tc.rate)))
			fCoh := float64(kFund) * float64(tc.rate) / float64(N)

			gen := NewPolyBLEPSquareGenerator()
			err := gen.Prepare(Config{
				SampleRate:   tc.rate,
				Frequency:    fCoh,
				Amplitude:    1.0,
				InitialPhase: 0.0,
				DCOffset:     0.0,
			})
			if err != nil {
				t.Fatalf("Prepare failed: %v", err)
			}

			samples := make([]float64, N)
			gen.Render(samples)

			// Compute FFT power spectrum using Blackman window to avoid boundary leakage
			windowed := make([]float64, N)
			for i, s := range samples {
				// Exact Blackman window
				a0 := 0.42
				a1 := 0.5
				a2 := 0.08
				theta := 2.0 * math.Pi * float64(i) / float64(N-1)
				w := a0 - a1*math.Cos(theta) + a2*math.Cos(2.0*theta)
				windowed[i] = s * w
			}

			power := ComputePowerSpectrum(windowed)
			half := len(power)

			// Fundamental power
			fundBin := kFund
			fundPower := 0.0
			for b := fundBin - 6; b <= fundBin+6; b++ {
				if b >= 0 && b < half && power[b] > fundPower {
					fundPower = power[b]
				}
			}

			// Define audible band: up to 20kHz or Nyquist (whichever is lower)
			audibleFreq := 20000.0
			if audibleFreq > float64(tc.rate)*0.5 {
				audibleFreq = float64(tc.rate) * 0.5
			}
			maxAudibleBin := int(math.Round(audibleFreq * float64(N) / float64(tc.rate)))
			if maxAudibleBin > half {
				maxAudibleBin = half
			}

			// Mask legitimate odd harmonics: (2m + 1) * fCoh
			isLegit := make([]bool, maxAudibleBin)
			minBin := int(math.Round(20.0 * float64(N) / float64(tc.rate))) // ignore < 20Hz DC

			for m := 0; m < 500; m++ {
				harmFreq := float64(2*m+1) * fCoh
				if harmFreq > float64(tc.rate)*0.5 {
					break
				}
				hb := int(math.Round(harmFreq * float64(N) / float64(tc.rate)))
				for b := hb - 8; b <= hb+8; b++ {
					if b >= 0 && b < maxAudibleBin {
						isLegit[b] = true
					}
				}
			}

			// Find maximum spurious / aliasing peak in the audible band
			maxSpuriousPower := 0.0
			maxSpuriousBin := 0
			for b := minBin; b < maxAudibleBin; b++ {
				if !isLegit[b] {
					if power[b] > maxSpuriousPower {
						maxSpuriousPower = power[b]
						maxSpuriousBin = b
					}
				}
			}

			spuriousDbc := -400.0
			if maxSpuriousPower > 0 && fundPower > 0 {
				spuriousDbc = 10.0 * math.Log10(maxSpuriousPower/fundPower)
			}

			spuriousFreq := float64(maxSpuriousBin) * float64(tc.rate) / float64(N)
			t.Logf("[%s] f_coh=%.1f Hz, max audible spurious at %.1f Hz: %.2f dBc",
				tc.desc, fCoh, spuriousFreq, spuriousDbc)

			if spuriousDbc >= -60.0 {
				t.Errorf("[%s] expected alias suppression < -60 dBc, got %.2f dBc", tc.desc, spuriousDbc)
			}
		})
	}
}
