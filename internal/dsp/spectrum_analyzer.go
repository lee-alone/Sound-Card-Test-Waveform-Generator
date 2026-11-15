package dsp

import (
	"fmt"
	"math"
	"math/cmplx"
	"time"
)

const (
	// DefaultSpectrumFFTSize is the base power-of-2 FFT length for spectrum analysis.
	// 16384 points at 48 kHz provides fine 2.93 Hz bin resolution to isolate low-frequency 1/3-octave bands.
	DefaultSpectrumFFTSize = 16384

	// DefaultMinDB is the bottom of the visualizer scale in dBFS.
	DefaultMinDB = -90.0

	// DefaultMaxDB is the top of the visualizer scale in dBFS.
	DefaultMaxDB = 0.0

	// DefaultDecayRateDBPerSec is how fast bars fall back to zero (in dB/s).
	DefaultDecayRateDBPerSec = 45.0

	// DefaultPeakDecayRateDBPerSec is how fast peak hold indicators fall (in dB/s).
	DefaultPeakDecayRateDBPerSec = 30.0

	// DefaultPeakHoldSec is how long the peak hold stays before starting to fall.
	DefaultPeakHoldSec = 0.40

	// BaseMaxTargetFreq is the default minimum upper display limit (20 kHz, human hearing limit).
	BaseMaxTargetFreq = 20000.0
)

// StandardISOCenterFreqs defines 1/3-octave center frequencies (Hz) from 25 Hz to 160 kHz.
var StandardISOCenterFreqs = []float64{
	25, 31.5, 40, 50, 63, 80, 100, 125, 160, 200,
	250, 315, 400, 500, 630, 800, 1000, 1250, 1600, 2000,
	2500, 3150, 4000, 5000, 6300, 8000, 10000, 12500, 16000, 20000,
	25000, 31500, 40000, 50000, 63000, 80000, 100000, 125000, 160000,
}

// StandardISO30CenterFreqs is preserved for compatibility with 20 kHz baseline references.
var StandardISO30CenterFreqs = StandardISOCenterFreqs[:30]

// SpectrumBand describes a single frequency band / column.
type SpectrumBand struct {
	CenterFreq float64
	MinFreq    float64
	MaxFreq    float64
	Label      string
	StartBin   int
	EndBin     int
}

// SpectrumConfig configures the spectrum analyzer and ballistics.
type SpectrumConfig struct {
	SampleRate            int
	FFTSize               int
	MinDB                 float64
	MaxDB                 float64
	DecayRateDBPerSec     float64
	PeakDecayRateDBPerSec float64
	PeakHoldSec           float64
}

// DefaultSpectrumConfig returns sensible instrument-grade defaults.
func DefaultSpectrumConfig(sampleRate int) SpectrumConfig {
	if sampleRate <= 0 {
		sampleRate = 48000
	}
	return SpectrumConfig{
		SampleRate:            sampleRate,
		FFTSize:               DetermineFFTSize(sampleRate),
		MinDB:                 DefaultMinDB,
		MaxDB:                 DefaultMaxDB,
		DecayRateDBPerSec:     DefaultDecayRateDBPerSec,
		PeakDecayRateDBPerSec: DefaultPeakDecayRateDBPerSec,
		PeakHoldSec:           DefaultPeakHoldSec,
	}
}

// DetermineFFTSize chooses an FFT size that maintains fine ~2.93 Hz bin resolution to isolate low bands.
func DetermineFFTSize(sampleRate int) int {
	switch {
	case sampleRate <= 48000:
		return 16384
	case sampleRate <= 96000:
		return 32768
	default:
		return 32768
	}
}

// SpectrumAnalyzer handles windowing, FFT computation, non-overlapping log-band binning,
// and ballistics (attack, decay, peak-hold). It uses a 4-term Blackman-Harris window
// (-92 dB sidelobe suppression) to prevent spectral leakage from polluting low frequencies.
type SpectrumAnalyzer struct {
	cfg           SpectrumConfig
	bands         []SpectrumBand
	targetMaxFreq float64

	window    []float64
	windowSum float64

	// Ballistics state for Left and Right channels
	curL      []float64
	curR      []float64
	peakL     []float64
	peakR     []float64
	peakHoldL []float64
	peakHoldR []float64

	fftBuf []complex128

	lastTime time.Time
}

// NewSpectrumAnalyzer creates a new configured SpectrumAnalyzer.
func NewSpectrumAnalyzer(cfg SpectrumConfig) *SpectrumAnalyzer {
	if cfg.SampleRate <= 0 {
		cfg.SampleRate = 48000
	}
	if cfg.FFTSize <= 0 || (cfg.FFTSize&(cfg.FFTSize-1)) != 0 {
		cfg.FFTSize = DetermineFFTSize(cfg.SampleRate)
	}
	if cfg.DecayRateDBPerSec <= 0 {
		cfg.DecayRateDBPerSec = DefaultDecayRateDBPerSec
	}
	if cfg.PeakDecayRateDBPerSec <= 0 {
		cfg.PeakDecayRateDBPerSec = DefaultPeakDecayRateDBPerSec
	}
	if cfg.PeakHoldSec <= 0 {
		cfg.PeakHoldSec = DefaultPeakHoldSec
	}

	a := &SpectrumAnalyzer{
		cfg:           cfg,
		targetMaxFreq: BaseMaxTargetFreq,
	}

	a.rebuild(cfg.SampleRate, cfg.FFTSize, BaseMaxTargetFreq)
	return a
}

// Configure dynamically adapts sample rate and target maximum frequency based on signal parameters.
func (a *SpectrumAnalyzer) Configure(sampleRate int, targetMaxFreq float64) {
	if sampleRate <= 0 {
		sampleRate = 48000
	}
	if targetMaxFreq < BaseMaxTargetFreq {
		targetMaxFreq = BaseMaxTargetFreq
	}

	// Physical Nyquist ceiling
	ceiling := float64(sampleRate) * 0.48
	if targetMaxFreq > ceiling {
		targetMaxFreq = ceiling
	}

	fftSize := DetermineFFTSize(sampleRate)

	if sampleRate == a.cfg.SampleRate &&
		fftSize == a.cfg.FFTSize &&
		math.Abs(targetMaxFreq-a.targetMaxFreq) < 1.0 {
		return
	}

	a.cfg.SampleRate = sampleRate
	a.cfg.FFTSize = fftSize
	a.targetMaxFreq = targetMaxFreq

	a.rebuild(sampleRate, fftSize, targetMaxFreq)
}

func (a *SpectrumAnalyzer) rebuild(sampleRate, fftSize int, targetMaxFreq float64) {
	bands := buildBands(sampleRate, fftSize, targetMaxFreq)
	nBands := len(bands)

	// Precompute periodic 4-term Blackman-Harris window (-92 dB peak sidelobe)
	// Coefficients: a0=0.35875, a1=0.48829, a2=0.14128, a3=0.01168
	win := make([]float64, fftSize)
	winSum := 0.0
	const (
		bhA0 = 0.35875
		bhA1 = 0.48829
		bhA2 = 0.14128
		bhA3 = 0.01168
	)
	twoPi := 2.0 * math.Pi
	for i := 0; i < fftSize; i++ {
		tNorm := float64(i) / float64(fftSize)
		w := bhA0 - bhA1*math.Cos(twoPi*tNorm) + bhA2*math.Cos(2.0*twoPi*tNorm) - bhA3*math.Cos(3.0*twoPi*tNorm)
		win[i] = w
		winSum += w
	}
	if winSum <= 0 {
		winSum = float64(fftSize) * bhA0
	}

	a.bands = bands
	a.window = win
	a.windowSum = winSum

	// Resize buffers while preserving current values where applicable
	a.curL = resizeBuffer(a.curL, nBands)
	a.curR = resizeBuffer(a.curR, nBands)
	a.peakL = resizeBuffer(a.peakL, nBands)
	a.peakR = resizeBuffer(a.peakR, nBands)
	a.peakHoldL = resizeBuffer(a.peakHoldL, nBands)
	a.peakHoldR = resizeBuffer(a.peakHoldR, nBands)

	if len(a.fftBuf) != fftSize {
		a.fftBuf = make([]complex128, fftSize)
	}
}

func resizeBuffer(oldBuf []float64, newSize int) []float64 {
	newBuf := make([]float64, newSize)
	if len(oldBuf) > 0 {
		copy(newBuf, oldBuf)
	}
	return newBuf
}

// Bands returns the precomputed frequency bands.
func (a *SpectrumAnalyzer) Bands() []SpectrumBand {
	return a.bands
}

// FFTSize returns the active FFT size.
func (a *SpectrumAnalyzer) FFTSize() int {
	return a.cfg.FFTSize
}

// TargetMaxFreq returns the current target maximum frequency.
func (a *SpectrumAnalyzer) TargetMaxFreq() float64 {
	return a.targetMaxFreq
}

// Reset clears all ballistics to silence.
func (a *SpectrumAnalyzer) Reset() {
	for i := range a.curL {
		a.curL[i] = 0
		a.curR[i] = 0
		a.peakL[i] = 0
		a.peakR[i] = 0
		a.peakHoldL[i] = 0
		a.peakHoldR[i] = 0
	}
	a.lastTime = time.Time{}
}

// SetSampleRate updates sample rate and re-computes band bin mappings if changed.
func (a *SpectrumAnalyzer) SetSampleRate(rate int) {
	a.Configure(rate, a.targetMaxFreq)
}

// Process transforms Left and Right time-domain buffers into visualizer heights
// in range [0, 90.0] representing [-90 dBFS, 0 dBFS].
func (a *SpectrumAnalyzer) Process(leftSamples, rightSamples []float64, now time.Time) (leftBars, rightBars, peakL, peakR []float64) {
	dt := 0.033
	if !a.lastTime.IsZero() {
		elapsed := now.Sub(a.lastTime).Seconds()
		if elapsed > 0 && elapsed < 0.5 {
			dt = elapsed
		}
	}
	a.lastTime = now

	rawL := a.analyzeChannel(leftSamples)
	rawR := a.analyzeChannel(rightSamples)

	a.updateBallistics(a.curL, a.peakL, a.peakHoldL, rawL, dt)
	a.updateBallistics(a.curR, a.peakR, a.peakHoldR, rawR, dt)

	outL := make([]float64, len(a.curL))
	outR := make([]float64, len(a.curR))
	outPL := make([]float64, len(a.peakL))
	outPR := make([]float64, len(a.peakR))

	copy(outL, a.curL)
	copy(outR, a.curR)
	copy(outPL, a.peakL)
	copy(outPR, a.peakR)

	return outL, outR, outPL, outPR
}

// analyzeChannel windows samples, computes Radix-2 FFT and returns visual heights [0, 90].
func (a *SpectrumAnalyzer) analyzeChannel(samples []float64) []float64 {
	nBands := len(a.bands)
	result := make([]float64, nBands)
	if len(samples) == 0 {
		return result
	}

	fftSize := a.cfg.FFTSize
	n := len(samples)
	if n > fftSize {
		n = fftSize
	}

	// Apply window and copy to complex buffer
	for i := 0; i < n; i++ {
		a.fftBuf[i] = complex(samples[i]*a.window[i], 0.0)
	}
	for i := n; i < fftSize; i++ {
		a.fftBuf[i] = complex(0.0, 0.0)
	}

	// Perform Radix-2 FFT
	fftRadix2(a.fftBuf)

	// Half-spectrum magnitudes normalized to peak sine = 1.0 (0 dBFS)
	// For Blackman-Harris window, coherent gain factor is 2.0 / windowSum
	coherentGain := 2.0 / a.windowSum
	half := fftSize / 2
	mags := make([]float64, half)
	for i := 0; i < half; i++ {
		mags[i] = cmplx.Abs(a.fftBuf[i]) * coherentGain
	}

	// Aggregate into frequency bands
	dbRange := a.cfg.MaxDB - a.cfg.MinDB // typically 90.0
	for i, b := range a.bands {
		maxMag := 0.0
		for bin := b.StartBin; bin <= b.EndBin && bin < half; bin++ {
			if mags[bin] > maxMag {
				maxMag = mags[bin]
			}
		}

		if maxMag < 1e-6 {
			result[i] = 0.0
			continue
		}

		dbFS := 20.0 * math.Log10(maxMag)
		if dbFS < a.cfg.MinDB {
			dbFS = a.cfg.MinDB
		} else if dbFS > a.cfg.MaxDB {
			dbFS = a.cfg.MaxDB
		}

		// Height [0.0, 90.0]
		height := dbFS - a.cfg.MinDB
		if height < 0 {
			height = 0
		} else if height > dbRange {
			height = dbRange
		}
		result[i] = height
	}

	return result
}

// updateBallistics applies fast-attack, exponential decay, and peak-hold.
func (a *SpectrumAnalyzer) updateBallistics(cur, peak, peakHold, targets []float64, dt float64) {
	decayStep := a.cfg.DecayRateDBPerSec * dt
	peakDecayStep := a.cfg.PeakDecayRateDBPerSec * dt

	for i := range cur {
		target := targets[i]

		// Attack / Decay for main bar
		if target >= cur[i] {
			// Fast attack
			cur[i] = target
		} else {
			// Linear decay
			cur[i] = math.Max(target, cur[i]-decayStep)
		}

		// Peak hold handling
		if cur[i] >= peak[i] {
			peak[i] = cur[i]
			peakHold[i] = a.cfg.PeakHoldSec
		} else {
			if peakHold[i] > 0 {
				peakHold[i] -= dt
			} else {
				peak[i] = math.Max(cur[i], peak[i]-peakDecayStep)
			}
		}
	}
}

// buildBands constructs 1/3-octave frequency bands up to targetMaxFreq mapped to non-overlapping FFT bins.
func buildBands(sampleRate, fftSize int, targetMaxFreq float64) []SpectrumBand {
	deltaF := float64(sampleRate) / float64(fftSize)
	ratio := math.Pow(2.0, 1.0/6.0) // 1/6 octave each side = 1/3 octave band
	half := fftSize / 2
	ceiling := float64(sampleRate) * 0.48

	if targetMaxFreq < BaseMaxTargetFreq {
		targetMaxFreq = BaseMaxTargetFreq
	}
	if targetMaxFreq > ceiling {
		targetMaxFreq = ceiling
	}

	// Select bands up to targetMaxFreq
	selectedFreqs := make([]float64, 0, len(StandardISOCenterFreqs))
	for _, fc := range StandardISOCenterFreqs {
		if fc > ceiling {
			break
		}
		selectedFreqs = append(selectedFreqs, fc)
		if fc >= targetMaxFreq {
			break
		}
	}
	if len(selectedFreqs) == 0 {
		selectedFreqs = StandardISOCenterFreqs[:30]
	}

	numBands := len(selectedFreqs)
	bands := make([]SpectrumBand, numBands)

	for i, fc := range selectedFreqs {
		var fMin, fMax float64

		if i == 0 {
			fMin = fc / ratio
		} else {
			fMin = math.Sqrt(selectedFreqs[i-1] * fc)
		}

		if i == numBands-1 {
			fMax = fc * ratio
		} else {
			fMax = math.Sqrt(fc * selectedFreqs[i+1])
		}

		if fMax > float64(sampleRate)*0.5 {
			fMax = float64(sampleRate) * 0.5
		}

		startBin := int(math.Round(fMin / deltaF))
		endBin := int(math.Round(fMax / deltaF)) - 1

		if startBin < 1 {
			startBin = 1
		}
		if endBin < startBin {
			endBin = startBin
		}
		if endBin >= half {
			endBin = half - 1
		}

		bands[i] = SpectrumBand{
			CenterFreq: fc,
			MinFreq:    fMin,
			MaxFreq:    fMax,
			Label:      formatBandLabel(fc),
			StartBin:   startBin,
			EndBin:     endBin,
		}
	}

	return bands
}

// formatBandLabel converts frequency into compact instrument labels like "1k", "31.5k".
func formatBandLabel(hz float64) string {
	switch {
	case hz >= 1000.0:
		val := hz / 1000.0
		if math.Abs(val-math.Round(val)) < 0.05 {
			return fmt.Sprintf("%.0fk", val)
		}
		return fmt.Sprintf("%.1fk", val)
	case math.Abs(hz-math.Round(hz)) < 0.05:
		return fmt.Sprintf("%.0f", hz)
	default:
		return fmt.Sprintf("%.1f", hz)
	}
}
