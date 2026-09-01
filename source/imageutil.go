package main

import (
	"image"
	"sync"
	"sync/atomic"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
)

var (
	tracedManagedImages     sync.Map
	tracedManagedImageCount atomic.Int64
	tracedManagedImageBytes atomic.Int64
)

func trackManagedImage(img *ebiten.Image, width, height int) *ebiten.Image {
	if img == nil || assetLoadTraceThreshold <= 0 {
		return img
	}
	bytes := uint64(max(0, width)) * uint64(max(0, height)) * 4
	if _, loaded := tracedManagedImages.LoadOrStore(img, bytes); loaded {
		return img
	}
	tracedManagedImageCount.Add(1)
	tracedManagedImageBytes.Add(int64(bytes))
	noteFrameManagedImageAdd(bytes)
	return img
}

func deallocateImage(img *ebiten.Image) {
	if img == nil {
		return
	}
	if value, ok := tracedManagedImages.LoadAndDelete(img); ok {
		bytes := value.(uint64)
		tracedManagedImageCount.Add(-1)
		tracedManagedImageBytes.Add(-int64(bytes))
		noteFrameManagedImageRemove(bytes)
	}
	img.Deallocate()
}

func tracedManagedImageTotals() (count, bytes int64) {
	return tracedManagedImageCount.Load(), tracedManagedImageBytes.Load()
}

// newManagedImage creates an image that can live on Ebitengine's automatic
// atlas. Use it only for images that remain cached for their full lifetime and
// are discarded only when a settings change invalidates the cache.
func newManagedImage(w, h int) *ebiten.Image {
	if gs.PotatoGPU {
		return newUnmanagedImage(w, h)
	}
	return trackManagedImage(ebiten.NewImage(w, h), w, h)
}

// newUnmanagedImage creates a standalone texture for scratch images and
// images that can be replaced, evicted, or removed during normal use.
func newUnmanagedImage(w, h int) *ebiten.Image {
	return ebiten.NewImageWithOptions(image.Rect(0, 0, w, h), &ebiten.NewImageOptions{Unmanaged: true})
}

func newManagedImageFromImage(src image.Image) *ebiten.Image {
	trace := currentAssetLoadFrameTrace()
	if trace == nil {
		if gs.PotatoGPU {
			return newUnmanagedImageFromImage(src)
		}
		bounds := src.Bounds()
		return trackManagedImage(ebiten.NewImageFromImage(src), bounds.Dx(), bounds.Dy())
	}
	started := time.Now()
	bounds := src.Bounds()
	var img *ebiten.Image
	if gs.PotatoGPU {
		img = newUnmanagedImageFromImage(src)
	} else {
		img = trackManagedImage(ebiten.NewImageFromImage(src), bounds.Dx(), bounds.Dy())
	}
	noteFrameImageCreation(bounds.Dx(), bounds.Dy(), time.Since(started))
	return img
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
