package assets

import (
	"bytes"
	_ "embed"
	"image"
	_ "image/png"
)

// Embedded icon assets at common operating system scales.
//
//go:embed icon_16.png
var icon16 []byte

//go:embed icon_32.png
var icon32 []byte

//go:embed icon_48.png
var icon48 []byte

//go:embed icon_64.png
var icon64 []byte

//go:embed icon_256.png
var icon256 []byte

// LoadWindowIcons returns decoded candidate application icons in multiple resolutions
// suitable for GLFW/Giu window titlebar and taskbar rendering.
func LoadWindowIcons() []image.Image {
	rawEntries := [][]byte{icon16, icon32, icon48, icon64, icon256}
	icons := make([]image.Image, 0, len(rawEntries))

	for _, raw := range rawEntries {
		if len(raw) == 0 {
			continue
		}
		img, _, err := image.Decode(bytes.NewReader(raw))
		if err == nil {
			icons = append(icons, img)
		}
	}

	return icons
}
