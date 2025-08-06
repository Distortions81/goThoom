package main

import "testing"

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

func TestAdjustBubbleRectClampRightScaled(t *testing.T) {
	scale := 2
	sw, sh := 200*scale, 200*scale
	width, height := 100*scale, 50*scale
	tailHeight := 10 * scale
	// place tail tip off-screen to the right
	x, y := sw+10*scale, 100*scale
	left, top, right, bottom, ax, ay := adjustBubbleRect(x, y, width, height, tailHeight, sw, sh, false)

	if right != sw {
		t.Fatalf("expected right to be clamped to %d, got %d", sw, right)
	}
	if left != sw-width {
		t.Fatalf("expected left to be %d, got %d", sw-width, left)
	}
	if ax != left+width/2 {
		t.Fatalf("tail x not shifted correctly: %d != %d", ax, left+width/2)
	}
	if ay != bottom+tailHeight {
		t.Fatalf("tail y not shifted correctly: %d != %d", ay, bottom+tailHeight)
	}
	if top != bottom-height {
		t.Fatalf("expected height %d, got %d", height, bottom-top)
	}
}

func TestAdjustBubbleRectClampBottomScaled(t *testing.T) {
	scale := 2
	sw, sh := 200*scale, 200*scale
	width, height := 100*scale, 50*scale
	tailHeight := 10 * scale
	// place tail tip off-screen at the bottom
	x, y := 100*scale, sh+10*scale
	left, top, right, bottom, ax, ay := adjustBubbleRect(x, y, width, height, tailHeight, sw, sh, false)

	if bottom != sh {
		t.Fatalf("expected bottom to be clamped to %d, got %d", sh, bottom)
	}
	if top != sh-height {
		t.Fatalf("expected top to be %d, got %d", sh-height, top)
	}
	if ax != left+width/2 {
		t.Fatalf("tail x not shifted correctly: %d != %d", ax, left+width/2)
	}
	if ay != bottom+tailHeight {
		t.Fatalf("tail y not shifted correctly: %d != %d", ay, bottom+tailHeight)
	}
	if right-left != width {
		t.Fatalf("expected width %d, got %d", width, right-left)
	}
}
