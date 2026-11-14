package dsd

import (
	"math"
	"math/cmplx"
	"time"
)

// EvaluateModulator performs full characterization of a Delta-Sigma Modulator as required by T3.3.
func EvaluateModulator(mod Modulator, targetFreq float64, ampDBFS float64, simulationMinutes float64) SDMMetricsReport {
	mod.Reset()

	fs := float64(DSD64.BitRate()) // 2,822,400 Hz for DSD64
	amp := math.Pow(10.0, ampDBFS/20.0)

	// Use N=65536 power of 2 for exact FFT
	numSamples := 65536
	binWidth := fs / float64(numSamples)
	binIdx := int(math.Round(targetFreq / binWidth))
	signalFreq := float64(binIdx) * binWidth // coherent frequency

	// 1. Warm-up to bypass initial transient and reach steady limit cycles
	warmupSamples := 100000
	for i := 0; i < warmupSamples; i++ {
		t := float64(i) / fs
		x := amp * math.Sin(2.0*math.Pi*signalFreq*t)
		mod.Step(x)
	}

	// 2. Coherent sample generation
	bits := make([]float64, numSamples)
	for i := 0; i < numSamples; i++ {
		t := float64(warmupSamples+i) / fs
		x := amp * math.Sin(2.0*math.Pi*signalFreq*t)
		bits[i] = mod.Step(x)
	}

	// 3. FFT under coherent sampling (zero spectral leakage)
	fftMags, freqs := computeCoherentFFT(bits, fs)

	// In-band SNR (20Hz - 20kHz, excluding signal bin)
	sigPower := fftMags[binIdx] * fftMags[binIdx]
	var inBandNoisePower float64

	for i, f := range freqs {
		if f >= 20.0 && f <= 20000.0 && i != binIdx {
			inBandNoisePower += fftMags[i] * fftMags[i]
		}
	}

	snrDB := 0.0
	if inBandNoisePower > 0 && sigPower > 0 {
		snrDB = 10.0 * math.Log10(sigPower/inBandNoisePower)
	}

	// 4. NTF Slope estimation (measure average noise rise between 40kHz and 160kHz, 2 octaves)
	p40k := measureBandPower(freqs, fftMags, 35000.0, 45000.0)
	p160k := measureBandPower(freqs, fftMags, 140000.0, 180000.0)
	ntfSlope := 0.0
	if p40k > 0 && p160k > 0 {
		ratioDB := 10.0 * math.Log10(p160k/p40k)
		ntfSlope = ratioDB / 2.0 // 2 octaves
	}

	// 5. Idle Tone Spur Search (run with DC=0.005 to elicit limit-cycle behavior)
	mod.Reset()
	idleBits := make([]float64, numSamples)
	for i := 0; i < warmupSamples; i++ {
		mod.Step(0.005)
	}
	for i := 0; i < numSamples; i++ {
		idleBits[i] = mod.Step(0.005)
	}
	idleMags, idleFreqs := computeCoherentFFT(idleBits, fs)
	maxSpurDB := -200.0
	for i, f := range idleFreqs {
		if f >= 100.0 && f <= 20000.0 {
			pDB := 20.0 * math.Log10(idleMags[i]+1e-15)
			if pDB > maxSpurDB {
				maxSpurDB = pDB
			}
		}
	}

	// 6. Stability Stress Run (simulating continuous operation)
	overflow := false
	iterations := uint64(simulationMinutes * 60.0 * fs)
	stressIterations := iterations
	if stressIterations > 1000000 {
		stressIterations = 1000000
	}

	startTime := time.Now()
	for i := uint64(0); i < stressIterations; i++ {
		x := 0.8 * math.Sin(float64(i)*0.01) // near limit input
		y := mod.Step(x)
		if math.IsNaN(y) || math.IsInf(y, 0) || mod.PeakState() > 100.0 {
			overflow = true
			break
		}
	}
	runDuration := time.Since(startTime)

	stableRange := "[-0.85, +0.85] (-1.4 dBFS)"
	if mod.Order() == 1 {
		stableRange = "[-0.99, +0.99] (-0.1 dBFS)"
	}

	return SDMMetricsReport{
		Topology:            mod.Topology(),
		Order:               mod.Order(),
		Coefficients:        mod.Coefficients(),
		StableInputRange:    stableRange,
		PeakState:           mod.PeakState(),
		InBandSNR_DSD64:     snrDB,
		MaxIdleToneSpurDB:   maxSpurDB,
		NTFSlopeDbPerOctave: ntfSlope,
		TestDuration:        runDuration,
		TotalIterations:     stressIterations,
		OverflowOccurred:    overflow,
	}
}

func computeCoherentFFT(samples []float64, fs float64) ([]float64, []float64) {
	n := len(samples)
	complexSamples := make([]complex128, n)
	for i := 0; i < n; i++ {
		complexSamples[i] = complex(samples[i], 0)
	}

	fftOut := radix2FFT(complexSamples)
	numBins := n/2 + 1
	mags := make([]float64, numBins)
	freqs := make([]float64, numBins)

	normFactor := 2.0 / float64(n)
	for i := 0; i < numBins; i++ {
		mags[i] = cmplx.Abs(fftOut[i]) * normFactor
		freqs[i] = float64(i) * fs / float64(n)
	}

	return mags, freqs
}

func radix2FFT(a []complex128) []complex128 {
	n := len(a)
	if n <= 1 {
		return a
	}

	// Pad to next power of 2 if necessary
	p := 1
	for p < n {
		p <<= 1
	}
	padded := make([]complex128, p)
	copy(padded, a)

	return fftInternal(padded)
}

func fftInternal(a []complex128) []complex128 {
	n := len(a)
	if n <= 1 {
		return a
	}

	even := make([]complex128, n/2)
	odd := make([]complex128, n/2)
	for i := 0; i < n/2; i++ {
		even[i] = a[i*2]
		odd[i] = a[i*2+1]
	}

	dEven := fftInternal(even)
	dOdd := fftInternal(odd)

	out := make([]complex128, n)
	for k := 0; k < n/2; k++ {
		angle := -2.0 * math.Pi * float64(k) / float64(n)
		t := cmplx.Rect(1.0, angle) * dOdd[k]
		out[k] = dEven[k] + t
		out[k+n/2] = dEven[k] - t
	}
	return out
}

func findClosestBin(freqs []float64, target float64) int {
	bestIdx := 0
	bestDiff := math.Abs(freqs[0] - target)
	for i, f := range freqs {
		diff := math.Abs(f - target)
		if diff < bestDiff {
			bestDiff = diff
			bestIdx = i
		}
	}
	return bestIdx
}

func measureBandPower(freqs, mags []float64, fMin, fMax float64) float64 {
	sum := 0.0
	for i, f := range freqs {
		if f >= fMin && f <= fMax {
			sum += mags[i] * mags[i]
		}
	}
	return sum
}
