//go:build windows

package wasapi

import (
	"errors"
	"fmt"
	"time"

	"sound-signal-generator/internal/audio"
)

const (
	// DoPRateDSD64 is the PCM carrier sample rate for DSD64 DoP (176.4 kHz).
	DoPRateDSD64 = 176400
	// DoPRateDSD128 is the PCM carrier sample rate for DSD128 DoP (352.8 kHz).
	DoPRateDSD128 = 352800
)

var (
	// ErrUnsupportedDoPRate is returned when a non-DoP carrier rate is requested.
	ErrUnsupportedDoPRate = errors.New("unsupported DoP carrier sample rate: must be 176400 Hz (DSD64) or 352800 Hz (DSD128)")
	// ErrDoPFormatNotSupported is returned when hardware does not support 24-bit exclusive at carrier rate.
	ErrDoPFormatNotSupported = errors.New("audio hardware does not support 24-bit exclusive mode at requested DoP carrier rate")
)

// IsDoPCarrierRate returns true if rate matches standard DoP carrier rates (176.4k or 352.8k).
func IsDoPCarrierRate(sampleRate int) bool {
	return sampleRate == DoPRateDSD64 || sampleRate == DoPRateDSD128
}

// CalculateDoPBufferParams determines buffer frames and 100ns periodicity aligned with hardware clock limits.
func CalculateDoPBufferParams(sampleRate int, requestedDuration time.Duration, defaultPeriod, minPeriod time.Duration) (uint32, int64) {
	targetPeriod := requestedDuration
	if targetPeriod <= 0 {
		targetPeriod = 10 * time.Millisecond
	}

	// Respect hardware minimum period if reported and greater than target
	if minPeriod > 0 && targetPeriod < minPeriod {
		targetPeriod = minPeriod
	}

	// Calculate integer frames at high sample rate
	frames := uint32(float64(targetPeriod.Nanoseconds()) * float64(sampleRate) / 1e9)
	if frames == 0 {
		frames = uint32(sampleRate / 100) // fallback 10ms
	}

	// Periodicity in 100ns units
	hnsPeriod := int64(float64(frames) * 10000000.0 / float64(sampleRate))
	return frames, hnsPeriod
}

// NegotiateDoPFormat checks a device for 24-bit format support (24-in-32 or 24-packed) at the carrier rate.
func NegotiateDoPFormat(device audio.Device, carrierRate int) (audio.AudioFormat, error) {
	if !IsDoPCarrierRate(carrierRate) {
		return audio.AudioFormat{}, ErrUnsupportedDoPRate
	}

	candidates := []audio.AudioFormat{
		{
			SampleRate:   carrierRate,
			Channels:     2,
			SampleFormat: audio.SampleFormat24In32,
			ChannelMask:  audio.StandardSpeakerChannelMaskStereo,
		},
		{
			SampleRate:   carrierRate,
			Channels:     2,
			SampleFormat: audio.SampleFormat24Packed,
			ChannelMask:  audio.StandardSpeakerChannelMaskStereo,
		},
	}

	results, err := device.ProbeCapabilities(candidates, audio.ShareModeExclusive)
	if err != nil {
		return audio.AudioFormat{}, fmt.Errorf("probe failed: %w", err)
	}

	for _, res := range results {
		if res.Status == audio.StatusSupported {
			return res.Format, nil
		}
	}

	return audio.AudioFormat{}, fmt.Errorf("%w for %d Hz", ErrDoPFormatNotSupported, carrierRate)
}

// OpenDoPExclusiveStream negotiates and opens an exclusive stream at 176.4k or 352.8k.
func OpenDoPExclusiveStream(device audio.Device, carrierRate int, requestedDuration time.Duration) (audio.AudioStream, audio.AudioFormat, error) {
	format, err := NegotiateDoPFormat(device, carrierRate)
	if err != nil {
		return nil, audio.AudioFormat{}, err
	}

	streamConfig := audio.StreamConfig{
		Format:         format,
		ShareMode:      audio.ShareModeExclusive,
		BufferDuration: requestedDuration,
		EventDriven:    true,
	}

	stream, err := device.OpenStream(streamConfig)
	if err != nil {
		return nil, audio.AudioFormat{}, fmt.Errorf("failed to open exclusive DoP stream: %w", err)
	}

	return stream, format, nil
}
