package climg

import (
	"image"
	"testing"
)

func TestVisibleFrameSizeMeasuresLargestOpaqueAnimationFrame(t *testing.T) {
	// Two 12x10 frames with the same transparent one-pixel sheet border used by
	// DecodeRGBA. The first visible frame is 3x2; the second is 5x4.
	pixels := image.NewRGBA(image.Rect(0, 0, 14, 22))
	fillAlphaRect(pixels, image.Rect(5, 4, 8, 6))
	fillAlphaRect(pixels, image.Rect(2, 14, 7, 18))

	if w, h := visibleFrameSize(pixels, 2); w != 5 || h != 4 {
		t.Fatalf("visibleFrameSize = %dx%d, want 5x4", w, h)
	}
}

func TestVisibleFrameSizeCachesPreparedArtworkMeasurement(t *testing.T) {
	ref := &dataLocation{numFrames: 2}
	images := &CLImages{idrefs: map[uint32]*dataLocation{7: ref}}
	pixels := image.NewRGBA(image.Rect(0, 0, 18, 18))
	fillAlphaRect(pixels, image.Rect(6, 3, 10, 8))

	images.cacheVisibleFrameSize(ref, pixels)
	if w, h := images.VisibleFrameSize(7); w != 4 || h != 5 {
		t.Fatalf("VisibleFrameSize(7) = %dx%d, want 4x5", w, h)
	}

	// A later decode cannot replace the immutable artwork's cached metric.
	fillAlphaRect(pixels, image.Rect(1, 1, 17, 17))
	images.cacheVisibleFrameSize(ref, pixels)
	if w, h := images.VisibleFrameSize(7); w != 4 || h != 5 {
		t.Fatalf("cached VisibleFrameSize(7) = %dx%d, want 4x5", w, h)
	}
}

func fillAlphaRect(pixels *image.RGBA, rect image.Rectangle) {
	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		for x := rect.Min.X; x < rect.Max.X; x++ {
			pixels.Pix[pixels.PixOffset(x, y)+3] = 0xff
		}
	}
}
