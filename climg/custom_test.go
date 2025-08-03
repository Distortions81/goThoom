package climg

import "testing"

func TestApplyCustomPalette(t *testing.T) {
	col := []uint16{0, 1, 2, 3, 4}
	mapping := []byte{2, 3}
	custom := []byte{10, 11}
	applyCustomPalette(col, mapping, custom)
	if col[2] != 10 || col[3] != 11 {
		t.Fatalf("unexpected palette: %v", col)
	}
}
