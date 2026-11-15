package giuui

import (
	"fmt"
	"image/color"

	g "github.com/AllenDang/giu"
)

// TopBar renders product identity, negotiated-format status and an always
// available safety-mute action.
type TopBar struct {
	state       *UIState
	onPanicMute func()
}

func NewTopBar(state *UIState, onPanicMute func()) *TopBar {
	return &TopBar{state: state, onPanicMute: onPanicMute}
}

// Build constructs the fixed application header.
func (b *TopBar) Build(width, height float32) g.Widget {
	diag := b.state.Diagnostics
	stateStr := diag.State.String()
	isRunning := stateStr == "Running" || stateStr == "4"

	statusText := "待机"
	statusDetail := "输出保持静音，参数可安全调整"
	statusBg := ColorDigitalCyanDark
	statusFg := ColorDigitalCyan

	if b.state.ErrorMessage != "" {
		statusText = "需要处理"
		statusDetail = truncateText(b.state.ErrorMessage, 66)
		statusBg = ColorAlarmDark
		statusFg = color.RGBA{R: 255, G: 228, B: 228, A: 255}
	} else if isRunning {
		statusText = "正在输出"
		statusDetail = fmt.Sprintf("%d Hz  |  %s", diag.SampleRate, diag.SampleFormat)
		statusBg = ColorPhosphorDark
		statusFg = color.RGBA{R: 237, G: 255, B: 243, A: 255}
	}

	deviceName := "默认输出设备"
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
		exclusiveMode = "独占"
	}
	baseline := fmt.Sprintf("%s  |  WASAPI %s  |  %s", deviceName, exclusiveMode, SampleFormatLabels[formatIndex])

	table := g.Table().
		Flags(g.TableFlagsNoSavedSettings|g.TableFlagsSizingStretchProp|g.TableFlagsNoBordersInBody).
		NoHeader(true).
		Columns(
			g.TableColumn("##brand").Flags(g.TableColumnFlagsWidthFixed|g.TableColumnFlagsNoResize).InnerWidthOrWeight(px(332)),
			g.TableColumn("##state").Flags(g.TableColumnFlagsWidthStretch|g.TableColumnFlagsNoResize).InnerWidthOrWeight(1),
			g.TableColumn("##action").Flags(g.TableColumnFlagsWidthFixed|g.TableColumnFlagsNoResize).InnerWidthOrWeight(px(178)),
		).
		Rows(
			g.TableRow(
				g.Column(
					titleLabel("声卡测试信号发生器"),
					captionLabel("Precision audio measurement"),
				),
				g.Column(
					g.Row(StyleBadge(statusText, statusBg, statusFg), g.Spacing(), fontLabel(statusDetail, 11.5, detailColor)),
					captionLabel(baseline),
				),
				StyleQuietButton(g.Button("安全静音").Size(px(104), px(32)).OnClick(b.onPanicMute)),
			),
		)

	return panelBox(width, height, ColorSurfaceRaise, table)
}

func exclusiveLabel(exclusive bool) string {
	if exclusive {
		return "独占"
	}
	return "共享"
}
