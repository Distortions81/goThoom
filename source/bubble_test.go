package main

import "testing"

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
