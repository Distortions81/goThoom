package main

import (
	"math"
	"strings"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
	text "github.com/hajimehoshi/ebiten/v2/text/v2"
)

func TestAdjustBubbleRectClampLeft(t *testing.T) {
	sw, sh := 200, 200
	width, height := 100, 50
	tailHeight := 10
	x, y := 10, 100
	left, _, right, bottom, ax, ay := adjustBubbleRect(x, y, width, height, tailHeight, sw, sh, false)

	if left != 0 {
		t.Fatalf("expected left to be clamped to 0, got %d", left)
	}
	if right-left != width {
		t.Fatalf("expected width %d, got %d", width, right-left)
	}
	if ax != left+width/2 {
		t.Fatalf("tail x not shifted correctly: %d != %d", ax, left+width/2)
	}
	if ay != bottom+tailHeight {
		t.Fatalf("tail y not shifted correctly: %d != %d", ay, bottom+tailHeight)
	}
}

func TestAdjustBubbleRectClampTop(t *testing.T) {
	sw, sh := 200, 200
	width, height := 100, 50
	tailHeight := 10
	x, y := 100, 20
	left, top, _, bottom, ax, ay := adjustBubbleRect(x, y, width, height, tailHeight, sw, sh, false)

	if top != 0 {
		t.Fatalf("expected top to be clamped to 0, got %d", top)
	}
	if bottom-top != height {
		t.Fatalf("expected height %d, got %d", height, bottom-top)
	}
	if ax != left+width/2 {
		t.Fatalf("tail x not shifted correctly: %d != %d", ax, left+width/2)
	}
	if ay != bottom+tailHeight {
		t.Fatalf("tail y not shifted correctly: %d != %d", ay, bottom+tailHeight)
	}
}

func TestDrawBubbleWideNoArtifacts(t *testing.T) {
	scale = 3
	initFont()

	pad := (4 + 2) * scale
	maxLineWidth := gameAreaSizeX/4*scale - 2*pad

	// Construct a single-line string near the maximum width.
	cw, _ := text.Measure("W", bubbleFont, 0)
	count := int(float64(maxLineWidth) / cw)
	txt := strings.Repeat("W", count)

	screen := ebiten.NewImage(gameAreaSizeX, gameAreaSizeY)
	borderCol, bgCol, textCol := bubbleColors(kBubbleNormal)
	drawBubble(screen, txt, gameAreaSizeX/2, gameAreaSizeY/2, kBubbleNormal, false, false, borderCol, bgCol, textCol)

	width, lines := wrapText(txt, bubbleFont, float64(maxLineWidth))
	if len(lines) != 1 {
		t.Fatalf("expected single line, got %d", len(lines))
	}
	metrics := bubbleFont.Metrics()
	lineHeight := int(math.Ceil(metrics.HAscent) + math.Ceil(metrics.HDescent) + math.Ceil(metrics.HLineGap))
	width += 2 * pad
	height := lineHeight*len(lines) + 2*pad
	left, top, right, bottom, _, _ := adjustBubbleRect(gameAreaSizeX/2, gameAreaSizeY/2, width, height, 10*scale, gameAreaSizeX, gameAreaSizeY, false)

	rowY := bottom - pad/2
	bgR, bgG, bgB, bgA := bgCol.RGBA()
	expR, expG, expB, expA := uint8(bgR>>8), uint8(bgG>>8), uint8(bgB>>8), uint8(bgA>>8)
	for x := left + pad; x < right-pad; x++ {
		r, g, b, a := screen.At(x, rowY).RGBA()
		if uint8(r>>8) != expR || uint8(g>>8) != expG || uint8(b>>8) != expB || uint8(a>>8) != expA {
			t.Fatalf("unexpected pixel at (%d,%d): rgba(%d,%d,%d,%d)", x, rowY, r>>8, g>>8, b>>8, a>>8)
		}
	}

	if rowY <= top || rowY >= bottom {
		t.Fatalf("row not inside bubble: %d", rowY)
	}
}
