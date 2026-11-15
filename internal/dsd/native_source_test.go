package dsd

import (
	"bytes"
	"testing"
)

func TestNativeDSDSourcePlanarLayoutAndLoop(t *testing.T) {
	source, err := NewNativeDSDSource([]byte{0x01, 0x02, 0x03}, 2)
	if err != nil {
		t.Fatalf("NewNativeDSDSource failed: %v", err)
	}

	dst := make([]byte, 4)
	if err := source.ProvideFrames(dst, 16); err != nil {
		t.Fatalf("ProvideFrames failed: %v", err)
	}
	want := []byte{0x01, 0x02, 0x01, 0x02}
	if !bytes.Equal(dst, want) {
		t.Fatalf("unexpected planar native layout: got % x want % x", dst, want)
	}

	next := make([]byte, 2)
	if err := source.ProvideFrames(next, 8); err != nil {
		t.Fatalf("second ProvideFrames failed: %v", err)
	}
	if !bytes.Equal(next, []byte{0x03, 0x03}) {
		t.Fatalf("source did not continue wrapping deterministically: % x", next)
	}

	source.Reset()
	again := make([]byte, 2)
	if err := source.ProvideFrames(again, 8); err != nil {
		t.Fatalf("ProvideFrames after Reset failed: %v", err)
	}
	if !bytes.Equal(again, []byte{0x01, 0x01}) {
		t.Fatalf("Reset did not rewind source: % x", again)
	}
}

func TestNativeDSDSourceRejectsShortBuffer(t *testing.T) {
	source, _ := NewNativeDSDSource([]byte{0xAA}, 2)
	if err := source.ProvideFrames(make([]byte, 1), 8); err != ErrNativeSourceTooSmall {
		t.Fatalf("expected ErrNativeSourceTooSmall, got %v", err)
	}
}
