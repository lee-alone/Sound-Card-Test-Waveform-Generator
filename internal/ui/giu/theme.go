package giuui

import (
	"image/color"

	g "github.com/AllenDang/giu"
)

// Lab palette. The interface stays neutral so cyan, green, amber and red can
// carry operational meaning instead of decoration.
var (
	ColorCanvas       = color.RGBA{R: 13, G: 16, B: 19, A: 255}
	ColorSurface      = color.RGBA{R: 22, G: 27, B: 32, A: 255}
	ColorSurfaceRaise = color.RGBA{R: 27, G: 33, B: 39, A: 255}
	ColorScopeBg      = color.RGBA{R: 9, G: 14, B: 17, A: 255}
	ColorBorder       = color.RGBA{R: 48, G: 57, B: 65, A: 255}
	ColorBorderSoft   = color.RGBA{R: 35, G: 43, B: 50, A: 255}
	ColorGrid         = color.RGBA{R: 47, G: 61, B: 69, A: 255}

	ColorTextBright = color.RGBA{R: 231, G: 237, B: 240, A: 255}
	ColorTextMuted  = color.RGBA{R: 139, G: 151, B: 160, A: 255}
	ColorTextDim    = color.RGBA{R: 101, G: 113, B: 122, A: 255}

	ColorDigitalCyan     = color.RGBA{R: 112, G: 193, B: 214, A: 255}
	ColorDigitalCyanDark = color.RGBA{R: 31, G: 76, B: 89, A: 255}
	ColorPhosphorGreen   = color.RGBA{R: 100, G: 201, B: 138, A: 255}
	ColorPhosphorDark    = color.RGBA{R: 32, G: 91, B: 57, A: 255}
	ColorAmberWarning    = color.RGBA{R: 225, G: 182, B: 107, A: 255}
	ColorAmberDark       = color.RGBA{R: 91, G: 68, B: 34, A: 255}
	ColorAlarmRed        = color.RGBA{R: 219, G: 118, B: 118, A: 255}
	ColorAlarmDark       = color.RGBA{R: 91, G: 38, B: 43, A: 255}
	ColorButtonActive    = color.RGBA{R: 49, G: 70, B: 82, A: 255}
)

// ApplyLabTheme configures a quiet, high-legibility measurement-console theme.
// Every explicit metric is multiplied by uiScale so the design remains intact
// on 125%, 150%, 200% and 250% Windows display scaling.
func ApplyLabTheme() *g.StyleSetter {
	return g.Style().
		SetStyleFloat(g.StyleVarWindowRounding, px(7)).
		SetStyleFloat(g.StyleVarChildRounding, px(7)).
		SetStyleFloat(g.StyleVarFrameRounding, px(5)).
		SetStyleFloat(g.StyleVarGrabRounding, px(4)).
		SetStyleFloat(g.StyleVarPopupRounding, px(7)).
		SetStyleFloat(g.StyleVarScrollbarRounding, px(4)).
		SetStyleFloat(g.StyleVarFrameBorderSize, px(1)).
		SetStyleFloat(g.StyleVarChildBorderSize, 0).
		SetStyleFloat(g.StyleVarPopupBorderSize, px(1)).
		SetStyleFloat(g.StyleVarScrollbarSize, px(12)).
		SetStyleFloat(g.StyleVarScrollbarRounding, px(6)).
		SetStyle(g.StyleVarWindowPadding, px(20), px(18)).
		SetStyle(g.StyleVarFramePadding, px(10), px(6)).
		SetStyle(g.StyleVarItemSpacing, px(8), px(7)).
		SetStyle(g.StyleVarItemInnerSpacing, px(7), px(6)).
		SetStyle(g.StyleVarButtonTextAlign, 0.5, 0.5).
		SetColor(g.StyleColorWindowBg, ColorCanvas).
		SetColor(g.StyleColorChildBg, ColorSurface).
		SetColor(g.StyleColorPopupBg, ColorSurfaceRaise).
		SetColor(g.StyleColorBorder, ColorBorder).
		SetColor(g.StyleColorBorderShadow, color.RGBA{}).
		SetColor(g.StyleColorFrameBg, ColorSurfaceRaise).
		SetColor(g.StyleColorFrameBgHovered, color.RGBA{R: 36, G: 44, B: 51, A: 255}).
		SetColor(g.StyleColorFrameBgActive, ColorButtonActive).
		SetColor(g.StyleColorTitleBg, ColorSurface).
		SetColor(g.StyleColorTitleBgActive, ColorSurfaceRaise).
		SetColor(g.StyleColorButton, ColorSurfaceRaise).
		SetColor(g.StyleColorButtonHovered, color.RGBA{R: 39, G: 48, B: 55, A: 255}).
		SetColor(g.StyleColorButtonActive, ColorButtonActive).
		SetColor(g.StyleColorHeader, ColorDigitalCyanDark).
		SetColor(g.StyleColorHeaderHovered, color.RGBA{R: 42, G: 95, B: 110, A: 255}).
		SetColor(g.StyleColorHeaderActive, ColorButtonActive).
		SetColor(g.StyleColorText, ColorTextBright).
		SetColor(g.StyleColorTextDisabled, ColorTextMuted).
		SetColor(g.StyleColorSeparator, ColorBorderSoft).
		SetColor(g.StyleColorSeparatorHovered, ColorBorder).
		SetColor(g.StyleColorSeparatorActive, ColorDigitalCyanDark).
		SetColor(g.StyleColorSliderGrab, ColorDigitalCyan).
		SetColor(g.StyleColorSliderGrabActive, ColorTextBright).
		SetColor(g.StyleColorCheckMark, ColorDigitalCyan).
		SetColor(g.StyleColorScrollbarBg, ColorCanvas).
		SetColor(g.StyleColorScrollbarGrab, ColorBorder).
		SetColor(g.StyleColorScrollbarGrabHovered, color.RGBA{R: 72, G: 85, B: 95, A: 255}).
		SetColor(g.StyleColorScrollbarGrabActive, ColorDigitalCyanDark).
		SetPlotColor(g.StylePlotColorFrameBg, ColorScopeBg).
		SetPlotColor(g.StylePlotColorPlotBg, ColorScopeBg).
		SetPlotColor(g.StylePlotColorPlotBorder, ColorBorder).
		SetPlotColor(g.StylePlotColorLegendBg, ColorSurfaceRaise).
		SetPlotColor(g.StylePlotColorLegendBorder, ColorBorder).
		SetPlotColor(g.StylePlotColorLegendText, ColorTextMuted).
		SetPlotColor(g.StylePlotColorTitleText, ColorTextBright).
		SetPlotColor(g.StylePlotColorInlayText, ColorTextMuted).
		SetPlotColor(g.StylePlotColorAxisText, ColorTextMuted).
		SetPlotColor(g.StylePlotColorAxisGrid, ColorGrid).
		SetPlotColor(g.StylePlotColorAxisTick, ColorTextDim).
		SetPlotColor(g.StylePlotColorAxisBg, ColorScopeBg).
		SetPlotColor(g.StylePlotColorAxisBgHovered, ColorSurfaceRaise).
		SetPlotColor(g.StylePlotColorAxisBgActive, ColorSurfaceRaise).
		SetPlotColor(g.StylePlotColorSelection, ColorDigitalCyanDark).
		SetPlotColor(g.StylePlotColorCrosshairs, ColorDigitalCyan).
		SetPlotStyleFloat(g.StylePlotVarPlotBorderSize, px(1)).
		SetPlotStyleFloat(g.StylePlotVarMinorAlpha, 0.22).
		SetPlotStyleFloat(g.StylePlotVarMajorGridSize, px(1)).
		SetPlotStyleFloat(g.StylePlotVarMinorGridSize, px(1)).
		SetPlotStyle(g.StylePlotVarPlotPadding, px(14), px(12))
}

// StyleColoredLabel renders a text label with an explicit foreground color.
func StyleColoredLabel(text string, col color.Color) g.Widget {
	return g.Style().SetColor(g.StyleColorText, col).To(g.Label(text))
}

// fontLabel gives labels an intentional size and color without changing the
// global typography for controls and editors.
func fontLabel(text string, logicalSize float32, col color.Color) g.Widget {
	return g.Style().
		SetColor(g.StyleColorText, col).
		SetFontSize(logicalSize).
		To(g.Label(text))
}

func titleLabel(text string) g.Widget {
	return fontLabel(text, 16.5, ColorTextBright)
}

func captionLabel(text string) g.Widget {
	return fontLabel(text, 10.5, ColorTextMuted)
}

func valueLabel(text string, col color.Color) g.Widget {
	return fontLabel(text, 16, col)
}

func sectionHeader(title, detail string) g.Widget {
	items := []g.Widget{titleLabel(title)}
	if detail != "" {
		items = append(items, g.Spacing(), captionLabel(detail))
	}
	return g.Row(items...)
}

// panelBox produces a single calm surface. Cards are intentionally not nested
// inside more cards; spacing and separators carry the structure instead.
func panelBox(width, height float32, bg color.Color, widgets ...g.Widget) g.Widget {
	child := g.Child().
		Size(width, height).
		Border(false).
		Flags(g.WindowFlagsNoScrollbar).
		Layout(widgets...)
	return g.Style().SetColor(g.StyleColorChildBg, bg).To(child)
}

func scrollPanel(width, height float32, bg color.Color, widgets ...g.Widget) g.Widget {
	child := g.Child().
		Size(width, height).
		Border(false).
		Layout(widgets...)
	return g.Style().SetColor(g.StyleColorChildBg, bg).To(child)
}

// StyleSuccessButton marks an action that starts or confirms live output.
func StyleSuccessButton(btn *g.ButtonWidget) g.Widget {
	return g.Style().
		SetColor(g.StyleColorButton, ColorPhosphorDark).
		SetColor(g.StyleColorButtonHovered, color.RGBA{R: 39, G: 112, B: 70, A: 255}).
		SetColor(g.StyleColorButtonActive, color.RGBA{R: 25, G: 78, B: 50, A: 255}).
		SetColor(g.StyleColorText, color.RGBA{R: 243, G: 255, B: 247, A: 255}).
		SetColor(g.StyleColorTextDisabled, color.RGBA{R: 161, G: 188, B: 171, A: 255}).
		To(btn)
}

// StyleDangerButton is reserved for stopping output and emergency mute.
func StyleDangerButton(btn *g.ButtonWidget) g.Widget {
	return g.Style().
		SetColor(g.StyleColorButton, ColorAlarmDark).
		SetColor(g.StyleColorButtonHovered, color.RGBA{R: 126, G: 49, B: 56, A: 255}).
		SetColor(g.StyleColorButtonActive, color.RGBA{R: 85, G: 31, B: 38, A: 255}).
		SetColor(g.StyleColorText, color.RGBA{R: 255, G: 236, B: 236, A: 255}).
		To(btn)
}

// StylePrimaryButton is used for the primary, non-destructive action.
func StylePrimaryButton(btn *g.ButtonWidget) g.Widget {
	return g.Style().
		SetColor(g.StyleColorButton, ColorDigitalCyanDark).
		SetColor(g.StyleColorButtonHovered, color.RGBA{R: 42, G: 104, B: 121, A: 255}).
		SetColor(g.StyleColorButtonActive, color.RGBA{R: 27, G: 73, B: 87, A: 255}).
		SetColor(g.StyleColorText, color.RGBA{R: 239, G: 251, B: 255, A: 255}).
		To(btn)
}

// StyleQuietButton is the low-emphasis treatment for utility actions.
func StyleQuietButton(btn *g.ButtonWidget) g.Widget {
	return g.Style().
		SetColor(g.StyleColorButton, color.RGBA{R: 31, G: 39, B: 46, A: 255}).
		SetColor(g.StyleColorButtonHovered, color.RGBA{R: 43, G: 54, B: 63, A: 255}).
		SetColor(g.StyleColorButtonActive, ColorButtonActive).
		To(btn)
}

// StyleBadge creates a non-interactive status pill.
func StyleBadge(text string, bgCol, textCol color.Color) g.Widget {
	return g.Style().
		SetColor(g.StyleColorButton, bgCol).
		SetColor(g.StyleColorButtonHovered, bgCol).
		SetColor(g.StyleColorButtonActive, bgCol).
		SetColor(g.StyleColorText, textCol).
		SetColor(g.StyleColorTextDisabled, textCol).
		To(g.Button(text).Disabled(true).Size(0, px(24)))
}

func metricCell(label, value string, valueColor color.Color) g.Widget {
	return g.Column(
		fontLabel(label, 10, ColorTextMuted),
		valueLabel(value, valueColor),
	)
}

func truncateText(text string, maxRunes int) string {
	runes := []rune(text)
	if maxRunes <= 0 || len(runes) <= maxRunes {
		return text
	}
	if maxRunes <= 1 {
		return "…"
	}
	return string(runes[:maxRunes-1]) + "…"
}

// barePanel is a fixed layout region without ImGui window padding. It is used
// for composed workspaces whose children already own their spacing.
func barePanel(width, height float32, bg color.Color, widgets ...g.Widget) g.Widget {
	child := g.Child().
		Size(width, height).
		Border(false).
		Flags(g.WindowFlagsNoScrollbar).
		Layout(widgets...)
	return g.Style().
		SetStyle(g.StyleVarWindowPadding, 0, 0).
		SetColor(g.StyleColorChildBg, bg).
		To(child)
}
