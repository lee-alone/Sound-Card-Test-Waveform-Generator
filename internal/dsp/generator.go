package dsp

import "errors"

var (
	// ErrInvalidSampleRate is returned when sample rate is zero or negative.
	ErrInvalidSampleRate = errors.New("invalid sample rate: must be positive")

	// ErrInvalidFrequency is returned when frequency is zero or negative.
	ErrInvalidFrequency = errors.New("invalid frequency: must be positive")

	// ErrNegativeAmplitude is returned when amplitude is negative.
	ErrNegativeAmplitude = errors.New("amplitude cannot be negative")
)

// ResetPolicy determines how the generator's internal state resets.
type ResetPolicy int

const (
	// ResetKeepPhase retains current running phase across reconfigurations for continuous output.
	ResetKeepPhase ResetPolicy = iota

	// ResetToZero resets the running phase back to initial phase.
	ResetToZero
)

// Config specifies general DSP generator parameters.
type Config struct {
	SampleRate   int     `json:"sample_rate"`
	Frequency    float64 `json:"frequency"`
	Amplitude    float64 `json:"amplitude"`     // Linear peak [0.0, 1.0]
	InitialPhase float64 `json:"initial_phase"` // In radians [0, 2*pi)
	DCOffset     float64 `json:"dc_offset"`
}

// Generator is the unified interface for all signal waveform generators.
type Generator interface {
	Prepare(cfg Config) error
	Reset(policy ResetPolicy)
	Render(dst []float64)
	MaxPeak() float64
}

// MultiChannelGenerator is an interface for generators that can render all audio
// channels simultaneously in a single pass to ensure phase coherence.
type MultiChannelGenerator interface {
	Generator
	RenderChannels(dst [][]float64)
}
