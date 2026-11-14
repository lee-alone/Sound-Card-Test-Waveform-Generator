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
	history [47]float64
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
}

// PushSample pushes one sample into the delay line.
func (d *HalfbandDecimator47) PushSample(x float64) {
	for i := len(d.history) - 1; i > 0; i-- {
		d.history[i] = d.history[i-1]
	}
	d.history[0] = x
}

// FilterCurrent computes the filtered sample at the current delay line state.
func (d *HalfbandDecimator47) FilterCurrent() float64 {
	h := &d.history
	sum := h[23] * halfband47CenterTap

	sum += halfband47TapsEven[0] * (h[0] + h[46])
	sum += halfband47TapsEven[1] * (h[2] + h[44])
	sum += halfband47TapsEven[2] * (h[4] + h[42])
	sum += halfband47TapsEven[3] * (h[6] + h[40])
	sum += halfband47TapsEven[4] * (h[8] + h[38])
	sum += halfband47TapsEven[5] * (h[10] + h[36])
	sum += halfband47TapsEven[6] * (h[12] + h[34])
	sum += halfband47TapsEven[7] * (h[14] + h[32])
	sum += halfband47TapsEven[8] * (h[16] + h[30])
	sum += halfband47TapsEven[9] * (h[18] + h[28])
	sum += halfband47TapsEven[10] * (h[20] + h[26])
	sum += halfband47TapsEven[11] * (h[22] + h[24])

	return sum
}
