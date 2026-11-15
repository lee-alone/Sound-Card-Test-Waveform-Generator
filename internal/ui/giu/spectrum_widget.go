package giuui

import (
	"math"
	"time"

	"github.com/AllenDang/cimgui-go/implot"
	g "github.com/AllenDang/giu"
	"sound-signal-generator/internal/controller"
	"sound-signal-generator/internal/dsp"
	"sound-signal-generator/internal/ui/i18n"
)

const (
	spectrumYMax = 95.0
)

// SpectrumWidget renders a real-time 1/3-octave frequency domain visualizer (FFT).
// It features smooth ballistics (attack, decay and peak-hold) with dual-channel bars,
// and dynamically adapts its upper frequency range based on configured signal parameters.
type SpectrumWidget struct {
	state    *UIState
	analyzer *dsp.SpectrumAnalyzer

	now          func() time.Time
	previewStart time.Time
	lastWave     controller.WaveformType

	lastSampleRate int
	lastTargetMax  float64

	noiseGen  dsp.Generator
	noiseWave controller.WaveformType
	noiseRate int
	noiseSeed int64

	samples []float64
	left    []float64
	right   []float64

	xTicks []g.PlotTicker
	yTicks []g.PlotTicker
}

// NewSpectrumWidget creates a newly initialized SpectrumWidget.
func NewSpectrumWidget(state *UIState) *SpectrumWidget {
	now := time.Now()
	sampleRate := 48000
	if int(state.SelectedSampleRateIndex) >= 0 && int(state.SelectedSampleRateIndex) < len(SampleRateOptions) {
		sampleRate = SampleRateOptions[state.SelectedSampleRateIndex]
	}

	cfg := dsp.DefaultSpectrumConfig(sampleRate)
	analyzer := dsp.NewSpectrumAnalyzer(cfg)

	w := &SpectrumWidget{
		state:          state,
		analyzer:       analyzer,
		now:            time.Now,
		previewStart:   now,
		lastSampleRate: sampleRate,
		lastTargetMax:  dsp.BaseMaxTargetFreq,
		samples:        make([]float64, analyzer.FFTSize()),
		left:           make([]float64, analyzer.FFTSize()),
		right:          make([]float64, analyzer.FFTSize()),
	}

	w.initTicks()
	return w
}

func (w *SpectrumWidget) initTicks() {
	bands := w.analyzer.Bands()
	milestones := []float64{
		25, 63, 125, 250, 500,
		1000, 2000, 4000, 8000, 16000, 20000,
		31500, 40000, 63000, 80000, 125000, 160000,
	}

	isMilestone := func(fc float64) bool {
		for _, m := range milestones {
			if math.Abs(fc-m) < 1.0 {
				return true
			}
		}
		return false
	}

	w.xTicks = make([]g.PlotTicker, 0, len(bands))
	for i, b := range bands {
		if isMilestone(b.CenterFreq) || i == len(bands)-1 {
			w.xTicks = append(w.xTicks, g.PlotTicker{
				Position: float64(i),
				Label:    b.Label,
			})
		}
	}

	// Y-axis ticks for dBFS levels
	w.yTicks = []g.PlotTicker{
		{Position: 0, Label: "-90"},
		{Position: 15, Label: "-75"},
		{Position: 30, Label: "-60"},
		{Position: 45, Label: "-45"},
		{Position: 60, Label: "-30"},
		{Position: 75, Label: "-15"},
		{Position: 90, Label: "0 dB"},
	}
}

// targetMaxFrequency inspects configured signal parameters to determine the required display ceiling.
func (w *SpectrumWidget) targetMaxFrequency() float64 {
	maxFreq := dsp.BaseMaxTargetFreq
	wave := w.selectedWaveform()

	switch wave {
	case controller.WaveformSine,
		controller.WaveformPolyBlepSquare,
		controller.WaveformPolyBlepTriangle:
		if float64(w.state.Frequency) > maxFreq {
			maxFreq = float64(w.state.Frequency)
		}

	case controller.WaveformLinearSweep,
		controller.WaveformLogarithmicSweep:
		if float64(w.state.SweepEndFreq) > maxFreq {
			maxFreq = float64(w.state.SweepEndFreq)
		}
		if float64(w.state.SweepStartFreq) > maxFreq {
			maxFreq = float64(w.state.SweepStartFreq)
		}

	case controller.WaveformWhiteNoise,
		controller.WaveformPinkNoise:
		// Full band noise uses maximum safe bandwidth of the output device
		maxSafe := float64(w.sampleRate()) * 0.45
		if maxSafe > maxFreq {
			maxFreq = maxSafe
		}
	}

	return maxFreq
}

// ensureAnalyzerConfig dynamically aligns the spectrum bands with the current signal parameters.
func (w *SpectrumWidget) ensureAnalyzerConfig() {
	sampleRate := w.sampleRate()
	targetMax := w.targetMaxFrequency()

	if sampleRate != w.lastSampleRate || math.Abs(targetMax-w.lastTargetMax) > 1.0 {
		w.analyzer.Configure(sampleRate, targetMax)
		w.initTicks()
		w.lastSampleRate = sampleRate
		w.lastTargetMax = targetMax
	}

	fftSize := w.analyzer.FFTSize()
	if len(w.samples) != fftSize {
		w.samples = make([]float64, fftSize)
		w.left = make([]float64, fftSize)
		w.right = make([]float64, fftSize)
	}
}

// NeedsAnimation reports whether the visualizer requires continuous frame refreshes.
// When audio output is live, ballistics decay requires continuous rendering.
func (w *SpectrumWidget) NeedsAnimation() bool {
	return w.showLiveTrace()
}

func (w *SpectrumWidget) showLiveTrace() bool {
	return w.state.IsOutputRunning()
}

func (w *SpectrumWidget) sampleRate() int {
	if int(w.state.SelectedSampleRateIndex) >= 0 && int(w.state.SelectedSampleRateIndex) < len(SampleRateOptions) {
		return SampleRateOptions[w.state.SelectedSampleRateIndex]
	}
	return 48000
}

func (w *SpectrumWidget) selectedWaveform() controller.WaveformType {
	index := clampIndex(w.state.SelectedWaveformIndex, len(WaveformOptions))
	if len(WaveformOptions) == 0 {
		return controller.WaveformSine
	}
	return WaveformOptions[index].ID
}

func (w *SpectrumWidget) previewElapsed(now time.Time) float64 {
	anchor := w.previewStart
	if !w.state.SignalStartTime.IsZero() {
		anchor = w.state.SignalStartTime
	}
	if anchor.IsZero() {
		anchor = now
	}
	elapsed := now.Sub(anchor).Seconds()
	if elapsed < 0 {
		return 0
	}
	return elapsed
}

func (w *SpectrumWidget) channelGains(wave controller.WaveformType) (float64, float64) {
	if wave == controller.WaveformDoP {
		return 1.0, 1.0
	}
	lGain := dbToLinear(float64(w.state.LeftVolumeDBFS))
	rGain := dbToLinear(float64(w.state.RightVolumeDBFS))

	switch ChannelMode(w.state.SelectedChannelMode) {
	case ChannelModeSoloL:
		rGain = 0
	case ChannelModeSoloR:
		lGain = 0
	case ChannelModeInvertR:
		rGain = -rGain
	case ChannelModeMono:
		monoGain := (lGain + rGain) * 0.5
		lGain = monoGain
		rGain = monoGain
	}
	return lGain, rGain
}

func (w *SpectrumWidget) ensureNoiseGenerator(wave controller.WaveformType, sampleRate int) {
	seed := int64(w.state.NoiseSeed)
	if seed == 0 {
		if wave == controller.WaveformPinkNoise {
			seed = 54321
		} else {
			seed = 12345
		}
	}
	if w.noiseGen != nil && w.noiseWave == wave && w.noiseRate == sampleRate && w.noiseSeed == seed {
		return
	}

	var gen dsp.Generator
	var err error
	noiseType := dsp.NoiseTypeWhite
	if wave == controller.WaveformPinkNoise {
		noiseType = dsp.NoiseTypePink
		if pink := dsp.NewVossPinkNoiseGenerator(); pink != nil {
			err = pink.PrepareNoise(dsp.NoiseConfig{
				SampleRate: sampleRate,
				Amplitude:  1.0,
				Seed:       uint64(seed),
				Type:       noiseType,
			})
			gen = pink
		}
	} else {
		if white := dsp.NewGaussianWhiteNoiseGenerator(); white != nil {
			err = white.PrepareNoise(dsp.NoiseConfig{
				SampleRate: sampleRate,
				Amplitude:  1.0,
				Seed:       uint64(seed),
				Type:       noiseType,
			})
			gen = white
		}
	}

	if err != nil {
		w.noiseGen = nil
		return
	}
	w.noiseGen = gen
	w.noiseWave = wave
	w.noiseRate = sampleRate
	w.noiseSeed = seed
}

// synthesizeSamples fills Left and Right audio buffers at the physical sample rate.
func (w *SpectrumWidget) synthesizeSamples(now time.Time) {
	wave := w.selectedWaveform()
	if wave != w.lastWave {
		w.previewStart = now
		w.lastWave = wave
	}

	sampleRate := w.sampleRate()
	dt := 1.0 / float64(sampleRate)
	elapsed := w.previewElapsed(now)
	freq := math.Max(1.0, float64(w.state.Frequency))
	fftPoints := len(w.samples)

	switch wave {
	case controller.WaveformSine:
		for i := 0; i < fftPoints; i++ {
			t := elapsed + float64(i)*dt
			w.samples[i] = math.Sin(twoPi * freq * t)
		}

	case controller.WaveformPolyBlepSquare:
		for i := 0; i < fftPoints; i++ {
			t := elapsed + float64(i)*dt
			w.samples[i] = math.Copysign(1.0, math.Sin(twoPi*freq*t))
		}

	case controller.WaveformPolyBlepTriangle:
		for i := 0; i < fftPoints; i++ {
			t := elapsed + float64(i)*dt
			w.samples[i] = (2.0 / math.Pi) * math.Asin(math.Sin(twoPi*freq*t))
		}

	case controller.WaveformLinearSweep, controller.WaveformLogarithmicSweep:
		start := math.Max(1.0, float64(w.state.SweepStartFreq))
		end := math.Max(1.0, float64(w.state.SweepEndFreq))
		duration := math.Max(0.1, float64(w.state.SweepDurationSec))
		isLog := wave == controller.WaveformLogarithmicSweep

		for i := 0; i < fftPoints; i++ {
			t := elapsed + float64(i)*dt
			if w.state.SweepLoop && duration > 0 {
				t = math.Mod(t, duration)
			}
			phase := sweepPhaseAt(start, end, duration, isLog, t)
			w.samples[i] = math.Sin(phase)
		}

	case controller.WaveformWhiteNoise, controller.WaveformPinkNoise:
		w.ensureNoiseGenerator(wave, sampleRate)
		if w.noiseGen != nil {
			w.noiseGen.Render(w.samples)
		} else {
			for i := range w.samples {
				w.samples[i] = 0
			}
		}

	case controller.WaveformMultitoneSMPTE:
		for i := 0; i < fftPoints; i++ {
			t := elapsed + float64(i)*dt
			w.samples[i] = 0.8*math.Sin(twoPi*60.0*t) + 0.2*math.Sin(twoPi*7000.0*t)
		}

	case controller.WaveformMultitoneCCIF:
		for i := 0; i < fftPoints; i++ {
			t := elapsed + float64(i)*dt
			w.samples[i] = 0.5*math.Sin(twoPi*19000.0*t) + 0.5*math.Sin(twoPi*20000.0*t)
		}

	case controller.WaveformDoP:
		for i := 0; i < fftPoints; i++ {
			t := elapsed + float64(i)*dt
			w.samples[i] = 0.8 * math.Sin(twoPi*1000.0*t)
		}

	default:
		for i := 0; i < fftPoints; i++ {
			t := elapsed + float64(i)*dt
			w.samples[i] = math.Sin(twoPi * freq * t)
		}
	}

	gainL, gainR := w.channelGains(wave)
	for i, s := range w.samples {
		w.left[i] = s * gainL
		w.right[i] = s * gainR
	}
}

// Build creates the Giu Plot widget for frequency spectrum analysis.
func (w *SpectrumWidget) Build(height int) g.Widget {
	if height < 120 {
		height = 120
	}

	w.ensureAnalyzerConfig()

	if !w.showLiveTrace() {
		w.analyzer.Reset()
		return w.buildStandbyPlot(height)
	}

	now := w.now()
	w.synthesizeSamples(now)
	barsL, barsR, _, _ := w.analyzer.Process(w.left, w.right, now)

	bandsCount := float64(len(w.analyzer.Bands()))

	return g.Plot("##spectrum").
		Size(-1, int(px(float32(height)))).
		SetXAxisLabel(g.AxisX1, w.formatAxisLabel(now)).
		SetYAxisLabel(g.AxisY1, i18n.T("spectrum.amplitude_dbfs")).
		AxisLimits(-0.6, bandsCount-0.4, 0, spectrumYMax, g.ConditionAlways).
		XTicks(w.xTicks, false).
		YTicks(w.yTicks, false, g.ImPlotYAxisLeft).
		Plots(
			g.Bar("L", barsL).Width(0.36).Shift(-0.19),
			g.Bar("R", barsR).Width(0.36).Shift(0.19),
		)
}

func (w *SpectrumWidget) buildStandbyPlot(height int) g.Widget {
	bandsCount := float64(len(w.analyzer.Bands()))
	return g.Plot("##spectrum").
		Size(-1, int(px(float32(height)))).
		SetXAxisLabel(g.AxisX1, w.formatAxisLabel(time.Time{})).
		SetYAxisLabel(g.AxisY1, i18n.T("spectrum.amplitude_dbfs")).
		AxisLimits(-0.6, bandsCount-0.4, 0, spectrumYMax, g.ConditionAlways).
		XTicks(w.xTicks, false).
		YTicks(w.yTicks, false, g.ImPlotYAxisLeft).
		Plots(
			g.Custom(func() {
				centerIdx := bandsCount * 0.5
				implot.PlotText(i18n.T("spectrum.standby"), centerIdx, 55.0)
				implot.PlotText(i18n.T("spectrum.standby_hint"), centerIdx, 42.0)
			}),
		)
}

// sweepDisplayState computes current momentary frequency and progress for sweeps.
func (w *SpectrumWidget) sweepDisplayState(wave controller.WaveformType, elapsed float64) (float64, float64) {
	start := math.Max(1.0, float64(w.state.SweepStartFreq))
	end := math.Max(1.0, float64(w.state.SweepEndFreq))
	duration := math.Max(0.5, float64(w.state.SweepDurationSec))
	loop := w.state.SweepLoop

	t := elapsed
	if loop {
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
	currentFreq := sweepFrequencyAt(start, end, duration, progress, wave == controller.WaveformLogarithmicSweep)
	return currentFreq, progress
}

// formatAxisLabel constructs the informational text displayed along the frequency X axis,
// including current physical frequency, sweep progress, and FFT resolution.
func (w *SpectrumWidget) formatAxisLabel(now time.Time) string {
	baseLabel := i18n.T("spectrum.frequency_axis")
	if !w.showLiveTrace() {
		return baseLabel + "  ·  " + i18n.T("spectrum.standby")
	}

	wave := w.selectedWaveform()
	var signalDesc string

	switch wave {
	case controller.WaveformSine,
		controller.WaveformPolyBlepSquare,
		controller.WaveformPolyBlepTriangle:
		freq := math.Max(1.0, float64(w.state.Frequency))
		signalDesc = i18n.T("spectrum.signal_tone_fmt", formatFrequency(freq))

	case controller.WaveformLinearSweep,
		controller.WaveformLogarithmicSweep:
		elapsed := w.previewElapsed(now)
		sweepFreq, sweepProg := w.sweepDisplayState(wave, elapsed)
		signalDesc = i18n.T("spectrum.signal_sweep_fmt", formatFrequency(sweepFreq), sweepProg*100.0)

	case controller.WaveformWhiteNoise:
		signalDesc = i18n.T("spectrum.signal_white_noise")

	case controller.WaveformPinkNoise:
		signalDesc = i18n.T("spectrum.signal_pink_noise")

	case controller.WaveformMultitoneSMPTE:
		signalDesc = i18n.T("scope.smpte_axis")

	case controller.WaveformMultitoneCCIF:
		signalDesc = i18n.T("scope.ccif_axis")

	case controller.WaveformDoP:
		signalDesc = i18n.T("scope.dop_axis")

	default:
		freq := math.Max(1.0, float64(w.state.Frequency))
		signalDesc = i18n.T("spectrum.signal_tone_fmt", formatFrequency(freq))
	}

	sampleRate := w.sampleRate()
	fftSize := w.analyzer.FFTSize()
	deltaF := float64(sampleRate) / float64(fftSize)
	fftInfo := i18n.T("spectrum.fft_info_fmt", deltaF)

	return baseLabel + "  ·  " + signalDesc + "  ·  " + fftInfo
}
