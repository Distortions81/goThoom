package main

import (
	"math"
	"testing"
	"time"

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

func TestPonderBubbleAnimationAdvances(t *testing.T) {
	if got := ponderBubblePhase(250 * time.Millisecond); math.Abs(got-1) > 1e-9 {
		t.Fatalf("ponder animation phase after 250ms = %v, want 1", got)
	}
	if first, next := ponderWaveOffset(0, 0, 6), ponderWaveOffset(math.Pi/2, 0, 6); first == next {
		t.Fatalf("ponder wave offset did not change between phases: %v", first)
	}
}

func TestBubbleAnimationCanBeDisabled(t *testing.T) {
	original := gs.AnimatedChatBubbles
	t.Cleanup(func() { gs.AnimatedChatBubbles = original })

	gs.AnimatedChatBubbles = false
	if phase := bubbleAnimationPhase(4); phase != 0 {
		t.Fatalf("disabled bubble animation phase = %v, want 0", phase)
	}
}
