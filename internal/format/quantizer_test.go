package format

import (
	"bytes"
	"math"
	"testing"
)

func TestQuantize16IntGolden(t *testing.T) {
	// 0.0 -> 0x00, 0x00
	// 1.0 -> 32767 -> 0xFF, 0x7F
	// -1.0 -> -32768 -> 0x00, 0x80
	// 0.5 -> 16384 -> 0x00, 0x40
	samples := []float64{0.0, 1.0, -1.0, 0.5}
	expected := []byte{
		0x00, 0x00,
		0xFF, 0x7F,
		0x00, 0x80,
		0x00, 0x40,
	}

	dst := make([]byte, len(samples)*2)
	if err := Quantize16Int(dst, samples, false); err != nil {
		t.Fatalf("Quantize16Int failed: %v", err)
	}

	if !bytes.Equal(dst, expected) {
		t.Errorf("Quantize16Int mismatch:\n got: %x\nwant: %x", dst, expected)
	}
}

func TestQuantize24PackedGolden(t *testing.T) {
	// 0.0 -> 0x00, 0x00, 0x00
	// 1.0 -> 8388607 (0x7FFFFF) -> 0xFF, 0xFF, 0x7F
	// -1.0 -> -8388608 (0x800000) -> 0x00, 0x00, 0x80
	samples := []float64{0.0, 1.0, -1.0}
	expected := []byte{
		0x00, 0x00, 0x00,
		0xFF, 0xFF, 0x7F,
		0x00, 0x00, 0x80,
	}

	dst := make([]byte, len(samples)*3)
	if err := Quantize24Packed(dst, samples, false); err != nil {
		t.Fatalf("Quantize24Packed failed: %v", err)
	}

	if !bytes.Equal(dst, expected) {
		t.Errorf("Quantize24Packed mismatch:\n got: %x\nwant: %x", dst, expected)
	}
}

func TestQuantize24In32Golden(t *testing.T) {
	// Left-aligned in 32-bit container:
	// 0.0 -> 0x00, 0x00, 0x00, 0x00
	// 1.0 -> 8388607 << 8 = 0x7FFFFF00 -> 0x00, 0xFF, 0xFF, 0x7F
	// -1.0 -> -8388608 << 8 = 0x80000000 -> 0x00, 0x00, 0x00, 0x80
	samples := []float64{0.0, 1.0, -1.0}
	expected := []byte{
		0x00, 0x00, 0x00, 0x00,
		0x00, 0xFF, 0xFF, 0x7F,
		0x00, 0x00, 0x00, 0x80,
	}

	dst := make([]byte, len(samples)*4)
	if err := Quantize24In32(dst, samples, false); err != nil {
		t.Fatalf("Quantize24In32 failed: %v", err)
	}

	if !bytes.Equal(dst, expected) {
		t.Errorf("Quantize24In32 mismatch:\n got: %x\nwant: %x", dst, expected)
	}
}

func TestQuantize32FloatGolden(t *testing.T) {
	samples := []float64{0.0, 1.0, -1.0}
	dst := make([]byte, len(samples)*4)
	if err := Quantize32Float(dst, samples, false); err != nil {
		t.Fatalf("Quantize32Float failed: %v", err)
	}

	// 0.0f -> 0x00, 0x00, 0x00, 0x00
	// 1.0f -> 0x3F800000 -> 0x00, 0x00, 0x80, 0x3F
	// -1.0f -> 0xBF800000 -> 0x00, 0x00, 0x80, 0xBF
	expected := []byte{
		0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x80, 0x3F,
		0x00, 0x00, 0x80, 0xBF,
	}
	if !bytes.Equal(dst, expected) {
		t.Errorf("Quantize32Float mismatch:\n got: %x\nwant: %x", dst, expected)
	}
}

func TestQuantizerOverloadRejection(t *testing.T) {
	overloaded := []float64{0.5, 1.0001, -0.2}
	dst16 := make([]byte, len(overloaded)*2)
	dst24 := make([]byte, len(overloaded)*3)
	dst24in32 := make([]byte, len(overloaded)*4)
	dst32f := make([]byte, len(overloaded)*4)

	// Rejection test (allowSaturation = false)
	if err := Quantize16Int(dst16, overloaded, false); err != ErrPeakExceedsFullScale {
		t.Errorf("Quantize16Int expected ErrPeakExceedsFullScale, got %v", err)
	}
	if err := Quantize24Packed(dst24, overloaded, false); err != ErrPeakExceedsFullScale {
		t.Errorf("Quantize24Packed expected ErrPeakExceedsFullScale, got %v", err)
	}
	if err := Quantize24In32(dst24in32, overloaded, false); err != ErrPeakExceedsFullScale {
		t.Errorf("Quantize24In32 expected ErrPeakExceedsFullScale, got %v", err)
	}
	if err := Quantize32Float(dst32f, overloaded, false); err != ErrPeakExceedsFullScale {
		t.Errorf("Quantize32Float expected ErrPeakExceedsFullScale, got %v", err)
	}

	// Saturation test (allowSaturation = true)
	if err := Quantize16Int(dst16, overloaded, true); err != nil {
		t.Errorf("Quantize16Int with saturation failed: %v", err)
	}
	if err := Quantize24Packed(dst24, overloaded, true); err != nil {
		t.Errorf("Quantize24Packed with saturation failed: %v", err)
	}
	if err := Quantize24In32(dst24in32, overloaded, true); err != nil {
		t.Errorf("Quantize24In32 with saturation failed: %v", err)
	}
	if err := Quantize32Float(dst32f, overloaded, true); err != nil {
		t.Errorf("Quantize32Float with saturation failed: %v", err)
	}
}

func TestQuantizerZeroAllocations(t *testing.T) {
	n := 1024
	samples := make([]float64, n)
	for i := range samples {
		samples[i] = math.Sin(float64(i) * 0.01)
	}

	dst16 := make([]byte, n*2)
	dst24 := make([]byte, n*3)
	dst24in32 := make([]byte, n*4)
	dst32f := make([]byte, n*4)

	allocs16 := testing.AllocsPerRun(1000, func() {
		_ = Quantize16Int(dst16, samples, false)
	})
	if allocs16 != 0 {
		t.Errorf("Quantize16Int allocs = %v, want 0", allocs16)
	}

	allocs24 := testing.AllocsPerRun(1000, func() {
		_ = Quantize24Packed(dst24, samples, false)
	})
	if allocs24 != 0 {
		t.Errorf("Quantize24Packed allocs = %v, want 0", allocs24)
	}

	allocs24in32 := testing.AllocsPerRun(1000, func() {
		_ = Quantize24In32(dst24in32, samples, false)
	})
	if allocs24in32 != 0 {
		t.Errorf("Quantize24In32 allocs = %v, want 0", allocs24in32)
	}

	allocs32f := testing.AllocsPerRun(1000, func() {
		_ = Quantize32Float(dst32f, samples, false)
	})
	if allocs32f != 0 {
		t.Errorf("Quantize32Float allocs = %v, want 0", allocs32f)
	}
}

func TestAudioRoundingMatchesMathRound(t *testing.T) {
	for i := -1000; i <= 1000; i++ {
		s := float64(i) / 1000.0
		want16 := int32(math.Round(s * maxInt16))
		if s < 0 {
			want16 = int32(math.Round(s * (-minInt16)))
		}
		want24 := int32(math.Round(s * maxInt24))
		if s < 0 {
			want24 = int32(math.Round(s * (-minInt24)))
		}

		if got := roundAudioSample16(s); got != want16 {
			t.Fatalf("roundAudioSample16(%v) = %d, want %d", s, got, want16)
		}
		if got := roundAudioSample24(s); got != want24 {
			t.Fatalf("roundAudioSample24(%v) = %d, want %d", s, got, want24)
		}
	}
}
