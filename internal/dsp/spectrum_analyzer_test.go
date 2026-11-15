package dsp

import (
	"math"
	"testing"
	"time"
)

func TestSpectrumAnalyzer_Sine1kHz(t *testing.T) {
	fs := 48000
	cfg := DefaultSpectrumConfig(fs)
	cfg.FFTSize = 1024
	analyzer := NewSpectrumAnalyzer(cfg)

	numSamples := cfg.FFTSize
	left := make([]float64, numSamples)
	right := make([]float64, numSamples)

	// 1 kHz pure sine wave, 0 dBFS amplitude (1.0)
	freq := 1000.0
	twoPi := 2.0 * math.Pi
	for i := 0; i < numSamples; i++ {
		tSec := float64(i) / float64(fs)
		left[i] = math.Sin(twoPi * freq * tSec)
		right[i] = 0.5 * math.Sin(twoPi * freq * tSec) // -6 dBFS
	}

	now := time.Now()
	barsL, barsR, peakL, peakR := analyzer.Process(left, right, now)

	if len(barsL) != len(StandardISO30CenterFreqs) {
		t.Fatalf("expected %d bands, got %d", len(StandardISO30CenterFreqs), len(barsL))
	}

	// Find the 1 kHz band index (center freq = 1000)
	band1kIdx := -1
	for i, b := range analyzer.Bands() {
		if math.Abs(b.CenterFreq-1000.0) < 1.0 {
			band1kIdx = i
			break
		}
	}
	if band1kIdx == -1 {
		t.Fatalf("could not find 1 kHz band")
	}

	// 1.0 amplitude is 0 dBFS, which corresponds to height ~ 90.0 (allow ±1.5 dB due to window mainlobe)
	if barsL[band1kIdx] < 88.0 || barsL[band1kIdx] > 91.0 {
		t.Errorf("expected Left 1kHz band to be ~90 dB, got %.2f", barsL[band1kIdx])
	}

	// 0.5 amplitude is -6 dBFS, corresponding to height ~ 84.0
	if barsR[band1kIdx] < 82.0 || barsR[band1kIdx] > 85.5 {
		t.Errorf("expected Right 1kHz band to be ~84 dB, got %.2f", barsR[band1kIdx])
	}

	// Peak should match current or exceed it
	if peakL[band1kIdx] < barsL[band1kIdx] {
		t.Errorf("peak Left should be >= bar level, got peak=%.2f, bar=%.2f", peakL[band1kIdx], barsL[band1kIdx])
	}
	if peakR[band1kIdx] < barsR[band1kIdx] {
		t.Errorf("peak Right should be >= bar level, got peak=%.2f, bar=%.2f", peakR[band1kIdx], barsR[band1kIdx])
	}

	// Far away bands (e.g. 50 Hz, 10 kHz) should be very low (< 30 dB)
	if barsL[3] > 30.0 { // 50 Hz
		t.Errorf("expected 50 Hz band to be quiet (< 30 dB), got %.2f", barsL[3])
	}
	if barsL[26] > 30.0 { // 10 kHz
		t.Errorf("expected 10 kHz band to be quiet (< 30 dB), got %.2f", barsL[26])
	}
}

func TestSpectrumAnalyzer_BallisticsDecay(t *testing.T) {
	fs := 48000
	cfg := DefaultSpectrumConfig(fs)
	cfg.DecayRateDBPerSec = 50.0 // 50 dB/s
	analyzer := NewSpectrumAnalyzer(cfg)

	// Step 1: Feed 1 kHz tone
	left := make([]float64, cfg.FFTSize)
	for i := range left {
		left[i] = math.Sin(2.0 * math.Pi * 1000.0 * float64(i) / float64(fs))
	}
	t0 := time.Now()
	barsL, _, _, _ := analyzer.Process(left, nil, t0)
	initialVal := barsL[16] // 1 kHz band
	if initialVal < 80.0 {
		t.Fatalf("expected high initial value, got %.2f", initialVal)
	}

	// Step 2: Feed silence after 0.2 seconds
	t1 := t0.Add(200 * time.Millisecond)
	silence := make([]float64, cfg.FFTSize)
	barsL1, _, peakL1, _ := analyzer.Process(silence, nil, t1)

	decayedVal := barsL1[16]
	expectedDecay := 50.0 * 0.2 // ~10 dB
	expectedVal := initialVal - expectedDecay
	if math.Abs(decayedVal-expectedVal) > 3.0 {
		t.Errorf("expected decayed val ~%.2f, got %.2f", expectedVal, decayedVal)
	}

	// Peak should be held (PeakHoldSec is 0.4s > 0.2s)
	if math.Abs(peakL1[16]-initialVal) > 1.0 {
		t.Errorf("expected peak to hold near %.2f, got %.2f", initialVal, peakL1[16])
	}
}

func TestSpectrumAnalyzer_StereoIsolation(t *testing.T) {
	fs := 48000
	analyzer := NewSpectrumAnalyzer(DefaultSpectrumConfig(fs))

	fftSize := analyzer.FFTSize()
	left := make([]float64, fftSize)
	right := make([]float64, fftSize)
	for i := range left {
		left[i] = math.Sin(2.0 * math.Pi * 1000.0 * float64(i) / float64(fs))
		right[i] = 0.0 // silent
	}

	barsL, barsR, _, _ := analyzer.Process(left, right, time.Now())
	if barsL[16] < 80.0 {
		t.Errorf("expected Left to be high, got %.2f", barsL[16])
	}
	if barsR[16] > 10.0 {
		t.Errorf("expected Right to be silent, got %.2f", barsR[16])
	}
}

func TestSpectrumAnalyzer_Reset(t *testing.T) {
	analyzer := NewSpectrumAnalyzer(DefaultSpectrumConfig(48000))
	fftSize := analyzer.FFTSize()
	left := make([]float64, fftSize)
	for i := range left {
		left[i] = 1.0
	}
	analyzer.Process(left, left, time.Now())
	analyzer.Reset()

	for i, b := range analyzer.curL {
		if b != 0 || analyzer.curR[i] != 0 || analyzer.peakL[i] != 0 || analyzer.peakR[i] != 0 {
			t.Errorf("band %d not reset", i)
		}
	}
}

func TestSpectrumAnalyzer_Ultrasound30kHz(t *testing.T) {
	fs := 96000
	cfg := DefaultSpectrumConfig(fs)
	analyzer := NewSpectrumAnalyzer(cfg)

	// Target 30 kHz ultrasound signal
	analyzer.Configure(fs, 30000.0)

	bands := analyzer.Bands()
	lastBand := bands[len(bands)-1]
	if lastBand.CenterFreq < 30000.0 {
		t.Fatalf("expected last band to cover >= 30 kHz, got %.0f Hz", lastBand.CenterFreq)
	}

	fftSize := analyzer.FFTSize()
	left := make([]float64, fftSize)
	for i := range left {
		left[i] = math.Sin(2.0 * math.Pi * 30000.0 * float64(i) / float64(fs))
	}

	barsL, _, _, _ := analyzer.Process(left, nil, time.Now())

	// Find the 31.5 kHz band
	band31kIdx := -1
	for i, b := range bands {
		if math.Abs(b.CenterFreq-31500.0) < 1.0 {
			band31kIdx = i
			break
		}
	}
	if band31kIdx == -1 {
		t.Fatalf("31.5 kHz band not found")
	}

	if barsL[band31kIdx] < 85.0 {
		t.Errorf("expected 31.5 kHz band to detect 30 kHz tone (> 85 dB), got %.2f", barsL[band31kIdx])
	}
}

func TestSpectrumAnalyzer_DynamicConfigureCeiling(t *testing.T) {
	fs := 48000
	analyzer := NewSpectrumAnalyzer(DefaultSpectrumConfig(fs))

	// Request frequency far above Nyquist
	analyzer.Configure(fs, 100000.0)

	bands := analyzer.Bands()
	lastBand := bands[len(bands)-1]
	ceiling := float64(fs) * 0.48
	if lastBand.MaxFreq > ceiling+100.0 {
		t.Errorf("expected last band to stay within Nyquist ceiling %.0f Hz, got %.0f Hz", ceiling, lastBand.MaxFreq)
	}
}

func TestSpectrumAnalyzer_Sine400HzCleanLowFrequency(t *testing.T) {
	fs := 48000
	analyzer := NewSpectrumAnalyzer(DefaultSpectrumConfig(fs))

	fftSize := analyzer.FFTSize()
	samples := make([]float64, fftSize)
	for i := range samples {
		samples[i] = math.Sin(2.0 * math.Pi * 400.0 * float64(i) / float64(fs))
	}

	barsL, _, _, _ := analyzer.Process(samples, nil, time.Now())
	bands := analyzer.Bands()

	// 400 Hz band (around index 12) should be near full height (> 85 dB)
	band400Idx := -1
	for i, b := range bands {
		if math.Abs(b.CenterFreq-400.0) < 1.0 {
			band400Idx = i
			break
		}
	}
	if band400Idx == -1 {
		t.Fatalf("400 Hz band not found")
	}
	if barsL[band400Idx] < 85.0 {
		t.Errorf("expected 400 Hz band to be strong (> 85 dB), got %.2f", barsL[band400Idx])
	}

	// Verify that low frequencies (25 Hz to 100 Hz, bands 0..6) have zero or negligible leakage (< 3.0 dB)
	for i := 0; i <= 6; i++ {
		if barsL[i] > 3.0 {
			t.Errorf("band %d (%s) has unexpected low-frequency leakage: %.2f dB", i, bands[i].Label, barsL[i])
		}
	}
}

func TestSpectrumAnalyzer_Sine100HzCleanLowFrequency(t *testing.T) {
	fs := 48000
	analyzer := NewSpectrumAnalyzer(DefaultSpectrumConfig(fs))

	fftSize := analyzer.FFTSize()
	samples := make([]float64, fftSize)
	for i := range samples {
		samples[i] = math.Sin(2.0 * math.Pi * 100.0 * float64(i) / float64(fs))
	}

	barsL, _, _, _ := analyzer.Process(samples, nil, time.Now())
	bands := analyzer.Bands()

	// 100 Hz band (index 6) should be strong (> 85 dB)
	if barsL[6] < 85.0 {
		t.Errorf("expected 100 Hz band to be strong (> 85 dB), got %.2f", barsL[6])
	}

	// Verify that all bands to the left of 100 Hz (25, 31.5, 40, 50, 63, 80 Hz, bands 0..5) are clean (< 3.0 dB)
	for i := 0; i <= 5; i++ {
		if barsL[i] > 3.0 {
			t.Errorf("band %d (%s) to the left of 100 Hz has leakage: %.2f dB", i, bands[i].Label, barsL[i])
		}
	}
}



