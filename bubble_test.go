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

func TestBubbleTailMatchesBackground(t *testing.T) {
	orig := gs
	gs = gsdef
	gs.GameScale = 1
	gs.BubbleScale = 1
	gs.BubbleOpacity = 0.8
	initFont()
	t.Cleanup(func() {
		gs = orig
		initFont()
	})

	screen := ebiten.NewImage(gameAreaSizeX, gameAreaSizeY)
	border, bg, text := bubbleColors(kBubbleNormal)
	drawBubble(screen, "tail test", 200, 200, kBubbleNormal, false, false, border, bg, text)

	sampleX, sampleY := 200, 195
	gotR, gotG, gotB, gotA := screen.At(sampleX, sampleY).RGBA()
	wantR, wantG, wantB, wantA := bg.RGBA()
	if gotR != wantR || gotG != wantG || gotB != wantB || gotA != wantA {
		t.Fatalf("tail pixel mismatch at (%d,%d): got (%#x,%#x,%#x,%#x), want (%#x,%#x,%#x,%#x)",
			sampleX, sampleY, gotR, gotG, gotB, gotA, wantR, wantG, wantB, wantA)
	}
}
