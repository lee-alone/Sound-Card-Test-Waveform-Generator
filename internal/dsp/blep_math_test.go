package dsp

import (
	"math"
	"testing"
)

func TestPolyBLEP4Properties(t *testing.T) {
	// Boundary zero checks
	if val := PolyBLEP4(-2.0); val != 0.0 {
		t.Errorf("expected PolyBLEP4(-2) == 0, got %v", val)
	}
	if val := PolyBLEP4(2.0); val != 0.0 {
		t.Errorf("expected PolyBLEP4(2) == 0, got %v", val)
	}
	if val := PolyBLEP4(-2.5); val != 0.0 {
		t.Errorf("expected PolyBLEP4(-2.5) == 0, got %v", val)
	}
	if val := PolyBLEP4(2.5); val != 0.0 {
		t.Errorf("expected PolyBLEP4(2.5) == 0, got %v", val)
	}

	// Midpoint check: at u=0, PolyBLEP4(0) == -0.5
	if val := PolyBLEP4(0.0); math.Abs(val-(-0.5)) > 1e-12 {
		t.Errorf("expected PolyBLEP4(0) == -0.5, got %v", val)
	}

	// Antisymmetry check: PolyBLEP4(u) + PolyBLEP4(-u) should relate to step correction
	// Specifically around u = -1 and u = 1:
	vNeg1 := PolyBLEP4(-1.0)
	vPos1 := PolyBLEP4(1.0)
	if math.Abs(vNeg1+vPos1) > 1e-12 {
		t.Errorf("expected PolyBLEP4(-1) + PolyBLEP4(1) == 0, got %v + %v = %v", vNeg1, vPos1, vNeg1+vPos1)
	}
}

func TestPolyBLAMP4Properties(t *testing.T) {
	c0, c1, c2, c3 := PolyBLAMP4(0.0)
	if math.Abs(c3) > 1e-12 {
		t.Errorf("expected c3 == 0 at d=0, got %v", c3)
	}
	if math.Abs(c0-(1.0/120.0)) > 1e-12 {
		t.Errorf("expected c0 == 1/120 at d=0, got %v", c0)
	}
	_ = c1
	_ = c2
}

func TestHalfbandDecimator47DCGain(t *testing.T) {
	var decimator HalfbandDecimator47
	decimator.Reset()

	// Feed 100 constant 1.0 samples
	for i := 0; i < 100; i++ {
		decimator.PushSample(1.0)
	}

	out := decimator.FilterCurrent()
	if math.Abs(out-1.0) > 1e-6 {
		t.Errorf("expected DC gain 1.0, got %v", out)
	}
}

func TestHalfbandDecimator47ZeroAlloc(t *testing.T) {
	var decimator HalfbandDecimator47
	decimator.Reset()

	allocs := testing.AllocsPerRun(1000, func() {
		decimator.PushSample(0.5)
		_ = decimator.FilterCurrent()
	})

	if allocs != 0 {
		t.Errorf("expected 0 allocs, got %v", allocs)
	}
}

func TestHalfbandDecimator47RingBufferMatchesShiftingReference(t *testing.T) {
	var decimator HalfbandDecimator47
	decimator.Reset()

	var reference [47]float64
	for i := 0; i < 256; i++ {
		x := math.Sin(float64(i)*0.137) + 0.25*math.Cos(float64(i)*0.031)

		decimator.PushSample(x)
		copy(reference[1:], reference[:len(reference)-1])
		reference[0] = x

		want := reference[23] * halfband47CenterTap
		want += halfband47TapsEven[0] * (reference[0] + reference[46])
		want += halfband47TapsEven[1] * (reference[2] + reference[44])
		want += halfband47TapsEven[2] * (reference[4] + reference[42])
		want += halfband47TapsEven[3] * (reference[6] + reference[40])
		want += halfband47TapsEven[4] * (reference[8] + reference[38])
		want += halfband47TapsEven[5] * (reference[10] + reference[36])
		want += halfband47TapsEven[6] * (reference[12] + reference[34])
		want += halfband47TapsEven[7] * (reference[14] + reference[32])
		want += halfband47TapsEven[8] * (reference[16] + reference[30])
		want += halfband47TapsEven[9] * (reference[18] + reference[28])
		want += halfband47TapsEven[10] * (reference[20] + reference[26])
		want += halfband47TapsEven[11] * (reference[22] + reference[24])

		got := decimator.FilterCurrent()
		if got != want {
			t.Fatalf("sample %d: ring output %v != reference %v", i, got, want)
		}
	}
}
