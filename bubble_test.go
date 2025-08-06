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
