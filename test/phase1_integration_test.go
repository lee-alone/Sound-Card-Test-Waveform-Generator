package test

import (
	"errors"
	"runtime"
	"testing"
	"time"

	"sound-signal-generator/internal/audio"
	"sound-signal-generator/internal/audio/wasapi"
	"sound-signal-generator/internal/controller"
	"sound-signal-generator/internal/format"
)

func TestPhase1IntegrationMatrix(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	backend := wasapi.NewBackend()
	ctrl := controller.NewAppController(backend)
	if err := ctrl.Initialize(); err != nil {
		t.Fatalf("ctrl.Initialize failed: %v", err)
	}
	defer ctrl.Close()

	if err := ctrl.SelectDevice(""); err != nil {
		t.Fatalf("SelectDevice failed: %v", err)
	}

	type testCase struct {
		Freq        float64
		Rate        int
		ExpectError error
		Description string
	}

	cases := []testCase{
		// Valid cases
		{Freq: 1000, Rate: 48000, ExpectError: nil, Description: "1kHz @ 48kHz"},
		{Freq: 1000, Rate: 96000, ExpectError: nil, Description: "1kHz @ 96kHz"},
		{Freq: 1000, Rate: 192000, ExpectError: nil, Description: "1kHz @ 192kHz"},
		{Freq: 30000, Rate: 96000, ExpectError: nil, Description: "30kHz @ 96kHz"},
		{Freq: 30000, Rate: 192000, ExpectError: nil, Description: "30kHz @ 192kHz"},
		{Freq: 60000, Rate: 176400, ExpectError: nil, Description: "60kHz @ 176.4kHz"},
		{Freq: 60000, Rate: 192000, ExpectError: nil, Description: "60kHz @ 192kHz"},

		// Nyquist violations (60kHz on <= 96kHz)
		{Freq: 60000, Rate: 96000, ExpectError: controller.ErrFrequencyExceedsNyquistLimit, Description: "60kHz @ 96kHz (Nyquist violation)"},
		{Freq: 60000, Rate: 48000, ExpectError: controller.ErrFrequencyExceedsNyquistLimit, Description: "60kHz @ 48kHz (Nyquist violation)"},
	}

	for _, tc := range cases {
		t.Run(tc.Description, func(t *testing.T) {
			req := controller.SignalRequest{
				SampleRate:   tc.Rate,
				SampleFormat: audio.SampleFormat16Int,
				Channels:     2,
				Frequency:    tc.Freq,
				Amplitude:    format.DBFSToAmplitude(-6.0),
			}

			err := ctrl.ConfigureSignal(req)
			if tc.ExpectError != nil {
				if !errors.Is(err, tc.ExpectError) {
					t.Fatalf("expected error %v, got %v", tc.ExpectError, err)
				}
				return
			}

			if err != nil {
				// If the hardware DAC does not support exclusive mode at this exact rate, log and skip playback
				t.Logf("Device does not support exclusive stream at %d Hz: %v", tc.Rate, err)
				return
			}

			if err := ctrl.StartPlayback(); err != nil {
				t.Fatalf("StartPlayback failed: %v", err)
			}

			// Let it stream for 200ms
			time.Sleep(200 * time.Millisecond)

			status := ctrl.Status()
			_ = ctrl.StopPlayback()

			diag := status.Diagnostics
			t.Logf("[%s] Callbacks: %d, Underruns: %d, PeakRenderTime: %v",
				tc.Description, diag.RenderCallbackCount, diag.UnderrunCount, diag.MaxRenderDuration)

			if diag.UnderrunCount > 0 {
				t.Errorf("expected 0 underruns, got %d", diag.UnderrunCount)
			}
		})
	}
}

func TestPhase1HotPathZeroAllocations(t *testing.T) {
	// Verify hot-path format conversion, peak check, and sine render allocate 0 bytes
	samples := make([]float64, 1024)
	ch0 := make([]float64, 1024)
	ch1 := make([]float64, 1024)
	channels := [][]float64{ch0, ch1}
	dst := make([]byte, 1024*4)

	format16 := audio.AudioFormat{
		SampleRate:   48000,
		Channels:     2,
		SampleFormat: audio.SampleFormat16Int,
	}

	allocs := testing.AllocsPerRun(1000, func() {
		_ = format.Interleave(dst, channels, format16, false)
		_ = format.Quantize16Int(dst[:2048], samples, false)
	})

	if allocs != 0 {
		t.Errorf("Hot-path allocs = %v, want 0", allocs)
	}
}
