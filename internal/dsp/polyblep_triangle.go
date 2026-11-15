package dsp

import (
	"math"
)

// PolyBLEPTriangleGenerator generates band-limited triangle waves using PolyBLAMP (integrated PolyBLEP)
// corner smoothing combined with 2x oversampled decimation filtering.
// Operates with zero memory allocations during Render.
type PolyBLEPTriangleGenerator struct {
	sampleRate   float64
	frequency    float64
	amplitude    float64
	initialPhase float64 // in [0, 1)
	dcOffset     float64

	phase     float64 // running normalized phase in [0, 1)
	phaseStep float64 // phase increment per 2x sub-sample: (f / (2 * Fs))
	invStep   float64 // precomputed 1 / phaseStep for the BLEP hot path

	decimator HalfbandDecimator47
}

// NewPolyBLEPTriangleGenerator creates an uninitialized PolyBLEPTriangleGenerator instance.
func NewPolyBLEPTriangleGenerator() *PolyBLEPTriangleGenerator {
	return &PolyBLEPTriangleGenerator{}
}

// Prepare configures generator parameters on a non-real-time thread.
func (g *PolyBLEPTriangleGenerator) Prepare(cfg Config) error {
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
	g.invStep = 1.0 / g.phaseStep
	g.phase = g.initialPhase

	g.decimator.Reset()
	g.warmUpDecimator()

	return nil
}

// warmUpDecimator primes the halfband decimator filter state.
func (g *PolyBLEPTriangleGenerator) warmUpDecimator() {
	dt := g.phaseStep
	invDt := g.invStep
	p := g.phase

	for i := 0; i < 96; i++ {
		s := g.computeSubSample(p, dt, invDt)
		g.decimator.PushSample(s)
		p += dt
		if p >= 1.0 {
			p -= 1.0
		}
	}
	g.phase = p
}

// Reset resets the phase accumulator and decimator according to policy.
func (g *PolyBLEPTriangleGenerator) Reset(policy ResetPolicy) {
	if policy == ResetToZero {
		g.phase = g.initialPhase
		g.decimator.Reset()
		g.warmUpDecimator()
	}
	// For ResetKeepPhase, running phase and decimator state are preserved
}

// computeSubSample computes a single PolyBLAMP-smoothed 2x oversampled sample.
func (g *PolyBLEPTriangleGenerator) computeSubSample(p, dt, invDt float64) float64 {
	// Naive bipolar triangle wave in [-1.0, +1.0]
	naive := 4.0*p - 1.0
	if p >= 0.5 {
		naive = 3.0 - 4.0*p
	}

	// Corner 0: trough at p = 0 (slope changes from -4 to +4: Δs = +8)
	d0 := p
	if d0 >= 0.5 {
		d0 -= 1.0
	}
	u0 := d0 * invDt
	b0 := PolyBLAMP4Continuous(u0)

	// Corner 1: peak at p = 0.5 (slope changes from +4 to -4: Δs = -8)
	d1 := p - 0.5
	if d1 < -0.5 {
		d1 += 1.0
	} else if d1 >= 0.5 {
		d1 -= 1.0
	}
	u1 := d1 * invDt
	b1 := PolyBLAMP4Continuous(u1)

	// Corner correction = Δs * dt * BLAMP(u)
	corr := 8.0*dt*b0 - 8.0*dt*b1

	return naive + corr
}

// Render populates dst with band-limited triangle wave samples.
// Executes in the real-time thread hot path and performs zero memory allocations.
func (g *PolyBLEPTriangleGenerator) Render(dst []float64) {
	amp := g.amplitude
	dc := g.dcOffset
	dt := g.phaseStep
	invDt := g.invStep
	p := g.phase

	for i := 0; i < len(dst); i++ {
		// Sub-step 1: compute first oversampled sample and push into decimator
		s1 := g.computeSubSample(p, dt, invDt)
		g.decimator.PushSample(s1)
		p += dt
		if p >= 1.0 {
			p -= 1.0
		}

		// Sub-step 2: compute second oversampled sample, push, and decimate
		s2 := g.computeSubSample(p, dt, invDt)
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

// MaxPeak returns the theoretical maximum peak amplitude.
// Triangle wave corners round inward, so peak does not exceed amplitude + |dc_offset|.
func (g *PolyBLEPTriangleGenerator) MaxPeak() float64 {
	return g.amplitude + math.Abs(g.dcOffset)
}

// Frequency returns configured nominal frequency.
func (g *PolyBLEPTriangleGenerator) Frequency() float64 {
	return g.frequency
}

// SampleRate returns configured sample rate.
func (g *PolyBLEPTriangleGenerator) SampleRate() float64 {
	return g.sampleRate
}
