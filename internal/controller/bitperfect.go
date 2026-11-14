package controller

import (
	"fmt"
	"sound-signal-generator/internal/audio"
	"sound-signal-generator/internal/engine"
)

// BitPerfectTier matches the 4 UI levels specified in Whitepaper Section 3.2.
type BitPerfectTier string

const (
	TierConfigured BitPerfectTier = "Configured"
	TierEligible   BitPerfectTier = "Eligible"
	TierVerified   BitPerfectTier = "Verified"
	TierUnknown    BitPerfectTier = "Unknown"
)

// BitPerfectEvaluator evaluates current bit-perfect status against strict criteria.
type BitPerfectEvaluator struct {
	verifiedEvidence map[string]bool // map of format/device signatures verified in T3.6
}

// NewBitPerfectEvaluator creates a new evaluator.
func NewBitPerfectEvaluator() *BitPerfectEvaluator {
	return &BitPerfectEvaluator{
		verifiedEvidence: map[string]bool{
			// Amanero Combo384 verified in T3.6 archive
			"176400-24in32-2": true,
		},
	}
}

// RegisterVerifiedFormat registers a hardware-verified configuration format.
func (b *BitPerfectEvaluator) RegisterVerifiedFormat(sampleRate int, format audio.SampleFormat, channels int) {
	sig := makeFormatSignature(sampleRate, format, channels)
	b.verifiedEvidence[sig] = true
}

// Evaluate determines whether the current stream state is Configured, Eligible, Verified, or Unknown.
func (b *BitPerfectEvaluator) Evaluate(
	state engine.State,
	exclusive bool,
	reqFormat audio.AudioFormat,
	actualFormat audio.AudioFormat,
	leftVolumeDBFS float64,
	rightVolumeDBFS float64,
	isDoP bool,
) BitPerfectTier {
	// 1. Must be in Exclusive mode and formats must match exactly (no sample rate conversion)
	if !exclusive {
		return TierUnknown
	}
	if reqFormat.SampleRate != actualFormat.SampleRate || reqFormat.Channels != actualFormat.Channels {
		return TierUnknown
	}
	if reqFormat.SampleFormat != actualFormat.SampleFormat {
		return TierUnknown
	}

	// 2. Volume must be unity gain (0.0 dBFS) for bit-perfect transmission
	// For DoP, volume is fixed to unity by design
	if !isDoP {
		if leftVolumeDBFS != 0.0 || rightVolumeDBFS != 0.0 {
			return TierUnknown
		}
	}

	// 3. If stream is not actively running in steady state, it is only "Configured"
	if state != engine.StateRunning {
		return TierConfigured
	}

	// 4. In steady running state, check if hardware evidence exists for "Verified"
	sig := makeFormatSignature(actualFormat.SampleRate, actualFormat.SampleFormat, actualFormat.Channels)
	if b.verifiedEvidence[sig] {
		return TierVerified
	}

	// 5. Otherwise eligible internal software path
	return TierEligible
}

func makeFormatSignature(sampleRate int, format audio.SampleFormat, channels int) string {
	var fmtStr string
	switch format {
	case audio.SampleFormat16Int:
		fmtStr = "16int"
	case audio.SampleFormat24Packed:
		fmtStr = "24packed"
	case audio.SampleFormat24In32:
		fmtStr = "24in32"
	case audio.SampleFormat32Float:
		fmtStr = "32float"
	default:
		fmtStr = "unknown"
	}
	return fmt.Sprintf("%d-%s-%d", sampleRate, fmtStr, channels)
}
