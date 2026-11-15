package asio

import (
	"testing"
	"time"

	"sound-signal-generator/internal/audio"
	"sound-signal-generator/internal/audio/asio/bridge"
)

func TestASIOStreamCreatesPCMBuffersAndRejectsWrongSampleType(t *testing.T) {
	driver := bridge.NewMockDriver(bridge.ProfileThesycon)
	stream, err := NewStream(driver, audio.StreamConfig{
		Format: audio.AudioFormat{
			SampleRate:   48000,
			Channels:     2,
			SampleFormat: audio.SampleFormat24In32,
		},
		BufferFrames:   128,
		BufferDuration: 10 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("NewStream PCM failed: %v", err)
	}
	if frames, _ := stream.GetBufferSize(); frames != 128 {
		t.Fatalf("expected explicit ASIO buffer size 128, got %d", frames)
	}
	if err := stream.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	driver2 := bridge.NewMockDriver(bridge.ProfileThesycon)
	_, err = NewStream(driver2, audio.StreamConfig{
		Format: audio.AudioFormat{
			SampleRate:   48000,
			Channels:     2,
			SampleFormat: audio.SampleFormat16Int,
		},
		BufferFrames: 128,
	})
	if err == nil {
		t.Fatalf("expected incompatible ASIO sample type error")
	}
}
