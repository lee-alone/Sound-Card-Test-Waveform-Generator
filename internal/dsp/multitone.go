package dsp

import (
	"errors"
	"math"
)

var (
	// ErrNoToneComponents is returned when no tone components are specified.
	ErrNoToneComponents = errors.New("multitone signal must have at least one frequency component")

	// ErrToneFrequencyOutOfRange is returned when a tone frequency is non-positive or exceeds Nyquist limit.
	ErrToneFrequencyOutOfRange = errors.New("tone frequency must be positive and below safe Nyquist limit")
)

// MultitonePreset identifies standard intermodulation test signal configurations.
type MultitonePreset int

const (
	// MultitonePresetCustom indicates user-defined tone components.
	MultitonePresetCustom MultitonePreset = iota

	// MultitonePresetSMPTE standard intermodulation test: 60Hz and 7kHz at 4:1 amplitude ratio.
	MultitonePresetSMPTE

	// MultitonePresetCCIF standard intermodulation test: 19kHz and 20kHz at 1:1 amplitude ratio.
	MultitonePresetCCIF

	// MultitonePresetDIN standard intermodulation test: 250Hz and 8kHz at 4:1 amplitude ratio.
	MultitonePresetDIN
)

// ToneComponent specifies an individual sine tone component.
type ToneComponent struct {
	Frequency    float64 `json:"frequency"`
	Amplitude    float64 `json:"amplitude"`     // relative or linear peak amplitude
	InitialPhase float64 `json:"initial_phase"` // in radians
}

// MultitoneConfig defines configuration parameters for a multitone signal generator.
type MultitoneConfig struct {
	SampleRate   int             `json:"sample_rate"`
	TargetPeak   float64         `json:"target_peak"` // maximum peak amplitude allowed (e.g. 1.0 for 0 dBFS)
	DCOffset     float64         `json:"dc_offset"`
	Components   []ToneComponent `json:"components"`
	AutoNormPAPR bool            `json:"auto_norm_papr"` // whether to automatically normalize peak to TargetPeak
	Preset       MultitonePreset `json:"preset"`
}

// toneState maintains running phase state for an individual tone component with zero heap allocations.
type toneState struct {
	amplitude float64
	phase     float64
	phaseStep float64
	initPhase float64
}

// MultitoneGenerator generates two-tone (SMPTE/CCIF/DIN) and multi-tone signals with
// analytical Crest Factor / PAPR calculation, automatic peak normalization to strictly prevent
// clipping, and zero heap allocations during Render.
type MultitoneGenerator struct {
	sampleRate float64
	targetPeak float64
	dcOffset   float64
	preset     MultitonePreset

	components []ToneComponent
	tones      []toneState

	scaleFactor     float64
	theoreticalPeak float64
	rmsPower        float64
	crestFactor     float64
	paprDB          float64
}

// NewMultitoneGenerator creates an uninitialized MultitoneGenerator instance.
func NewMultitoneGenerator() *MultitoneGenerator {
	return &MultitoneGenerator{}
}

// NewSMPTEConfig returns a standard SMPTE intermodulation configuration (60Hz + 7kHz, 4:1).
func NewSMPTEConfig(sampleRate int, targetPeak, dcOffset float64) MultitoneConfig {
	return MultitoneConfig{
		SampleRate: sampleRate,
		TargetPeak: targetPeak,
		DCOffset:   dcOffset,
		Preset:     MultitonePresetSMPTE,
		AutoNormPAPR: true,
		Components: []ToneComponent{
			{Frequency: 60.0, Amplitude: 0.8, InitialPhase: 0.0},
			{Frequency: 7000.0, Amplitude: 0.2, InitialPhase: 0.0},
		},
	}
}

// NewCCIFConfig returns a standard CCIF intermodulation configuration (19kHz + 20kHz, 1:1).
func NewCCIFConfig(sampleRate int, targetPeak, dcOffset float64) MultitoneConfig {
	return MultitoneConfig{
		SampleRate: sampleRate,
		TargetPeak: targetPeak,
		DCOffset:   dcOffset,
		Preset:     MultitonePresetCCIF,
		AutoNormPAPR: true,
		Components: []ToneComponent{
			{Frequency: 19000.0, Amplitude: 0.5, InitialPhase: 0.0},
			{Frequency: 20000.0, Amplitude: 0.5, InitialPhase: 0.0},
		},
	}
}

// NewDINConfig returns a standard DIN intermodulation configuration (250Hz + 8kHz, 4:1).
func NewDINConfig(sampleRate int, targetPeak, dcOffset float64) MultitoneConfig {
	return MultitoneConfig{
		SampleRate: sampleRate,
		TargetPeak: targetPeak,
		DCOffset:   dcOffset,
		Preset:     MultitonePresetDIN,
		AutoNormPAPR: true,
		Components: []ToneComponent{
			{Frequency: 250.0, Amplitude: 0.8, InitialPhase: 0.0},
			{Frequency: 8000.0, Amplitude: 0.2, InitialPhase: 0.0},
		},
	}
}

// Prepare implements general Generator interface using SMPTE preset at given amplitude.
func (g *MultitoneGenerator) Prepare(cfg Config) error {
	mcfg := NewSMPTEConfig(cfg.SampleRate, cfg.Amplitude, cfg.DCOffset)
	return g.PrepareMultitone(mcfg)
}

// PrepareMultitone configures the multitone generator, calculates Crest Factor and PAPR,
// and applies automatic peak normalization.
func (g *MultitoneGenerator) PrepareMultitone(cfg MultitoneConfig) error {
	if cfg.SampleRate <= 0 {
		return ErrInvalidSampleRate
	}
	if len(cfg.Components) == 0 {
		return ErrNoToneComponents
	}
	if cfg.TargetPeak < 0 {
		return ErrNegativeAmplitude
	}

	maxAllowedFreq := float64(cfg.SampleRate) * 0.45
	sumAmps := 0.0
	sumSquared := 0.0

	for _, c := range cfg.Components {
		if c.Frequency <= 0 || c.Frequency > maxAllowedFreq {
			return ErrToneFrequencyOutOfRange
		}
		if c.Amplitude < 0 {
			return ErrNegativeAmplitude
		}
		sumAmps += c.Amplitude
		sumSquared += c.Amplitude * c.Amplitude
	}

	if sumAmps <= 0 {
		return errors.New("total component amplitude must be positive")
	}

	g.sampleRate = float64(cfg.SampleRate)
	g.targetPeak = cfg.TargetPeak
	g.dcOffset = cfg.DCOffset
	g.preset = cfg.Preset

	// Calculate scale factor to guarantee theoretical max peak <= targetPeak.
	// AC headroom is TargetPeak - |dc_offset|.
	availableAC := cfg.TargetPeak - math.Abs(cfg.DCOffset)
	if availableAC < 0 {
		availableAC = 0
	}

	scale := 1.0
	if cfg.AutoNormPAPR || (sumAmps+math.Abs(cfg.DCOffset)) > cfg.TargetPeak {
		if sumAmps > 0 {
			scale = availableAC / sumAmps
		}
	}
	g.scaleFactor = scale

	// Pre-allocate tone states
	g.components = make([]ToneComponent, len(cfg.Components))
	copy(g.components, cfg.Components)

	g.tones = make([]toneState, len(cfg.Components))
	scaledSumAmps := 0.0
	scaledSumSquared := 0.0

	for i, c := range cfg.Components {
		scaledAmp := c.Amplitude * scale
		normInit := math.Mod(c.InitialPhase, twoPi)
		if normInit < 0 {
			normInit += twoPi
		}

		g.tones[i] = toneState{
			amplitude: scaledAmp,
			phase:     normInit,
			phaseStep: (twoPi * c.Frequency) / g.sampleRate,
			initPhase: normInit,
		}

		scaledSumAmps += scaledAmp
		scaledSumSquared += scaledAmp * scaledAmp
	}

	// Analytical statistics
	g.theoreticalPeak = scaledSumAmps + math.Abs(g.dcOffset)
	pAvg := scaledSumSquared / 2.0
	g.rmsPower = math.Sqrt(pAvg + g.dcOffset*g.dcOffset)

	if g.rmsPower > 0 {
		g.crestFactor = g.theoreticalPeak / g.rmsPower
		g.paprDB = 20.0 * math.Log10(g.crestFactor)
	} else {
		g.crestFactor = 1.0
		g.paprDB = 0.0
	}

	return nil
}

// Reset resets the phase accumulators of all tones according to policy.
func (g *MultitoneGenerator) Reset(policy ResetPolicy) {
	if policy == ResetToZero {
		for i := range g.tones {
			g.tones[i].phase = g.tones[i].initPhase
		}
	}
}

// Render populates dst with multitone samples.
// Executes in the real-time thread hot path and performs zero memory allocations.
func (g *MultitoneGenerator) Render(dst []float64) {
	dc := g.dcOffset
	nTones := len(g.tones)

	// Fill with DC offset initially
	for i := 0; i < len(dst); i++ {
		dst[i] = dc
	}

	// Accumulate each tone component across all destination samples
	for t := 0; t < nTones; t++ {
		tone := &g.tones[t]
		amp := tone.amplitude
		step := tone.phaseStep
		p := tone.phase

		for i := 0; i < len(dst); i++ {
			dst[i] += amp * math.Sin(p)
			p += step
			if p >= twoPi {
				p -= twoPi
				if p >= twoPi {
					p = math.Mod(p, twoPi)
				}
			}
		}

		tone.phase = p
	}
}

// MaxPeak returns the theoretical peak amplitude (scaled sum of amplitudes + |dc_offset|).
func (g *MultitoneGenerator) MaxPeak() float64 {
	return g.theoreticalPeak
}

// ScaleFactor returns the applied digital scaling factor for peak normalization.
func (g *MultitoneGenerator) ScaleFactor() float64 {
	return g.scaleFactor
}

// TheoreticalPeak returns the maximum theoretical peak amplitude.
func (g *MultitoneGenerator) TheoreticalPeak() float64 {
	return g.theoreticalPeak
}

// RMSPower returns theoretical RMS level of the scaled signal.
func (g *MultitoneGenerator) RMSPower() float64 {
	return g.rmsPower
}

// CrestFactor returns the theoretical Crest Factor (Peak / RMS).
func (g *MultitoneGenerator) CrestFactor() float64 {
	return g.crestFactor
}

// PAPR_dB returns the theoretical Peak-to-Average Power Ratio in decibels.
func (g *MultitoneGenerator) PAPR_dB() float64 {
	return g.paprDB
}

// Preset returns active preset type.
func (g *MultitoneGenerator) Preset() MultitonePreset {
	return g.preset
}

// Components returns a copy of configured tone components.
func (g *MultitoneGenerator) Components() []ToneComponent {
	res := make([]ToneComponent, len(g.components))
	copy(res, g.components)
	return res
}
