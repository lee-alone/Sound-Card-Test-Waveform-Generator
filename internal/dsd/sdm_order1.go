package dsd

import "math"

// FirstOrderSDM implements a single-loop 1st-order Delta-Sigma modulator.
// Integrator update: v[n+1] = v[n] + (x[n] - y[n])
// Quantizer:         y[n] = sgn(v[n]) (outputs +1.0 or -1.0)
// Theoretical NTF:   1 - z^-1 (slope: +6 dB/octave)
type FirstOrderSDM struct {
	v         float64 // single integrator state
	peakState float64
}

// NewFirstOrderSDM creates a 1st-order modulator.
func NewFirstOrderSDM() *FirstOrderSDM {
	return &FirstOrderSDM{}
}

func (m *FirstOrderSDM) Order() int {
	return 1
}

func (m *FirstOrderSDM) Topology() string {
	return "First-Order Single-Loop Error-Feedback SDM"
}

func (m *FirstOrderSDM) Coefficients() string {
	return "b1=1.0, a1=1.0"
}

func (m *FirstOrderSDM) PeakState() float64 {
	return m.peakState
}

func (m *FirstOrderSDM) Reset() {
	m.v = 0.0
	m.peakState = 0.0
}

func (m *FirstOrderSDM) Step(input float64) float64 {
	// Quantizer: +1.0 if v >= 0 else -1.0
	y := 1.0
	if m.v < 0 {
		y = -1.0
	}

	// Update integrator: v = v + (input - y)
	err := input - y
	m.v += err

	absV := math.Abs(m.v)
	if absV > m.peakState {
		m.peakState = absV
	}

	return y
}
