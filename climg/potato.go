package climg

import (
	"image"

	"github.com/hajimehoshi/ebiten/v2"
)

var potatoMode bool

// SetPotatoMode toggles creation of unmanaged ebiten images.
func SetPotatoMode(v bool) {
	potatoMode = v
}

func newImageFromImage(src image.Image) *ebiten.Image {
	if potatoMode {
		return ebiten.NewImageFromImageWithOptions(src, &ebiten.NewImageFromImageOptions{Unmanaged: true})
	}
	return ebiten.NewImageFromImage(src)
}
