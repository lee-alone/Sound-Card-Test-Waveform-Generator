package giuui

import (
	"fmt"
	"image/color"
	"time"

	g "github.com/AllenDang/giu"
	"sound-signal-generator/internal/controller"
	"sound-signal-generator/internal/engine"
	"sound-signal-generator/internal/ui/i18n"
)

// DiagnosticsPanel is the compact real-time health strip beneath the working
// surface. It favours scan-friendly values over log-like prose.
type DiagnosticsPanel struct {
	uiService *controller.UIService
	state     *UIState
}

func NewDiagnosticsPanel(uiService *controller.UIService, state *UIState) *DiagnosticsPanel {
	return &DiagnosticsPanel{uiService: uiService, state: state}
}

func (p *DiagnosticsPanel) PollMetrics() {
	p.state.Diagnostics = p.uiService.GetDiagnostics()
	p.state.IsActiveStream = p.state.Diagnostics.State == engine.StateRunning
	if p.state.IsAsioSelected() {
		if status, err := p.uiService.GetAsioStatus(); err == nil {
			p.state.AsioStatus = status
			p.state.NativeDSDConfirmed = status.NativeDSDConfirmed && status.NativeDSDMode
		} else {
			p.state.ResetNativeDSDSession()
		}
	} else {
		p.state.ResetNativeDSDSession()
	}
}

func (p *DiagnosticsPanel) Build(width, height float32) g.Widget {
	diag := p.state.Diagnostics
	stateStr := diag.State.String()
	stateBg := ColorSurfaceRaise
	stateFg := ColorTextMuted
	stateText := stateStr
	if stateText == "" {
		stateText = i18n.T("diagnostics.standby")
	}

	isRunning := stateStr == "Running" || stateStr == "4"
	if isRunning {
		stateBg = ColorPhosphorDark
		stateFg = ColorPhosphorGreen
		stateText = i18n.T("diagnostics.running")
	} else if stateStr == "OpenMuted" || stateStr == "1" {
		stateBg = ColorDigitalCyanDark
		stateFg = ColorDigitalCyan
		stateText = i18n.T("diagnostics.muted")
	} else if p.state.ErrorMessage != "" {
		stateBg = ColorAlarmDark
		stateFg = ColorAlarmRed
		stateText = i18n.T("diagnostics.fault")
	}

	uptime := "—"
	if !p.state.StreamStartTime.IsZero() && isRunning {
		d := time.Since(p.state.StreamStartTime)
		uptime = fmt.Sprintf("%02d:%02d:%02d", int(d.Hours()), int(d.Minutes())%60, int(d.Seconds())%60)
	}

	rateValue := "—"
	if diag.SampleRate > 0 {
		rateValue = fmt.Sprintf("%.1f kHz", float64(diag.SampleRate)/1000.0)
	}
	formatValue := diag.SampleFormat
	if formatValue == "" {
		formatValue = i18n.T("diagnostics.not_negotiated")
	}
	bufferValue := "—"
	if diag.BufferPeriodMs > 0 {
		bufferValue = fmt.Sprintf("%.2f ms", diag.BufferPeriodMs)
	}
	callbackValue := "—"
	if diag.RenderCallbackCount > 0 {
		callbackValue = i18n.T("diagnostics.count_fmt", diag.RenderCallbackCount)
	}
	underrunColor := ColorPhosphorGreen
	underrunValue := i18n.T("diagnostics.zero_count")
	if diag.UnderrunCount > 0 {
		underrunColor = ColorAlarmRed
		underrunValue = i18n.T("diagnostics.count_fmt", diag.UnderrunCount)
	}
	latencyColor := ColorTextBright
	latencyValue := i18n.T("diagnostics.zero_count")
	if diag.LateCallbackCount > 0 {
		latencyColor = ColorAmberWarning
		latencyValue = i18n.T("diagnostics.count_fmt", diag.LateCallbackCount)
	}

	stats := g.Table().
		Flags(g.TableFlagsNoSavedSettings | g.TableFlagsSizingStretchSame | g.TableFlagsNoBordersInBody).
		NoHeader(true).
		Rows(g.TableRow(
			metricCell(i18n.T("diagnostics.uptime"), uptime, ColorTextBright),
			metricCell(i18n.T("diagnostics.sample_rate"), rateValue, ColorTextBright),
			metricCell(i18n.T("diagnostics.container_format"), formatValue, ColorTextBright),
			metricCell(i18n.T("diagnostics.buffer"), bufferValue, ColorTextBright),
			metricCell(i18n.T("diagnostics.callbacks_underruns"), fmt.Sprintf("%s / %s", callbackValue, underrunValue), colorForUnderrun(callbackValue, underrunColor)),
			metricCell(i18n.T("diagnostics.late_wakeups"), latencyValue, latencyColor),
		))

	header := g.Row(
		titleLabel(i18n.T("diagnostics.runtime_monitor")),
		g.Spacing(),
		StyleBadge(stateText, stateBg, stateFg),
		g.Spacing(),
		captionLabel(i18n.T("diagnostics.p999_fmt", diag.P999PercentOfBuffer, diag.P999RenderTimeUs, diag.BitPerfectStatus)),
	)

	return panelBox(width, height, ColorSurface, header, g.Separator(), stats)
}

func colorForUnderrun(callbackValue string, fallback color.Color) color.Color {
	if callbackValue == "—" {
		return ColorTextMuted
	}
	return fallback
}
