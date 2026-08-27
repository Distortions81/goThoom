package main

import (
	"image"

	"github.com/hajimehoshi/ebiten/v2"
)

// newManagedImage creates an image that can live on Ebitengine's automatic
// atlas. Use it only for images that remain cached for their full lifetime and
// are discarded only when a settings change invalidates the cache.
func newManagedImage(w, h int) *ebiten.Image {
	if gs.PotatoGPU {
		return newUnmanagedImage(w, h)
	}
	return ebiten.NewImage(w, h)
}

// newUnmanagedImage creates a standalone texture for scratch images and
// images that can be replaced, evicted, or removed during normal use.
func newUnmanagedImage(w, h int) *ebiten.Image {
	return ebiten.NewImageWithOptions(image.Rect(0, 0, w, h), &ebiten.NewImageOptions{Unmanaged: true})
}

func newManagedImageFromImage(src image.Image) *ebiten.Image {
	if gs.PotatoGPU {
		return newUnmanagedImageFromImage(src)
	}
	return ebiten.NewImageFromImage(src)
}

func newUnmanagedImageFromImage(src image.Image) *ebiten.Image {
	return ebiten.NewImageFromImageWithOptions(src, &ebiten.NewImageFromImageOptions{Unmanaged: true})
}

// mirrorImage returns a horizontally mirrored standalone image. Mirrored
// images used by stable caches should use mirrorManagedImage instead.
func mirrorImage(img *ebiten.Image) *ebiten.Image {
	return mirrorImageInto(img, false)
}

func mirrorManagedImage(img *ebiten.Image) *ebiten.Image {
	return mirrorImageInto(img, true)
}

func mirrorImageInto(img *ebiten.Image, managed bool) *ebiten.Image {
	if img == nil {
		return nil
	}
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	var out *ebiten.Image
	if managed {
		out = newManagedImage(w, h)
	} else {
		out = newUnmanagedImage(w, h)
	}
	op := &ebiten.DrawImageOptions{Filter: ebiten.FilterNearest, DisableMipmaps: true}
	op.GeoM.Scale(-1, 1)
	op.GeoM.Translate(float64(w), 0)
	out.DrawImage(img, op)
	return out
}
