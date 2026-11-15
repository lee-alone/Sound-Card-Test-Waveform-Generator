package giuui

import (
	"testing"
	"time"

	"sound-signal-generator/internal/controller"
)

func TestSpectrumWidget_Defaults(t *testing.T) {
	state := NewUIState()
	w := NewSpectrumWidget(state)

	if w == nil {
		t.Fatal("expected non-nil SpectrumWidget")
	}
	if w.NeedsAnimation() {
		t.Error("expected NeedsAnimation() to be false when output is stopped")
	}
	if len(w.xTicks) == 0 {
		t.Error("expected non-empty xTicks")
	}
	if len(w.yTicks) == 0 {
		t.Error("expected non-empty yTicks")
	}
}

func TestSpectrumWidget_NeedsAnimationWhenRunning(t *testing.T) {
	state := NewUIState()
	state.IsActiveStream = true
	state.StreamStartTime = time.Now()
	w := NewSpectrumWidget(state)

	if !w.NeedsAnimation() {
		t.Error("expected NeedsAnimation() to be true when output is running")
	}
}

func TestSpectrumWidget_SynthesizeSamples(t *testing.T) {
	state := NewUIState()
	state.IsActiveStream = true
	state.StreamStartTime = time.Now()
	state.Frequency = 1000.0
	state.SelectedWaveformIndex = 0 // Sine

	w := NewSpectrumWidget(state)
	now := time.Now()
	w.synthesizeSamples(now)

	hasNonZero := false
	for _, s := range w.samples {
		if s != 0 {
			hasNonZero = true
			break
		}
	}
	if !hasNonZero {
		t.Error("expected non-zero synthesized samples for sine wave")
	}
}

func TestSpectrumWidget_ChannelGains(t *testing.T) {
	state := NewUIState()
	state.LeftVolumeDBFS = 0.0
	state.RightVolumeDBFS = 0.0
	w := NewSpectrumWidget(state)

	// Stereo
	state.SelectedChannelMode = int32(ChannelModeStereo)
	gL, gR := w.channelGains(controller.WaveformSine)
	if gL != 1.0 || gR != 1.0 {
		t.Errorf("expected 1.0, 1.0 for stereo, got %f, %f", gL, gR)
	}

	// Solo Left
	state.SelectedChannelMode = int32(ChannelModeSoloL)
	gL, gR = w.channelGains(controller.WaveformSine)
	if gL != 1.0 || gR != 0.0 {
		t.Errorf("expected 1.0, 0.0 for solo L, got %f, %f", gL, gR)
	}

	// Solo Right
	state.SelectedChannelMode = int32(ChannelModeSoloR)
	gL, gR = w.channelGains(controller.WaveformSine)
	if gL != 0.0 || gR != 1.0 {
		t.Errorf("expected 0.0, 1.0 for solo R, got %f, %f", gL, gR)
	}
}

func TestSpectrumWidget_DynamicFrequencyCeiling(t *testing.T) {
	state := NewUIState()
	// Set 96 kHz sample rate (index 3 in SampleRateOptions: 44100, 48000, 88200, 96000)
	state.SelectedSampleRateIndex = 3
	state.Frequency = 35000.0 // 35 kHz tone

	w := NewSpectrumWidget(state)
	w.ensureAnalyzerConfig()

	bands := w.analyzer.Bands()
	lastBand := bands[len(bands)-1]

	// Upper limit should expand to cover 35 kHz (e.g. 40 kHz band)
	if lastBand.CenterFreq < 35000.0 {
		t.Errorf("expected spectrum bands to expand past 35 kHz, got last band = %.0f Hz", lastBand.CenterFreq)
	}

	// Verify sweep expands as well
	state.SelectedWaveformIndex = 3 // Linear sweep
	state.SweepEndFreq = 42000.0
	w.ensureAnalyzerConfig()

	bandsSweep := w.analyzer.Bands()
	lastSweepBand := bandsSweep[len(bandsSweep)-1]
	if lastSweepBand.CenterFreq < 40000.0 {
		t.Errorf("expected sweep bands to cover up to 40+ kHz, got %.0f Hz", lastSweepBand.CenterFreq)
	}
}

func TestSpectrumWidget_FormatAxisLabel(t *testing.T) {
	state := NewUIState()
	w := NewSpectrumWidget(state)

	// Standby mode
	standbyLabel := w.formatAxisLabel(time.Now())
	if standbyLabel == "" {
		t.Error("expected non-empty standby axis label")
	}

	// Live mode with 400 Hz Sine
	state.IsActiveStream = true
	state.StreamStartTime = time.Now()
	state.Frequency = 400.0
	state.SelectedWaveformIndex = 0 // Sine

	label := w.formatAxisLabel(time.Now())
	if label == "" {
		t.Error("expected non-empty live axis label")
	}

	// Sweep mode
	state.SelectedWaveformIndex = 3 // Linear sweep
	state.SweepStartFreq = 20.0
	state.SweepEndFreq = 20000.0
	state.SweepDurationSec = 5.0

	sweepLabel := w.formatAxisLabel(time.Now())
	if sweepLabel == "" {
		t.Error("expected non-empty sweep axis label")
	}

	// Noise mode
	state.SelectedWaveformIndex = 5 // White noise
	noiseLabel := w.formatAxisLabel(time.Now())
	if noiseLabel == "" {
		t.Error("expected non-empty noise axis label")
	}
}
