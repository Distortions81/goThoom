package main

import "testing"

func TestToolbarHandsMatchButtonHeight(t *testing.T) {
	w, h := toolbarHandsSize(84, 42, 48)
	if w != 96 || h != 48 || w/2 != 48 {
		t.Fatalf("two-hand toolbar size = %dx%d, want two 48x48 slots", w, h)
	}
	itemX, itemY := pictureSourceScale(42, 42, 168, 168)
	if int(168*itemX) != 42 || int(168*itemY) != 42 {
		t.Fatalf("HD held item footprint = %vx%v, want 42x42", 168*itemX, 168*itemY)
	}
}
