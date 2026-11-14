package controller

import (
	"errors"
	"testing"
)

func TestNyquistLimitValidation(t *testing.T) {
	// Quantified criteria from Schedule T1.6:
	// When requesting 60kHz signal:
	// - At Fs <= 96kHz: strictly rejected with ErrFrequencyExceedsNyquistLimit
	// - At Fs = 176.4kHz and Fs = 192kHz: allowed
	freq60k := 60000.0

	// 1. Rejection test on Fs <= 96kHz
	rejectedRates := []int{44100, 48000, 88200, 96000}
	for _, fs := range rejectedRates {
		err := ValidateFrequency(freq60k, fs)
		if !errors.Is(err, ErrFrequencyExceedsNyquistLimit) {
			t.Errorf("ValidateFrequency(60kHz, %d) expected ErrFrequencyExceedsNyquistLimit, got %v", fs, err)
		}
	}

	// 2. Acceptance test on Fs = 176.4k and 192k
	// 176400 * 0.45 = 79380 Hz > 60000 Hz -> OK
	// 192000 * 0.45 = 86400 Hz > 60000 Hz -> OK
	allowedRates := []int{176400, 192000}
	for _, fs := range allowedRates {
		err := ValidateFrequency(freq60k, fs)
		if err != nil {
			t.Errorf("ValidateFrequency(60kHz, %d) expected pass, got %v", fs, err)
		}
	}
}

func TestAmplitudeValidation(t *testing.T) {
	// 0 dBFS (1.0) -> Valid
	if err := ValidateAmplitude(1.0, 0.0); err != nil {
		t.Errorf("ValidateAmplitude(1.0, 0.0) failed: %v", err)
	}

	// > 0 dBFS (> 1.0) -> Invalid
	if err := ValidateAmplitude(1.0001, 0.0); !errors.Is(err, ErrAmplitudeOutOfRange) {
		t.Errorf("ValidateAmplitude(1.0001, 0.0) expected ErrAmplitudeOutOfRange, got %v", err)
	}

	// Negative amplitude -> Invalid
	if err := ValidateAmplitude(-0.1, 0.0); !errors.Is(err, ErrAmplitudeOutOfRange) {
		t.Errorf("ValidateAmplitude(-0.1, 0.0) expected ErrAmplitudeOutOfRange, got %v", err)
	}

	// Amplitude + DC offset exceeding 1.0 -> Invalid
	if err := ValidateAmplitude(0.9, 0.15); !errors.Is(err, ErrAmplitudeOutOfRange) {
		t.Errorf("ValidateAmplitude(0.9, 0.15) expected ErrAmplitudeOutOfRange, got %v", err)
	}
}
