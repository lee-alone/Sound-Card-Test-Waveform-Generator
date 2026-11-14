package format

import (
	"math"
	"testing"
)

func TestPeakCalculationAndDBFS(t *testing.T) {
	samples := []float64{-0.5, 0.2, 0.70710678, -0.9}
	peak := CalculatePeak(samples)
	if math.Abs(peak-0.9) > 1e-9 {
		t.Errorf("CalculatePeak = %f, want 0.9", peak)
	}

	// 1.0 -> 0 dBFS
	dbfs := AmplitudeToDBFS(1.0)
	if math.Abs(dbfs) > 1e-9 {
		t.Errorf("AmplitudeToDBFS(1.0) = %f, want 0.0", dbfs)
	}

	// 0.5 -> -6.0206 dBFS
	dbfsHalf := AmplitudeToDBFS(0.5)
	if math.Abs(dbfsHalf-(-6.0205999)) > 1e-5 {
		t.Errorf("AmplitudeToDBFS(0.5) = %f, want -6.0206", dbfsHalf)
	}

	// Round-trip check
	amp := DBFSToAmplitude(dbfsHalf)
	if math.Abs(amp-0.5) > 1e-7 {
		t.Errorf("DBFSToAmplitude = %f, want 0.5", amp)
	}
}

func TestPeakGuardStrictRejection(t *testing.T) {
	guard := NewPeakGuard(false)

	// Valid signal under 0 dBFS
	if err := guard.Check(1.0); err != nil {
		t.Errorf("guard.Check(1.0) failed: %v", err)
	}

	// Overloaded signal
	if err := guard.Check(1.0001); err != ErrPeakExceedsFullScale {
		t.Errorf("guard.Check(1.0001) expected ErrPeakExceedsFullScale, got %v", err)
	}

	// In test mode (saturation allowed)
	guardTest := NewPeakGuard(true)
	if err := guardTest.Check(1.5); err != nil {
		t.Errorf("guardTest.Check(1.5) with saturation failed: %v", err)
	}
}

func TestPeakGuardInspectZeroAllocations(t *testing.T) {
	n := 1024
	samples := make([]float64, n)
	guard := NewPeakGuard(false)

	allocs := testing.AllocsPerRun(1000, func() {
		_, _ = guard.Inspect(samples)
	})
	if allocs != 0 {
		t.Errorf("guard.Inspect allocs = %v, want 0", allocs)
	}
}
