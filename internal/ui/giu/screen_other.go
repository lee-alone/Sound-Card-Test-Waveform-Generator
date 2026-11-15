//go:build !windows

package giuui

func primaryWorkArea() (x, y, width, height int) {
	return 0, 0, 1536, 864
}

func systemScale() float32 { return 1 }
