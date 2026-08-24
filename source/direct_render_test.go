package main

import (
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
)

func TestWorldArtworkFilterFollowsPixelArtSetting(t *testing.T) {
	original := gs.PixelArtScaling
	t.Cleanup(func() { gs.PixelArtScaling = original })

	gs.PixelArtScaling = true
	if got := worldArtworkFilter(); got != ebiten.FilterNearest {
		t.Fatalf("pixel-art filter = %v, want nearest", got)
	}
	gs.PixelArtScaling = false
	if got := worldArtworkFilter(); got != ebiten.FilterLinear {
		t.Fatalf("smooth filter = %v, want linear", got)
	}
}

func TestScaledSpriteSpanKeepsSharedTileEdgesClosed(t *testing.T) {
	for _, scale := range []float64{0.73, 1, 1.1, 4.0 / 3.0, 1.75, 2.5} {
		for _, offset := range []float64{-127.4, 0, 0.37, 91.8} {
			const edge = 100.0
			const leftSize, rightSize = 24, 40
			leftCenter := (edge - float64(leftSize)/2 + offset) * scale
			rightCenter := (edge + float64(rightSize)/2 + offset) * scale
			_, leftRight := scaledSpriteSpan(leftCenter, leftSize, scale)
			rightLeft, _ := scaledSpriteSpan(rightCenter, rightSize, scale)
			if leftRight != rightLeft {
				t.Fatalf("scale %.3f offset %.2f produced a tile gap: left right=%d, right left=%d", scale, offset, leftRight, rightLeft)
			}
		}
	}
}
