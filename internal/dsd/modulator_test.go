package dsd

import (
	"testing"
)

func TestFirstOrderModulator(t *testing.T) {
	mod := NewFirstOrderSDM()
	report := EvaluateModulator(mod, 1000.0, -6.0, 10.0)

	t.Logf("\n%s", report.String())

	if report.OverflowOccurred {
		t.Errorf("1st-order modulator overflowed during stress test")
	}

	// 1st order NTF slope should be ~6 dB/octave
	if report.NTFSlopeDbPerOctave < 3.0 || report.NTFSlopeDbPerOctave > 10.0 {
		t.Errorf("1st-order NTF slope out of expected range [3.0, 10.0]: got %.2f dB/oct", report.NTFSlopeDbPerOctave)
	}

	// In-band SNR for 1st-order at DSD64
	if report.InBandSNR_DSD64 < 20.0 {
		t.Errorf("1st-order in-band SNR suspiciously low: got %.2f dB", report.InBandSNR_DSD64)
	}
}

func TestSecondOrderModulator(t *testing.T) {
	mod := NewSecondOrderSDM()
	report := EvaluateModulator(mod, 1000.0, -6.0, 10.0)

	t.Logf("\n%s", report.String())

	if report.OverflowOccurred {
		t.Errorf("2nd-order modulator overflowed during stress test")
	}

	// 2nd order NTF slope should be ~12 dB/octave
	if report.NTFSlopeDbPerOctave < 8.0 || report.NTFSlopeDbPerOctave > 16.0 {
		t.Errorf("2nd-order NTF slope out of expected range [8.0, 16.0]: got %.2f dB/oct", report.NTFSlopeDbPerOctave)
	}

	// 2nd-order in-band SNR should be superior to 1st-order (>60 dB)
	if report.InBandSNR_DSD64 < 60.0 {
		t.Errorf("2nd-order in-band SNR expected >60 dB, got %.2f dB", report.InBandSNR_DSD64)
	}
}
