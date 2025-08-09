package main

import (
	"image/color"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
)

// BenchmarkNonTransparentPixels verifies that repeated calls do not repeatedly
// read pixels from the GPU by benchmarking successive invocations.
func BenchmarkNonTransparentPixels(b *testing.B) {
	// Prepare a small image with all opaque pixels.
	img := ebiten.NewImage(16, 16)
	img.Fill(color.White)

	// Prime the image cache so loadImage returns our image.
	imageMu.Lock()
	imageCache[makeImageKey(1, 0)] = img
	imageMu.Unlock()

	// Clear caches to ensure the first call performs the ReadPixels, and
	// subsequent calls reuse the cached buffer.
	pixelCountMu.Lock()
	pixelCountCache = make(map[uint16]int)
	pixelCountMu.Unlock()
	pixelDataMu.Lock()
	pixelDataCache = make(map[uint16][]byte)
	pixelDataMu.Unlock()

	// Warm up once so the benchmark measures cached calls.
	if c := nonTransparentPixels(1); c != 16*16 {
		b.Fatalf("unexpected pixel count %d", c)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if nonTransparentPixels(1) != 16*16 {
			b.Fatal("pixel count mismatch")
		}
	}
}
