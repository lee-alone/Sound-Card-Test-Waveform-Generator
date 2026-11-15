package giuui

import (
	g "github.com/AllenDang/giu"
)

// TransportBar anchors the two high-consequence controls in one permanent
// location: start output and stop/mute output.
type TransportBar struct {
	state             *UIState
	onStart           func()
	onStop            func()
	isNyquistExceeded func() bool
}

func NewTransportBar(
	state *UIState,
	onStart func(),
	onStop func(),
	isNyquistExceeded func() bool,
) *TransportBar {
	return &TransportBar{
		state:             state,
		onStart:           onStart,
		onStop:            onStop,
		isNyquistExceeded: isNyquistExceeded,
	}
}

func (b *TransportBar) Build(width, height float32) g.Widget {
	nyquistBlocked := b.isNyquistExceeded()
	stateStr := b.state.Diagnostics.State.String()
	isRunning := stateStr == "Running" || stateStr == "4"

	statusColor := ColorTextMuted
	statusBg := ColorSurfaceRaise
	statusText := "输出静音"
	detail := "参数可安全调整；开始输出前会重新协商设备格式。"
	if nyquistBlocked {
		statusColor = ColorAlarmRed
		statusBg = ColorAlarmDark
		statusText = "频率超限"
		detail = "当前频率超过采样率安全上限，开始输出已锁定。"
	} else if isRunning {
		statusColor = ColorPhosphorGreen
		statusBg = ColorPhosphorDark
		statusText = "信号输出中"
		detail = "波形与频率实时生效；采样率和样本格式已锁定。"
	}

	actions := g.Row(
		StylePrimaryButton(
			g.Button("开始输出").
				Size(px(190), px(38)).
				Disabled(nyquistBlocked || isRunning).
				OnClick(b.onStart),
		),
		g.Spacing(),
		StyleDangerButton(
			g.Button("停止并静音").
				Size(px(190), px(38)).
				OnClick(b.onStop),
		),
	)

	table := g.Table().
		Flags(g.TableFlagsNoSavedSettings|g.TableFlagsSizingStretchProp|g.TableFlagsNoBordersInBody).
		NoHeader(true).
		Columns(
			g.TableColumn("##actions").Flags(g.TableColumnFlagsWidthFixed|g.TableColumnFlagsNoResize).InnerWidthOrWeight(px(408)),
			g.TableColumn("##status").Flags(g.TableColumnFlagsWidthStretch|g.TableColumnFlagsNoResize).InnerWidthOrWeight(1),
		).
		Rows(g.TableRow(
			actions,
			g.Column(
				g.Row(StyleBadge(statusText, statusBg, statusColor), g.Spacing(), fontLabel(detail, 11.5, ColorTextMuted)),
			),
		))

	return panelBox(width, height, ColorSurfaceRaise, table)
}
