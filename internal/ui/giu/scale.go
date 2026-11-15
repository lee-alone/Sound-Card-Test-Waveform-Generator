package giuui

// uiScale converts logical design units used by this package into the
// framebuffer pixels used by Dear ImGui on the current monitor.
//
// GIU applies the monitor content scale to fonts and built-in style metrics.
// Explicit widget sizes must use the same scale, otherwise panels become
// unusably narrow on 4K/high-DPI Windows configurations.
var uiScale float32 = 1

func setUIScale(scale float32) {
	if scale < 1 {
		scale = 1
	}
	uiScale = scale
}

// px converts a logical design pixel to a framebuffer pixel.
func px(value float32) float32 {
	return value * uiScale
}
