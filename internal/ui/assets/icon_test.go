package assets

import (
	"testing"
)

func TestLoadWindowIcons(t *testing.T) {
	icons := LoadWindowIcons()
	if len(icons) == 0 {
		t.Fatalf("expected embedded window icons to be loaded, got 0")
	}

	for i, img := range icons {
		bounds := img.Bounds()
		if bounds.Dx() <= 0 || bounds.Dy() <= 0 {
			t.Errorf("icon %d has invalid dimensions: %dx%d", i, bounds.Dx(), bounds.Dy())
		}
	}
}
