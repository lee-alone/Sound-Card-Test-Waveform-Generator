package dsp

import (
	"math"
	"testing"
)

func TestPolyBLEPTriangleZeroAllocations(t *testing.T) {
	gen := NewPolyBLEPTriangleGenerator()
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

func TestPolyBLEPTriangleDCOffset(t *testing.T) {
	// For 1kHz at 48kHz, period is 48 samples.
	// Render an exact multiple of periods to measure steady-state DC.
	gen := NewPolyBLEPTriangleGenerator()
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

	t.Logf("1kHz Triangle Mean DC: %e", meanDC)
	if math.Abs(meanDC) >= 1e-6 {
		t.Errorf("expected DC offset < 1e-6, got %e", meanDC)
	}
}

func TestPolyBLEPTriangleHarmonicSlopesAndAliasing(t *testing.T) {
	const (
		rate = 48000
		freq = 1000.0
		N    = 131072
	)

	kFund := int(math.Round(freq * float64(N) / float64(rate)))
	fCoh := float64(kFund) * float64(rate) / float64(N)

	gen := NewPolyBLEPTriangleGenerator()
	err := gen.Prepare(Config{
		SampleRate:   rate,
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

	// Window with Blackman
	windowed := make([]float64, N)
	for i, s := range samples {
		theta := 2.0 * math.Pi * float64(i) / float64(N-1)
		w := 0.42 - 0.5*math.Cos(theta) + 0.08*math.Cos(2.0*theta)
		windowed[i] = s * w
	}

	power := ComputePowerSpectrum(windowed)
	half := len(power)

	findPeakPower := func(targetBin int) float64 {
		maxP := 0.0
		for b := targetBin - 4; b <= targetBin+4; b++ {
			if b >= 0 && b < half && power[b] > maxP {
				maxP = power[b]
			}
		}
		return maxP
	}

	pFund := findPeakPower(kFund)
	pH3 := findPeakPower(3 * kFund)
	pH5 := findPeakPower(5 * kFund)
	pH7 := findPeakPower(7 * kFund)
	pH9 := findPeakPower(9 * kFund)

	dBH3 := 10.0 * math.Log10(pH3/pFund)
	dBH5 := 10.0 * math.Log10(pH5/pFund)
	dBH7 := 10.0 * math.Log10(pH7/pFund)
	dBH9 := 10.0 * math.Log10(pH9/pFund)

	t.Logf("Harmonics: H3=%.2f dBc, H5=%.2f dBc, H7=%.2f dBc, H9=%.2f dBc", dBH3, dBH5, dBH7, dBH9)

	// Measure overall decay slope across 3, 5, 7, 9 harmonics using linear regression
	// and end-to-end slope from H3 to H9.
	oct3 := math.Log2(3.0)
	oct5 := math.Log2(5.0)
	oct7 := math.Log2(7.0)
	oct9 := math.Log2(9.0)

	// Mean of X (octaves relative to fundamental) and Y (dB relative to fundamental)
	meanX := (oct3 + oct5 + oct7 + oct9) / 4.0
	meanY := (dBH3 + dBH5 + dBH7 + dBH9) / 4.0

	var num, den float64
	for _, pt := range [][2]float64{{oct3, dBH3}, {oct5, dBH5}, {oct7, dBH7}, {oct9, dBH9}} {
		dx := pt[0] - meanX
		dy := pt[1] - meanY
		num += dx * dy
		den += dx * dx
	}
	regressionSlope := num / den
	overallSlope3to9 := (dBH9 - dBH3) / (oct9 - oct3)

	t.Logf("Harmonic Decay Slope: Linear Regression = %.2f dB/oct, H3->H9 Overall = %.2f dB/oct",
		regressionSlope, overallSlope3to9)

	const (
		targetSlope = -12.0
		slopeTol    = 0.5
	)

	if math.Abs(regressionSlope-targetSlope) > slopeTol {
		t.Errorf("expected regression slope in [%.1f, %.1f], got %.2f dB/oct",
			targetSlope-slopeTol, targetSlope+slopeTol, regressionSlope)
	}
	if math.Abs(overallSlope3to9-targetSlope) > slopeTol {
		t.Errorf("expected overall H3->H9 slope in [%.1f, %.1f], got %.2f dB/oct",
			targetSlope-slopeTol, targetSlope+slopeTol, overallSlope3to9)
	}

	// Verify alias suppression in audible band (< -70 dBc)
	maxAudibleBin := int(math.Round(20000.0 * float64(N) / float64(rate)))
	isLegit := make([]bool, maxAudibleBin)
	minBin := int(math.Round(20.0 * float64(N) / float64(rate)))

	for m := 0; m < 500; m++ {
		harmFreq := float64(2*m+1) * fCoh
		if harmFreq > float64(rate)*0.5 {
			break
		}
		hb := int(math.Round(harmFreq * float64(N) / float64(rate)))
		for b := hb - 8; b <= hb+8; b++ {
			if b >= 0 && b < maxAudibleBin {
				isLegit[b] = true
			}
		}
	}

	maxSpuriousPower := 0.0
	for b := minBin; b < maxAudibleBin; b++ {
		if !isLegit[b] && power[b] > maxSpuriousPower {
			maxSpuriousPower = power[b]
		}
	}

	spuriousDbc := -400.0
	if maxSpuriousPower > 0 && pFund > 0 {
		spuriousDbc = 10.0 * math.Log10(maxSpuriousPower/pFund)
	}

	t.Logf("Max Audible Spurious: %.2f dBc", spuriousDbc)
	if spuriousDbc >= -70.0 {
		t.Errorf("expected alias suppression < -70 dBc, got %.2f dBc", spuriousDbc)
	}
}

func TestPolyBLEPTriangleAliasingSuppressionMatrix(t *testing.T) {
	testMatrix := []struct {
		rate int
		freq float64
		desc string
	}{
		{rate: 48000, freq: 2000.0, desc: "2kHz @ 48kHz"},
		{rate: 48000, freq: 5000.0, desc: "5kHz @ 48kHz"},
		{rate: 48000, freq: 10000.0, desc: "10kHz @ 48kHz"},
		{rate: 48000, freq: 19200.0, desc: "19.2kHz @ 48kHz"},
		{rate: 96000, freq: 38400.0, desc: "38.4kHz @ 96kHz"},
		{rate: 192000, freq: 76800.0, desc: "76.8kHz @ 192kHz"},
	}

	const N = 131072

	for _, tc := range testMatrix {
		t.Run(tc.desc, func(t *testing.T) {
			kFund := int(math.Round(tc.freq * float64(N) / float64(tc.rate)))
			fCoh := float64(kFund) * float64(tc.rate) / float64(N)

			gen := NewPolyBLEPTriangleGenerator()
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

			windowed := make([]float64, N)
			for i, s := range samples {
				theta := 2.0 * math.Pi * float64(i) / float64(N-1)
				w := 0.42 - 0.5*math.Cos(theta) + 0.08*math.Cos(2.0*theta)
				windowed[i] = s * w
			}

			power := ComputePowerSpectrum(windowed)
			half := len(power)

			fundBin := kFund
			fundPower := 0.0
			for b := fundBin - 6; b <= fundBin+6; b++ {
				if b >= 0 && b < half && power[b] > fundPower {
					fundPower = power[b]
				}
			}

			audibleFreq := 20000.0
			if audibleFreq > float64(tc.rate)*0.5 {
				audibleFreq = float64(tc.rate) * 0.5
			}
			maxAudibleBin := int(math.Round(audibleFreq * float64(N) / float64(tc.rate)))
			if maxAudibleBin > half {
				maxAudibleBin = half
			}

			isLegit := make([]bool, maxAudibleBin)
			minBin := int(math.Round(20.0 * float64(N) / float64(tc.rate)))

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

			maxSpuriousPower := 0.0
			for b := minBin; b < maxAudibleBin; b++ {
				if !isLegit[b] && power[b] > maxSpuriousPower {
					maxSpuriousPower = power[b]
				}
			}

			spuriousDbc := -400.0
			if maxSpuriousPower > 0 && fundPower > 0 {
				spuriousDbc = 10.0 * math.Log10(maxSpuriousPower/fundPower)
			}

			t.Logf("[%s] f_coh=%.1f Hz: max audible spurious = %.2f dBc", tc.desc, fCoh, spuriousDbc)
			if spuriousDbc >= -70.0 {
				t.Errorf("[%s] expected alias suppression < -70 dBc, got %.2f dBc", tc.desc, spuriousDbc)
			}
		})
	}
}
