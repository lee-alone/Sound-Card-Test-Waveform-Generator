package controller

import (
	"sound-signal-generator/internal/audio"
	"sound-signal-generator/internal/dsd"
)

// WaveformType identifies the DSP signal generator algorithm.
type WaveformType string

const (
	WaveformSine             WaveformType = "sine"
	WaveformPolyBlepSquare   WaveformType = "square"
	WaveformPolyBlepTriangle WaveformType = "triangle"
	WaveformLinearSweep      WaveformType = "linear_sweep"
	WaveformLogarithmicSweep WaveformType = "log_sweep"
	WaveformWhiteNoise       WaveformType = "white_noise"
	WaveformPinkNoise        WaveformType = "pink_noise"
	WaveformMultitoneSMPTE   WaveformType = "multitone_smpte"
	WaveformMultitoneCCIF    WaveformType = "multitone_ccif"
	WaveformMultitoneCustom  WaveformType = "multitone_custom"
	WaveformDoP              WaveformType = "dop"
)

// SweepParams holds parameters for linear or logarithmic sweep generators.
type SweepParams struct {
	StartFreq     float64 `json:"start_freq"`
	EndFreq       float64 `json:"end_freq"`
	DurationSec   float64 `json:"duration_sec"`
	TaperFraction float64 `json:"taper_fraction"`
	Loop          bool    `json:"loop"`
}

// NoiseParams holds parameters for Gaussian white or Voss pink noise generators.
type NoiseParams struct {
	Seed int64 `json:"seed"`
}

// MultitoneParams holds custom multitone frequency and amplitude vectors.
type MultitoneParams struct {
	Frequencies []float64 `json:"frequencies"`
	Amplitudes  []float64 `json:"amplitudes"`
	Phases      []float64 `json:"phases,omitempty"`
}

// DoPParams holds DoP parameters.
type DoPParams struct {
	DSDFormat dsd.DSDFormat `json:"dsd_format"` // DSD64 or DSD128
	BitOrder  dsd.BitOrder  `json:"bit_order"`
}

// ExtendedSignalRequest defines the full parameter payload from GUI to Controller.
type ExtendedSignalRequest struct {
	DeviceID        string             `json:"device_id"`
	SampleRate      int                `json:"sample_rate"`
	SampleFormat    audio.SampleFormat `json:"sample_format"`
	Channels        int                `json:"channels"`
	Waveform        WaveformType       `json:"waveform"`
	Frequency       float64            `json:"frequency"`
	Amplitude       float64            `json:"amplitude"` // Master linear amplitude [0.0, 1.0]
	DCOffset        float64            `json:"dc_offset"`
	LeftVolumeDBFS  float64            `json:"left_volume_dbfs"`  // Independent Left dBFS (<= 0.0)
	RightVolumeDBFS float64            `json:"right_volume_dbfs"` // Independent Right dBFS (<= 0.0)
	Sweep           SweepParams        `json:"sweep,omitempty"`
	Noise           NoiseParams        `json:"noise,omitempty"`
	Multitone       MultitoneParams    `json:"multitone,omitempty"`
	DoP             DoPParams          `json:"dop,omitempty"`
}

// DefaultExtendedSignalRequest returns a safe default 1kHz stereo sine request.
func DefaultExtendedSignalRequest() ExtendedSignalRequest {
	return ExtendedSignalRequest{
		SampleRate:      48000,
		SampleFormat:    audio.SampleFormat16Int,
		Channels:        2,
		Waveform:        WaveformSine,
		Frequency:       1000.0,
		Amplitude:       0.501187, // -6 dBFS
		LeftVolumeDBFS:  -6.0,
		RightVolumeDBFS: -6.0,
		Sweep: SweepParams{
			StartFreq:     20.0,
			EndFreq:       20000.0,
			DurationSec:   5.0,
			TaperFraction: 0.10,
			Loop:          true,
		},
		Noise: NoiseParams{
			Seed: 12345,
		},
		DoP: DoPParams{
			DSDFormat: dsd.DSD64,
			BitOrder:  dsd.BitOrderLSBFirst,
		},
	}
}
