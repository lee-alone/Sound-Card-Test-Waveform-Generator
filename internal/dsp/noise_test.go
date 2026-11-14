package dsp

import (
	"math"
	"testing"
)

func TestNoiseZeroAllocations(t *testing.T) {
	// White noise zero alloc
	white := NewGaussianWhiteNoiseGenerator()
	_ = white.PrepareNoise(NoiseConfig{
		SampleRate: 48000,
		Amplitude:  1.0,
		Seed:       42,
		Type:       NoiseTypeWhite,
	})
	dst := make([]float64, 512)
	allocsWhite := testing.AllocsPerRun(1000, func() {
		white.Render(dst)
	})
	if allocsWhite != 0 {
		t.Errorf("expected 0 allocs for white noise, got %v", allocsWhite)
	}

	// Pink noise zero alloc
	pink := NewVossPinkNoiseGenerator()
	_ = pink.PrepareNoise(NoiseConfig{
		SampleRate: 48000,
		Amplitude:  1.0,
		Seed:       42,
		Type:       NoiseTypePink,
	})
	allocsPink := testing.AllocsPerRun(1000, func() {
		pink.Render(dst)
	})
	if allocsPink != 0 {
		t.Errorf("expected 0 allocs for pink noise, got %v", allocsPink)
	}
}

func TestNoiseSeedReproducibility(t *testing.T) {
	gen1 := NewVossPinkNoiseGenerator()
	_ = gen1.PrepareNoise(NoiseConfig{
		SampleRate: 48000,
		Amplitude:  0.8,
		Seed:       987654321,
		Type:       NoiseTypePink,
	})

	gen2 := NewVossPinkNoiseGenerator()
	_ = gen2.PrepareNoise(NoiseConfig{
		SampleRate: 48000,
		Amplitude:  0.8,
		Seed:       987654321,
		Type:       NoiseTypePink,
	})

	buf1 := make([]float64, 1024)
	buf2 := make([]float64, 1024)
	gen1.Render(buf1)
	gen2.Render(buf2)

	for i := range buf1 {
		if buf1[i] != buf2[i] {
			t.Fatalf("seed reproducibility failed at sample %d: %v != %v", i, buf1[i], buf2[i])
		}
	}

	// Verify ResetToZero also restores exact same sequence
	gen1.Reset(ResetToZero)
	bufReset := make([]float64, 1024)
	gen1.Render(bufReset)
	for i := range buf1 {
		if buf1[i] != bufReset[i] {
			t.Fatalf("ResetToZero failed to reproduce sequence at sample %d: %v != %v", i, buf1[i], bufReset[i])
		}
	}
}

func TestVossPinkNoiseWelchPSDSlopeAndResidual(t *testing.T) {
	const (
		rate  = 48000
		nfft  = 8192
		step  = nfft / 2 // 50% overlap
		nSegs = 200      // >= 100 averages
	)

	totalSamples := (nSegs-1)*step + nfft
	gen := NewVossPinkNoiseGenerator()
	err := gen.PrepareNoise(NoiseConfig{
		SampleRate: rate,
		Amplitude:  1.0,
		Seed:       12345,
		Type:       NoiseTypePink,
	})
	if err != nil {
		t.Fatalf("PrepareNoise failed: %v", err)
	}

	samples := make([]float64, totalSamples)
	gen.Render(samples)

	// Hann window
	win := make([]float64, nfft)
	winPower := 0.0
	for i := 0; i < nfft; i++ {
		w := 0.5 * (1.0 - math.Cos(2.0*math.Pi*float64(i)/float64(nfft-1)))
		win[i] = w
		winPower += w * w
	}

	half := nfft / 2
	psdAvg := make([]float64, half)
	segBuf := make([]float64, nfft)

	for s := 0; s < nSegs; s++ {
		start := s * step
		for i := 0; i < nfft; i++ {
			segBuf[i] = samples[start+i] * win[i]
		}
		pSpec := ComputePowerSpectrum(segBuf)
		for b := 0; b < half; b++ {
			psdAvg[b] += pSpec[b] / (float64(rate) * winPower)
		}
	}

	for b := 0; b < half; b++ {
		psdAvg[b] /= float64(nSegs)
	}

	// Standard 1/3-octave band center frequencies from 20Hz to 20kHz
	bandCenters := []float64{
		25, 31.5, 40, 50, 63, 80, 100, 125, 160, 200, 250, 315, 400, 500, 630, 800, 1000,
		1250, 1600, 2000, 2500, 3150, 4000, 5000, 6300, 8000, 10000, 12500, 16000, 20000,
	}

	bandPowers := make([]float64, len(bandCenters))
	deltaF := float64(rate) / float64(nfft)
	sixthFactor := math.Pow(2.0, 1.0/6.0)

	for i, fc := range bandCenters {
		fLow := fc / sixthFactor
		fHigh := fc * sixthFactor

		bLow := int(math.Round(fLow / deltaF))
		bHigh := int(math.Round(fHigh / deltaF))
		if bLow < 1 {
			bLow = 1
		}
		if bHigh >= half {
			bHigh = half - 1
		}
		if bLow > bHigh {
			bHigh = bLow
		}

		sumP := 0.0
		count := 0
		for b := bLow; b <= bHigh; b++ {
			sumP += psdAvg[b]
			count++
		}
		if count > 0 {
			bandPowers[i] = sumP / float64(count)
		}
	}

	// Find index of 1000 Hz
	idx1k := 16 // 1000 Hz is at index 16
	p1k := bandPowers[idx1k]
	if p1k <= 0 {
		t.Fatalf("invalid 1kHz band power: %v", p1k)
	}

	psdDBNorm := make([]float64, len(bandCenters))
	octaves := make([]float64, len(bandCenters))
	for i, fc := range bandCenters {
		psdDBNorm[i] = 10.0 * math.Log10(bandPowers[i]/p1k)
		octaves[i] = math.Log2(fc / 1000.0)
	}

	// Mid-band definition: 50 Hz to 10 kHz
	var midX, midY []float64
	for i, fc := range bandCenters {
		if fc >= 50.0 && fc <= 10000.0 {
			midX = append(midX, octaves[i])
			midY = append(midY, psdDBNorm[i])
		}
	}

	// Least-squares fit on mid-band: y = slope * x + intercept
	meanX, meanY := 0.0, 0.0
	for i := range midX {
		meanX += midX[i]
		meanY += midY[i]
	}
	meanX /= float64(len(midX))
	meanY /= float64(len(midY))

	var num, den float64
	for i := range midX {
		dx := midX[i] - meanX
		dy := midY[i] - meanY
		num += dx * dy
		den += dx * dx
	}
	slope := num / den
	intercept := meanY - slope*meanX

	maxMidResidual := 0.0
	for i := range midX {
		predicted := slope*midX[i] + intercept
		res := math.Abs(midY[i] - predicted)
		if res > maxMidResidual {
			maxMidResidual = res
		}
	}

	t.Logf("Welch PSD (200 averages): Fitted Slope = %.3f dB/oct (Target: -3.0 dB/oct)", slope)
	t.Logf("Max Residual in Mid-Band (50Hz - 10kHz): %.3f dB (Tolerance: <= 0.5 dB)", maxMidResidual)

	// Record edge band residuals (<50Hz and >10kHz)
	for i, fc := range bandCenters {
		if fc < 50.0 || fc > 10000.0 {
			predicted := slope*octaves[i] + intercept
			edgeRes := psdDBNorm[i] - predicted
			t.Logf("Edge Band %.1f Hz: Normalized PSD = %.2f dB, Predicted = %.2f dB, Residual = %+.2f dB",
				fc, psdDBNorm[i], predicted, edgeRes)
		}
	}

	// Acceptance criteria:
	// Target slope is -3 dB/oct, tolerance within 0.3 dB/oct
	if math.Abs(slope-(-3.0)) > 0.3 {
		t.Errorf("expected PSD slope near -3.0 dB/oct, got %.3f dB/oct", slope)
	}

	// Mid-band residual <= 0.5 dB
	if maxMidResidual > 0.5 {
		t.Errorf("expected mid-band residual <= 0.5 dB, got %.3f dB", maxMidResidual)
	}
}
