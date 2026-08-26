package main

import (
	"image"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
)

func TestLocalLightingPositionRemovesSubimageOrigin(t *testing.T) {
	x, y := localLightingPosition(117, 83, image.Rect(37, 29, 584, 569))
	if x != 80 || y != 54 {
		t.Fatalf("local lighting position = (%v, %v), want (80, 54)", x, y)
	}
}

func TestWorldArtworkFilterFollowsPixelArtSetting(t *testing.T) {
	original := gs
	t.Cleanup(func() { gs = original })

	setArtworkUpscaleMode(artworkUpscaleBalanced)
	gs.PixelArtScaling = true
	if got := worldArtworkFilter(); got != ebiten.FilterNearest {
		t.Fatalf("pixel-art filter = %v, want nearest", got)
	}
	gs.PixelArtScaling = false
	if got := worldArtworkFilter(); got != ebiten.FilterLinear {
		t.Fatalf("smooth filter = %v, want linear", got)
	}

	setArtworkUpscaleMode(artworkUpscaleOff)
	if got := worldArtworkFilter(); got != ebiten.FilterNearest {
		t.Fatalf("disabled scaler filter = %v, want nearest", got)
	}
}

func TestReplacementEffectKinds(t *testing.T) {
	tests := []struct {
		pictID uint16
		want   replacementEffectKind
		ok     bool
	}{
		{1759, replacementEffectHealing, true},
		{1760, replacementEffectHealing, true},
		{1286, replacementEffectMysticWard, true},
		{1, 0, false},
	}
	for _, test := range tests {
		got, ok := replacementEffectKindForPict(test.pictID)
		if got != test.want || ok != test.ok {
			t.Errorf("replacementEffectKindForPict(%d) = (%d, %v), want (%d, %v)", test.pictID, got, ok, test.want, test.ok)
		}
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

func TestLinearFilteredTileSpansOverlapAtSharedEdges(t *testing.T) {
	const scale = 1.37
	_, leftRight := filteredSpriteSpan(84*scale, 32, scale, ebiten.FilterLinear)
	rightLeft, _ := filteredSpriteSpan(116*scale, 32, scale, ebiten.FilterLinear)
	if overlap := leftRight - rightLeft; overlap != 1 {
		t.Fatalf("linear tile overlap = %v pixels, want 1", overlap)
	}

	_, nearestRight := filteredSpriteSpan(84*scale, 32, scale, ebiten.FilterNearest)
	nearestLeft, _ := filteredSpriteSpan(116*scale, 32, scale, ebiten.FilterNearest)
	if nearestRight != nearestLeft {
		t.Fatalf("nearest tile edges = %v and %v, want a shared edge", nearestRight, nearestLeft)
	}
}
