package giuui

import (
	"fmt"
	"math"

	g "github.com/AllenDang/giu"
	"sound-signal-generator/internal/controller"
)

// MetrologyWidget presents the secondary readouts directly under the scope.
// Values are kept compact so the eye can compare them without leaving the
// waveform.
type MetrologyWidget struct {
	state *UIState
}

func NewMetrologyWidget(state *UIState) *MetrologyWidget {
	return &MetrologyWidget{state: state}
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
	nyquistText := fmt.Sprintf("%.1f %%  ·  余量 %.0f Hz", nyquistUsagePct, nyquistMargin)
	if nyquistUsagePct > 100.0 {
		nyquistColor = ColorAlarmRed
		nyquistText = fmt.Sprintf("%.1f %%  ·  超限", nyquistUsagePct)
	}

	return g.Table().
		Flags(g.TableFlagsNoSavedSettings | g.TableFlagsSizingStretchSame | g.TableFlagsNoBordersInBody).
		NoHeader(true).
		Rows(g.TableRow(
			metricCell("峰值", fmt.Sprintf("%.4f FS", vPeakFS), ColorPhosphorGreen),
			metricCell("RMS", fmt.Sprintf("%.4f FS", vRmsFS), ColorDigitalCyan),
			metricCell("峰值因数", fmt.Sprintf("%.2f dB", crestFactorDB), ColorTextBright),
			metricCell("周期", fmt.Sprintf("%.3f ms", periodMs), ColorTextBright),
			metricCell("采样密度", fmt.Sprintf("%.1f 点/周期", samplesPerCycle), ColorTextMuted),
			metricCell("安全余量", nyquistText, nyquistColor),
		))
}
