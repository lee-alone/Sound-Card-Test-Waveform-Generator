package giuui

import (
	"fmt"
	"image/color"
	"time"

	g "github.com/AllenDang/giu"
	"sound-signal-generator/internal/controller"
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
		stateText = "Standby"
	}

	isRunning := stateStr == "Running" || stateStr == "4"
	if isRunning {
		stateBg = ColorPhosphorDark
		stateFg = ColorPhosphorGreen
		stateText = "Running"
	} else if stateStr == "OpenMuted" || stateStr == "1" {
		stateBg = ColorDigitalCyanDark
		stateFg = ColorDigitalCyan
		stateText = "Muted"
	} else if p.state.ErrorMessage != "" {
		stateBg = ColorAlarmDark
		stateFg = ColorAlarmRed
		stateText = "Fault"
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
		formatValue = "未协商"
	}
	bufferValue := "—"
	if diag.BufferPeriodMs > 0 {
		bufferValue = fmt.Sprintf("%.2f ms", diag.BufferPeriodMs)
	}
	callbackValue := "—"
	if diag.RenderCallbackCount > 0 {
		callbackValue = fmt.Sprintf("%d 次", diag.RenderCallbackCount)
	}
	underrunColor := ColorPhosphorGreen
	underrunValue := "0 次"
	if diag.UnderrunCount > 0 {
		underrunColor = ColorAlarmRed
		underrunValue = fmt.Sprintf("%d 次", diag.UnderrunCount)
	}
	latencyColor := ColorTextBright
	latencyValue := "0 次"
	if diag.LateCallbackCount > 0 {
		latencyColor = ColorAmberWarning
		latencyValue = fmt.Sprintf("%d 次", diag.LateCallbackCount)
	}

	stats := g.Table().
		Flags(g.TableFlagsNoSavedSettings | g.TableFlagsSizingStretchSame | g.TableFlagsNoBordersInBody).
		NoHeader(true).
		Rows(g.TableRow(
			metricCell("运行时间", uptime, ColorTextBright),
			metricCell("采样率", rateValue, ColorTextBright),
			metricCell("容器格式", formatValue, ColorTextBright),
			metricCell("缓冲", bufferValue, ColorTextBright),
			metricCell("回调 / 欠载", fmt.Sprintf("%s / %s", callbackValue, underrunValue), colorForUnderrun(callbackValue, underrunColor)),
			metricCell("迟到唤醒", latencyValue, latencyColor),
		))

	header := g.Row(
		titleLabel("运行监测"),
		g.Spacing(),
		StyleBadge(stateText, stateBg, stateFg),
		g.Spacing(),
		captionLabel(fmt.Sprintf("P99.9 回调负载 %.1f%%  |  %.1f µs  |  %s", diag.P999PercentOfBuffer, diag.P999RenderTimeUs, diag.BitPerfectStatus)),
	)

	return panelBox(width, height, ColorSurface, header, g.Separator(), stats)
}

func colorForUnderrun(callbackValue string, fallback color.Color) color.Color {
	if callbackValue == "—" {
		return ColorTextMuted
	}
	return fallback
}
