package giuui

import (
	g "github.com/AllenDang/giu"
	"sound-signal-generator/internal/ui/i18n"
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
	statusText := i18n.T("transport.muted")
	detail := i18n.T("transport.muted_detail")
	if nyquistBlocked {
		statusColor = ColorAlarmRed
		statusBg = ColorAlarmDark
		statusText = i18n.T("transport.nyquist")
		detail = i18n.T("transport.nyquist_detail")
	} else if isRunning {
		statusColor = ColorPhosphorGreen
		statusBg = ColorPhosphorDark
		statusText = i18n.T("transport.running")
		detail = i18n.T("transport.running_detail")
	}

	actions := g.Row(
		StylePrimaryButton(
			g.Button(i18n.T("transport.start")).
				Size(px(190), px(38)).
				Disabled(nyquistBlocked || isRunning).
				OnClick(b.onStart),
		),
		g.Spacing(),
		StyleDangerButton(
			g.Button(i18n.T("transport.stop_mute")).
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
