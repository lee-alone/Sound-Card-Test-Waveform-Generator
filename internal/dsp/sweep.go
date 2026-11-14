package dsp

import (
	"errors"
	"math"
)

var (
	// ErrInvalidDuration is returned when sweep duration is non-positive.
	ErrInvalidDuration = errors.New("sweep duration must be positive")

	// ErrInvalidTaperFraction is returned when taper fraction is not in (0, 0.5).
	ErrInvalidTaperFraction = errors.New("taper fraction must be between 0.0 and 0.5")
)

// SweepMode defines the frequency sweep curvature.
type SweepMode int

const (
	// SweepModeLinear increases frequency linearly over time.
	SweepModeLinear SweepMode = iota

	// SweepModeLogarithmic increases frequency exponentially over time.
	SweepModeLogarithmic
)

// SweepConfig defines detailed parameters for linear or logarithmic sweep generators.
type SweepConfig struct {
	SampleRate    int       `json:"sample_rate"`
	StartFreq     float64   `json:"start_freq"`
	EndFreq       float64   `json:"end_freq"`
	DurationSec   float64   `json:"duration_sec"`
	Amplitude     float64   `json:"amplitude"`
	DCOffset      float64   `json:"dc_offset"`
	InitialPhase  float64   `json:"initial_phase"`  // in radians
	TaperFraction float64   `json:"taper_fraction"` // fraction for fade-in and fade-out (default 0.10)
	Mode          SweepMode `json:"mode"`
	Loop          bool      `json:"loop"` // whether to loop continuously
}

// SweepGenerator generates chirp signals with linear or logarithmic frequency progression
// and built-in Hann window tapering at endpoints to ensure smooth transitions without clicks.
// Operates with zero memory allocations during Render.
type SweepGenerator struct {
	sampleRate   float64
	startFreq    float64
	endFreq      float64
	durationSec  float64
	totalSamples int64
	amplitude    float64
	dcOffset     float64
	initialPhase float64
	taperSamples int64
	mode         SweepMode
	loop         bool

	// Analytical pre-calculated factors
	linearRate float64 // (f1 - f0) / T for linear
	logBeta    float64 // ln(f1 / f0) / T for log

	// Playback state
	currentSample int64
}

// NewSweepGenerator creates an uninitialized SweepGenerator instance.
func NewSweepGenerator() *SweepGenerator {
	return &SweepGenerator{
		loop: true,
	}
}

// Prepare implements the general Generator interface using default sweep ranges.
func (g *SweepGenerator) Prepare(cfg Config) error {
	endFreq := cfg.Frequency * 10.0
	if endFreq > float64(cfg.SampleRate)*0.45 {
		endFreq = float64(cfg.SampleRate) * 0.45
	}
	if endFreq <= cfg.Frequency {
		endFreq = cfg.Frequency * 2.0
	}

	scfg := SweepConfig{
		SampleRate:    cfg.SampleRate,
		StartFreq:     cfg.Frequency,
		EndFreq:       endFreq,
		DurationSec:   1.0,
		Amplitude:     cfg.Amplitude,
		DCOffset:      cfg.DCOffset,
		InitialPhase:  cfg.InitialPhase,
		TaperFraction: 0.10,
		Mode:          SweepModeLogarithmic,
		Loop:          true,
	}
	return g.PrepareSweep(scfg)
}

// PrepareSweep fully configures the sweep generator.
func (g *SweepGenerator) PrepareSweep(cfg SweepConfig) error {
	if cfg.SampleRate <= 0 {
		return ErrInvalidSampleRate
	}
	if cfg.StartFreq <= 0 || cfg.EndFreq <= 0 {
		return ErrInvalidFrequency
	}
	if cfg.DurationSec <= 0 {
		return ErrInvalidDuration
	}
	if cfg.Amplitude < 0 {
		return ErrNegativeAmplitude
	}

	taperFrac := cfg.TaperFraction
	if taperFrac <= 0 {
		taperFrac = 0.10 // default 10%
	}
	if taperFrac >= 0.5 {
		return ErrInvalidTaperFraction
	}

	g.sampleRate = float64(cfg.SampleRate)
	g.startFreq = cfg.StartFreq
	g.endFreq = cfg.EndFreq
	g.durationSec = cfg.DurationSec
	g.amplitude = cfg.Amplitude
	g.dcOffset = cfg.DCOffset
	g.initialPhase = math.Mod(cfg.InitialPhase, twoPi)
	if g.initialPhase < 0 {
		g.initialPhase += twoPi
	}
	g.mode = cfg.Mode
	g.loop = cfg.Loop

	g.totalSamples = int64(math.Round(g.durationSec * g.sampleRate))
	if g.totalSamples < 10 {
		g.totalSamples = 10
	}

	g.taperSamples = int64(math.Round(taperFrac * float64(g.totalSamples)))
	if g.taperSamples < 1 {
		g.taperSamples = 1
	}

	T := g.durationSec
	if g.mode == SweepModeLinear {
		g.linearRate = (g.endFreq - g.startFreq) / T
	} else {
		g.logBeta = math.Log(g.endFreq/g.startFreq) / T
	}

	g.currentSample = 0
	return nil
}

// Reset resets playback state according to policy.
func (g *SweepGenerator) Reset(policy ResetPolicy) {
	if policy == ResetToZero {
		g.currentSample = 0
	}
}

// InstantaneousFrequency calculates theoretical instantaneous frequency at time t (seconds).
func (g *SweepGenerator) InstantaneousFrequency(t float64) float64 {
	if t < 0 {
		t = 0
	}
	if t > g.durationSec {
		t = g.durationSec
	}

	if g.mode == SweepModeLinear {
		return g.startFreq + g.linearRate*t
	}
	return g.startFreq * math.Exp(g.logBeta*t)
}

// calculatePhase computes analytical phase in radians at time t.
func (g *SweepGenerator) calculatePhase(t float64) float64 {
	if g.mode == SweepModeLinear {
		// phi(t) = 2*pi * (f0*t + 0.5*k*t^2) + phi0
		return twoPi*(g.startFreq*t+0.5*g.linearRate*t*t) + g.initialPhase
	}
	// phi(t) = 2*pi * f0 * (exp(beta*t) - 1) / beta + phi0
	return twoPi*g.startFreq*(math.Exp(g.logBeta*t)-1.0)/g.logBeta + g.initialPhase
}

// computeEnvelope computes the Hann tapering window factor [0.0, 1.0] at sample index n.
func (g *SweepGenerator) computeEnvelope(n int64) float64 {
	taper := g.taperSamples
	total := g.totalSamples

	if n < taper {
		// Fade-in: 0.5 * (1 - cos(pi * n / taper))
		theta := math.Pi * float64(n) / float64(taper)
		return 0.5 * (1.0 - math.Cos(theta))
	}

	fadeStart := total - taper
	if n >= fadeStart {
		// Fade-out: 0.5 * (1 + cos(pi * (n - fadeStart) / taper))
		theta := math.Pi * float64(n-fadeStart) / float64(taper)
		return 0.5 * (1.0 + math.Cos(theta))
	}

	return 1.0
}

// Render populates dst with sweep samples.
// Executes in the real-time thread hot path and performs zero memory allocations.
func (g *SweepGenerator) Render(dst []float64) {
	amp := g.amplitude
	dc := g.dcOffset
	fs := g.sampleRate
	total := g.totalSamples
	loop := g.loop
	n := g.currentSample

	for i := 0; i < len(dst); i++ {
		if n >= total {
			if loop {
				n = 0
			} else {
				// Single shot completed, output DC offset
				dst[i] = dc
				continue
			}
		}

		t := float64(n) / fs
		phi := g.calculatePhase(t)
		env := g.computeEnvelope(n)

		dst[i] = amp*env*math.Sin(phi) + dc
		n++
	}

	g.currentSample = n
}

// MaxPeak returns the theoretical maximum peak amplitude.
// Hann window envelope <= 1.0, so peak is bounded by amplitude + |dc_offset|.
func (g *SweepGenerator) MaxPeak() float64 {
	return g.amplitude + math.Abs(g.dcOffset)
}

// Mode returns configured sweep mode.
func (g *SweepGenerator) Mode() SweepMode {
	return g.mode
}

// StartFreq returns configured starting frequency.
func (g *SweepGenerator) StartFreq() float64 {
	return g.startFreq
}

// EndFreq returns configured ending frequency.
func (g *SweepGenerator) EndFreq() float64 {
	return g.endFreq
}

// DurationSec returns sweep duration in seconds.
func (g *SweepGenerator) DurationSec() float64 {
	return g.durationSec
}
