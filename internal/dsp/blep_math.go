package dsp

// PolyBLEP4 computes the 4-point B-spline integrated band-limited step (BLEP) residual.
// The parameter u is the normalized sub-sample distance from the discontinuity:
// u = (phase - transitionPhase) / dt, where dt = f / Fs.
// When |u| >= 2, the residual is exactly 0.
func PolyBLEP4(u float64) float64 {
	if u < -2.0 || u >= 2.0 {
		return 0.0
	}
	if u < -1.0 {
		t := u + 2.0
		t2 := t * t
		return (t2 * t2) * (1.0 / 24.0)
	}
	if u < 0.0 {
		t := u + 1.0
		t2 := t * t
		t3 := t2 * t
		t4 := t2 * t2
		return -t4*(1.0/8.0) + t3*(1.0/6.0) + t2*(1.0/4.0) + t*(1.0/6.0) + (1.0 / 24.0)
	}
	if u < 1.0 {
		t := u
		t2 := t * t
		t3 := t2 * t
		t4 := t2 * t2
		return t4*(1.0/8.0) - t3*(1.0/3.0) + t*(2.0/3.0) - 0.5
	}
	// 1.0 <= u < 2.0
	t := u - 1.0
	t2 := t * t
	t3 := t2 * t
	t4 := t2 * t2
	return -t4*(1.0/24.0) + t3*(1.0/6.0) - t2*(1.0/4.0) + t*(1.0/6.0) - (1.0 / 24.0)
}

// PolyBLAMP4 computes 4-point PolyBLAMP (Band-Limited Ramp) residuals for a slope discontinuity.
// The parameter d is the fractional delay to the corner in [0, 1).
// Returns the 4 residuals (c0, c1, c2, c3) to be scaled by the slope change Δs.
func PolyBLAMP4(d float64) (float64, float64, float64, float64) {
	d2 := d * d
	d3 := d2 * d
	d4 := d2 * d2
	d5 := d4 * d

	c0 := -d5*(1.0/120.0) + d4*(1.0/24.0) - d3*(1.0/12.0) + d2*(1.0/12.0) - d*(1.0/24.0) + (1.0 / 120.0)
	c1 := d5*(1.0/40.0) - d4*(1.0/12.0) + d2*(1.0/3.0) - d*0.5 + (7.0 / 30.0)
	c2 := -d5*(1.0/40.0) + d4*(1.0/24.0) + d3*(1.0/12.0) + d2*(1.0/12.0) + d*(1.0/24.0) + (1.0 / 120.0)
	c3 := d5 * (1.0 / 120.0)

	return c0, c1, c2, c3
}

// PolyBLAMP4Continuous computes the 4-point B-spline integrated BLEP residual for continuous normalized phase offset u = d / dt.
// Used for rounding corner discontinuities (e.g. triangle wave peaks and troughs).
// Returns the correction residual, strictly 0 for |u| >= 2.
func PolyBLAMP4Continuous(u float64) float64 {
	if u <= -2.0 || u >= 2.0 {
		return 0.0
	}
	if u < -1.0 {
		t := u + 2.0
		t2 := t * t
		t4 := t2 * t2
		return (t4 * t) * (1.0 / 120.0)
	}
	if u < 0.0 {
		t := u + 1.0
		t2 := t * t
		t3 := t2 * t
		t4 := t2 * t2
		t5 := t4 * t
		return -t5*(1.0/40.0) + t4*(1.0/24.0) + t3*(1.0/12.0) + t2*(1.0/12.0) + t*(1.0/24.0) + (1.0 / 120.0)
	}
	if u < 1.0 {
		t := u
		t2 := t * t
		t4 := t2 * t2
		t5 := t4 * t
		return t5*(1.0/40.0) - t4*(1.0/12.0) + t2*(1.0/3.0) - 0.5*t + (7.0 / 30.0)
	}
	t := u - 1.0
	t2 := t * t
	t3 := t2 * t
	t4 := t2 * t2
	t5 := t4 * t
	return -t5*(1.0/120.0) + t4*(1.0/24.0) - t3*(1.0/12.0) + t2*(1.0/12.0) - t*(1.0/24.0) + (1.0 / 120.0)
}

// HalfbandDecimator47 implements a symmetric 47-tap halfband FIR decimation filter (2:1).
// Provides >85dB stopband attenuation and operates with zero heap allocations during streaming.
type HalfbandDecimator47 struct {
	// history is a power-of-two ring buffer. head is the next write index.
	// Keeping 47 active taps in a 64-slot buffer removes all delay-line
	// shifting while retaining the original filter response.
	history [64]float64
	head    uint8
}

// Halfband taps (symmetric, zero on odd taps except center tap 23)
var halfband47TapsEven = [12]float64{
	-5.160821732982026e-05, // tap 0 & 46
	2.894114117594008e-04,  // tap 2 & 44
	-8.594591783620736e-04, // tap 4 & 42
	1.991966847266895e-03,  // tap 6 & 40
	-3.996564757854868e-03, // tap 8 & 38
	7.283358184082787e-03,  // tap 10 & 36
	-1.242151330284007e-02, // tap 12 & 34
	2.030776203929431e-02,  // tap 14 & 32
	-3.267419363011147e-02, // tap 16 & 30
	5.388155736288766e-02,  // tap 18 & 28
	-9.996114757625872e-02, // tap 20 & 26
	3.162104308174660e-01,  // tap 22 & 24
}

const halfband47CenterTap = 0.5

// Reset clears the internal history delay line.
func (d *HalfbandDecimator47) Reset() {
	for i := range d.history {
		d.history[i] = 0.0
	}
	d.head = 0
}

// PushSample pushes one sample into the delay line.
func (d *HalfbandDecimator47) PushSample(x float64) {
	d.history[d.head&63] = x
	d.head = (d.head + 1) & 63
}

// FilterCurrent computes the filtered sample at the current delay line state.
func (d *HalfbandDecimator47) FilterCurrent() float64 {
	h := &d.history
	// head is the next write position. The oldest tap (tap 0) therefore
	// starts 47 samples behind head; (-47 mod 64) == 17.
	base := int((d.head + 17) & 63)
	sum := h[(base+23)&63] * halfband47CenterTap

	sum += halfband47TapsEven[0] * (h[base] + h[(base+46)&63])
	sum += halfband47TapsEven[1] * (h[(base+2)&63] + h[(base+44)&63])
	sum += halfband47TapsEven[2] * (h[(base+4)&63] + h[(base+42)&63])
	sum += halfband47TapsEven[3] * (h[(base+6)&63] + h[(base+40)&63])
	sum += halfband47TapsEven[4] * (h[(base+8)&63] + h[(base+38)&63])
	sum += halfband47TapsEven[5] * (h[(base+10)&63] + h[(base+36)&63])
	sum += halfband47TapsEven[6] * (h[(base+12)&63] + h[(base+34)&63])
	sum += halfband47TapsEven[7] * (h[(base+14)&63] + h[(base+32)&63])
	sum += halfband47TapsEven[8] * (h[(base+16)&63] + h[(base+30)&63])
	sum += halfband47TapsEven[9] * (h[(base+18)&63] + h[(base+28)&63])
	sum += halfband47TapsEven[10] * (h[(base+20)&63] + h[(base+26)&63])
	sum += halfband47TapsEven[11] * (h[(base+22)&63] + h[(base+24)&63])

	return sum
}
