package dsd

import (
	"fmt"
	"time"
)

// Modulator defines the interface for discrete-time Delta-Sigma modulators.
type Modulator interface {
	// Step processes one input sample and returns the 1-bit output (+1.0 or -1.0).
	Step(input float64) float64
	// Reset clears internal integrator states.
	Reset()
	// Order returns the order of the modulator filter loop.
	Order() int
	// Topology returns the architectural name of the modulator.
	Topology() string
	// Coefficients returns description of the filter coefficients.
	Coefficients() string
	// PeakState returns the maximum absolute integrator state observed since last Reset.
	PeakState() float64
}

// SDMMetricsReport encapsulates the quantitative characterization results required by Task T3.3.
type SDMMetricsReport struct {
	Topology              string        `json:"topology"`
	Order                 int           `json:"order"`
	Coefficients          string        `json:"coefficients"`
	StableInputRange      string        `json:"stable_input_range"`
	PeakState             float64       `json:"peak_state"`
	InBandSNR_DSD64       float64       `json:"in_band_snr_dsd64_db"`
	MaxIdleToneSpurDB     float64       `json:"max_idle_tone_spur_db"`
	NTFSlopeDbPerOctave   float64       `json:"ntf_slope_db_per_octave"`
	TestDuration          time.Duration `json:"test_duration"`
	TotalIterations       uint64        `json:"total_iterations"`
	OverflowOccurred      bool          `json:"overflow_occurred"`
}

func (r SDMMetricsReport) String() string {
	return fmt.Sprintf(
		"[%s (Order %d)]\n"+
			"  Coefficients:       %s\n"+
			"  Stable Input Range: %s\n"+
			"  Peak State:         %.4f\n"+
			"  DSD64 In-Band SNR:  %.2f dB\n"+
			"  Max Idle Tone Spur: %.2f dBFS\n"+
			"  NTF Slope:          %.2f dB/octave\n"+
			"  Overflow:           %v (over %d iterations, %v duration)",
		r.Topology, r.Order, r.Coefficients, r.StableInputRange, r.PeakState,
		r.InBandSNR_DSD64, r.MaxIdleToneSpurDB, r.NTFSlopeDbPerOctave,
		r.OverflowOccurred, r.TotalIterations, r.TestDuration,
	)
}
