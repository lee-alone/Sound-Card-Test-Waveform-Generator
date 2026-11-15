package giuui

import (
	"fmt"
	"math"

	g "github.com/AllenDang/giu"
	"sound-signal-generator/internal/controller"
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
	metrologyWidget   *MetrologyWidget
	benchPresets      *BenchPresetsBar
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
		p.state.StatusMessage = "DoP 待安全确认：输出保持静音"
		return
	}

	if p.IsNyquistExceeded() {
		p.state.ErrorMessage = "频率已超过奈奎斯特安全上限，禁止启动。"
		return
	}

	p.state.ErrorMessage = ""
	p.onParamChanged()
	if p.state.ErrorMessage != "" {
		return
	}

	if err := p.uiService.StartPlayback(); err != nil {
		p.state.ErrorMessage = fmt.Sprintf("启动输出失败: %v", err)
		return
	}
	p.state.StatusMessage = "信号输出中"
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
			p.state.StatusMessage = "DoP 待安全确认：输出保持静音"
			return
		}
	}
	p.onParamChanged()
}

func (p *GeneratorPanel) onDoPParametersChanged() {
	p.state.ApplyDoPConstraints()
	if !p.state.DoPConfirmed {
		p.requestDoPConfirmation()
		p.state.StatusMessage = "DoP 待安全确认：输出保持静音"
		return
	}
	p.onParamChanged()
}

func (p *GeneratorPanel) StopPlayback() {
	p.state.ErrorMessage = ""
	if err := p.uiService.StopPlayback(); err != nil {
		p.state.ErrorMessage = fmt.Sprintf("停止输出失败: %v", err)
		return
	}
	p.state.StatusMessage = "输出已静音"
}

func (p *GeneratorPanel) setFrequency(value float32) {
	maxSafe := float32(p.GetMaxSafeFrequency())
	if value > maxSafe {
		value = maxSafe
	}
	p.state.Frequency = value
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
	waveIndex := clampIndex(p.state.SelectedWaveformIndex, len(WaveformOptions))
	wavePreview := WaveformOptions[waveIndex].Label

	waveLabels := make([]string, len(WaveformOptions))
	for i, w := range WaveformOptions {
		waveLabels[i] = w.Label
	}
	channelIndex := p.state.SelectedChannelMode
	if channelIndex < 0 || int(channelIndex) >= len(ChannelModeLabels) {
		channelIndex = 0
	}
	channelPreview := ChannelModeLabels[channelIndex]

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

	scopeHeader := g.Row(
		captionLabel("时域预览"),
		g.Spacing(),
		fontLabel(waveDescription(selectedWave), 10.5, ColorTextMuted),
		g.Spacing(),
		fontLabel("L 左声道", 10.5, ColorDigitalCyan),
		g.Spacing(),
		fontLabel("R 右声道", 10.5, ColorAmberWarning),
	)
	scopeLogicalHeight := int(bodyH/px(1)) - 92
	if scopeLogicalHeight < 150 {
		scopeLogicalHeight = 150
	}

	scopeColumn := barePanel(
		scopeW,
		bodyH,
		ColorSurface,
		scopeHeader,
		p.scopeWidget.Build(scopeLogicalHeight),
		p.metrologyWidget.Build(),
	)

	railContentW := railW - px(40)
	widgets := []g.Widget{
		sectionHeader("信号参数", ""),
	}

	switch selectedWave {
	case controller.WaveformSine, controller.WaveformPolyBlepSquare, controller.WaveformPolyBlepTriangle:
		widgets = append(widgets,
			captionLabel(fmt.Sprintf("频率  ·  最高 %.0f Hz", p.GetMaxSafeFrequency())),
			g.SliderFloat(&p.state.Frequency, 1.0, float32(p.GetMaxSafeFrequency())).
				Format("%.1f Hz").
				Size(railContentW).
				OnChange(p.onParamChanged),
			g.Row(
				quickButton("100", railContentW, func() { p.setFrequency(100) }),
				quickButton("440", railContentW, func() { p.setFrequency(440) }),
				quickButton("1k", railContentW, func() { p.setFrequency(1000) }),
				quickButton("10k", railContentW, func() { p.setFrequency(10000) }),
				quickButton("20k", railContentW, func() { p.setFrequency(20000) }),
			),
		)

	case controller.WaveformLinearSweep, controller.WaveformLogarithmicSweep:
		widgets = append(widgets,
			captionLabel("起始频率"),
			g.SliderFloat(&p.state.SweepStartFreq, 1.0, float32(p.GetMaxSafeFrequency())).
				Format("%.0f Hz").Size(railContentW).OnChange(p.onParamChanged),
			captionLabel("终止频率"),
			g.SliderFloat(&p.state.SweepEndFreq, 1.0, float32(p.GetMaxSafeFrequency())).
				Format("%.0f Hz").Size(railContentW).OnChange(p.onParamChanged),
			captionLabel("扫描时长"),
			g.SliderFloat(&p.state.SweepDurationSec, 0.5, 30.0).
				Format("%.1f 秒").Size(railContentW).OnChange(p.onParamChanged),
			g.Checkbox("循环扫描", &p.state.SweepLoop).OnChange(p.onParamChanged),
		)

	case controller.WaveformWhiteNoise, controller.WaveformPinkNoise:
		widgets = append(widgets,
			fontLabel("全频带噪声", 13, ColorTextBright),
			captionLabel(waveDescription(selectedWave)),
		)

	case controller.WaveformMultitoneSMPTE, controller.WaveformMultitoneCCIF:
		widgets = append(widgets,
			fontLabel("互调失真测试", 13, ColorTextBright),
			captionLabel(waveDescription(selectedWave)),
		)

	case controller.WaveformDoP:
		formatIndex := clampIndex(p.state.DoPFormatIndex, len(DoPFormatOptions))
		bitOrderIndex := clampIndex(p.state.DoPBitOrderIndex, len(DoPBitOrderLabels))
		formatLabels := make([]string, len(DoPFormatOptions))
		for i, option := range DoPFormatOptions {
			formatLabels[i] = option.Label
		}
		format := DoPFormatOptions[formatIndex]
		bitOrder := DoPBitOrderOptions[bitOrderIndex]

		widgets = append(widgets,
			fontLabel("dCS DoP v1.1 载波", 13, ColorTextBright),
			captionLabel("1-bit DSD 封装于 24-bit PCM 帧；不是 Native DSD。"),
			captionLabel("DSD 位流规格"),
			g.Combo("##dopFormat", format.Label, formatLabels, &p.state.DoPFormatIndex).
				Size(railContentW).
				OnChange(p.onDoPParametersChanged),
			captionLabel("Bit Order"),
			g.Combo("##dopBitOrder", DoPBitOrderLabels[bitOrderIndex], DoPBitOrderLabels, &p.state.DoPBitOrderIndex).
				Size(railContentW).
				OnChange(p.onDoPParametersChanged),
			g.Separator(),
			lockedParamRow("载波采样率", fmt.Sprintf("%.1f kHz", float64(format.CarrierRate)/1000.0), railContentW),
			lockedParamRow("PCM 容器", "24-in-32 LE · 强制", railContentW),
			lockedParamRow("输出增益", "0 dBFS Unity Gain", railContentW),
			lockedParamRow("位序", bitOrder.String(), railContentW),
			captionLabel("淡化、左右电平与校准微调在此路径禁用。"),
			fontLabel("请先断开扬声器/耳机和模拟功放。", 10.5, ColorAmberWarning),
		)

		if p.state.DoPConfirmed {
			widgets = append(widgets, StyleBadge("DoP 已安全解锁", ColorPhosphorDark, ColorPhosphorGreen))
		} else {
			widgets = append(widgets,
				StyleDangerButton(
					g.Button("进行 DoP 安全确认").
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
			fontLabel("DoP 固定电平", 12, ColorTextBright),
			lockedParamRow("L", "0.0 dBFS", railContentW),
			lockedParamRow("R", "0.0 dBFS", railContentW),
			captionLabel("Unity Gain 是 DoP 位完美传输的必要条件。"),
		)
	} else {
		widgets = append(widgets,
			g.Separator(),
			g.Row(
				fontLabel("通道电平", 12, ColorTextBright),
				g.Spacing(),
				g.Checkbox("联动", &p.state.LinkChannels),
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
			sectionHeader("信号发生器", ""),
			g.Spacing(),
			StyleBadge(channelPreview, ColorDigitalCyanDark, ColorDigitalCyan),
		),
		p.benchPresets.Build(),
		g.Row(
			captionLabel("波形"),
			g.Combo("##waveCombo", wavePreview, waveLabels, &p.state.SelectedWaveformIndex).
				Size(waveComboW).
				OnChange(p.onWaveformChanged),
			g.Spacing(),
			captionLabel("路由"),
			g.Combo("##chanCombo", channelPreview, ChannelModeLabels, &p.state.SelectedChannelMode).
				Size(routeComboW).
				OnChange(p.onParamChanged),
		),
		g.Row(scopeColumn, rail),
	)
}

func waveDescription(wave controller.WaveformType) string {
	switch wave {
	case controller.WaveformSine:
		return "低失真正弦基准"
	case controller.WaveformPolyBlepSquare:
		return "带限方波 / PolyBLEP"
	case controller.WaveformPolyBlepTriangle:
		return "带限三角波 / PolyBLEP"
	case controller.WaveformLinearSweep:
		return "线性啁啾扫频"
	case controller.WaveformLogarithmicSweep:
		return "对数扫频 / 20 Hz–20 kHz"
	case controller.WaveformWhiteNoise:
		return "平坦功率谱白噪声"
	case controller.WaveformPinkNoise:
		return "按倍频程衰减的粉红噪声"
	case controller.WaveformMultitoneSMPTE:
		return "60 Hz + 7 kHz 互调"
	case controller.WaveformMultitoneCCIF:
		return "19 kHz + 20 kHz 差频"
	case controller.WaveformDoP:
		return "DSD over PCM 载波"
	default:
		return "测试信号"
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

var _ = math.Pow
