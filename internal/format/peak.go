package format

import (
	"math"
)

// AmplitudeToDBFS converts linear amplitude [0, 1.0] to decibels relative to full scale (dBFS).
func AmplitudeToDBFS(amplitude float64) float64 {
	if amplitude <= 1e-12 {
		return -240.0
	}
	return 20.0 * math.Log10(amplitude)
}

// DBFSToAmplitude converts dBFS to linear amplitude.
func DBFSToAmplitude(dbfs float64) float64 {
	if dbfs <= -240.0 {
		return 0.0
	}
	return math.Pow(10.0, dbfs/20.0)
}

// CalculatePeak finds the absolute maximum amplitude in a slice of float64 samples.
func CalculatePeak(samples []float64) float64 {
	maxVal := 0.0
	for _, s := range samples {
		abs := math.Abs(s)
		if abs > maxVal {
			maxVal = abs
		}
	}
	return maxVal
}

// CalculateMultiChannelPeak finds the absolute maximum amplitude across multiple channels.
func CalculateMultiChannelPeak(channels [][]float64) float64 {
	maxVal := 0.0
	for _, ch := range channels {
		p := CalculatePeak(ch)
		if p > maxVal {
			maxVal = p
		}
	}
	return maxVal
}

// PeakGuard enforces strict overload protection against clipping.
type PeakGuard struct {
	AllowSaturation bool
}

// NewPeakGuard creates a peak guard instance with strict rejection enabled by default.
func NewPeakGuard(allowSaturation bool) *PeakGuard {
	return &PeakGuard{
		AllowSaturation: allowSaturation,
	}
}

// Check verifies that the provided peak amplitude does not exceed 0 dBFS (1.0).
// If peak > 1.0 and AllowSaturation is false, ErrPeakExceedsFullScale is returned.
func (pg *PeakGuard) Check(peak float64) error {
	if peak > 1.0+floatTolerance {
		if !pg.AllowSaturation {
			return ErrPeakExceedsFullScale
		}
	}
	return nil
}

// Inspect scans samples, calculates peak, and verifies against overload limits.
func (pg *PeakGuard) Inspect(samples []float64) (float64, error) {
	peak := CalculatePeak(samples)
	return peak, pg.Check(peak)
}

// InspectChannels scans multi-channel samples, calculates peak, and verifies against overload limits.
func (pg *PeakGuard) InspectChannels(channels [][]float64) (float64, error) {
	peak := CalculateMultiChannelPeak(channels)
	return peak, pg.Check(peak)
}
