package dsp

import (
	"math"
)

const twoPi = 2.0 * math.Pi

// SineGenerator implements Generator for high-purity sine waves.
type SineGenerator struct {
	sampleRate   float64
	frequency    float64
	amplitude    float64
	initialPhase float64
	dcOffset     float64

	phase     float64
	phaseStep float64
}

// NewSineGenerator creates an uninitialized SineGenerator instance.
func NewSineGenerator() *SineGenerator {
	return &SineGenerator{}
}

// Prepare configures the generator parameters on a non-real-time thread.
func (g *SineGenerator) Prepare(cfg Config) error {
	if cfg.SampleRate <= 0 {
		return ErrInvalidSampleRate
	}
	if cfg.Frequency <= 0 {
		return ErrInvalidFrequency
	}
	if cfg.Amplitude < 0 {
		return ErrNegativeAmplitude
	}

	g.sampleRate = float64(cfg.SampleRate)
	g.frequency = cfg.Frequency
	g.amplitude = cfg.Amplitude
	g.initialPhase = math.Mod(cfg.InitialPhase, twoPi)
	if g.initialPhase < 0 {
		g.initialPhase += twoPi
	}
	g.dcOffset = cfg.DCOffset

	// Analytical phase step calculation
	g.phaseStep = (twoPi * g.frequency) / g.sampleRate
	g.phase = g.initialPhase

	return nil
}

// Reset resets the phase accumulator according to the policy.
func (g *SineGenerator) Reset(policy ResetPolicy) {
	if policy == ResetToZero {
		g.phase = g.initialPhase
	}
	// For ResetKeepPhase, g.phase is kept as is
}

// Render populates dst with pure sine wave samples.
// This executes in the real-time thread hot path and performs zero memory allocations.
func (g *SineGenerator) Render(dst []float64) {
	amp := g.amplitude
	dc := g.dcOffset
	step := g.phaseStep
	p := g.phase

	for i := 0; i < len(dst); i++ {
		dst[i] = amp*math.Sin(p) + dc
		p += step
		if p >= twoPi {
			p -= twoPi
			// Handle large steps safely
			if p >= twoPi {
				p = math.Mod(p, twoPi)
			}
		}
	}

	g.phase = p
}

// MaxPeak returns the theoretical maximum peak amplitude of the configured signal.
func (g *SineGenerator) MaxPeak() float64 {
	return g.amplitude + math.Abs(g.dcOffset)
}

// PhaseStep returns the analytical phase step in radians per sample.
func (g *SineGenerator) PhaseStep() float64 {
	return g.phaseStep
}

// Frequency returns the configured nominal frequency.
func (g *SineGenerator) Frequency() float64 {
	return g.frequency
}

// SampleRate returns the configured sample rate.
func (g *SineGenerator) SampleRate() float64 {
	return g.sampleRate
}
