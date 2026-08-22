package eui

import (
	"image"
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
)

var potatoMode bool

// SetPotatoMode toggles creation of unmanaged ebiten images.
func SetPotatoMode(v bool) {
	if potatoMode == v {
		return
	}
	potatoMode = v
	whiteImage = newImage(3, 3)
	whiteImage.Fill(color.White)
}

func newImage(w, h int) *ebiten.Image {
	if potatoMode {
		return ebiten.NewImageWithOptions(image.Rect(0, 0, w, h), &ebiten.NewImageOptions{Unmanaged: true})
	}
	return ebiten.NewImage(w, h)
}

func newImageFast(w, h int) *ebiten.Image {
	return newImage(w, h)
}
