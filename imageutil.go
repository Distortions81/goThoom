package main

import (
	"image"

	"github.com/hajimehoshi/ebiten/v2"
)

func newImage(w, h int) *ebiten.Image {
	if gs.PotatoComputer {
		return ebiten.NewImageWithOptions(image.Rect(0, 0, w, h), &ebiten.NewImageOptions{Unmanaged: true})
	}
	return ebiten.NewImage(w, h)
}

func newImageFromImage(src image.Image) *ebiten.Image {
	if gs.PotatoComputer {
		return ebiten.NewImageFromImageWithOptions(src, &ebiten.NewImageFromImageOptions{Unmanaged: true})
	}
	return ebiten.NewImageFromImage(src)
}

// mirrorImage returns a horizontally mirrored copy of img.
func mirrorImage(img *ebiten.Image) *ebiten.Image {
	if img == nil {
		return nil
	}
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	out := newImage(w, h)
	op := &ebiten.DrawImageOptions{Filter: ebiten.FilterNearest, DisableMipmaps: true}
	op.GeoM.Scale(-1, 1)
	op.GeoM.Translate(float64(w), 0)
	out.DrawImage(img, op)
	return out
}
