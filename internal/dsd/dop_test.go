package dsd

import (
	"bytes"
	"errors"
	"testing"

	"sound-signal-generator/internal/audio"
)

func TestDoPByteLayout24Packed(t *testing.T) {
	// Sample test payload: 4 bytes (2 frames)
	testBitstream := []byte{0x12, 0x34, 0xAB, 0xCD}
	packer, err := NewDoPPacker(testBitstream, 2, audio.SampleFormat24Packed, BitOrderMSBFirst)
	if err != nil {
		t.Fatalf("failed to create DoPPacker: %v", err)
	}

	buf := make([]byte, 2*2*3) // 2 frames * 2 channels * 3 bytes = 12 bytes
	n, err := packer.PackFrames(buf, 2)
	if err != nil {
		t.Fatalf("PackFrames failed: %v", err)
	}
	if n != 12 {
		t.Errorf("expected 12 bytes written, got %d", n)
	}

	// Frame 0: Marker must be 0x05
	// Channel 0: [0x12, 0x34, 0x05]
	// Channel 1: [0x12, 0x34, 0x05]
	expectedFrame0 := []byte{
		0x12, 0x34, 0x05, // ch0
		0x12, 0x34, 0x05, // ch1
	}
	if !bytes.Equal(buf[0:6], expectedFrame0) {
		t.Errorf("Frame 0 24-packed layout mismatch: got %X, want %X", buf[0:6], expectedFrame0)
	}

	// Frame 1: Marker must be 0xFA
	// Channel 0: [0xAB, 0xCD, 0xFA]
	// Channel 1: [0xAB, 0xCD, 0xFA]
	expectedFrame1 := []byte{
		0xAB, 0xCD, 0xFA, // ch0
		0xAB, 0xCD, 0xFA, // ch1
	}
	if !bytes.Equal(buf[6:12], expectedFrame1) {
		t.Errorf("Frame 1 24-packed layout mismatch: got %X, want %X", buf[6:12], expectedFrame1)
	}
}

func TestDoPByteLayout24In32(t *testing.T) {
	testBitstream := []byte{0x12, 0x34, 0xAB, 0xCD}
	packer, err := NewDoPPacker(testBitstream, 2, audio.SampleFormat24In32, BitOrderMSBFirst)
	if err != nil {
		t.Fatalf("failed to create DoPPacker: %v", err)
	}

	buf := make([]byte, 2*2*4) // 2 frames * 2 channels * 4 bytes = 16 bytes
	n, err := packer.PackFrames(buf, 2)
	if err != nil {
		t.Fatalf("PackFrames failed: %v", err)
	}
	if n != 16 {
		t.Errorf("expected 16 bytes written, got %d", n)
	}

	// Frame 0: Marker 0x05, [0x00, 0x12, 0x34, 0x05]
	expectedFrame0 := []byte{
		0x00, 0x12, 0x34, 0x05, // ch0
		0x00, 0x12, 0x34, 0x05, // ch1
	}
	if !bytes.Equal(buf[0:8], expectedFrame0) {
		t.Errorf("Frame 0 24-in-32 layout mismatch: got %X, want %X", buf[0:8], expectedFrame0)
	}

	// Frame 1: Marker 0xFA, [0x00, 0xAB, 0xCD, 0xFA]
	expectedFrame1 := []byte{
		0x00, 0xAB, 0xCD, 0xFA, // ch0
		0x00, 0xAB, 0xCD, 0xFA, // ch1
	}
	if !bytes.Equal(buf[8:16], expectedFrame1) {
		t.Errorf("Frame 1 24-in-32 layout mismatch: got %X, want %X", buf[8:16], expectedFrame1)
	}
}

func TestDoPMarkerAlternationAndChannelSync(t *testing.T) {
	// Long run with Golden Vector DSD64
	golden, err := GetGoldenVector(DSD64)
	if err != nil {
		t.Fatalf("failed to get DSD64 golden vector: %v", err)
	}

	channels := 2
	packer, err := NewDoPPacker(golden.Data, channels, audio.SampleFormat24Packed, BitOrderMSBFirst)
	if err != nil {
		t.Fatalf("failed to create DoPPacker: %v", err)
	}

	numFrames := 1000
	buf := make([]byte, numFrames*channels*3)
	_, err = packer.PackFrames(buf, numFrames)
	if err != nil {
		t.Fatalf("PackFrames failed: %v", err)
	}

	expectedMarker := DoPMarker05
	for f := 0; f < numFrames; f++ {
		offset := f * channels * 3
		markerCh0 := buf[offset+2]
		markerCh1 := buf[offset+5]

		// 1. All channels must have identical marker
		if markerCh0 != markerCh1 {
			t.Fatalf("Frame %d marker mismatch between channels: ch0=0x%02X, ch1=0x%02X", f, markerCh0, markerCh1)
		}

		// 2. Marker must match expected alternation
		if markerCh0 != expectedMarker {
			t.Fatalf("Frame %d unexpected marker: got 0x%02X, want 0x%02X", f, markerCh0, expectedMarker)
		}

		// Toggle expected marker
		if expectedMarker == DoPMarker05 {
			expectedMarker = DoPMarkerFA
		} else {
			expectedMarker = DoPMarker05
		}
	}
}

func TestDoPResetDeterministicMarkerRecovery(t *testing.T) {
	testBitstream := []byte{0x01, 0x02, 0x03, 0x04}
	packer, err := NewDoPPacker(testBitstream, 2, audio.SampleFormat24Packed, BitOrderMSBFirst)
	if err != nil {
		t.Fatalf("failed to create DoPPacker: %v", err)
	}

	buf := make([]byte, 1*2*3)

	// Step 1: Pack 1 frame (marker will be 0x05, next will be 0xFA)
	_, _ = packer.PackFrames(buf, 1)
	if packer.CurrentMarker() != DoPMarkerFA {
		t.Errorf("expected next marker to be 0xFA, got 0x%02X", packer.CurrentMarker())
	}

	// Step 2: Trigger Reset (simulating pause, underrun recovery, or device reopen)
	packer.Reset()

	// Step 3: Must restart from deterministic starting marker 0x05
	if packer.CurrentMarker() != DoPMarker05 {
		t.Errorf("after Reset(), CurrentMarker() must be 0x05, got 0x%02X", packer.CurrentMarker())
	}

	_, _ = packer.PackFrames(buf, 1)
	if buf[2] != DoPMarker05 || buf[5] != DoPMarker05 {
		t.Errorf("first frame after Reset() did not write 0x05: ch0=0x%02X, ch1=0x%02X", buf[2], buf[5])
	}
}

func TestDoPBitOrderLSBFirst(t *testing.T) {
	// 0x01 (00000001b) reversed is 0x80 (10000000b)
	// 0x0F (00001111b) reversed is 0xF0 (11110000b)
	testBitstream := []byte{0x01, 0x0F}
	packer, err := NewDoPPacker(testBitstream, 1, audio.SampleFormat24Packed, BitOrderLSBFirst)
	if err != nil {
		t.Fatalf("failed to create DoPPacker: %v", err)
	}

	buf := make([]byte, 3)
	_, err = packer.PackFrames(buf, 1)
	if err != nil {
		t.Fatalf("PackFrames failed: %v", err)
	}

	if buf[0] != 0x80 || buf[1] != 0xF0 {
		t.Errorf("LSBFirst bit reversal failed: got [%02X, %02X], want [80, F0]", buf[0], buf[1])
	}
}

func TestDoPPackerHotPathZeroAllocations(t *testing.T) {
	golden, _ := GetGoldenVector(DSD64)
	packer, _ := NewDoPPacker(golden.Data, 2, audio.SampleFormat24Packed, BitOrderMSBFirst)

	numFrames := 1764 // 10ms at 176.4kHz
	buf := make([]byte, numFrames*2*3)

	allocs := testing.AllocsPerRun(1000, func() {
		_, _ = packer.PackFrames(buf, numFrames)
	})

	if allocs != 0 {
		t.Errorf("DoPPacker.PackFrames in hot path had %f heap allocations, want 0", allocs)
	}
}

func TestDoPPackerErrorCases(t *testing.T) {
	// Nil bitstream
	_, err := NewDoPPacker(nil, 2, audio.SampleFormat24Packed, BitOrderMSBFirst)
	if !errors.Is(err, ErrNilBitstreamSource) {
		t.Errorf("expected ErrNilBitstreamSource, got %v", err)
	}

	// Unsupported formats (16-bit, 32-bit float)
	_, err = NewDoPPacker([]byte{0x00}, 2, audio.SampleFormat16Int, BitOrderMSBFirst)
	if !errors.Is(err, ErrUnsupportedDoPFormat) {
		t.Errorf("expected ErrUnsupportedDoPFormat for 16Int, got %v", err)
	}
	_, err = NewDoPPacker([]byte{0x00}, 2, audio.SampleFormat32Float, BitOrderMSBFirst)
	if !errors.Is(err, ErrUnsupportedDoPFormat) {
		t.Errorf("expected ErrUnsupportedDoPFormat for 32Float, got %v", err)
	}

	// Destination too small
	p, _ := NewDoPPacker([]byte{0x00, 0x00}, 2, audio.SampleFormat24Packed, BitOrderMSBFirst)
	smallBuf := make([]byte, 5)
	_, err = p.PackFrames(smallBuf, 1) // needs 6 bytes
	if !errors.Is(err, ErrDestinationTooSmall) {
		t.Errorf("expected ErrDestinationTooSmall, got %v", err)
	}
}
