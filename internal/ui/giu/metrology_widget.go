package giuui

import (
	"fmt"
	"math"
	"time"

	g "github.com/AllenDang/giu"
	"sound-signal-generator/internal/controller"
	"sound-signal-generator/internal/dsp"
	"sound-signal-generator/internal/ui/i18n"
)

// MetrologyWidget presents the secondary readouts directly under the scope or spectrum analyzer.
// Values are kept compact so the eye can compare them without leaving the chart.
type MetrologyWidget struct {
	state *UIState
}

func NewMetrologyWidget(state *UIState) *MetrologyWidget {
	return &MetrologyWidget{state: state}
}

func (m *MetrologyWidget) formatSignalFrequency(waveID controller.WaveformType) string {
	switch waveID {
	case controller.WaveformSine,
		controller.WaveformPolyBlepSquare,
		controller.WaveformPolyBlepTriangle:
		return formatFrequency(math.Max(1.0, float64(m.state.Frequency)))

	case controller.WaveformLinearSweep, controller.WaveformLogarithmicSweep:
		start := math.Max(1.0, float64(m.state.SweepStartFreq))
		end := math.Max(1.0, float64(m.state.SweepEndFreq))
		if m.state.IsOutputRunning() {
			anchor := m.state.SignalStartTime
			if anchor.IsZero() {
				anchor = m.state.StreamStartTime
			}
			if !anchor.IsZero() {
				elapsed := time.Since(anchor).Seconds()
				duration := math.Max(0.5, float64(m.state.SweepDurationSec))
				t := elapsed
				if m.state.SweepLoop && duration > 0 {
					t = math.Mod(elapsed, duration)
					if t < 0 {
						t += duration
					}
				} else if t > duration {
					t = duration
				}
				progress := 1.0
				if duration > 0 {
					progress = t / duration
				}
				currentFreq := sweepFrequencyAt(start, end, duration, progress, waveID == controller.WaveformLogarithmicSweep)
				return fmt.Sprintf("%s (%.0f%%)", formatFrequency(currentFreq), progress*100.0)
			}
		}
		return fmt.Sprintf("%s – %s", formatFrequency(start), formatFrequency(end))

	case controller.WaveformWhiteNoise:
		return i18n.T("spectrum.signal_white_noise")

	case controller.WaveformPinkNoise:
		return i18n.T("spectrum.signal_pink_noise")

	case controller.WaveformMultitoneSMPTE:
		return "60 Hz + 7 kHz"

	case controller.WaveformMultitoneCCIF:
		return "19 kHz + 20 kHz"

	case controller.WaveformDoP:
		return "1 kHz (DSD64)"

	default:
		return formatFrequency(math.Max(1.0, float64(m.state.Frequency)))
	}
}

func (m *MetrologyWidget) Build() g.Widget {
	maxVolDB := math.Max(float64(m.state.LeftVolumeDBFS), float64(m.state.RightVolumeDBFS))
	vPeakFS := math.Pow(10, maxVolDB/20.0)

	waveID := controller.WaveformSine
	if int(m.state.SelectedWaveformIndex) >= 0 && int(m.state.SelectedWaveformIndex) < len(WaveformOptions) {
		waveID = WaveformOptions[m.state.SelectedWaveformIndex].ID
	}

	crestFactorDB := 3.01
	vRmsFS := vPeakFS / math.Sqrt(2)
	switch waveID {
	case controller.WaveformPolyBlepSquare:
		crestFactorDB = 0.00
		vRmsFS = vPeakFS
	case controller.WaveformPolyBlepTriangle:
		crestFactorDB = 4.77
		vRmsFS = vPeakFS / math.Sqrt(3)
	case controller.WaveformWhiteNoise, controller.WaveformPinkNoise:
		crestFactorDB = 12.04
		vRmsFS = vPeakFS / 4.0
	case controller.WaveformMultitoneSMPTE:
		crestFactorDB = 6.85
		vRmsFS = vPeakFS / 2.2
	case controller.WaveformMultitoneCCIF:
		crestFactorDB = 6.02
		vRmsFS = vPeakFS / 2.0
	}

	targetRate := 48000
	if int(m.state.SelectedSampleRateIndex) >= 0 && int(m.state.SelectedSampleRateIndex) < len(SampleRateOptions) {
		targetRate = SampleRateOptions[m.state.SelectedSampleRateIndex]
	}

	freq := float64(m.state.Frequency)
	periodMs := 0.0
	samplesPerCycle := 0.0
	if freq > 0 {
		periodMs = 1000.0 / freq
		samplesPerCycle = float64(targetRate) / freq
	}

	nyquistLimit := float64(targetRate) * 0.45
	nyquistMargin := nyquistLimit - freq
	nyquistUsagePct := (freq / nyquistLimit) * 100.0
	nyquistColor := ColorTextMuted
	nyquistText := i18n.T("metrology.nyquist_margin_fmt", nyquistUsagePct, nyquistMargin)
	if nyquistUsagePct > 100.0 {
		nyquistColor = ColorAlarmRed
		nyquistText = i18n.T("metrology.over_limit_fmt", nyquistUsagePct)
	}

	if m.state.ScopeDisplayMode == ScopeDisplaySpectrum {
		freqText := m.formatSignalFrequency(waveID)
		deltaF := float64(targetRate) / float64(dsp.DefaultSpectrumFFTSize)
		fftResText := fmt.Sprintf("Δf %.2f Hz", deltaF)

		return g.Table().
			Flags(g.TableFlagsNoSavedSettings | g.TableFlagsSizingStretchSame | g.TableFlagsNoBordersInBody).
			NoHeader(true).
			Rows(g.TableRow(
				metricCell(i18n.T("metrology.peak"), fmt.Sprintf("%.4f FS", vPeakFS), ColorPhosphorGreen),
				metricCell("RMS", fmt.Sprintf("%.4f FS", vRmsFS), ColorDigitalCyan),
				metricCell(i18n.T("metrology.crest_factor"), fmt.Sprintf("%.2f dB", crestFactorDB), ColorTextBright),
				metricCell(i18n.T("metrology.spectrum_freq"), freqText, ColorTextBright),
				metricCell(i18n.T("metrology.fft_resolution"), fftResText, ColorTextMuted),
				metricCell(i18n.T("metrology.safety_margin"), nyquistText, nyquistColor),
			))
	}

	return g.Table().
		Flags(g.TableFlagsNoSavedSettings | g.TableFlagsSizingStretchSame | g.TableFlagsNoBordersInBody).
		NoHeader(true).
		Rows(g.TableRow(
			metricCell(i18n.T("metrology.peak"), fmt.Sprintf("%.4f FS", vPeakFS), ColorPhosphorGreen),
			metricCell("RMS", fmt.Sprintf("%.4f FS", vRmsFS), ColorDigitalCyan),
			metricCell(i18n.T("metrology.crest_factor"), fmt.Sprintf("%.2f dB", crestFactorDB), ColorTextBright),
			metricCell(i18n.T("metrology.period"), fmt.Sprintf("%.3f ms", periodMs), ColorTextBright),
			metricCell(i18n.T("metrology.sample_density"), i18n.T("metrology.samples_per_cycle_fmt", samplesPerCycle), ColorTextMuted),
			metricCell(i18n.T("metrology.safety_margin"), nyquistText, nyquistColor),
		))
}
