package dsp

import (
	"math"
)

// PolyBLEPSquareGenerator generates band-limited square waves using the PolyBLEP algorithm
// combined with 2x oversampled decimation filtering to suppress alias foldback below -60 dBc.
// Operates with zero memory allocations during Render.
type PolyBLEPSquareGenerator struct {
	sampleRate   float64
	frequency    float64
	amplitude    float64
	initialPhase float64 // in [0, 1)
	dcOffset     float64

	phase     float64 // running normalized phase in [0, 1)
	phaseStep float64 // phase increment per 2x sub-sample: (f / (2 * Fs))

	decimator HalfbandDecimator47
}

// NewPolyBLEPSquareGenerator creates a new uninitialized PolyBLEPSquareGenerator instance.
func NewPolyBLEPSquareGenerator() *PolyBLEPSquareGenerator {
	return &PolyBLEPSquareGenerator{}
}

// Prepare configures generator parameters on a non-real-time thread.
func (g *PolyBLEPSquareGenerator) Prepare(cfg Config) error {
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
	g.dcOffset = cfg.DCOffset

	// Normalize initial phase to [0, 1)
	normInit := math.Mod(cfg.InitialPhase/twoPi, 1.0)
	if normInit < 0 {
		normInit += 1.0
	}
	g.initialPhase = normInit

	// 2x oversampling phase increment per sub-step
	g.phaseStep = g.frequency / (2.0 * g.sampleRate)
	g.phase = g.initialPhase

	g.decimator.Reset()
	g.warmUpDecimator()

	return nil
}

// warmUpDecimator primes the halfband decimator filter state.
func (g *PolyBLEPSquareGenerator) warmUpDecimator() {
	dt := g.phaseStep
	p := g.phase

	for i := 0; i < 96; i++ {
		s := g.computeSubSample(p, dt)
		g.decimator.PushSample(s)
		p += dt
		if p >= 1.0 {
			p -= 1.0
		}
	}
	g.phase = p
}

// Reset resets the phase accumulator and decimator according to policy.
func (g *PolyBLEPSquareGenerator) Reset(policy ResetPolicy) {
	if policy == ResetToZero {
		g.phase = g.initialPhase
		g.decimator.Reset()
		g.warmUpDecimator()
	}
	// For ResetKeepPhase, running phase and decimator state are preserved
}

// computeSubSample computes a single PolyBLEP-corrected 2x oversampled sample.
func (g *PolyBLEPSquareGenerator) computeSubSample(p, dt float64) float64 {
	// Transition 0: rising edge at p = 0 (jump from -1 to +1: +2)
	d0 := p
	if d0 >= 0.5 {
		d0 -= 1.0
	}
	u0 := d0 / dt
	b0 := PolyBLEP4(u0)

	// Transition 1: falling edge at p = 0.5 (jump from +1 to -1: -2)
	d1 := p - 0.5
	if d1 < -0.5 {
		d1 += 1.0
	} else if d1 >= 0.5 {
		d1 -= 1.0
	}
	u1 := d1 / dt
	b1 := PolyBLEP4(u1)

	// Naive bipolar square wave (-1.0 or +1.0)
	naive := 1.0
	if p >= 0.5 {
		naive = -1.0
	}

	return naive + 2.0*b0 - 2.0*b1
}

// Render populates dst with PolyBLEP square wave samples.
// Executes in the real-time thread hot path and performs zero memory allocations.
func (g *PolyBLEPSquareGenerator) Render(dst []float64) {
	amp := g.amplitude
	dc := g.dcOffset
	dt := g.phaseStep
	p := g.phase

	for i := 0; i < len(dst); i++ {
		// Sub-step 1: compute first oversampled sample and push into decimator
		s1 := g.computeSubSample(p, dt)
		g.decimator.PushSample(s1)
		p += dt
		if p >= 1.0 {
			p -= 1.0
		}

		// Sub-step 2: compute second oversampled sample, push, and decimate
		s2 := g.computeSubSample(p, dt)
		g.decimator.PushSample(s2)
		p += dt
		if p >= 1.0 {
			p -= 1.0
		}

		filtered := g.decimator.FilterCurrent()
		dst[i] = amp*filtered + dc
	}

	g.phase = p
}

// MaxPeak returns the theoretical maximum peak amplitude, including Gibbs overshoot (~1.18x).
func (g *PolyBLEPSquareGenerator) MaxPeak() float64 {
	return 1.18*g.amplitude + math.Abs(g.dcOffset)
}

// Frequency returns configured nominal frequency.
func (g *PolyBLEPSquareGenerator) Frequency() float64 {
	return g.frequency
}

// SampleRate returns configured sample rate.
func (g *PolyBLEPSquareGenerator) SampleRate() float64 {
	return g.sampleRate
}
