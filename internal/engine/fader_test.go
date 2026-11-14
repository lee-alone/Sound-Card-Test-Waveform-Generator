package engine

import (
	"math"
	"testing"
)

func TestFaderRampProgression(t *testing.T) {
	fader := NewFader()
	sampleRate := 48000
	durationMs := 10.0 // 480 samples

	// 1. Initial state is muted
	if fader.State() != FaderStateMuted || fader.CurrentGain() != 0.0 {
		t.Fatalf("fader not muted initially")
	}

	// 2. Start Fade In
	fader.StartFadeIn(sampleRate, durationMs)
	if fader.State() != FaderStateFadingIn {
		t.Fatalf("fader state want FadingIn, got %v", fader.State())
	}

	buf := make([]float64, 480)
	for i := range buf {
		buf[i] = 1.0
	}

	done := fader.Apply(buf)
	if done {
		t.Errorf("fade-in should not return fadeOutFinished=true")
	}
	if fader.State() != FaderStateUnity || fader.CurrentGain() != 1.0 {
		t.Errorf("after 480 samples, fader should be Unity (1.0), got %f (%v)", fader.CurrentGain(), fader.State())
	}

	// Check envelope smoothness: first sample should be small, last sample 1.0
	if buf[0] <= 0.0 || buf[0] > 0.01 {
		t.Errorf("buf[0] = %f, expected small positive ramp start", buf[0])
	}
	if math.Abs(buf[479]-1.0) > 1e-3 {
		t.Errorf("buf[479] = %f, expected ~1.0", buf[479])
	}

	// 3. Start Fade Out
	fader.StartFadeOut(sampleRate, durationMs)
	if fader.State() != FaderStateFadingOut {
		t.Fatalf("fader state want FadingOut, got %v", fader.State())
	}

	for i := range buf {
		buf[i] = 1.0
	}

	done = fader.Apply(buf)
	if !done {
		t.Errorf("fade-out should complete after 480 samples")
	}
	if fader.State() != FaderStateMuted || fader.CurrentGain() != 0.0 {
		t.Errorf("after fade-out, fader should be Muted, got %f (%v)", fader.CurrentGain(), fader.State())
	}
}

func TestFaderZeroAllocations(t *testing.T) {
	fader := NewFader()
	fader.StartFadeIn(48000, 10.0)

	ch0 := make([]float64, 256)
	ch1 := make([]float64, 256)
	channels := [][]float64{ch0, ch1}

	allocs := testing.AllocsPerRun(1000, func() {
		fader.ApplyChannels(channels)
	})
	if allocs != 0 {
		t.Errorf("Fader.ApplyChannels allocs = %v, want 0", allocs)
	}
}
