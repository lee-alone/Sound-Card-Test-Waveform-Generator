package giuui

import (
	"fmt"
	"image/color"
	"strings"

	g "github.com/AllenDang/giu"
	"sound-signal-generator/internal/audio"
	"sound-signal-generator/internal/controller"
)

// DevicePanel owns hardware selection, exclusive-mode policy, output format
// and capability probing. It is deliberately the only place that negotiates
// the physical output path.
type DevicePanel struct {
	uiService           *controller.UIService
	state               *UIState
	onFormatChanged     func()
	requestNativeDSDYes func()
}

func NewDevicePanel(uiService *controller.UIService, state *UIState, onFormatChanged func(), requestNativeDSDYes func()) *DevicePanel {
	return &DevicePanel{
		uiService:           uiService,
		state:               state,
		onFormatChanged:     onFormatChanged,
		requestNativeDSDYes: requestNativeDSDYes,
	}
}

func (p *DevicePanel) RefreshDevices() {
	devices, err := p.uiService.GetDevices()
	if err != nil {
		p.state.ErrorMessage = fmt.Sprintf("获取音频设备失败: %v", err)
		return
	}

	names := make([]string, 0, len(devices))
	ids := make([]string, 0, len(devices))
	backends := make([]audio.BackendType, 0, len(devices))
	defaultIdx := preferredDeviceIndex(devices)

	for _, dev := range devices {
		displayName := dev.Name
		if dev.IsDefault {
			displayName += "  [默认]"
		}
		if dev.State != audio.StateActive {
			displayName += fmt.Sprintf("  (%s)", dev.State.String())
		}
		names = append(names, displayName)
		ids = append(ids, dev.ID)
		backends = append(backends, dev.Backend)
	}

	if len(names) == 0 {
		names = []string{"无可用音频输出设备"}
		ids = []string{""}
		backends = []audio.BackendType{""}
	}

	p.state.DeviceNames = names
	p.state.DeviceIDs = ids
	p.state.DeviceBackends = backends
	p.state.SelectedDeviceIndex = defaultIdx
	if int(defaultIdx) < len(ids) && ids[defaultIdx] != "" {
		if err := p.uiService.SelectDevice(ids[defaultIdx]); err != nil {
			p.state.ErrorMessage = fmt.Sprintf("选择默认音频设备失败: %v", err)
		} else {
			p.state.ErrorMessage = ""
		}
	}
	p.RefreshAsioStatus()
}

// preferredDeviceIndex selects the Windows default WASAPI endpoint first.
// ASIO devices may be marked as defaults by the ASIO backend itself, but they
// are not the Windows default and can be unavailable to a 64-bit host.
func preferredDeviceIndex(devices []audio.DeviceInfo) int32 {
	wasapiDefault := -1
	wasapiActive := -1
	anyActive := -1
	for i, dev := range devices {
		if dev.State != audio.StateActive {
			continue
		}
		if anyActive < 0 {
			anyActive = i
		}
		if dev.Backend == audio.BackendWASAPI {
			if wasapiActive < 0 {
				wasapiActive = i
			}
			if dev.IsDefault {
				wasapiDefault = i
				break
			}
		}
	}
	switch {
	case wasapiDefault >= 0:
		return int32(wasapiDefault)
	case wasapiActive >= 0:
		return int32(wasapiActive)
	case anyActive >= 0:
		return int32(anyActive)
	default:
		return 0
	}
}

// RefreshAsioStatus updates the real driver mode, clock, buffer and capability report.
func (p *DevicePanel) RefreshAsioStatus() {
	if !p.state.IsAsioSelected() {
		p.state.AsioStatus = controller.AsioStatus{}
		p.state.ResetNativeDSDSession()
		return
	}
	status, err := p.uiService.GetAsioStatus()
	if err != nil {
		p.state.NativeDSDError = err.Error()
		return
	}
	p.state.AsioStatus = status
	p.state.NativeDSDConfirmed = status.NativeDSDConfirmed && status.NativeDSDMode
	if !status.NativeDSDMode {
		p.state.ShowNativeDSDConfirm = false
		p.state.NativeDSDConfirmation = ""
	}
	p.state.SelectedClockIndex = int32(status.ActiveClockIndex)
	if status.ActiveBufferSize > 0 {
		p.state.AsioBufferSize = int32(status.ActiveBufferSize)
	}
	if !status.NativeDSDMode && !p.state.IsDoPSelected() {
		if format, ok := asioSampleTypeToFormat(status.SampleType); ok {
			for i, option := range SampleFormatOptions {
				if option == format {
					p.state.SelectedSampleFormatIndex = int32(i)
					break
				}
			}
		}
	}
}

func asioSampleTypeToFormat(sampleType string) (audio.SampleFormat, bool) {
	switch {
	case strings.Contains(sampleType, "Int16"):
		return audio.SampleFormat16Int, true
	case strings.Contains(sampleType, "Int24"):
		return audio.SampleFormat24Packed, true
	case strings.Contains(sampleType, "Int32"):
		return audio.SampleFormat24In32, true
	case strings.Contains(sampleType, "Float32"):
		return audio.SampleFormat32Float, true
	default:
		return 0, false
	}
}

func (p *DevicePanel) RunProbe() {
	p.state.IsProbing = true
	p.state.ErrorMessage = ""
	go func() {
		results, err := p.uiService.ProbeDeviceCapabilities()
		p.state.IsProbing = false
		if err != nil {
			p.state.ErrorMessage = fmt.Sprintf("硬件能力探测失败: %v", err)
			g.Update()
			return
		}
		p.state.ProbeResults = results
		p.state.ShowProbeResults = true
		g.Update()
	}()
}

func (p *DevicePanel) Build(width, height float32) g.Widget {
	contentW := width - px(40)
	if contentW < px(220) {
		contentW = px(220)
	}

	currentDevPreview := "未发现设备"
	if int(p.state.SelectedDeviceIndex) >= 0 && int(p.state.SelectedDeviceIndex) < len(p.state.DeviceNames) {
		currentDevPreview = p.state.DeviceNames[p.state.SelectedDeviceIndex]
	}

	rateIndex := clampIndex(p.state.SelectedSampleRateIndex, len(SampleRateLabels))
	formatIndex := clampIndex(p.state.SelectedSampleFormatIndex, len(SampleFormatLabels))
	isDoP := p.state.IsDoPSelected()
	probeText := "检测支持格式"
	if p.state.IsProbing {
		probeText = "正在检测格式..."
	}

	exclusiveLabel := "WASAPI 独占模式"
	if p.state.IsAsioSelected() {
		exclusiveLabel = "ASIO 独占驱动（固定）"
	}
	var exclusiveControl g.Widget = g.Checkbox(exclusiveLabel, &p.state.IsExclusive).
		OnChange(func() {
			p.uiService.SetExclusiveMode(p.state.IsExclusive)
			p.onFormatChanged()
		})
	var trimControl g.Widget = g.SliderFloat(&p.state.MasterTrimDB, -6.0, 6.0).
		Format("%+.1f dB").
		Size(px(112)).
		OnChange(p.onFormatChanged)
	formatControls := []g.Widget{
		captionLabel("采样率"),
		g.Combo("##rateCombo", SampleRateLabels[rateIndex], SampleRateLabels, &p.state.SelectedSampleRateIndex).
			Size(contentW).
			OnChange(p.onFormatChanged),
		captionLabel("样本格式"),
		g.Combo("##fmtCombo", SampleFormatLabels[formatIndex], SampleFormatLabels, &p.state.SelectedSampleFormatIndex).
			Size(contentW).
			OnChange(p.onFormatChanged),
	}

	if isDoP {
		carrierRate := p.state.SelectedDoPFormat().DoPCarrierSampleRate()
		exclusiveControl = g.Row(
			captionLabel("DoP 独占模式"),
			g.Spacing(),
			StyleBadge("强制启用", ColorDigitalCyanDark, ColorDigitalCyan),
		)
		trimControl = fontLabel("0.0 dBFS · Unity Gain 锁定", 11, ColorTextMuted)
		formatControls = []g.Widget{
			captionLabel("DoP 载波格式"),
			lockedParamRow("采样率", fmt.Sprintf("%.1f kHz", float64(carrierRate)/1000.0), contentW),
			lockedParamRow("容器", "24-in-32 LE", contentW),
			fontLabel("载波参数由 DSD 规格自动决定，不允许手动修改。", 10.5, ColorTextMuted),
		}
	}

	deviceComboW := contentW - px(82) - px(8)
	widgets := []g.Widget{
		sectionHeader("输出设置", ""),
		g.Separator(),
		captionLabel("输出设备"),
		g.Row(
			g.Combo("##devCombo", currentDevPreview, p.state.DeviceNames, &p.state.SelectedDeviceIndex).
				Size(deviceComboW).
				OnChange(func() {
					if int(p.state.SelectedDeviceIndex) >= 0 && int(p.state.SelectedDeviceIndex) < len(p.state.DeviceIDs) {
						p.state.ResetNativeDSDSession()
						_ = p.uiService.SelectDevice(p.state.DeviceIDs[p.state.SelectedDeviceIndex])
						p.RefreshAsioStatus()
						p.onFormatChanged()
					}
				}),
			StyleQuietButton(g.Button("刷新").Size(px(82), px(32)).OnClick(p.RefreshDevices)),
		),
		captionLabel(truncateText(currentDevPreview, 44)),
		g.Row(
			exclusiveControl,
			g.Spacing(),
			captionLabel("校准微调"),
			trimControl,
		),
		g.Separator(),
	}

	widgets = append(widgets, formatControls...)

	if p.state.IsAsioSelected() {
		status := p.state.AsioStatus
		mode := status.OperatingMode
		if mode == "" {
			mode = "PCM"
		}
		sampleType := status.SampleType
		if sampleType == "" {
			sampleType = "未协商"
		}
		clockLabels := make([]string, len(status.ClockSources))
		for i, clock := range status.ClockSources {
			clockLabels[i] = fmt.Sprintf("%s #%d", clock.Name, clock.Index)
		}
		clockIndex := clampIndex(p.state.SelectedClockIndex, len(clockLabels))
		clockPreview := "无可用时钟源"
		if len(clockLabels) > 0 {
			clockPreview = clockLabels[clockIndex]
		}

		widgets = append(widgets,
			g.Separator(),
			sectionHeader("ASIO 驱动", "PCM 输出状态"),
			g.Row(
				StyleBadge(mode, ColorDigitalCyanDark, ColorDigitalCyan),
				g.Spacing(),
				fontLabel("SampleType: "+sampleType, 10.5, ColorTextMuted),
			),
			captionLabel(fmt.Sprintf("缓冲范围 [%d, %d]，当前 %d；驱动 %s", status.BufferSizes.MinSize, status.BufferSizes.MaxSize, status.ActiveBufferSize, status.DriverVersion)),
			captionLabel("硬件时钟源"),
			g.Combo("##asioClock", clockPreview, clockLabels, &p.state.SelectedClockIndex).
				Size(contentW).
				OnChange(func() {
					if err := p.uiService.SetAsioClockSource(int(p.state.SelectedClockIndex)); err != nil {
						p.state.NativeDSDError = err.Error()
					} else {
						p.RefreshAsioStatus()
					}
				}),
			g.Row(
				captionLabel("缓冲帧数"),
				g.InputInt(&p.state.AsioBufferSize).Size(px(104)).StepSize(64),
				StyleQuietButton(g.Button("应用").Size(px(76), px(30)).OnClick(func() {
					if err := p.uiService.SetAsioBufferSize(int(p.state.AsioBufferSize)); err != nil {
						p.state.NativeDSDError = err.Error()
					} else {
						p.state.NativeDSDError = ""
						p.RefreshAsioStatus()
					}
				})),
			),
		)

		if status.NativeDSDMode {
			nativeFormat := status.SelectedDSDFormat
			if nativeFormat == "" {
				nativeFormat = p.state.SelectedNativeDSDFormat().String()
			}
			widgets = append(widgets,
				g.Separator(),
				sectionHeader("Native DSD 测试模式", "已显式启用"),
				g.Row(
					StyleBadge("ACTIVE", ColorAmberDark, ColorAmberWarning),
					g.Spacing(),
					fontLabel(nativeFormat, 10.5, ColorTextBright),
				),
				captionLabel(fmt.Sprintf("SampleType: %s；1-bit 缓冲 %d 帧", sampleType, status.NativeDSDBufferFrames)),
				captionLabel("Native DSD 能力探测"),
			)
			for _, capability := range status.NativeCapabilities {
				label := fmt.Sprintf("%s  [不支持]", capability.Format)
				color := ColorTextDim
				if capability.Supported {
					label = fmt.Sprintf("%s  [%s]", capability.Format, capability.SampleType)
					color = ColorPhosphorGreen
				}
				widgets = append(widgets, fontLabel(label, 10.5, color))
				if !capability.Supported && capability.Reason != "" {
					widgets = append(widgets, captionLabel("  "+truncateText(capability.Reason, 54)))
				}
			}
			widgets = append(widgets, StyleDangerButton(g.Button("关闭 Native DSD · 恢复 PCM").Size(contentW, px(34)).OnClick(func() {
				if err := p.uiService.DisableNativeDSD(); err != nil {
					p.state.NativeDSDError = err.Error()
				} else {
					p.state.NativeDSDError = ""
					p.state.ResetNativeDSDSession()
					p.state.StatusMessage = "已关闭 Native DSD，PCM 模式已静音恢复"
					p.onFormatChanged()
				}
				p.RefreshAsioStatus()
			})))
		} else {
			widgets = append(widgets,
				g.Separator(),
				sectionHeader("Native DSD 测试模式", "默认关闭 · 当前 PCM"),
				captionLabel("只有主动进入并完成安全确认后，才会显示 Native DSD 专用测试控制。"),
				StyleDangerButton(g.Button("进入 Native DSD 测试模式…").Size(contentW, px(34)).OnClick(func() {
					if p.requestNativeDSDYes != nil {
						p.requestNativeDSDYes()
					}
				})),
			)
		}
		if p.state.NativeDSDError != "" {
			widgets = append(widgets, fontLabel(p.state.NativeDSDError, 10.5, ColorAlarmRed))
		}
	}

	widgets = append(widgets,

		g.Row(
			StylePrimaryButton(g.Button(probeText).Size(contentW-px(108), px(34)).Disabled(p.state.IsProbing).OnClick(p.RunProbe)),
			g.Spacing(),
			func() g.Widget {
				if p.state.ShowProbeResults && len(p.state.ProbeResults) > 0 {
					return StyleQuietButton(g.Button("收起").Size(px(92), px(34)).OnClick(func() {
						p.state.ShowProbeResults = false
					}))
				}
				return g.Dummy(px(92), px(34))
			}(),
		),
		g.Separator(),
	)

	if p.state.ShowProbeResults && len(p.state.ProbeResults) > 0 {
		widgets = append(widgets, sectionHeader("格式兼容性", "点击支持的组合可直接采用"))
		for _, result := range p.state.ProbeResults {
			targetRate := result.Format.SampleRate
			targetFmt := result.Format.SampleFormat
			supported := result.Status.String() == "Supported"
			label := fmt.Sprintf("%.1f kHz  /  %s", float64(targetRate)/1000.0, targetFmt.String())

			if supported && !p.state.IsDoPSelected() {
				widgets = append(widgets,
					StyleQuietButton(g.Button(label).Size(contentW, px(28)).OnClick(func() {
						applyProbeFormat(p.state, targetRate, targetFmt)
						p.onFormatChanged()
					})),
				)
			} else if supported {
				widgets = append(widgets, fontLabel(label+"  [DoP 参数锁定]", 10.5, ColorTextDim))
			} else {
				widgets = append(widgets, fontLabel(label+"  [不支持]", 10.5, ColorTextDim))
			}
		}
	} else {
		widgets = append(widgets,
			sectionHeader("当前策略", ""),
			captionLabel("独占模式会绕过系统混音器。"),
			captionLabel("格式不匹配时，引擎保持静音而不是降级输出。"),
			captionLabel("使用“检测支持格式”确认 DAC 的能力边界。"),
		)
	}

	return scrollPanel(width, height, ColorSurface, widgets...)
}

func applyProbeFormat(state *UIState, rate int, format audio.SampleFormat) {
	for i, option := range SampleRateOptions {
		if option == rate {
			state.SelectedSampleRateIndex = int32(i)
			break
		}
	}
	for i, option := range SampleFormatOptions {
		if option == format {
			state.SelectedSampleFormatIndex = int32(i)
			break
		}
	}
}

// keep the color import meaningful if this panel grows richer status states.
var _ color.Color
