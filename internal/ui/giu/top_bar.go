package giuui

import (
	"image/color"

	g "github.com/AllenDang/giu"
	"sound-signal-generator/internal/ui/i18n"
)

// TopBar renders product identity, negotiated-format status and an always
// available safety-mute action.
type TopBar struct {
	state             *UIState
	onPanicMute       func()
	onLanguageChanged func(int32)
}

func NewTopBar(state *UIState, onPanicMute func(), onLanguageChanged func(int32)) *TopBar {
	return &TopBar{state: state, onPanicMute: onPanicMute, onLanguageChanged: onLanguageChanged}
}

// Build constructs the fixed application header.
func (b *TopBar) Build(width, height float32) g.Widget {
	diag := b.state.Diagnostics
	stateStr := diag.State.String()
	isRunning := stateStr == "Running" || stateStr == "4"

	statusText := i18n.T("top.standby")
	statusDetail := i18n.T("top.standby_detail")
	statusBg := ColorDigitalCyanDark
	statusFg := ColorDigitalCyan

	if b.state.ErrorMessage != "" {
		statusText = i18n.T("top.needs_attention")
		statusDetail = truncateText(b.state.ErrorMessage, 66)
		statusBg = ColorAlarmDark
		statusFg = color.RGBA{R: 255, G: 228, B: 228, A: 255}
	} else if isRunning {
		statusText = i18n.T("top.running")
		statusDetail = i18n.T("top.running_detail_fmt", diag.SampleRate, diag.SampleFormat)
		statusBg = ColorPhosphorDark
		statusFg = color.RGBA{R: 237, G: 255, B: 243, A: 255}
	}

	deviceName := i18n.T("device.default_output")
	if int(b.state.SelectedDeviceIndex) >= 0 && int(b.state.SelectedDeviceIndex) < len(b.state.DeviceNames) {
		deviceName = truncateText(b.state.DeviceNames[b.state.SelectedDeviceIndex], 34)
	}

	detailColor := ColorTextMuted
	if b.state.ErrorMessage != "" {
		detailColor = ColorAlarmRed
	}
	formatIndex := clampIndex(b.state.SelectedSampleFormatIndex, len(SampleFormatLabels))
	exclusiveMode := exclusiveLabel(b.state.IsExclusive)
	if b.state.IsDoPSelected() {
		exclusiveMode = i18n.T("top.exclusive")
	}
	baseline := i18n.T("top.baseline_fmt", deviceName, exclusiveMode, SampleFormatLabels[formatIndex])

	table := g.Table().
		Flags(g.TableFlagsNoSavedSettings|g.TableFlagsSizingStretchProp|g.TableFlagsNoBordersInBody).
		NoHeader(true).
		Columns(
			g.TableColumn("##brand").Flags(g.TableColumnFlagsWidthFixed|g.TableColumnFlagsNoResize).InnerWidthOrWeight(px(332)),
			g.TableColumn("##state").Flags(g.TableColumnFlagsWidthStretch|g.TableColumnFlagsNoResize).InnerWidthOrWeight(1),
			g.TableColumn("##action").Flags(g.TableColumnFlagsWidthFixed|g.TableColumnFlagsNoResize).InnerWidthOrWeight(px(300)),
		).
		Rows(
			g.TableRow(
				g.Column(
					titleLabel(i18n.T("brand.title")),
					captionLabel(i18n.T("brand.subtitle")),
				),
				g.Column(
					g.Row(StyleBadge(statusText, statusBg, statusFg), g.Spacing(), fontLabel(statusDetail, 11.5, detailColor)),
					captionLabel(baseline),
				),
				g.Row(
					g.Combo("##language", i18n.LanguageName(i18n.Current()), languageLabels(), &b.state.SelectedLanguageIndex).
						Size(px(108)).
						OnChange(func() {
							if b.onLanguageChanged != nil {
								b.onLanguageChanged(b.state.SelectedLanguageIndex)
							}
						}),
					g.Spacing(),
					StyleQuietButton(g.Button(i18n.T("top.safety_mute")).Size(px(104), px(32)).OnClick(b.onPanicMute)),
				),
			),
		)

	return panelBox(width, height, ColorSurfaceRaise, table)
}

func exclusiveLabel(exclusive bool) string {
	if exclusive {
		return i18n.T("top.exclusive")
	}
	return i18n.T("top.shared")
}

func languageLabels() []string {
	locales := i18n.Supported()
	labels := make([]string, len(locales))
	for i, locale := range locales {
		labels[i] = i18n.LanguageName(locale)
	}
	return labels
}
