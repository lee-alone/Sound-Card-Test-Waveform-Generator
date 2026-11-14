package controller

import (
	"errors"
	"fmt"
	"sound-signal-generator/internal/audio"
)

var (
	// ErrFrequencyExceedsNyquistLimit is returned when signal frequency exceeds 0.45 * Fs.
	ErrFrequencyExceedsNyquistLimit = errors.New("signal frequency exceeds safe Nyquist limit (0.45 * Fs)")

	// ErrAmplitudeOutOfRange is returned when amplitude is outside [0.0, 1.0].
	ErrAmplitudeOutOfRange = errors.New("amplitude must be within range [0.0, 1.0] (0 dBFS max)")

	// ErrInvalidSampleRate is returned when sample rate is non-positive or unsupported.
	ErrInvalidSampleRate = errors.New("invalid or unsupported sample rate")

	// ErrDeviceNotSelected is returned when playback is requested without an active device.
	ErrDeviceNotSelected = errors.New("no audio device selected")
)

// SafeNyquistRatio is the maximum allowed frequency ratio relative to sampling frequency (0.45).
const SafeNyquistRatio = 0.45

// SignalRequest specifies a high-level test signal configuration.
type SignalRequest struct {
	DeviceID     string            `json:"device_id"`
	SampleRate   int               `json:"sample_rate"`
	SampleFormat audio.SampleFormat `json:"sample_format"`
	Channels     int               `json:"channels"`
	Frequency    float64           `json:"frequency"`
	Amplitude    float64           `json:"amplitude"` // linear amplitude [0.0, 1.0]
	DCOffset     float64           `json:"dc_offset"`
}

// ValidateFrequency strictly checks if the requested frequency satisfies Fs >= f / 0.45.
func ValidateFrequency(frequency float64, sampleRate int) error {
	if frequency <= 0 {
		return errors.New("frequency must be positive")
	}
	if sampleRate <= 0 {
		return ErrInvalidSampleRate
	}

	maxAllowed := float64(sampleRate) * SafeNyquistRatio
	if frequency > maxAllowed {
		return fmt.Errorf("%w: requested %.2f Hz exceeds maximum safe limit %.2f Hz for Fs=%d Hz",
			ErrFrequencyExceedsNyquistLimit, frequency, maxAllowed, sampleRate)
	}

	return nil
}

// ValidateAmplitude checks that amplitude + |dc_offset| does not exceed 1.0 (0 dBFS).
func ValidateAmplitude(amplitude, dcOffset float64) error {
	if amplitude < 0.0 || amplitude > 1.0 {
		return ErrAmplitudeOutOfRange
	}
	if amplitude+abs(dcOffset) > 1.0+1e-12 {
		return ErrAmplitudeOutOfRange
	}
	return nil
}

// ValidateSignalRequest performs full parameter validation.
func ValidateSignalRequest(req SignalRequest) error {
	if err := ValidateFrequency(req.Frequency, req.SampleRate); err != nil {
		return err
	}
	if err := ValidateAmplitude(req.Amplitude, req.DCOffset); err != nil {
		return err
	}
	if req.Channels <= 0 {
		return errors.New("channel count must be at least 1")
	}
	return nil
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}
