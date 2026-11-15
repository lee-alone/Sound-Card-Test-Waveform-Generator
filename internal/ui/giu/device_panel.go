package giuui

import (
	"fmt"
	"image/color"
	"strings"

	g "github.com/AllenDang/giu"
	"sound-signal-generator/internal/audio"
	"sound-signal-generator/internal/controller"
	"sound-signal-generator/internal/ui/i18n"
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
		p.state.ErrorMessage = i18n.T("device.error.enumerate_fmt", err)
		return
	}

	names := make([]string, 0, len(devices))
	ids := make([]string, 0, len(devices))
	backends := make([]audio.BackendType, 0, len(devices))
	defaultIdx := preferredDeviceIndex(devices)

	for _, dev := range devices {
		displayName := dev.Name
		if dev.IsDefault {
			displayName += i18n.T("device.default_badge")
		}
		if dev.State != audio.StateActive {
			displayName += fmt.Sprintf("  (%s)", dev.State.String())
		}
		names = append(names, displayName)
		ids = append(ids, dev.ID)
		backends = append(backends, dev.Backend)
	}

	if len(names) == 0 {
		names = []string{i18n.T("device.none")}
		ids = []string{""}
		backends = []audio.BackendType{""}
	}

	p.state.DeviceNames = names
	p.state.DeviceIDs = ids
	p.state.DeviceBackends = backends
	p.state.SelectedDeviceIndex = defaultIdx
	if int(defaultIdx) < len(ids) && ids[defaultIdx] != "" {
		if err := p.uiService.SelectDevice(ids[defaultIdx]); err != nil {
			p.state.ErrorMessage = i18n.T("device.error.select_default_fmt", err)
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
			p.state.ErrorMessage = i18n.T("device.error.probe_fmt", err)
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
	streamLocked := p.state.IsOutputRunning()

	currentDevPreview := i18n.T("device.not_found")
	if int(p.state.SelectedDeviceIndex) >= 0 && int(p.state.SelectedDeviceIndex) < len(p.state.DeviceNames) {
		currentDevPreview = p.state.DeviceNames[p.state.SelectedDeviceIndex]
	}

	rateIndex := clampIndex(p.state.SelectedSampleRateIndex, len(SampleRateLabels))
	formatIndex := clampIndex(p.state.SelectedSampleFormatIndex, len(SampleFormatLabels))
	isDoP := p.state.IsDoPSelected()
	probeText := i18n.T("device.probe.button")
	if p.state.IsProbing {
		probeText = i18n.T("device.probe.running")
	}

	exclusiveLabel := i18n.T("device.exclusive.wasapi")
	if p.state.IsAsioSelected() {
		exclusiveLabel = i18n.T("device.exclusive.asio_fixed")
	}
	var exclusiveControl g.Widget = g.Checkbox(exclusiveLabel, &p.state.IsExclusive).
		OnChange(func() {
			p.uiService.SetExclusiveMode(p.state.IsExclusive)
			p.onFormatChanged()
		})
	exclusiveControl = disableWhen(streamLocked, exclusiveControl)
	var trimControl g.Widget = g.SliderFloat(&p.state.MasterTrimDB, -6.0, 6.0).
		Format("%+.1f dB").
		Size(px(112)).
		OnChange(p.onFormatChanged)
	rateCaption := i18n.T("device.sample_rate")
	formatCaption := i18n.T("device.sample_format")
	if streamLocked {
		rateCaption += i18n.T("device.locked_suffix")
		formatCaption += i18n.T("device.locked_suffix")
	}
	formatControls := []g.Widget{
		captionLabel(rateCaption),
		disableWhen(streamLocked,
			g.Combo("##rateCombo", SampleRateLabels[rateIndex], SampleRateLabels, &p.state.SelectedSampleRateIndex).
				Size(contentW).
				OnChange(p.onFormatChanged)),
		captionLabel(formatCaption),
		disableWhen(streamLocked,
			g.Combo("##fmtCombo", SampleFormatLabels[formatIndex], SampleFormatLabels, &p.state.SelectedSampleFormatIndex).
				Size(contentW).
				OnChange(p.onFormatChanged)),
	}

	if isDoP {
		carrierRate := p.state.SelectedDoPFormat().DoPCarrierSampleRate()
		exclusiveControl = g.Row(
			captionLabel(i18n.T("device.dop.exclusive")),
			g.Spacing(),
			StyleBadge(i18n.T("device.dop.forced"), ColorDigitalCyanDark, ColorDigitalCyan),
		)
		trimControl = fontLabel(i18n.T("device.unity_locked"), 11, ColorTextMuted)
		formatControls = []g.Widget{
			captionLabel(i18n.T("device.dop.carrier_format")),
			lockedParamRow(i18n.T("device.sample_rate"), fmt.Sprintf("%.1f kHz", float64(carrierRate)/1000.0), contentW),
			lockedParamRow(i18n.T("device.container"), "24-in-32 LE", contentW),
			fontLabel(i18n.T("device.dop.auto_params"), 10.5, ColorTextMuted),
		}
	}

	deviceComboW := contentW - px(82) - px(8)
	widgets := []g.Widget{
		sectionHeader(i18n.T("device.output_settings"), ""),
		g.Separator(),
		captionLabel(i18n.T("device.output_device")),
		g.Row(
			disableWhen(streamLocked,
				g.Combo("##devCombo", currentDevPreview, p.state.DeviceNames, &p.state.SelectedDeviceIndex).
					Size(deviceComboW).
					OnChange(func() {
						if int(p.state.SelectedDeviceIndex) >= 0 && int(p.state.SelectedDeviceIndex) < len(p.state.DeviceIDs) {
							p.state.ResetNativeDSDSession()
							_ = p.uiService.SelectDevice(p.state.DeviceIDs[p.state.SelectedDeviceIndex])
							p.RefreshAsioStatus()
							p.onFormatChanged()
						}
					})),
			StyleQuietButton(g.Button(i18n.T("device.refresh")).Size(px(82), px(32)).Disabled(streamLocked).OnClick(p.RefreshDevices)),
		),
		captionLabel(truncateText(currentDevPreview, 44)),
		g.Row(
			exclusiveControl,
			g.Spacing(),
			captionLabel(i18n.T("device.calibration_trim")),
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
			sampleType = i18n.T("device.not_negotiated")
		}
		clockLabels := make([]string, len(status.ClockSources))
		for i, clock := range status.ClockSources {
			clockLabels[i] = fmt.Sprintf("%s #%d", clock.Name, clock.Index)
		}
		clockIndex := clampIndex(p.state.SelectedClockIndex, len(clockLabels))
		clockPreview := i18n.T("device.asio.no_clock")
		if len(clockLabels) > 0 {
			clockPreview = clockLabels[clockIndex]
		}

		widgets = append(widgets,
			g.Separator(),
			sectionHeader(i18n.T("device.asio.driver"), i18n.T("device.asio.pcm_status")),
			g.Row(
				StyleBadge(mode, ColorDigitalCyanDark, ColorDigitalCyan),
				g.Spacing(),
				fontLabel("SampleType: "+sampleType, 10.5, ColorTextMuted),
			),
			captionLabel(i18n.T("device.asio.buffer_range_fmt", status.BufferSizes.MinSize, status.BufferSizes.MaxSize, status.ActiveBufferSize, status.DriverVersion)),
			captionLabel(i18n.T("device.asio.clock_source")),
			disableWhen(streamLocked,
				g.Combo("##asioClock", clockPreview, clockLabels, &p.state.SelectedClockIndex).
					Size(contentW).
					OnChange(func() {
						if err := p.uiService.SetAsioClockSource(int(p.state.SelectedClockIndex)); err != nil {
							p.state.NativeDSDError = err.Error()
						} else {
							p.RefreshAsioStatus()
						}
					})),
			g.Row(
				captionLabel(i18n.T("device.asio.buffer_frames")),
				disableWhen(streamLocked, g.InputInt(&p.state.AsioBufferSize).Size(px(104)).StepSize(64)),
				StyleQuietButton(g.Button(i18n.T("device.apply")).Size(px(76), px(30)).Disabled(streamLocked).OnClick(func() {
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
				sectionHeader(i18n.T("device.native.mode"), i18n.T("device.native.explicitly_enabled")),
				g.Row(
					StyleBadge("ACTIVE", ColorAmberDark, ColorAmberWarning),
					g.Spacing(),
					fontLabel(nativeFormat, 10.5, ColorTextBright),
				),
				captionLabel(i18n.T("device.native.buffer_fmt", sampleType, status.NativeDSDBufferFrames)),
				captionLabel(i18n.T("device.native.capabilities")),
			)
			for _, capability := range status.NativeCapabilities {
				label := capability.Format + i18n.T("device.native.unsupported_suffix")
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
			widgets = append(widgets, StyleDangerButton(g.Button(i18n.T("device.native.disable")).Size(contentW, px(34)).OnClick(func() {
				if err := p.uiService.DisableNativeDSD(); err != nil {
					p.state.NativeDSDError = err.Error()
				} else {
					p.state.NativeDSDError = ""
					p.state.ResetNativeDSDSession()
					p.state.StatusMessage = i18n.T("device.native.disabled_status")
					p.onFormatChanged()
				}
				p.RefreshAsioStatus()
			})))
		} else {
			widgets = append(widgets,
				g.Separator(),
				sectionHeader(i18n.T("device.native.mode"), i18n.T("device.native.default_off_pcm")),
				captionLabel(i18n.T("device.native.hidden_hint")),
				StyleDangerButton(g.Button(i18n.T("device.native.enter")).Size(contentW, px(34)).OnClick(func() {
					if p.requestNativeDSDYes != nil {
						p.requestNativeDSDYes()
					}
				})),
			)
		}
		if p.state.NativeDSDError != "" {
			widgets = append(widgets, fontLabel(p.state.NativeDSDError, 10.5, ColorAlarmRed))
			if strings.Contains(strings.ToUpper(p.state.NativeDSDError), "ASIOSTINT32") {
				widgets = append(widgets, captionLabel(i18n.T("device.native.combo_hint")))
			}
		}
	}

	widgets = append(widgets,

		g.Row(
			StylePrimaryButton(g.Button(probeText).Size(contentW-px(108), px(34)).Disabled(p.state.IsProbing || streamLocked).OnClick(p.RunProbe)),
			g.Spacing(),
			func() g.Widget {
				if p.state.ShowProbeResults && len(p.state.ProbeResults) > 0 {
					return StyleQuietButton(g.Button(i18n.T("device.probe.hide")).Size(px(92), px(34)).OnClick(func() {
						p.state.ShowProbeResults = false
					}))
				}
				return g.Dummy(px(92), px(34))
			}(),
		),
		g.Separator(),
	)

	if p.state.ShowProbeResults && len(p.state.ProbeResults) > 0 {
		widgets = append(widgets, sectionHeader(i18n.T("device.formats.compat"), i18n.T("device.formats.click_supported")))
		for _, result := range p.state.ProbeResults {
			targetRate := result.Format.SampleRate
			targetFmt := result.Format.SampleFormat
			supported := result.Status.String() == "Supported"
			label := fmt.Sprintf("%.1f kHz  /  %s", float64(targetRate)/1000.0, targetFmt.String())

			if supported && !p.state.IsDoPSelected() && !streamLocked {
				widgets = append(widgets,
					StyleQuietButton(g.Button(label).Size(contentW, px(28)).OnClick(func() {
						applyProbeFormat(p.state, targetRate, targetFmt)
						p.onFormatChanged()
					})),
				)
			} else if supported && p.state.IsDoPSelected() {
				widgets = append(widgets, fontLabel(label+i18n.T("device.format.dop_locked_suffix"), 10.5, ColorTextDim))
			} else if supported && streamLocked {
				widgets = append(widgets, fontLabel(label+i18n.T("device.format.running_locked_suffix"), 10.5, ColorTextDim))
			} else {
				widgets = append(widgets, fontLabel(label+i18n.T("device.native.unsupported_suffix"), 10.5, ColorTextDim))
			}
		}
	} else {
		widgets = append(widgets,
			sectionHeader(i18n.T("device.policy.title"), ""),
			captionLabel(i18n.T("device.policy.exclusive")),
			captionLabel(i18n.T("device.policy.mismatch")),
			captionLabel(i18n.T("device.policy.probe")),
		)
	}

	return scrollPanel(width, height, ColorSurface, widgets...)
}

func disableWhen(disabled bool, widget g.Widget) g.Widget {
	if !disabled {
		return widget
	}
	return g.Style().SetDisabled(true).To(widget)
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
