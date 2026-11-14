package test

import (
	"errors"
	"math"
	"math/cmplx"
	"os"
	"testing"
	"time"

	"sound-signal-generator/internal/audio"
	"sound-signal-generator/internal/audio/wasapi"
	"sound-signal-generator/internal/controller"
	"sound-signal-generator/internal/dsd"
	"sound-signal-generator/internal/engine"
)

// WidebandCaptureMetrics records measurements from wideband capture / ADC loopback.
type WidebandCaptureMetrics struct {
	FundamentalFreqHz   float64 `json:"fundamental_freq_hz"`
	MeasuredAmplitudeDB float64 `json:"measured_amplitude_db"`
	AmplitudeErrorDB    float64 `json:"amplitude_error_db"`
	THDPercent          float64 `json:"thd_percent"`
	THD_dB              float64 `json:"thd_db"`
	InBandNoiseDB       float64 `json:"in_band_noise_db"`
	UltrasonicNoiseDB   float64 `json:"ultrasonic_noise_db"`
	HasUltrasonicNoise  bool    `json:"has_ultrasonic_noise"`
}

// simulateWidebandADC processes 1-bit DSD stream through a wideband analog front-end model.
func simulateWidebandADC(dopBytes []byte, sampleFormat audio.SampleFormat, sampleRate int) WidebandCaptureMetrics {
	// 1. Extract 1-bit DSD audio samples from DoP container
	// Each PCM frame has 16 bits of DSD per channel.
	bytesPerSample := sampleFormat.BytesPerSample()
	bytesPerFrame := bytesPerSample * 2 // stereo
	numFrames := len(dopBytes) / bytesPerFrame

	// Reconstruct 1-bit samples (+1.0 / -1.0) at DSD rate (64 * 44100 = 2.8224MHz)
	dsdFs := float64(sampleRate * 16)
	totalDSDSamples := numFrames * 16
	dsdAudio := make([]float64, totalDSDSamples)

	sampleIdx := 0
	for f := 0; f < numFrames; f++ {
		offset := f * bytesPerFrame
		var b0, b1 byte
		if sampleFormat == audio.SampleFormat24Packed {
			b0 = dopBytes[offset]
			b1 = dopBytes[offset+1]
		} else {
			b0 = dopBytes[offset+1]
			b1 = dopBytes[offset+2]
		}

		// Channel 0 MSB-first: 16 bits
		word := uint16(b0) | (uint16(b1) << 8)
		for bit := 15; bit >= 0; bit-- {
			if (word & (1 << bit)) != 0 {
				dsdAudio[sampleIdx] = 1.0
			} else {
				dsdAudio[sampleIdx] = -1.0
			}
			sampleIdx++
		}
	}

	// 2. Wideband FFT analysis on DSD bitstream (power of 2 FFT)
	fftLen := 65536
	if len(dsdAudio) < fftLen {
		fftLen = len(dsdAudio)
	}
	complexIn := make([]complex128, fftLen)
	wSum := 0.0
	for i := 0; i < fftLen; i++ {
		// Hanning window
		w := 0.5 * (1.0 - math.Cos(2.0*math.Pi*float64(i)/float64(fftLen-1)))
		wSum += w
		complexIn[i] = complex(dsdAudio[i]*w, 0)
	}

	fftOut := fftSimple(complexIn)
	numBins := fftLen/2 + 1
	mags := make([]float64, numBins)
	freqs := make([]float64, numBins)
	norm := 2.0 / wSum

	for i := 0; i < numBins; i++ {
		mags[i] = cmplx.Abs(fftOut[i]) * norm
		freqs[i] = float64(i) * dsdFs / float64(fftLen)
	}

	// 3. Measure 1kHz fundamental
	fIdx := 0
	minDiff := 1e9
	for i, f := range freqs {
		diff := math.Abs(f - 1000.0)
		if diff < minDiff {
			minDiff = diff
			fIdx = i
		}
	}

	// Measure fundamental with parabolic interpolation to remove scalloping loss
	y0 := 20.0 * math.Log10(mags[fIdx-1] + 1e-12)
	y1 := 20.0 * math.Log10(mags[fIdx] + 1e-12)
	y2 := 20.0 * math.Log10(mags[fIdx+1] + 1e-12)
	delta := 0.5 * (y0 - y2) / (y0 - 2.0*y1 + y2)
	measAmpDB := y1 - 0.25*(y0-y2)*delta
	targetAmpDB := -6.0
	ampErrDB := measAmpDB - targetAmpDB

	fundPower := mags[fIdx] * mags[fIdx]

	// 4. Measure Harmonics (2kHz to 9kHz)
	harmPower := 0.0
	for h := 2; h <= 9; h++ {
		targetH := float64(h) * 1000.0
		hIdx := 0
		hMinDiff := 1e9
		for i, f := range freqs {
			diff := math.Abs(f - targetH)
			if diff < hMinDiff {
				hMinDiff = diff
				hIdx = i
			}
		}
		for i := max(0, hIdx-1); i <= min(numBins-1, hIdx+1); i++ {
			harmPower += mags[i] * mags[i]
		}
	}
	thdRatio := math.Sqrt(harmPower / fundPower)
	thdDB := 20.0 * math.Log10(thdRatio + 1e-12)

	// 5. Measure In-band Noise (20Hz - 20kHz, excluding fundamental and harmonics)
	inBandNoisePower := 0.0
	for i, f := range freqs {
		if f >= 20.0 && f <= 20000.0 {
			// Exclude fundamental and harmonics
			isHarmonic := false
			for h := 1; h <= 10; h++ {
				if math.Abs(f-float64(h)*1000.0) < 150.0 {
					isHarmonic = true
					break
				}
			}
			if !isHarmonic {
				inBandNoisePower += mags[i] * mags[i]
			}
		}
	}
	inBandNoiseDB := 10.0 * math.Log10(inBandNoisePower + 1e-15)

	// 6. Measure Ultrasonic Noise (>20kHz up to 100kHz)
	ultraPower := 0.0
	for i, f := range freqs {
		if f > 20000.0 && f <= 100000.0 {
			ultraPower += mags[i] * mags[i]
		}
	}
	ultraNoiseDB := 10.0 * math.Log10(ultraPower + 1e-15)

	return WidebandCaptureMetrics{
		FundamentalFreqHz:   freqs[fIdx],
		MeasuredAmplitudeDB: measAmpDB,
		AmplitudeErrorDB:    ampErrDB,
		THDPercent:          thdRatio * 100.0,
		THD_dB:              thdDB,
		InBandNoiseDB:       inBandNoiseDB,
		UltrasonicNoiseDB:   ultraNoiseDB,
		HasUltrasonicNoise:  ultraNoiseDB > inBandNoiseDB, // DSD MUST have higher ultrasonic noise than in-band!
	}
}

func fftSimple(a []complex128) []complex128 {
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
	dEven := fftSimple(even)
	dOdd := fftSimple(odd)
	out := make([]complex128, n)
	for k := 0; k < n/2; k++ {
		angle := -2.0 * math.Pi * float64(k) / float64(n)
		t := cmplx.Rect(1.0, angle) * dOdd[k]
		out[k] = dEven[k] + t
		out[k+n/2] = dEven[k] - t
	}
	return out
}

func TestDoPWidebandADCLoopbackVerification(t *testing.T) {
	// Retrieve Golden Vector DSD64
	golden, err := dsd.GetGoldenVector(dsd.DSD64)
	if err != nil {
		t.Fatalf("failed to get DSD64 golden vector: %v", err)
	}

	// Pack 5000 frames (approx 28ms of DoP data at 176.4kHz)
	packer, err := dsd.NewDoPPacker(golden.Data, 2, audio.SampleFormat24In32, dsd.BitOrderMSBFirst)
	if err != nil {
		t.Fatalf("failed to create DoP packer: %v", err)
	}

	numFrames := 4096 // 4096 frames * 16 bits = 65536 DSD samples
	dopBuf := make([]byte, numFrames*2*4)
	_, err = packer.PackFrames(dopBuf, numFrames)
	if err != nil {
		t.Fatalf("PackFrames failed: %v", err)
	}

	// Run wideband ADC simulation
	metrics := simulateWidebandADC(dopBuf, audio.SampleFormat24In32, 176400)

	t.Logf("\n=== DoP DSD64 Wideband ADC Closed-Loop Analysis ===")
	t.Logf("  Fundamental Freq:       %.2f Hz", metrics.FundamentalFreqHz)
	t.Logf("  Fundamental Amplitude:  %.2f dBFS (Error: %.2f dB)", metrics.MeasuredAmplitudeDB, metrics.AmplitudeErrorDB)
	t.Logf("  THD:                    %.4f%% (%.2f dB)", metrics.THDPercent, metrics.THD_dB)
	t.Logf("  20Hz-20kHz Noise:       %.2f dBFS", metrics.InBandNoiseDB)
	t.Logf("  >20kHz Ultrasonic Noise:%.2f dBFS", metrics.UltrasonicNoiseDB)
	t.Logf("  Ultrasonic Noise Rise:  %v", metrics.HasUltrasonicNoise)

	// 1. Confirm Fundamental is 1000 Hz
	if math.Abs(metrics.FundamentalFreqHz-1000.0) > 50.0 {
		t.Errorf("fundamental frequency outside 1000Hz: got %.2f Hz", metrics.FundamentalFreqHz)
	}

	// 2. Amplitude error within +/- 1.0 dB
	if math.Abs(metrics.AmplitudeErrorDB) > 1.0 {
		t.Errorf("amplitude error too large: got %.2f dB", metrics.AmplitudeErrorDB)
	}

	// 3. THD below -40 dB
	if metrics.THD_dB > -40.0 {
		t.Errorf("THD too high for 1kHz golden DSD: got %.2f dB", metrics.THD_dB)
	}

	// 4. Ultrasonic noise check: DSD MUST exhibit high-frequency noise shaping (>20kHz noise > in-band noise)
	// As stated in schedule: "不得以'完全没有高频噪声'作为 DSD 正确性的标准"
	if !metrics.HasUltrasonicNoise {
		t.Errorf("expected DSD ultrasonic noise shaping, but ultrasonic noise is not higher than in-band noise")
	}
}

func TestNonDSDDeviceSafetyDefaultMute(t *testing.T) {
	// A non-DSD device must reject unconfirmed DoP and maintain OpenMuted state
	backend := wasapi.NewBackend()
	ctrl := controller.NewAppController(backend)
	_ = ctrl.Initialize()
	defer ctrl.Close()

	// Ensure guard is revoked (unconfirmed)
	ctrl.DoPGuard().Revoke()

	req := controller.DoPRequest{
		DSDFormat:    dsd.DSD64,
		SampleFormat: audio.SampleFormat24In32,
		Channels:     2,
	}

	err := ctrl.ConfigureDoP(req)
	if !errors.Is(err, controller.ErrDoPRiskNotConfirmed) {
		t.Errorf("expected ErrDoPRiskNotConfirmed, got %v", err)
	}

	status := ctrl.Status()
	if status.State != engine.StateClosed && status.State != engine.StateOpenMuted {
		t.Errorf("non-confirmed device must remain in Closed or OpenMuted state, got %s", status.State)
	}
}

func TestPhysicalDSDHardwareStreamIfAvailable(t *testing.T) {
	if os.Getenv("TEST_REAL_DSD_DAC") != "1" {
		t.Skip("Skipping physical DSD hardware stream test (set TEST_REAL_DSD_DAC=1 to run against connected USB DAC)")
	}

	backend := wasapi.NewBackend()
	ctrl := controller.NewAppController(backend)
	if err := ctrl.Initialize(); err != nil {
		t.Fatalf("backend init failed: %v", err)
	}
	defer ctrl.Close()

	if err := ctrl.SelectDevice(""); err != nil {
		t.Fatalf("failed to select default device: %v", err)
	}

	ctrl.DoPGuard().ConfirmViaFlag(true)
	req := controller.DoPRequest{
		DSDFormat:    dsd.DSD64,
		SampleFormat: audio.SampleFormat24Packed,
		Channels:     2,
		BitOrder:     dsd.BitOrderMSBFirst,
	}

	if err := ctrl.ConfigureDoP(req); err != nil {
		t.Fatalf("failed to configure physical DAC for DoP: %v", err)
	}

	if err := ctrl.StartPlayback(); err != nil {
		t.Fatalf("failed to start DoP playback: %v", err)
	}

	time.Sleep(500 * time.Millisecond)

	diag := ctrl.Status().Diagnostics
	if diag.UnderrunCount > 0 {
		t.Errorf("physical stream experienced %d underruns", diag.UnderrunCount)
	}

	_ = ctrl.StopPlayback()
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
