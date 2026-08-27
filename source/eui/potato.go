package eui

import (
	"image"
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
)

var potatoMode bool
var windowShadows = true

// SetPotatoMode toggles creation of unmanaged ebiten images.
func SetPotatoMode(v bool) {
	if potatoMode == v {
		return
	}
	potatoMode = v
	if whiteImage != nil {
		whiteImage.Deallocate()
	}
	whiteImage = newManagedImage(3, 3)
	whiteImage.Fill(color.White)
}

// SetWindowShadows controls drop shadows for windows and dropdown menus.
func SetWindowShadows(v bool) {
	if windowShadows == v {
		return
	}
	windowShadows = v
	for _, win := range windows {
		win.Dirty = true
	}
}

// newManagedImage is reserved for images that remain in place for their full
// lifetime. Replaceable window and item render targets must be unmanaged so
// normal UI activity cannot fragment Ebitengine's automatic atlas.
func newManagedImage(w, h int) *ebiten.Image {
	if potatoMode {
		return newUnmanagedImage(w, h)
	}
	return ebiten.NewImage(w, h)
}

func newUnmanagedImage(w, h int) *ebiten.Image {
	return ebiten.NewImageWithOptions(image.Rect(0, 0, w, h), &ebiten.NewImageOptions{Unmanaged: true})
}
