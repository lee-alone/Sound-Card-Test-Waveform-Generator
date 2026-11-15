//go:build windows

package giuui

import (
	"syscall"
	"unsafe"
)

const spiGetWorkArea = 0x0030

var (
	user32DLL                 = syscall.NewLazyDLL("user32.dll")
	procSystemParametersInfoW = user32DLL.NewProc("SystemParametersInfoW")
	procGetDpiForSystem       = user32DLL.NewProc("GetDpiForSystem")
)

type winRect struct {
	Left   int32
	Top    int32
	Right  int32
	Bottom int32
}

// primaryWorkArea returns the usable desktop rectangle in physical pixels.
// It excludes the taskbar and is the correct frame of reference for a
// per-monitor DPI-aware GLFW window.
func primaryWorkArea() (x, y, width, height int) {
	rect := winRect{}
	ok, _, _ := procSystemParametersInfoW.Call(
		spiGetWorkArea,
		0,
		uintptr(unsafe.Pointer(&rect)),
		0,
	)
	if ok == 0 || rect.Right <= rect.Left || rect.Bottom <= rect.Top {
		return 0, 0, 1536, 864
	}

	return int(rect.Left), int(rect.Top), int(rect.Right - rect.Left), int(rect.Bottom - rect.Top)
}

// systemScale returns the system DPI scale for use before the first frame,
// when the ImGui backend content scale may not have been observed yet.
func systemScale() float32 {
	dpi, _, _ := procGetDpiForSystem.Call()
	if dpi == 0 {
		return 1
	}
	return float32(dpi) / 96.0
}
