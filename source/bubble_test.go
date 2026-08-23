package main

import (
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
)

func TestAdjustBubbleRectNoTail(t *testing.T) {
	sw, sh := 100, 100
	width, height := 20, 20
	tailHeight := 10
	x, y := 50, 90
	_, _, _, bottom := adjustBubbleRect(x, y, width, height, tailHeight, sw, sh, true)
	if bottom != y {
		t.Fatalf("expected bottom %d, got %d", y, bottom)
	}
}

func TestThoughtBubbleMaskUsesMaximumAlpha(t *testing.T) {
	if thoughtBubbleMaskBlend.BlendOperationRGB != ebiten.BlendOperationMax {
		t.Fatal("thought-bubble mask does not combine color coverage by maximum")
	}
	if thoughtBubbleMaskBlend.BlendOperationAlpha != ebiten.BlendOperationMax {
		t.Fatal("thought-bubble mask does not combine alpha coverage by maximum")
	}
}
