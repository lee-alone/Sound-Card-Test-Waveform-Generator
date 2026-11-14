package controller

import (
	"errors"
	"fmt"
	"sound-signal-generator/internal/dsp"
)

var (
	// ErrUnknownWaveformType is returned when an unrecognized waveform name is requested.
	ErrUnknownWaveformType = errors.New("unknown waveform type")
)

// GeneratorFactory instantiates and prepares DSP generators according to request.
type GeneratorFactory struct{}

// NewGeneratorFactory creates a new generator factory instance.
func NewGeneratorFactory() *GeneratorFactory {
	return &GeneratorFactory{}
}

// CreateGenerator instantiates and prepares the requested waveform generator.
func (f *GeneratorFactory) CreateGenerator(req ExtendedSignalRequest) (dsp.Generator, error) {
	if req.Amplitude < 0.0 || req.Amplitude > 1.0 {
		return nil, ErrAmplitudeOutOfRange
	}

	baseCfg := dsp.Config{
		SampleRate:   req.SampleRate,
		Frequency:    req.Frequency,
		Amplitude:    req.Amplitude,
		DCOffset:     req.DCOffset,
		InitialPhase: 0.0,
	}

	switch req.Waveform {
	case WaveformSine:
		if err := ValidateFrequency(req.Frequency, req.SampleRate); err != nil {
			return nil, err
		}
		gen := dsp.NewSineGenerator()
		if err := gen.Prepare(baseCfg); err != nil {
			return nil, fmt.Errorf("sine generator prepare: %w", err)
		}
		return gen, nil

	case WaveformPolyBlepSquare:
		if err := ValidateFrequency(req.Frequency, req.SampleRate); err != nil {
			return nil, err
		}
		gen := dsp.NewPolyBLEPSquareGenerator()
		if err := gen.Prepare(baseCfg); err != nil {
			return nil, fmt.Errorf("polyblep square generator prepare: %w", err)
		}
		return gen, nil

	case WaveformPolyBlepTriangle:
		if err := ValidateFrequency(req.Frequency, req.SampleRate); err != nil {
			return nil, err
		}
		gen := dsp.NewPolyBLEPTriangleGenerator()
		if err := gen.Prepare(baseCfg); err != nil {
			return nil, fmt.Errorf("polyblep triangle generator prepare: %w", err)
		}
		return gen, nil

	case WaveformLinearSweep, WaveformLogarithmicSweep:
		mode := dsp.SweepModeLinear
		if req.Waveform == WaveformLogarithmicSweep {
			mode = dsp.SweepModeLogarithmic
		}
		startF := req.Sweep.StartFreq
		endF := req.Sweep.EndFreq
		if startF <= 0 {
			startF = 20.0
		}
		if endF <= 0 {
			endF = 20000.0
		}
		dur := req.Sweep.DurationSec
		if dur <= 0 {
			dur = 5.0
		}
		taper := req.Sweep.TaperFraction
		if taper <= 0 {
			taper = 0.10
		}

		if err := ValidateFrequency(startF, req.SampleRate); err != nil {
			return nil, fmt.Errorf("sweep start frequency: %w", err)
		}
		if err := ValidateFrequency(endF, req.SampleRate); err != nil {
			return nil, fmt.Errorf("sweep end frequency: %w", err)
		}

		gen := dsp.NewSweepGenerator()
		err := gen.PrepareSweep(dsp.SweepConfig{
			SampleRate:    req.SampleRate,
			StartFreq:     startF,
			EndFreq:       endF,
			DurationSec:   dur,
			Amplitude:     req.Amplitude,
			TaperFraction: taper,
			Mode:          mode,
			Loop:          req.Sweep.Loop,
		})
		if err != nil {
			return nil, fmt.Errorf("sweep generator prepare: %w", err)
		}
		return gen, nil

	case WaveformWhiteNoise:
		gen := dsp.NewGaussianWhiteNoiseGenerator()
		seed := req.Noise.Seed
		if seed == 0 {
			seed = 12345
		}
		err := gen.PrepareNoise(dsp.NoiseConfig{
			SampleRate: req.SampleRate,
			Amplitude:  req.Amplitude,
			Seed:       uint64(seed),
			Type:       dsp.NoiseTypeWhite,
		})
		if err != nil {
			return nil, fmt.Errorf("white noise generator prepare: %w", err)
		}
		return gen, nil

	case WaveformPinkNoise:
		gen := dsp.NewVossPinkNoiseGenerator()
		seed := req.Noise.Seed
		if seed == 0 {
			seed = 54321
		}
		err := gen.PrepareNoise(dsp.NoiseConfig{
			SampleRate: req.SampleRate,
			Amplitude:  req.Amplitude,
			Seed:       uint64(seed),
			Type:       dsp.NoiseTypePink,
		})
		if err != nil {
			return nil, fmt.Errorf("pink noise generator prepare: %w", err)
		}
		return gen, nil

	case WaveformMultitoneSMPTE:
		gen := dsp.NewMultitoneGenerator()
		cfg := dsp.NewSMPTEConfig(req.SampleRate, req.Amplitude, req.DCOffset)
		if err := gen.PrepareMultitone(cfg); err != nil {
			return nil, fmt.Errorf("smpte multitone prepare: %w", err)
		}
		return gen, nil

	case WaveformMultitoneCCIF:
		gen := dsp.NewMultitoneGenerator()
		cfg := dsp.NewCCIFConfig(req.SampleRate, req.Amplitude, req.DCOffset)
		if err := gen.PrepareMultitone(cfg); err != nil {
			return nil, fmt.Errorf("ccif multitone prepare: %w", err)
		}
		return gen, nil

	case WaveformMultitoneCustom:
		gen := dsp.NewMultitoneGenerator()
		numTones := len(req.Multitone.Frequencies)
		if numTones == 0 {
			return nil, errors.New("custom multitone requires at least 1 frequency tone")
		}
		comps := make([]dsp.ToneComponent, numTones)
		for i := 0; i < numTones; i++ {
			f := req.Multitone.Frequencies[i]
			if err := ValidateFrequency(f, req.SampleRate); err != nil {
				return nil, fmt.Errorf("custom multitone tone %d (%.1f Hz): %w", i, f, err)
			}
			amp := 1.0
			if i < len(req.Multitone.Amplitudes) {
				amp = req.Multitone.Amplitudes[i]
			}
			var phase float64
			if i < len(req.Multitone.Phases) {
				phase = req.Multitone.Phases[i]
			}
			comps[i] = dsp.ToneComponent{
				Frequency:    f,
				Amplitude:    amp,
				InitialPhase: phase,
			}
		}
		cfg := dsp.MultitoneConfig{
			SampleRate:   req.SampleRate,
			TargetPeak:   req.Amplitude,
			DCOffset:     req.DCOffset,
			Components:   comps,
			AutoNormPAPR: true,
			Preset:       dsp.MultitonePresetCustom,
		}
		if err := gen.PrepareMultitone(cfg); err != nil {
			return nil, fmt.Errorf("custom multitone prepare: %w", err)
		}
		return gen, nil

	default:
		return nil, fmt.Errorf("%w: %s", ErrUnknownWaveformType, req.Waveform)
	}
}
