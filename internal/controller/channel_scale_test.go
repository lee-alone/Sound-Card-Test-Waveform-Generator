package controller

import (
	"math"
	"testing"

	"sound-signal-generator/internal/dsp"
)

func TestChannelScaleGenerator_RenderChannelsInPhase(t *testing.T) {
	sine := dsp.NewSineGenerator()
	err := sine.Prepare(dsp.Config{
		SampleRate:   48000,
		Frequency:    1000.0,
		Amplitude:    1.0,
		InitialPhase: 0.0,
	})
	if err != nil {
		t.Fatalf("sine prepare failed: %v", err)
	}

	// Left: 0 dBFS (scale = 1.0), Right: -6 dBFS (scale ≈ 0.501187)
	gen := NewChannelScaleGenerator(sine, 0.0, -6.0)

	numFrames := 480
	channels := [][]float64{
		make([]float64, numFrames),
		make([]float64, numFrames),
	}

	gen.RenderChannels(channels)

	scaleR := dbfsToLinear(-6.0)
	for i := 0; i < numFrames; i++ {
		expectedR := channels[0][i] * scaleR
		if math.Abs(channels[1][i]-expectedR) > 1e-6 {
			t.Fatalf("frame %d: expected right channel %f, got %f", i, expectedR, channels[1][i])
		}
	}
}

func TestChannelScaleGenerator_SequentialRenderInPhase(t *testing.T) {
	sine := dsp.NewSineGenerator()
	_ = sine.Prepare(dsp.Config{
		SampleRate:   48000,
		Frequency:    1000.0,
		Amplitude:    1.0,
		InitialPhase: 0.0,
	})

	gen := NewChannelScaleGenerator(sine, 0.0, -6.0)

	numFrames := 480
	left := make([]float64, numFrames)
	right := make([]float64, numFrames)

	gen.Render(left)
	gen.Render(right)

	scaleR := dbfsToLinear(-6.0)
	for i := 0; i < numFrames; i++ {
		expectedR := left[i] * scaleR
		if math.Abs(right[i]-expectedR) > 1e-6 {
			t.Fatalf("frame %d: expected right channel %f, got %f", i, expectedR, right[i])
		}
	}
}
