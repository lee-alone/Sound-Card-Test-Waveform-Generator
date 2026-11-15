package giuui

import (
	"strings"
	"testing"
	"time"

	"sound-signal-generator/internal/controller"
)

func TestMetrologyWidget_FormatSignalFrequency(t *testing.T) {
	state := NewUIState()
	m := NewMetrologyWidget(state)

	// Sine tone at 400 Hz
	state.Frequency = 400.0
	freqText := m.formatSignalFrequency(controller.WaveformSine)
	if !strings.Contains(freqText, "400 Hz") {
		t.Errorf("expected frequency text to contain '400 Hz', got %q", freqText)
	}

	// 1 kHz Sine tone
	state.Frequency = 1000.0
	freq1k := m.formatSignalFrequency(controller.WaveformSine)
	if !strings.Contains(freq1k, "1.00 kHz") {
		t.Errorf("expected frequency text to contain '1.00 kHz', got %q", freq1k)
	}

	// Sweep standby
	state.SweepStartFreq = 20.0
	state.SweepEndFreq = 20000.0
	state.SweepDurationSec = 5.0
	state.IsActiveStream = false
	sweepStandbyText := m.formatSignalFrequency(controller.WaveformLinearSweep)
	if !strings.Contains(sweepStandbyText, "20 Hz") || !strings.Contains(sweepStandbyText, "20.00 kHz") {
		t.Errorf("expected sweep standby text to contain start and end freq, got %q", sweepStandbyText)
	}

	// Sweep running
	state.IsActiveStream = true
	state.StreamStartTime = time.Now()
	sweepRunningText := m.formatSignalFrequency(controller.WaveformLinearSweep)
	if sweepRunningText == "" {
		t.Error("expected non-empty sweep running text")
	}

	// Noise
	noiseText := m.formatSignalFrequency(controller.WaveformWhiteNoise)
	if noiseText == "" {
		t.Error("expected non-empty noise text")
	}
}
