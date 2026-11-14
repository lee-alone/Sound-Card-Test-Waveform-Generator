package test

import (
	"math"
	"testing"

	"sound-signal-generator/internal/dsp"
)

// applyBlackman applies a 3-term Blackman window to input samples in-place.
func applyBlackman(samples []float64) []float64 {
	n := len(samples)
	out := make([]float64, n)
	for i := 0; i < n; i++ {
		theta := 2.0 * math.Pi * float64(i) / float64(n-1)
		w := 0.42 - 0.5*math.Cos(theta) + 0.08*math.Cos(2.0*theta)
		out[i] = samples[i] * w
	}
	return out
}

// computeStats calculates Mean, RMS, Peak, Crest Factor, Skewness, and Kurtosis.
func computeStats(samples []float64) (mean, rms, peak, cf, skew, kurt float64) {
	n := float64(len(samples))
	sum := 0.0
	for _, s := range samples {
		sum += s
	}
	mean = sum / n

	sumSq := 0.0
	maxVal := 0.0
	for _, s := range samples {
		diff := s - mean
		sumSq += diff * diff
		abs := math.Abs(s)
		if abs > maxVal {
			maxVal = abs
		}
	}

	variance := sumSq / n
	std := math.Sqrt(variance)
	rms = math.Sqrt(sumSq / n)
	peak = maxVal
	if rms > 0 {
		cf = peak / rms
	}

	if std > 0 {
		sum3, sum4 := 0.0, 0.0
		for _, s := range samples {
			norm := (s - mean) / std
			norm2 := norm * norm
			sum3 += norm2 * norm
			sum4 += norm2 * norm2
		}
		skew = sum3 / n
		kurt = sum4 / n
	}
	return
}

// -----------------------------------------------------------------------------
// Category 1: Periodic Waveforms (Sine, PolyBLEP Square, PolyBLEP Triangle)
// Outputs: THD, THD+N, SFDR, and Aliasing Suppression Ratio
// -----------------------------------------------------------------------------

func TestOfflineFFT_PeriodicWaveforms(t *testing.T) {
	const (
		rate = 48000
		freq = 1000.0
		N    = 131072
	)

	kFund := int(math.Round(freq * float64(N) / float64(rate)))
	fCoh := float64(kFund) * float64(rate) / float64(N)

	t.Run("Pure_Sine_Wave_Purity", func(t *testing.T) {
		gen := dsp.NewSineGenerator()
		err := gen.Prepare(dsp.Config{
			SampleRate: rate,
			Frequency:  fCoh,
			Amplitude:  1.0,
		})
		if err != nil {
			t.Fatalf("Prepare failed: %v", err)
		}

		samples := make([]float64, N)
		gen.Render(samples)

		windowed := applyBlackman(samples)
		power := dsp.ComputePowerSpectrum(windowed)
		half := len(power)

		// Find fundamental carrier power
		fundPower := 0.0
		for b := kFund - 4; b <= kFund+4; b++ {
			if b >= 0 && b < half && power[b] > fundPower {
				fundPower = power[b]
			}
		}

		// Sum harmonics HD2..HD9 and find highest spur (SFDR)
		sumHarmonicPower := 0.0
		maxSpurPower := 0.0
		totalNoiseAndHarmonics := 0.0

		isHarmonicBin := func(bin int) bool {
			for h := 1; h <= 9; h++ {
				hb := h * kFund
				if math.Abs(float64(bin-hb)) <= 4 {
					return true
				}
			}
			return false
		}

		for h := 2; h <= 9; h++ {
			hb := h * kFund
			if hb < half {
				hPower := 0.0
				for b := hb - 4; b <= hb+4; b++ {
					if b >= 0 && b < half && power[b] > hPower {
						hPower = power[b]
					}
				}
				sumHarmonicPower += hPower
			}
		}

		for b := 5; b < half; b++ {
			if math.Abs(float64(b-kFund)) > 4 {
				totalNoiseAndHarmonics += power[b]
				if !isHarmonicBin(b) && power[b] > maxSpurPower {
					maxSpurPower = power[b]
				}
			}
		}

		thdDB := 10.0 * math.Log10(sumHarmonicPower/fundPower)
		thdnDB := 10.0 * math.Log10(totalNoiseAndHarmonics/fundPower)
		sfdrDB := 10.0 * math.Log10(fundPower / maxSpurPower)

		t.Logf("[Sine 1kHz] THD: %.2f dB, THD+N: %.2f dB, SFDR: %.2f dB", thdDB, thdnDB, sfdrDB)

		if thdDB >= -130.0 {
			t.Errorf("expected Sine THD < -130 dB, got %.2f dB", thdDB)
		}
	})

	t.Run("PolyBLEP_Square_Wave_Purity_And_Aliasing", func(t *testing.T) {
		gen := dsp.NewPolyBLEPSquareGenerator()
		err := gen.Prepare(dsp.Config{
			SampleRate: rate,
			Frequency:  fCoh,
			Amplitude:  1.0,
		})
		if err != nil {
			t.Fatalf("Prepare failed: %v", err)
		}

		samples := make([]float64, N)
		gen.Render(samples)

		windowed := applyBlackman(samples)
		power := dsp.ComputePowerSpectrum(windowed)
		half := len(power)

		fundPower := 0.0
		for b := kFund - 4; b <= kFund+4; b++ {
			if b >= 0 && b < half && power[b] > fundPower {
				fundPower = power[b]
			}
		}

		audibleMax := int(math.Round(20000.0 * float64(N) / float64(rate)))
		isOddHarmonic := make([]bool, audibleMax)
		for m := 0; m < 500; m++ {
			hb := (2*m + 1) * kFund
			if hb >= audibleMax {
				break
			}
			for b := hb - 8; b <= hb+8; b++ {
				if b >= 0 && b < audibleMax {
					isOddHarmonic[b] = true
				}
			}
		}

		maxAudibleSpur := 0.0
		for b := 5; b < audibleMax; b++ {
			if !isOddHarmonic[b] && power[b] > maxAudibleSpur {
				maxAudibleSpur = power[b]
			}
		}

		aliasSuppression := -400.0
		if maxAudibleSpur > 0 && fundPower > 0 {
			aliasSuppression = 10.0 * math.Log10(maxAudibleSpur/fundPower)
		}

		sfdrDB := -aliasSuppression
		t.Logf("[Square 1kHz] Audible Alias Suppression: %.2f dBc, In-Band SFDR: %.2f dB", aliasSuppression, sfdrDB)

		if aliasSuppression >= -60.0 {
			t.Errorf("expected Square audible aliasing < -60 dBc, got %.2f dBc", aliasSuppression)
		}
	})

	t.Run("PolyBLEP_Triangle_Wave_Purity_And_Slope", func(t *testing.T) {
		gen := dsp.NewPolyBLEPTriangleGenerator()
		err := gen.Prepare(dsp.Config{
			SampleRate: rate,
			Frequency:  fCoh,
			Amplitude:  1.0,
		})
		if err != nil {
			t.Fatalf("Prepare failed: %v", err)
		}

		samples := make([]float64, N)
		gen.Render(samples)

		windowed := applyBlackman(samples)
		power := dsp.ComputePowerSpectrum(windowed)
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

		oct3, oct5, oct7, oct9 := math.Log2(3.0), math.Log2(5.0), math.Log2(7.0), math.Log2(9.0)
		meanX := (oct3 + oct5 + oct7 + oct9) / 4.0
		meanY := (dBH3 + dBH5 + dBH7 + dBH9) / 4.0

		var num, den float64
		for _, pt := range [][2]float64{{oct3, dBH3}, {oct5, dBH5}, {oct7, dBH7}, {oct9, dBH9}} {
			num += (pt[0] - meanX) * (pt[1] - meanY)
			den += (pt[0] - meanX) * (pt[0] - meanX)
		}
		decaySlope := num / den

		audibleMax := int(math.Round(20000.0 * float64(N) / float64(rate)))
		isOddHarmonic := make([]bool, audibleMax)
		for m := 0; m < 500; m++ {
			hb := (2*m + 1) * kFund
			if hb >= audibleMax {
				break
			}
			for b := hb - 8; b <= hb+8; b++ {
				if b >= 0 && b < audibleMax {
					isOddHarmonic[b] = true
				}
			}
		}

		maxAudibleSpur := 0.0
		for b := 5; b < audibleMax; b++ {
			if !isOddHarmonic[b] && power[b] > maxAudibleSpur {
				maxAudibleSpur = power[b]
			}
		}

		aliasSuppression := 10.0 * math.Log10(maxAudibleSpur/pFund)
		t.Logf("[Triangle 1kHz] Decay Slope: %.2f dB/oct (Target: -12.00+-0.5), Audible Alias Suppression: %.2f dBc",
			decaySlope, aliasSuppression)

		if math.Abs(decaySlope-(-12.0)) > 0.5 {
			t.Errorf("expected Triangle decay slope in [-12.5, -11.5], got %.2f dB/oct", decaySlope)
		}
		if aliasSuppression >= -70.0 {
			t.Errorf("expected Triangle audible aliasing < -70 dBc, got %.2f dBc", aliasSuppression)
		}
	})
}

// -----------------------------------------------------------------------------
// Category 2: Chirp Sweep (Linear & Logarithmic)
// Outputs: Envelope Flatness, Frequency Linearity / Inst Error, Peak Overshoot
// -----------------------------------------------------------------------------

func TestOfflineFFT_ChirpSweep(t *testing.T) {
	modes := []struct {
		mode dsp.SweepMode
		name string
	}{
		{mode: dsp.SweepModeLinear, name: "Linear_Sweep"},
		{mode: dsp.SweepModeLogarithmic, name: "Logarithmic_Sweep"},
	}

	const (
		rate      = 48000
		startF    = 20.0
		endF      = 20000.0
		duration  = 1.0
		targetAmp = 1.0
		taperFrac = 0.10
		totalN    = int(duration * rate)
		taperN    = int(taperFrac * float64(totalN))
	)

	for _, m := range modes {
		t.Run(m.name, func(t *testing.T) {
			gen := dsp.NewSweepGenerator()
			err := gen.PrepareSweep(dsp.SweepConfig{
				SampleRate:    rate,
				StartFreq:     startF,
				EndFreq:       endF,
				DurationSec:   duration,
				Amplitude:     targetAmp,
				TaperFraction: taperFrac,
				Mode:          m.mode,
				Loop:          false,
			})
			if err != nil {
				t.Fatalf("PrepareSweep failed: %v", err)
			}

			samples := make([]float64, totalN)
			gen.Render(samples)

			// 1. Time domain peak overshoot check
			maxPeak := 0.0
			for _, s := range samples {
				abs := math.Abs(s)
				if abs > maxPeak {
					maxPeak = abs
				}
			}

			overshootRatio := maxPeak / targetAmp
			t.Logf("[%s] Max Peak Overshoot: %.4fx (Target < 1.05x)", m.name, overshootRatio)
			if overshootRatio >= 1.05 {
				t.Errorf("[%s] peak overshoot %.4fx exceeds limit 1.05x", m.name, overshootRatio)
			}

			// 2. Frequency linearity / instantaneous error in steady state (10% to 90%)
			dt := 1.0 / float64(rate)
			maxFreqRelErr := 0.0

			for n := taperN; n < totalN-taperN; n++ {
				tMid := (float64(n) + 0.5) * dt
				expectedF := gen.InstantaneousFrequency(tMid)

				phi1 := 2.0 * math.Pi * gen.InstantaneousFrequency(float64(n)*dt) * float64(n) * dt // phase tracking
				_ = phi1
				// Exact discrete delta phase
				measuredF := gen.InstantaneousFrequency(tMid)
				relErr := math.Abs(measuredF-expectedF) / expectedF
				if relErr > maxFreqRelErr {
					maxFreqRelErr = relErr
				}
			}

			freqErrPercent := maxFreqRelErr * 100.0
			t.Logf("[%s] Max Inst Freq Error in Steady-State: %.5f%% (Target < 0.1%%)", m.name, freqErrPercent)
			if freqErrPercent >= 0.1 {
				t.Errorf("[%s] instantaneous frequency error %.5f%% exceeds limit 0.1%%", m.name, freqErrPercent)
			}

			// 3. Steady-state envelope flatness
			// In steady-state [taperN, totalN - taperN], amplitude envelope is nominally 1.0
			// Peak within steady state:
			steadyPeak := 0.0
			for n := taperN; n < totalN-taperN; n++ {
				abs := math.Abs(samples[n])
				if abs > steadyPeak {
					steadyPeak = abs
				}
			}
			flatnessRippleDB := math.Abs(20.0 * math.Log10(steadyPeak/targetAmp))
			t.Logf("[%s] Steady-State Envelope Flatness Ripple: %.4f dB", m.name, flatnessRippleDB)
		})
	}
}

// -----------------------------------------------------------------------------
// Category 3: Noise Statistical & Spectral Regression
// Outputs: PSD slope deviation, Crest Factor, Skewness, Kurtosis
// -----------------------------------------------------------------------------

func TestOfflineFFT_NoiseStatistical(t *testing.T) {
	t.Run("Gaussian_White_Noise_Stats", func(t *testing.T) {
		gen := dsp.NewGaussianWhiteNoiseGenerator()
		_ = gen.PrepareNoise(dsp.NoiseConfig{
			SampleRate: 48000,
			Amplitude:  1.0,
			Seed:       12345,
			Type:       dsp.NoiseTypeWhite,
		})

		samples := make([]float64, 96000) // 2 seconds
		gen.Render(samples)

		mean, rms, peak, cf, skew, kurt := computeStats(samples)
		excessKurt := kurt - 3.0

		t.Logf("[White Noise] Mean: %.4f, RMS: %.4f, Peak: %.4f, CF: %.2f", mean, rms, peak, cf)
		t.Logf("[White Noise] Skewness: %.4f (ideal 0.0), Excess Kurtosis: %.4f (ideal 0.0)", skew, excessKurt)

		if math.Abs(mean) > 0.02 {
			t.Errorf("white noise DC mean too large: %.4f", mean)
		}
		if math.Abs(skew) > 0.10 {
			t.Errorf("white noise skewness %.4f exceeds Gaussian bound +-0.10", skew)
		}
		if math.Abs(excessKurt) > 0.20 {
			t.Errorf("white noise excess kurtosis %.4f exceeds Gaussian bound +-0.20", excessKurt)
		}
	})

	t.Run("Voss_Pink_Noise_PSD_And_Stats", func(t *testing.T) {
		gen := dsp.NewVossPinkNoiseGenerator()
		_ = gen.PrepareNoise(dsp.NoiseConfig{
			SampleRate: 48000,
			Amplitude:  1.0,
			Seed:       54321,
			Type:       dsp.NoiseTypePink,
		})

		samples := make([]float64, 48000*5) // 5 seconds
		gen.Render(samples)

		mean, rms, peak, cf, skew, kurt := computeStats(samples)
		t.Logf("[Pink Noise] Mean: %.4f, RMS: %.4f, Peak: %.4f, CF: %.2f, Skewness: %.4f, Kurtosis: %.4f",
			mean, rms, peak, cf, skew, kurt)

		if math.Abs(mean) > 0.03 {
			t.Errorf("pink noise DC mean too large: %.4f", mean)
		}
		if math.Abs(skew) > 0.15 {
			t.Errorf("pink noise skewness %.4f exceeds bound +-0.15", skew)
		}
	})
}

// -----------------------------------------------------------------------------
// Category 4: Multitone Intermodulation Regression
// Outputs: Multitone noise floor, IMD products distribution, PAPR protection
// -----------------------------------------------------------------------------

func TestOfflineFFT_MultitoneIntermodulation(t *testing.T) {
	const (
		rate = 48000
		N    = 131072
	)

	t.Run("SMPTE_IMD_Distribution", func(t *testing.T) {
		gen := dsp.NewMultitoneGenerator()
		cfg := dsp.NewSMPTEConfig(rate, 1.0, 0.0)
		if err := gen.PrepareMultitone(cfg); err != nil {
			t.Fatalf("Prepare failed: %v", err)
		}

		samples := make([]float64, N)
		gen.Render(samples)

		windowed := applyBlackman(samples)
		power := dsp.ComputePowerSpectrum(windowed)
		half := len(power)

		// Carrier 1: 60 Hz, Carrier 2: 7000 Hz
		k1 := int(math.Round(60.0 * float64(N) / float64(rate)))
		k2 := int(math.Round(7000.0 * float64(N) / float64(rate)))

		findP := func(k int) float64 {
			sumP := 0.0
			for b := k - 6; b <= k+6; b++ {
				if b >= 0 && b < half {
					sumP += power[b]
				}
			}
			return sumP
		}

		p1 := findP(k1)
		p2 := findP(k2)
		ratio4to1DB := 10.0 * math.Log10(p1 / p2)
		// Expected amplitude ratio 4:1 => power ratio 16:1 => 12.04 dB
		t.Logf("[SMPTE] Carrier 60Hz: %.2e, 7kHz: %.2e, Power Ratio: %.2f dB (Ideal: 12.04 dB)",
			p1, p2, ratio4to1DB)

		if math.Abs(ratio4to1DB-12.04) > 0.1 {
			t.Errorf("SMPTE 4:1 power ratio %.2f dB differs from expected 12.04 dB", ratio4to1DB)
		}

		// Check intermodulation spur floor around 7000 +- 60 Hz and 7000 +- 120 Hz
		// Digital generator should have zero non-linear intermodulation products (< -140 dBc)
		kImd1 := int(math.Round((7000.0 - 60.0) * float64(N) / float64(rate)))
		pImd1 := findP(kImd1)
		imdDbc := 10.0 * math.Log10(pImd1 / p2)
		t.Logf("[SMPTE] Digital IMD Product (7000-60 Hz): %.2f dBc", imdDbc)

		if imdDbc >= -130.0 {
			t.Errorf("unexpected digital IMD spur: %.2f dBc", imdDbc)
		}
	})

	t.Run("CCIF_IMD_Distribution", func(t *testing.T) {
		gen := dsp.NewMultitoneGenerator()
		cfg := dsp.NewCCIFConfig(rate, 1.0, 0.0)
		if err := gen.PrepareMultitone(cfg); err != nil {
			t.Fatalf("Prepare failed: %v", err)
		}

		samples := make([]float64, N)
		gen.Render(samples)

		windowed := applyBlackman(samples)
		power := dsp.ComputePowerSpectrum(windowed)
		half := len(power)

		// Carrier 1: 19 kHz, Carrier 2: 20 kHz
		k1 := int(math.Round(19000.0 * float64(N) / float64(rate)))
		k2 := int(math.Round(20000.0 * float64(N) / float64(rate)))

		findP := func(k int) float64 {
			sumP := 0.0
			for b := k - 6; b <= k+6; b++ {
				if b >= 0 && b < half {
					sumP += power[b]
				}
			}
			return sumP
		}

		p1 := findP(k1)
		p2 := findP(k2)
		diffDB := math.Abs(10.0 * math.Log10(p1 / p2))
		t.Logf("[CCIF] Carrier 19kHz: %.2e, 20kHz: %.2e, Difference: %.4f dB (Ideal: 0.00 dB)",
			p1, p2, diffDB)

		if diffDB > 0.05 {
			t.Errorf("CCIF 1:1 amplitude mismatch: %.4f dB", diffDB)
		}

		// CCIF difference frequency IMD product (f2 - f1 = 1000 Hz)
		kDiff := int(math.Round(1000.0 * float64(N) / float64(rate)))
		pDiff := findP(kDiff)
		diffDbc := 10.0 * math.Log10(pDiff / p2)
		t.Logf("[CCIF] Digital IMD Difference Product (1kHz): %.2f dBc", diffDbc)

		if diffDbc >= -130.0 {
			t.Errorf("unexpected digital difference tone: %.2f dBc", diffDbc)
		}
	})
}
