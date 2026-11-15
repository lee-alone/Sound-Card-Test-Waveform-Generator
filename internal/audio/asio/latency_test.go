package asio

import (
	"testing"

	"sound-signal-generator/internal/audio"
	"sound-signal-generator/internal/audio/asio/bridge"
)

func TestASIOBufferLatencyAndJitterAcrossSizes(t *testing.T) {
	bufferSizes := []int{64, 128, 256}
	sampleRate := 48000.0
	reps := 100

	driver := bridge.NewMockDriver(bridge.ProfileThesycon)
	defer driver.Close()

	for _, bufSize := range bufferSizes {
		t.Run("BufferSize", func(t *testing.T) {
			_ = driver.SetSampleRate(sampleRate)
			_ = driver.CreateBuffers(2, bufSize)

			result, err := MeasureLoopbackLatency(driver, bufSize, sampleRate, reps)
			if err != nil {
				t.Fatalf("MeasureLoopbackLatency failed for bufSize %d: %v", bufSize, err)
			}

			t.Logf("\n=== ASIO Latency & Jitter Report (%d frames) ===", bufSize)
			t.Logf("  Reported In Latency:  %d frames", result.ReportedInputLatency)
			t.Logf("  Reported Out Latency: %d frames", result.ReportedOutputLatency)
			t.Logf("  Total Reported:       %d frames", result.ReportedTotalLatency)
			t.Logf("  Measured Round-Trip:  %.3f frames", result.MeasuredRoundTripLatency)
			t.Logf("  Converter Offset:     %.3f frames", result.ConverterOffset)
			t.Logf("  Jitter StdDev:        %.4f samples (must be <= 1.0)", result.JitterStandardDeviation)

			// Acceptance criteria:
			// 1. Fixed configuration repeat 100 times: latency jitter std dev <= 1 sample
			if result.JitterStandardDeviation > 1.0 {
				t.Errorf("jitter standard deviation %.4f exceeded 1 sample limit", result.JitterStandardDeviation)
			}

			// 2. Reported input + output latency must be at least bufferSize
			if result.ReportedTotalLatency < bufSize {
				t.Errorf("reported total latency %d is unexpectedly less than buffer size %d", result.ReportedTotalLatency, bufSize)
			}
		})
	}
}

func TestASIOAudioStreamStreamingLifecycle(t *testing.T) {
	backend := NewMockBackend(bridge.ProfileThesycon)
	if err := backend.Initialize(); err != nil {
		t.Fatalf("backend.Initialize failed: %v", err)
	}
	defer backend.Close()

	dev, err := backend.GetDefaultDevice()
	if err != nil {
		t.Fatalf("GetDefaultDevice failed: %v", err)
	}

	streamConfig := audio.StreamConfig{
		Format: audio.AudioFormat{
			SampleRate:   48000,
			Channels:     2,
			SampleFormat: audio.SampleFormat24In32,
		},
		ShareMode: audio.ShareModeExclusive,
	}

	stream, err := dev.OpenStream(streamConfig)
	if err != nil {
		t.Fatalf("OpenStream failed: %v", err)
	}
	defer stream.Close()

	if err := stream.Start(); err != nil {
		t.Fatalf("stream.Start failed: %v", err)
	}

	bufSize, err := stream.GetBufferSize()
	if err != nil || bufSize == 0 {
		t.Fatalf("GetBufferSize failed: %d, %v", bufSize, err)
	}

	// Request buffer and commit
	ptr, err := stream.GetBuffer(int(bufSize))
	if err != nil || ptr == 0 {
		t.Fatalf("GetBuffer failed: ptr=%v, err=%v", ptr, err)
	}

	if err := stream.ReleaseBuffer(int(bufSize), 0); err != nil {
		t.Fatalf("ReleaseBuffer failed: %v", err)
	}

	if err := stream.Stop(); err != nil {
		t.Fatalf("stream.Stop failed: %v", err)
	}
}
