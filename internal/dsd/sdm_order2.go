package dsd

import "math"

// SecondOrderSDM implements a textbook 2nd-order Delta-Sigma modulator (Boser-Wooley topology).
// Integrator 1: v1[n+1] = v1[n] + (x[n] - y[n])
// Integrator 2: v2[n+1] = v2[n] + (v1[n] - y[n])
// Quantizer:    y[n] = +1.0 if v2[n] >= 0 else -1.0
// Theoretical NTF: (1 - z^-1)^2 (slope: +12 dB/octave)
type SecondOrderSDM struct {
	v1        float64
	v2        float64
	peakState float64
}

// NewSecondOrderSDM creates a 2nd-order Boser-Wooley modulator.
func NewSecondOrderSDM() *SecondOrderSDM {
	return &SecondOrderSDM{}
}

func (m *SecondOrderSDM) Order() int {
	return 2
}

func (m *SecondOrderSDM) Topology() string {
	return "Second-Order Boser-Wooley Cascade SDM"
}

func (m *SecondOrderSDM) Coefficients() string {
	return "b1=1.0, a1=1.0, b2=1.0, a2=1.0"
}

func (m *SecondOrderSDM) PeakState() float64 {
	return m.peakState
}

func (m *SecondOrderSDM) Reset() {
	m.v1 = 0.0
	m.v2 = 0.0
	m.peakState = 0.0
}

func (m *SecondOrderSDM) Step(input float64) float64 {
	// Quantizer: +1.0 if v2 >= 0 else -1.0
	y := 1.0
	if m.v2 < 0 {
		y = -1.0
	}

	// Update integrators
	m.v1 += (input - y)
	m.v2 += (m.v1 - y)

	maxV := math.Max(math.Abs(m.v1), math.Abs(m.v2))
	if maxV > m.peakState {
		m.peakState = maxV
	}

	return y
}
