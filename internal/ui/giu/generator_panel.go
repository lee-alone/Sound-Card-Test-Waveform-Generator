package giuui

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync/atomic"

	"github.com/AllenDang/cimgui-go/imgui"
	g "github.com/AllenDang/giu"
	"sound-signal-generator/internal/controller"
	"sound-signal-generator/internal/ui/i18n"
)

type preciseFrequencyField struct {
	text    string
	editing bool
}

const (
	frequencySliderMin = 1.0
	frequencyWheelStep = 1.10
)

// GeneratorPanel owns the signal source, the time-domain preview and the
// channel-level rail. The oscilloscope is the visual anchor; controls stay in
// one adjacent column so the operator does not scan across the whole window.
type GeneratorPanel struct {
	uiService         *controller.UIService
	state             *UIState
	onParamChanged    func()
	requestDoPConfirm func()
	scopeWidget       *ScopeWidget
	spectrumWidget    *SpectrumWidget
	metrologyWidget   *MetrologyWidget
	benchPresets      *BenchPresetsBar
	animateScope      atomic.Bool

	frequencyField  preciseFrequencyField
	sweepStartField preciseFrequencyField
	sweepEndField   preciseFrequencyField
}

func NewGeneratorPanel(
	uiService *controller.UIService,
	state *UIState,
	onParamChanged func(),
	requestDoPConfirm func(),
) *GeneratorPanel {
	return &GeneratorPanel{
		uiService:         uiService,
		state:             state,
		onParamChanged:    onParamChanged,
		requestDoPConfirm: requestDoPConfirm,
		scopeWidget:       NewScopeWidget(state),
		spectrumWidget:    NewSpectrumWidget(state),
		metrologyWidget:   NewMetrologyWidget(state),
		benchPresets:      NewBenchPresetsBar(state, onParamChanged),
	}
}

func (p *GeneratorPanel) GetMaxSafeFrequency() float64 {
	targetRate := 48000
	if int(p.state.SelectedSampleRateIndex) >= 0 && int(p.state.SelectedSampleRateIndex) < len(SampleRateOptions) {
		targetRate = SampleRateOptions[p.state.SelectedSampleRateIndex]
	}
	return float64(targetRate) * 0.45
}

func (p *GeneratorPanel) selectedWaveform() controller.WaveformType {
	if int(p.state.SelectedWaveformIndex) >= 0 && int(p.state.SelectedWaveformIndex) < len(WaveformOptions) {
		return WaveformOptions[p.state.SelectedWaveformIndex].ID
	}
	return controller.WaveformSine
}

// ScopeNeedsAnimation is sampled by the UI refresh ticker. Keeping it on the
// panel makes the refresh policy follow the same waveform selection as Build.
func (p *GeneratorPanel) ScopeNeedsAnimation() bool {
	return p.animateScope.Load()
}

func (p *GeneratorPanel) IsNyquistExceeded() bool {
	maxSafe := p.GetMaxSafeFrequency()
	selectedWave := p.selectedWaveform()

	switch selectedWave {
	case controller.WaveformSine, controller.WaveformPolyBlepSquare, controller.WaveformPolyBlepTriangle:
		return float64(p.state.Frequency) > maxSafe
	case controller.WaveformLinearSweep, controller.WaveformLogarithmicSweep:
		return float64(p.state.SweepEndFreq) > maxSafe
	default:
		return false
	}
}

func (p *GeneratorPanel) StartPlayback() {
	if p.state.IsDoPSelected() && !p.state.DoPConfirmed {
		p.requestDoPConfirmation()
		p.state.StatusMessage = i18n.T("generator.dop_pending")
		return
	}

	if p.IsNyquistExceeded() {
		p.state.ErrorMessage = i18n.T("generator.error.nyquist")
		return
	}

	p.state.ErrorMessage = ""
	p.state.PendingSafetyStart = true
	p.onParamChanged()
	if p.state.ShowSafetyConfirm {
		return
	}
	p.state.PendingSafetyStart = false
	if p.state.ErrorMessage != "" {
		return
	}

	if err := p.uiService.StartPlayback(); err != nil {
		p.state.ErrorMessage = i18n.T("generator.error.start_fmt", err)
		return
	}
	p.state.StatusMessage = i18n.T("generator.status.running")
}

func (p *GeneratorPanel) requestDoPConfirmation() {
	if p.requestDoPConfirm != nil {
		p.requestDoPConfirm()
		return
	}
	p.state.ShowDoPConfirm = true
}

func (p *GeneratorPanel) onWaveformChanged() {
	if p.state.IsDoPSelected() {
		p.state.ApplyDoPConstraints()
		if !p.state.DoPConfirmed {
			p.requestDoPConfirmation()
			p.state.StatusMessage = i18n.T("generator.dop_pending")
			return
		}
	}
	p.onParamChanged()
}

func (p *GeneratorPanel) onDoPParametersChanged() {
	p.state.ApplyDoPConstraints()
	if !p.state.DoPConfirmed {
		p.requestDoPConfirmation()
		p.state.StatusMessage = i18n.T("generator.dop_pending")
		return
	}
	p.onParamChanged()
}

func (p *GeneratorPanel) StopPlayback() {
	p.state.ErrorMessage = ""
	if err := p.uiService.StopPlayback(); err != nil {
		p.state.ErrorMessage = i18n.T("generator.error.stop_fmt", err)
		return
	}
	p.state.StatusMessage = i18n.T("generator.status.muted")
}

func (p *GeneratorPanel) setFrequency(value float32) {
	maxSafe := float32(p.GetMaxSafeFrequency())
	if value > maxSafe {
		value = maxSafe
	}
	p.state.Frequency = value
	p.onParamChanged()
}

// adjustFrequencyForWheel applies multiplicative wheel steps so the control
// remains useful across the full audio band instead of changing by fixed Hz.
func adjustFrequencyForWheel(current, minValue, maxValue, wheel float64) float64 {
	if maxValue < minValue {
		maxValue = minValue
	}
	if math.Abs(wheel) < 1e-6 {
		return math.Max(minValue, math.Min(maxValue, current))
	}
	next := current * math.Pow(frequencyWheelStep, wheel)
	next = math.Max(minValue, math.Min(maxValue, next))

	precision := 0.1
	switch {
	case next >= 1000:
		precision = 10
	case next >= 100:
		precision = 1
	}
	next = math.Round(next/precision) * precision
	return math.Max(minValue, math.Min(maxValue, next))
}

// logarithmicFrequencySlider keeps the visual drag logarithmic while exposing
// the real frequency in Hz. Wheel adjustment is accepted only while hovered.
func (p *GeneratorPanel) logarithmicFrequencySlider(id string, value *float32, maxValue, width float32) g.Widget {
	return g.Custom(func() {
		valueMin := float32(frequencySliderMin)
		imgui.SetNextItemWidth(width)
		changed := imgui.SliderFloatV(
			id,
			value,
			valueMin,
			maxValue,
			"%.1f Hz",
			imgui.SliderFlagsLogarithmic,
		)
		if changed {
			p.onParamChanged()
		}

		if imgui.IsItemHovered() {
			// Claim the wheel before the surrounding scroll panel processes it.
			imgui.SetItemKeyOwner(imgui.KeyMouseWheelY)
			wheel := float64(imgui.CurrentIO().MouseWheel())
			next := adjustFrequencyForWheel(float64(*value), float64(valueMin), float64(maxValue), wheel)
			if math.Abs(next-float64(*value)) > 1e-9 {
				*value = float32(next)
				p.onParamChanged()
			}
		}
	})
}

func (p *GeneratorPanel) preciseFrequencyInput(field *preciseFrequencyField, value *float32, maxSafe float64, width float32, id string) g.Widget {
	return g.Custom(func() {
		if !field.editing {
			field.text = formatEditableFrequency(float64(*value))
		}
		g.InputText(&field.text).
			Label("Hz" + id).
			Hint("Hz").
			Flags(g.InputTextFlagsCharsDecimal | g.InputTextFlagsEnterReturnsTrue).
			Size(width).
			OnChange(func() {
				p.commitPreciseFrequency(field, value, maxSafe)
			}).
			Build()

		if imgui.IsItemActive() {
			field.editing = true
		}
		if imgui.IsItemDeactivated() {
			p.commitPreciseFrequency(field, value, maxSafe)
			field.editing = false
			field.text = formatEditableFrequency(float64(*value))
		}
	})
}

func (p *GeneratorPanel) commitPreciseFrequency(field *preciseFrequencyField, value *float32, maxSafe float64) {
	corrected := correctFrequencyInput(field.text, float64(*value), maxSafe)
	field.text = formatEditableFrequency(corrected)
	if math.Abs(corrected-float64(*value)) < 1e-9 {
		return
	}
	*value = float32(corrected)
	p.onParamChanged()
}

func (p *GeneratorPanel) applyLevel(dbFS float32) {
	p.state.LeftVolumeDBFS = dbFS
	p.state.RightVolumeDBFS = dbFS
	p.state.LinkChannels = true
	p.onParamChanged()
}

// Build uses a scope-first composition: source pickers on top, the live plot
// in the wide column and a dedicated parameter rail on the right.
func (p *GeneratorPanel) Build(width, height float32) g.Widget {
	contentW := width - px(40)
	selectedWave := p.selectedWaveform()
	streamLocked := p.state.IsOutputRunning()
	scopeNeedsAnim := p.scopeWidget.NeedsAnimation()
	if p.state.ScopeDisplayMode == ScopeDisplaySpectrum {
		scopeNeedsAnim = p.spectrumWidget.NeedsAnimation()
	}
	p.animateScope.Store(streamLocked && scopeNeedsAnim)
	waveIndex := clampIndex(p.state.SelectedWaveformIndex, len(WaveformOptions))
	wavePreview := WaveformLabelAt(waveIndex)

	waveLabels := WaveformLabels()
	channelIndex := p.state.SelectedChannelMode
	if channelIndex < 0 || int(channelIndex) >= len(ChannelModeLabelKeys) {
		channelIndex = 0
	}
	channelPreview := ChannelModeLabelAt(int(channelIndex))
	channelLabels := ChannelModeLabels()

	headerH := px(32)
	presetH := px(30)
	selectorH := px(34)
	bodyH := height - px(36) - headerH - presetH - selectorH - px(21)
	if bodyH < px(250) {
		bodyH = px(250)
	}

	railW := px(306)
	scopeW := contentW - railW - px(8)
	if scopeW < px(420) {
		scopeW = contentW * 0.68
		railW = contentW - scopeW - px(8)
	}

	waveComboW := px(286)
	routeComboW := px(226)
	if contentW < px(850) {
		waveComboW = contentW * 0.36
		routeComboW = contentW * 0.28
	}

	scopeResetControl := func() g.Widget {
		if p.state.ScopeDisplayMode != ScopeDisplayWaveform || p.scopeWidget.XZoomIsDefault() {
			return g.Dummy(px(1), px(1))
		}
		return StyleQuietButton(
			g.Button(i18n.T("generator.restore_x_axis")).
				Size(px(108), px(22)).
				OnClick(p.scopeWidget.ResetXZoom),
		)
	}()

	scopeSegmentedControl := func() g.Widget {
		isWave := p.state.ScopeDisplayMode == ScopeDisplayWaveform
		btnWave := g.Button(i18n.T("generator.scope.tab_waveform")).Size(px(88), px(22)).OnClick(func() {
			p.state.ScopeDisplayMode = ScopeDisplayWaveform
		})
		btnSpec := g.Button(i18n.T("generator.scope.tab_spectrum")).Size(px(88), px(22)).OnClick(func() {
			p.state.ScopeDisplayMode = ScopeDisplaySpectrum
		})

		var w1, w2 g.Widget
		if isWave {
			w1 = StylePrimaryButton(btnWave)
			w2 = StyleQuietButton(btnSpec)
		} else {
			w1 = StyleQuietButton(btnWave)
			w2 = StylePrimaryButton(btnSpec)
		}
		return g.Row(w1, w2)
	}()

	scopeHeader := g.Row(
		scopeSegmentedControl,
		g.Spacing(),
		func() g.Widget {
			if streamLocked {
				return StyleBadge(i18n.T("generator.scope.live"), ColorPhosphorDark, ColorPhosphorGreen)
			}
			return StyleBadge(i18n.T("generator.scope.standby"), ColorSurfaceRaise, ColorTextMuted)
		}(),
		g.Spacing(),
		fontLabel(waveDescription(selectedWave), 10.5, ColorTextMuted),
		g.Spacing(),
		fontLabel(i18n.T("generator.scope.left"), 10.5, ColorDigitalCyan),
		g.Spacing(),
		fontLabel(i18n.T("generator.scope.right"), 10.5, ColorAmberWarning),
		g.Spacing(),
		scopeResetControl,
	)
	scopeLogicalHeight := int(bodyH/px(1)) - 92
	if scopeLogicalHeight < 150 {
		scopeLogicalHeight = 150
	}

	var displayPlot g.Widget
	if p.state.ScopeDisplayMode == ScopeDisplaySpectrum {
		displayPlot = p.spectrumWidget.Build(scopeLogicalHeight)
	} else {
		displayPlot = p.scopeWidget.Build(scopeLogicalHeight)
	}

	scopeColumn := barePanel(
		scopeW,
		bodyH,
		ColorSurface,
		scopeHeader,
		displayPlot,
		p.metrologyWidget.Build(),
	)

	railContentW := railW - px(40)
	widgets := []g.Widget{
		sectionHeader(i18n.T("generator.signal_params"), ""),
	}

	switch selectedWave {
	case controller.WaveformSine, controller.WaveformPolyBlepSquare, controller.WaveformPolyBlepTriangle:
		frequencyFieldW := px(86)
		frequencyInputW := px(64)
		frequencySliderW := railContentW - frequencyFieldW - px(8)
		widgets = append(widgets,
			captionLabel(i18n.T("generator.frequency_fmt", p.GetMaxSafeFrequency())),
			g.Row(
				p.logarithmicFrequencySlider("##frequencyLogSlider", &p.state.Frequency, float32(p.GetMaxSafeFrequency()), frequencySliderW),
				g.Spacing(),
				p.preciseFrequencyInput(&p.frequencyField, &p.state.Frequency, p.GetMaxSafeFrequency(), frequencyInputW, "##frequencyInput"),
			),
			g.Row(
				quickButton("100", railContentW, func() { p.setFrequency(100) }),
				quickButton("440", railContentW, func() { p.setFrequency(440) }),
				quickButton("1k", railContentW, func() { p.setFrequency(1000) }),
				quickButton("10k", railContentW, func() { p.setFrequency(10000) }),
				quickButton("20k", railContentW, func() { p.setFrequency(20000) }),
			),
		)

	case controller.WaveformLinearSweep, controller.WaveformLogarithmicSweep:
		frequencyFieldW := px(86)
		frequencyInputW := px(64)
		frequencySliderW := railContentW - frequencyFieldW - px(8)
		widgets = append(widgets,
			captionLabel(i18n.T("generator.sweep.start")),
			g.Row(
				p.logarithmicFrequencySlider("##sweepStartLogSlider", &p.state.SweepStartFreq, float32(p.GetMaxSafeFrequency()), frequencySliderW),
				g.Spacing(),
				p.preciseFrequencyInput(&p.sweepStartField, &p.state.SweepStartFreq, p.GetMaxSafeFrequency(), frequencyInputW, "##sweepStartInput"),
			),
			captionLabel(i18n.T("generator.sweep.end")),
			g.Row(
				p.logarithmicFrequencySlider("##sweepEndLogSlider", &p.state.SweepEndFreq, float32(p.GetMaxSafeFrequency()), frequencySliderW),
				g.Spacing(),
				p.preciseFrequencyInput(&p.sweepEndField, &p.state.SweepEndFreq, p.GetMaxSafeFrequency(), frequencyInputW, "##sweepEndInput"),
			),
			captionLabel(i18n.T("generator.sweep.duration")),
			g.SliderFloat(&p.state.SweepDurationSec, 0.5, 30.0).
				Format(i18n.T("generator.sweep.seconds_fmt")).Size(railContentW).OnChange(p.onParamChanged),
			g.Checkbox(i18n.T("generator.sweep.loop"), &p.state.SweepLoop).OnChange(p.onParamChanged),
		)

	case controller.WaveformWhiteNoise, controller.WaveformPinkNoise:
		widgets = append(widgets,
			fontLabel(i18n.T("generator.noise.full_band"), 13, ColorTextBright),
			captionLabel(waveDescription(selectedWave)),
		)

	case controller.WaveformMultitoneSMPTE, controller.WaveformMultitoneCCIF:
		widgets = append(widgets,
			fontLabel(i18n.T("generator.multitone.imd"), 13, ColorTextBright),
			captionLabel(waveDescription(selectedWave)),
		)

	case controller.WaveformDoP:
		formatIndex := clampIndex(p.state.DoPFormatIndex, len(DoPFormatOptions))
		bitOrderIndex := clampIndex(p.state.DoPBitOrderIndex, len(DoPBitOrderLabelKeys))
		formatLabels := DoPFormatLabels()
		bitOrderLabels := DoPBitOrderLabels()
		format := DoPFormatOptions[formatIndex]
		bitOrder := DoPBitOrderOptions[bitOrderIndex]

		widgets = append(widgets,
			fontLabel(i18n.T("generator.dop.title"), 13, ColorTextBright),
			captionLabel(i18n.T("generator.dop.description")),
			captionLabel(i18n.T("generator.dop.stream_format")),
			disableWhen(streamLocked,
				g.Combo("##dopFormat", i18n.T(format.LabelKey), formatLabels, &p.state.DoPFormatIndex).
					Size(railContentW).
					OnChange(p.onDoPParametersChanged)),
			captionLabel(i18n.T("generator.bit_order")),
			disableWhen(streamLocked,
				g.Combo("##dopBitOrder", DoPBitOrderLabelAt(bitOrderIndex), bitOrderLabels, &p.state.DoPBitOrderIndex).
					Size(railContentW).
					OnChange(p.onDoPParametersChanged)),
			g.Separator(),
			lockedParamRow(i18n.T("generator.dop.carrier_rate"), fmt.Sprintf("%.1f kHz", float64(format.CarrierRate)/1000.0), railContentW),
			lockedParamRow(i18n.T("generator.pcm_container"), i18n.T("generator.pcm_container_forced"), railContentW),
			lockedParamRow(i18n.T("generator.output_gain"), "0 dBFS Unity Gain", railContentW),
			lockedParamRow(i18n.T("generator.bit_order"), bitOrder.String(), railContentW),
			func() g.Widget {
				if streamLocked {
					return captionLabel(i18n.T("generator.dop.locked_hint"))
				}
				return g.Dummy(px(1), px(1))
			}(),
			captionLabel(i18n.T("generator.dop.disabled_features")),
			fontLabel(i18n.T("generator.dop.disconnect"), 10.5, ColorAmberWarning),
		)

		if p.state.DoPConfirmed {
			widgets = append(widgets, StyleBadge(i18n.T("generator.dop.unlocked"), ColorPhosphorDark, ColorPhosphorGreen))
		} else {
			widgets = append(widgets,
				StyleDangerButton(
					g.Button(i18n.T("generator.dop.confirm")).
						Size(railContentW, px(32)).
						OnClick(p.requestDoPConfirmation),
				),
			)
		}
	}

	levelRow := func(label string, value *float32, sync func()) g.Widget {
		return g.Row(
			fontLabel(label, 11, ColorTextMuted),
			g.SliderFloat(value, -144.0, 0.0).
				Format("%.1f dBFS").
				Size(railContentW-px(26)).
				OnChange(func() {
					sync()
					p.onParamChanged()
				}),
		)
	}

	if selectedWave == controller.WaveformDoP {
		widgets = append(widgets,
			g.Separator(),
			fontLabel(i18n.T("generator.dop.fixed_level"), 12, ColorTextBright),
			lockedParamRow("L", "0.0 dBFS", railContentW),
			lockedParamRow("R", "0.0 dBFS", railContentW),
			captionLabel(i18n.T("generator.dop.unity_req")),
		)
	} else {
		widgets = append(widgets,
			g.Separator(),
			g.Row(
				fontLabel(i18n.T("generator.channel_level"), 12, ColorTextBright),
				g.Spacing(),
				g.Checkbox(i18n.T("generator.link"), &p.state.LinkChannels),
			),
			levelRow("L", &p.state.LeftVolumeDBFS, func() {
				if p.state.LinkChannels {
					p.state.RightVolumeDBFS = p.state.LeftVolumeDBFS
				}
			}),
			levelRow("R", &p.state.RightVolumeDBFS, func() {
				if p.state.LinkChannels {
					p.state.LeftVolumeDBFS = p.state.RightVolumeDBFS
				}
			}),
			g.Row(
				levelButton("0", railContentW, func() { p.applyLevel(0) }),
				levelButton("-6", railContentW, func() { p.applyLevel(-6) }),
				levelButton("-12", railContentW, func() { p.applyLevel(-12) }),
				levelButton("-60", railContentW, func() { p.applyLevel(-60) }),
			),
		)
	}

	rail := scrollPanel(railW, bodyH, ColorSurfaceRaise, widgets...)

	return panelBox(width, height, ColorSurface,
		g.Row(
			sectionHeader(i18n.T("generator.signal_generator"), ""),
			g.Spacing(),
			StyleBadge(channelPreview, ColorDigitalCyanDark, ColorDigitalCyan),
		),
		p.benchPresets.Build(),
		g.Row(
			captionLabel(i18n.T("generator.waveform")),
			g.Combo("##waveCombo", wavePreview, waveLabels, &p.state.SelectedWaveformIndex).
				Size(waveComboW).
				OnChange(p.onWaveformChanged),
			g.Spacing(),
			captionLabel(i18n.T("generator.routing")),
			g.Combo("##chanCombo", channelPreview, channelLabels, &p.state.SelectedChannelMode).
				Size(routeComboW).
				OnChange(p.onParamChanged),
		),
		g.Row(scopeColumn, rail),
	)
}

func waveDescription(wave controller.WaveformType) string {
	switch wave {
	case controller.WaveformSine:
		return i18n.T("wave.description.sine")
	case controller.WaveformPolyBlepSquare:
		return i18n.T("wave.description.square")
	case controller.WaveformPolyBlepTriangle:
		return i18n.T("wave.description.triangle")
	case controller.WaveformLinearSweep:
		return i18n.T("wave.description.linear")
	case controller.WaveformLogarithmicSweep:
		return i18n.T("wave.description.log")
	case controller.WaveformWhiteNoise:
		return i18n.T("wave.description.white")
	case controller.WaveformPinkNoise:
		return i18n.T("wave.description.pink")
	case controller.WaveformMultitoneSMPTE:
		return i18n.T("wave.description.smpte")
	case controller.WaveformMultitoneCCIF:
		return i18n.T("wave.description.ccif")
	case controller.WaveformDoP:
		return i18n.T("wave.description.dop")
	default:
		return i18n.T("wave.description.test")
	}
}

func quickButton(label string, contentW float32, onClick func()) g.Widget {
	width := (contentW - px(4*8)) / 5
	if width < px(36) {
		width = px(36)
	}
	return StyleQuietButton(g.Button(label).Size(width, px(26)).OnClick(onClick))
}

func levelButton(label string, contentW float32, onClick func()) g.Widget {
	width := (contentW - px(3*8)) / 4
	return StyleQuietButton(g.Button(label+" dB").Size(width, px(26)).OnClick(onClick))
}

func lockedParamRow(label, value string, contentW float32) g.Widget {
	return g.Row(
		fontLabel(label, 10.5, ColorTextMuted),
		g.Spacing(),
		fontLabel(value, 11, ColorTextBright),
	)
}

func formatEditableFrequency(hz float64) string {
	text := strconv.FormatFloat(hz, 'f', 3, 64)
	text = strings.TrimRight(text, "0")
	text = strings.TrimRight(text, ".")
	if text == "" || text == "-0" {
		return "0"
	}
	return text
}

// correctFrequencyInput silently repairs invalid or out-of-range operator
// input. Invalid text restores the previous value; numeric values are clamped
// to the supported generator range without opening an error dialog.
func correctFrequencyInput(raw string, current, maxSafe float64) float64 {
	minSafe := 1.0
	if maxSafe < minSafe {
		maxSafe = minSafe
	}
	current = math.Max(minSafe, math.Min(maxSafe, current))

	text := strings.TrimSpace(raw)
	text = strings.TrimSpace(strings.TrimSuffix(strings.TrimSuffix(text, "Hz"), "hz"))
	value, err := strconv.ParseFloat(text, 64)
	if err != nil || math.IsNaN(value) || math.IsInf(value, 0) {
		return current
	}
	return math.Max(minSafe, math.Min(maxSafe, value))
}

var _ = math.Pow
