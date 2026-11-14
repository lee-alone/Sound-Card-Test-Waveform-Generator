package format

import (
	"bytes"
	"testing"

	"sound-signal-generator/internal/audio"
)

func TestChannelInterleave(t *testing.T) {
	ch0 := []float64{0.0, 1.0}
	ch1 := []float64{-1.0, 0.5}
	channels := [][]float64{ch0, ch1}

	format16 := audio.AudioFormat{
		SampleRate:   48000,
		Channels:     2,
		SampleFormat: audio.SampleFormat16Int,
	}

	dst := make([]byte, 2*2*2) // 2 frames * 2 channels * 2 bytes = 8 bytes
	if err := Interleave(dst, channels, format16, false); err != nil {
		t.Fatalf("Interleave failed: %v", err)
	}

	// Frame 0: ch0=0 (00 00), ch1=-1 (00 80)
	// Frame 1: ch0=1 (FF 7F), ch1=0.5 (00 40)
	expected := []byte{
		0x00, 0x00, 0x00, 0x80,
		0xFF, 0x7F, 0x00, 0x40,
	}

	if !bytes.Equal(dst, expected) {
		t.Errorf("Interleave mismatch:\n got: %x\nwant: %x", dst, expected)
	}
}

func TestInterleaveZeroAllocations(t *testing.T) {
	n := 512
	ch0 := make([]float64, n)
	ch1 := make([]float64, n)
	channels := [][]float64{ch0, ch1}

	format16 := audio.AudioFormat{
		SampleRate:   48000,
		Channels:     2,
		SampleFormat: audio.SampleFormat16Int,
	}
	dst := make([]byte, n*2*2)

	allocs := testing.AllocsPerRun(1000, func() {
		_ = Interleave(dst, channels, format16, false)
	})
	if allocs != 0 {
		t.Errorf("Interleave allocs = %v, want 0", allocs)
	}
}

func TestInterleaveOverloadRejection(t *testing.T) {
	ch0 := []float64{0.0, 1.05}
	ch1 := []float64{0.0, 0.5}
	channels := [][]float64{ch0, ch1}

	format16 := audio.AudioFormat{
		SampleRate:   48000,
		Channels:     2,
		SampleFormat: audio.SampleFormat16Int,
	}
	dst := make([]byte, 8)

	if err := Interleave(dst, channels, format16, false); err != ErrPeakExceedsFullScale {
		t.Errorf("Interleave expected ErrPeakExceedsFullScale, got %v", err)
	}

	if err := Interleave(dst, channels, format16, true); err != nil {
		t.Errorf("Interleave with saturation failed: %v", err)
	}
}
