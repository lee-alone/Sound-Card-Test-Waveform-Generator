package giuui

import (
	"fmt"
	"math"
	"time"

	"github.com/AllenDang/cimgui-go/imgui"
	"github.com/AllenDang/cimgui-go/implot"
	g "github.com/AllenDang/giu"
	"sound-signal-generator/internal/controller"
	"sound-signal-generator/internal/dsp"
)

const (
	scopeCycleWindow    = 3.0
	scopeNoiseWindowSec = 0.010
	scopeDefaultPoints  = 480
	scopeMinPoints      = 32
	scopeTickIntervals  = 5

	scopeDefaultXZoom  = 1.0
	scopeMinXZoom      = 0.25
	scopeMaxXZoom      = 8.0
	scopeWheelZoomStep = 0.90
)

// ScopeFrame contains a display-ready time-domain trace. X is in seconds so
// every waveform shares one physical axis model instead of plotting point IDs.
type ScopeFrame struct {
	X           []float64
	Left        []float64
	Right       []float64
	DurationSec float64
	AxisLabel   string
}

// ScopeWidget renders a deterministic preview of the synthesized signal. The
// plot is intentionally larger than every other control: it is the fastest way
// to confirm waveform, timing, routing and level before engaging hardware.
type ScopeWidget struct {
	state *UIState

	now          func() time.Time
	previewStart time.Time
	lastWave     controller.WaveformType
	plotHovered  bool
	xZoom        float64

	noiseGen  dsp.Generator
	noiseWave controller.WaveformType
	noiseRate int
	noiseSeed int64
}

func NewScopeWidget(state *UIState) *ScopeWidget {
	now := time.Now()
	return &ScopeWidget{
		state:        state,
		now:          time.Now,
		previewStart: now,
		xZoom:        scopeDefaultXZoom,
	}
}

// NeedsAnimation reports whether the selected preview depends on advancing
// time. Periodic waves are intentionally stationary so the trace remains easy
// to read; noise and sweeps must keep moving.
func (s *ScopeWidget) NeedsAnimation() bool {
	if !s.showLiveTrace() {
		return false
	}

	switch s.selectedWaveform() {
	case controller.WaveformLinearSweep,
		controller.WaveformLogarithmicSweep,
		controller.WaveformWhiteNoise,
		controller.WaveformPinkNoise:
		return true
	default:
		return false
	}
}

// showLiveTrace gates the oscilloscope on the physical output state. A
// stopped generator has no signal to display, so the UI must not synthesize a
// preview or advance any display clock while in standby.
func (s *ScopeWidget) showLiveTrace() bool {
	return s.state.IsOutputRunning()
}

// GenerateTraces preserves the original trace-only API used by existing tests
// and callers.
func (s *ScopeWidget) GenerateTraces(numPoints int) ([]float64, []float64) {
	frame := s.GenerateFrame(numPoints)
	return frame.Left, frame.Right
}

// GenerateFrame builds one time-domain preview frame. X is measured from the
// start of the display window in seconds; callers render it with LineXY.
func (s *ScopeWidget) GenerateFrame(numPoints int) ScopeFrame {
	if numPoints < scopeMinPoints {
		numPoints = scopeDefaultPoints
	}

	wave := s.selectedWaveform()
	now := s.now()
	if s.previewStart.IsZero() {
		s.previewStart = now
	}

	// Restart the visual sweep/noise clock when the operator changes source.
	if wave != s.lastWave {
		s.previewStart = now
		s.lastWave = wave
		s.xZoom = scopeDefaultXZoom
	}

	sampleRate := s.sampleRate()
	samples := make([]float64, numPoints)

	duration := scopeCycleWindow / math.Max(1.0, float64(s.state.Frequency))
	axisDescription := "3 周期窗口"
	sweepFrequency := 0.0
	sweepProgress := 0.0

	switch wave {
	case controller.WaveformSine,
		controller.WaveformPolyBlepSquare,
		controller.WaveformPolyBlepTriangle:
		duration = scopeCycleWindow / math.Max(1.0, float64(s.state.Frequency))
		axisDescription = fmt.Sprintf("%.0fT 窗口 · %s", scopeCycleWindow, formatFrequency(float64(s.state.Frequency)))

	case controller.WaveformLinearSweep, controller.WaveformLogarithmicSweep:
		elapsed := s.previewElapsed(now)
		duration, sweepFrequency, sweepProgress = s.sweepDisplayWindow(wave, elapsed)
		axisDescription = fmt.Sprintf("固定时基 · 当前 %s · 进度 %.0f%%", formatFrequency(sweepFrequency), sweepProgress*100.0)

	case controller.WaveformWhiteNoise, controller.WaveformPinkNoise:
		duration = scopeNoiseWindowSec
		axisDescription = "10 ms 动态噪声窗口"

	case controller.WaveformMultitoneSMPTE:
		duration = scopeCycleWindow / 60.0
		axisDescription = "SMPTE 60 Hz + 7 kHz"

	case controller.WaveformMultitoneCCIF:
		duration = scopeCycleWindow / 19000.0
		axisDescription = "CCIF 19 kHz + 20 kHz"

	case controller.WaveformDoP:
		duration = scopeCycleWindow / 1000.0
		axisDescription = "DoP 载波示意"

	default:
		duration = scopeCycleWindow / math.Max(1.0, float64(s.state.Frequency))
	}

	if duration <= 0 || math.IsNaN(duration) || math.IsInf(duration, 0) {
		duration = 0.003
	}
	duration *= s.XZoom()
	if !s.XZoomIsDefault() {
		axisDescription += fmt.Sprintf(" · 窗口 %.2f×", s.XZoom())
	}

	x := make([]float64, numPoints)
	step := duration / float64(numPoints-1)
	for i := 0; i < numPoints; i++ {
		x[i] = float64(i) * step
	}

	switch wave {
	case controller.WaveformSine,
		controller.WaveformPolyBlepSquare,
		controller.WaveformPolyBlepTriangle:
		freq := math.Max(1.0, float64(s.state.Frequency))
		for i, t := range x {
			theta := twoPi * freq * t
			switch wave {
			case controller.WaveformSine:
				samples[i] = math.Sin(theta)
			case controller.WaveformPolyBlepSquare:
				samples[i] = math.Copysign(1.0, math.Sin(theta))
			case controller.WaveformPolyBlepTriangle:
				samples[i] = (2.0 / math.Pi) * math.Asin(math.Sin(theta))
			}
		}

	case controller.WaveformLinearSweep, controller.WaveformLogarithmicSweep:
		elapsed := s.previewElapsed(now)
		s.fillSweepPreview(samples, x, wave, elapsed)

	case controller.WaveformWhiteNoise, controller.WaveformPinkNoise:
		s.ensureNoiseGenerator(wave, sampleRate)
		if s.noiseGen != nil {
			s.noiseGen.Render(samples)
		}

	case controller.WaveformMultitoneSMPTE:
		for i, t := range x {
			samples[i] = 0.8*math.Sin(twoPi*60.0*t) + 0.2*math.Sin(twoPi*7000.0*t)
		}

	case controller.WaveformMultitoneCCIF:
		for i, t := range x {
			samples[i] = 0.5*math.Sin(twoPi*19000.0*t) + 0.5*math.Sin(twoPi*20000.0*t)
		}

	case controller.WaveformDoP:
		for i := range samples {
			tNorm := float64(i) / float64(numPoints-1)
			marker := 0.8
			if int(tNorm*16)%2 == 0 {
				marker = -0.8
			}
			samples[i] = marker * math.Sin(twoPi*4.0*tNorm)
		}

	default:
		for i, t := range x {
			samples[i] = math.Sin(twoPi * math.Max(1.0, float64(s.state.Frequency)) * t)
		}
	}

	gainL, gainR := s.channelGains(wave)
	left := make([]float64, numPoints)
	right := make([]float64, numPoints)
	for i, sample := range samples {
		left[i] = sample * gainL
		right[i] = sample * gainR
	}

	_, axisLabel := makeTimeAxis(duration)
	return ScopeFrame{
		X:           x,
		Left:        left,
		Right:       right,
		DurationSec: duration,
		AxisLabel:   axisLabel + " · " + axisDescription,
	}
}

func (s *ScopeWidget) selectedWaveform() controller.WaveformType {
	index := clampIndex(s.state.SelectedWaveformIndex, len(WaveformOptions))
	if len(WaveformOptions) == 0 {
		return controller.WaveformSine
	}
	return WaveformOptions[index].ID
}

func (s *ScopeWidget) sampleRate() int {
	if int(s.state.SelectedSampleRateIndex) >= 0 && int(s.state.SelectedSampleRateIndex) < len(SampleRateOptions) {
		return SampleRateOptions[s.state.SelectedSampleRateIndex]
	}
	return 48000
}

func (s *ScopeWidget) previewElapsed(now time.Time) float64 {
	base := s.previewStart
	if !s.state.StreamStartTime.IsZero() {
		base = s.state.StreamStartTime
	}
	elapsed := now.Sub(base).Seconds()
	if elapsed < 0 {
		return 0
	}
	return elapsed
}

func (s *ScopeWidget) sweepDisplayWindow(wave controller.WaveformType, elapsed float64) (float64, float64, float64) {
	start := math.Max(1.0, float64(s.state.SweepStartFreq))
	end := math.Max(1.0, float64(s.state.SweepEndFreq))
	duration := math.Max(0.5, float64(s.state.SweepDurationSec))
	loop := s.state.SweepLoop

	t := elapsed
	if loop {
		t = math.Mod(elapsed, duration)
		if t < 0 {
			t += duration
		}
	} else if t > duration {
		t = duration
	}
	progress := math.Max(0.0, math.Min(1.0, t/duration))
	frequency := sweepFrequencyAt(start, end, duration, progress, wave == controller.WaveformLogarithmicSweep)
	// Keep the sweep timebase fixed across the entire sweep. Re-deriving the
	// window from the instantaneous frequency makes every frame contain roughly
	// the same three cycles, so the curve appears stationary while only the
	// axis numbers change. The geometric center gives a useful compromise
	// between low-frequency readability and high-frequency density.
	referenceFrequency := math.Sqrt(start * end)
	window := scopeCycleWindow / math.Max(1.0, referenceFrequency)
	return window, frequency, progress
}

func (s *ScopeWidget) fillSweepPreview(dst, x []float64, wave controller.WaveformType, elapsed float64) {
	start := math.Max(1.0, float64(s.state.SweepStartFreq))
	end := math.Max(1.0, float64(s.state.SweepEndFreq))
	duration := math.Max(0.5, float64(s.state.SweepDurationSec))
	logarithmic := wave == controller.WaveformLogarithmicSweep

	t := elapsed
	if s.state.SweepLoop {
		t = math.Mod(elapsed, duration)
		if t < 0 {
			t += duration
		}
	} else if t > duration {
		t = duration
	}
	basePhase := sweepPhaseAt(start, end, duration, logarithmic, t)
	for i, offset := range x {
		phase := sweepPhaseAt(start, end, duration, logarithmic, t+offset) - basePhase
		dst[i] = math.Sin(phase)
	}
}

func (s *ScopeWidget) channelGains(wave controller.WaveformType) (float64, float64) {
	if wave == controller.WaveformDoP {
		return 1.0, 1.0
	}

	lGain := dbToLinear(float64(s.state.LeftVolumeDBFS))
	rGain := dbToLinear(float64(s.state.RightVolumeDBFS))

	switch ChannelMode(s.state.SelectedChannelMode) {
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

func (s *ScopeWidget) ensureNoiseGenerator(wave controller.WaveformType, sampleRate int) {
	seed := int64(s.state.NoiseSeed)
	if seed == 0 {
		if wave == controller.WaveformPinkNoise {
			seed = 54321
		} else {
			seed = 12345
		}
	}
	if s.noiseGen != nil && s.noiseWave == wave && s.noiseRate == sampleRate && s.noiseSeed == seed {
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
		s.noiseGen = nil
		return
	}
	s.noiseGen = gen
	s.noiseWave = wave
	s.noiseRate = sampleRate
	s.noiseSeed = seed
}

func (s *ScopeWidget) Build(height int) g.Widget {
	if height < 120 {
		height = 120
	}

	if !s.showLiveTrace() {
		s.plotHovered = false
		return s.buildStandbyPlot(height)
	}

	s.applyHoveredWheelZoom()

	frame := s.GenerateFrame(scopeDefaultPoints)
	ticks, _ := makeTimeAxis(frame.DurationSec)

	plot := g.Plot("##oscilloscope").
		Size(-1, int(px(float32(height)))).
		SetXAxisLabel(g.AxisX1, frame.AxisLabel).
		SetYAxisLabel(g.AxisY1, "幅度 / FS").
		AxisLimits(0, frame.DurationSec, -1.08, 1.08, g.ConditionAlways).
		XTicks(ticks, false).
		Plots(
			g.LineXY("L", frame.X, frame.Left),
			g.LineXY("R", frame.X, frame.Right),
			g.Custom(func() {
				hovered := implot.IsPlotHovered()
				if hovered && !s.plotHovered {
					s.applyXZoomWheel(float64(imgui.CurrentIO().MouseWheel()))
				}
				s.plotHovered = hovered
			}),
		)
	return plot
}

// buildStandbyPlot preserves the scope frame and grid while making it clear
// that no waveform is being generated. It intentionally contains no LineXY
// widgets, so opening the application does not start preview synthesis.
func (s *ScopeWidget) buildStandbyPlot(height int) g.Widget {
	return g.Plot("##oscilloscope").
		Size(-1, int(px(float32(height)))).
		SetXAxisLabel(g.AxisX1, "时间").
		SetYAxisLabel(g.AxisY1, "幅度 / FS").
		AxisLimits(0, 1, -1.08, 1.08, g.ConditionAlways).
		Plots(
			g.Custom(func() {
				implot.PlotText("输出待机", 0.5, 0.10)
				implot.PlotText("点击“开始输出”后显示波形", 0.5, -0.18)
			}),
		)
}

// XZoom returns the multiplier applied to the default X-axis window. Values
// below 1 zoom in; values above 1 zoom out.
func (s *ScopeWidget) XZoom() float64 {
	if s.xZoom <= 0 {
		return scopeDefaultXZoom
	}
	return s.xZoom
}

// XZoomIsDefault reports whether the reset control should be visible.
func (s *ScopeWidget) XZoomIsDefault() bool {
	return math.Abs(s.XZoom()-scopeDefaultXZoom) < 1e-6
}

// ResetXZoom restores the stable default time window.
func (s *ScopeWidget) ResetXZoom() {
	s.xZoom = scopeDefaultXZoom
}

func (s *ScopeWidget) applyHoveredWheelZoom() {
	if !s.plotHovered {
		return
	}
	s.applyXZoomWheel(float64(imgui.CurrentIO().MouseWheel()))
}

func (s *ScopeWidget) applyXZoomWheel(wheel float64) {
	if math.Abs(wheel) < 1e-6 {
		return
	}
	// Wheel up is positive and zooms in by shrinking the X window.
	next := s.XZoom() * math.Pow(scopeWheelZoomStep, wheel)
	next = math.Max(scopeMinXZoom, math.Min(scopeMaxXZoom, next))
	if math.Abs(next-scopeDefaultXZoom) < 1e-3 {
		next = scopeDefaultXZoom
	}
	s.xZoom = next
}

func makeTimeAxis(durationSec float64) ([]g.PlotTicker, string) {
	if durationSec <= 0 || math.IsNaN(durationSec) || math.IsInf(durationSec, 0) {
		durationSec = 0.003
	}

	unit := "ms"
	scale := 1000.0
	switch {
	case durationSec >= 1.0:
		unit = "s"
		scale = 1.0
	case durationSec >= 0.001:
		unit = "ms"
		scale = 1000.0
	default:
		unit = "µs"
		scale = 1_000_000.0
	}

	ticks := make([]g.PlotTicker, 0, scopeTickIntervals+1)
	for i := 0; i <= scopeTickIntervals; i++ {
		position := durationSec * float64(i) / float64(scopeTickIntervals)
		label := formatAxisTime(position*scale, durationSec*scale)
		ticks = append(ticks, g.PlotTicker{Position: position, Label: label})
	}
	return ticks, "时间 / " + unit
}

func formatAxisTime(value, span float64) string {
	if math.Abs(value) < 1e-12 {
		return "0"
	}
	switch {
	case span >= 100:
		return fmt.Sprintf("%.0f", value)
	case span >= 10:
		return fmt.Sprintf("%.1f", value)
	case span >= 1:
		return fmt.Sprintf("%.2f", value)
	default:
		return fmt.Sprintf("%.3f", value)
	}
}

func formatFrequency(hz float64) string {
	if hz >= 1000.0 {
		return fmt.Sprintf("%.2f kHz", hz/1000.0)
	}
	return fmt.Sprintf("%.0f Hz", hz)
}

func dbToLinear(dbfs float64) float64 {
	if dbfs >= 0 {
		return 1.0
	}
	if dbfs <= -144 {
		return 0.0
	}
	return math.Pow(10, dbfs/20.0)
}

func sweepFrequencyAt(start, end, duration, progress float64, logarithmic bool) float64 {
	progress = math.Max(0, math.Min(1, progress))
	if logarithmic && start > 0 && end > 0 && start != end {
		return start * math.Pow(end/start, progress)
	}
	return start + (end-start)*progress
}

func sweepPhaseAt(start, end, duration float64, logarithmic bool, t float64) float64 {
	if t <= 0 {
		return 0
	}
	if duration <= 0 {
		duration = 1
	}
	if logarithmic && start > 0 && end > 0 && start != end {
		k := math.Log(end/start) / duration
		if math.Abs(k) < 1e-15 {
			return twoPi * start * t
		}
		return twoPi * start * (math.Exp(k*t) - 1.0) / k
	}
	rate := (end - start) / duration
	return twoPi * (start*t + 0.5*rate*t*t)
}

const twoPi = 2.0 * math.Pi
