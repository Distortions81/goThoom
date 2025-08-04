package main

import "testing"

// TestLerpBar verifies that fastBars disables interpolation for decreases while
// still interpolating increases.
func TestLerpBar(t *testing.T) {
	prev := 100
	cur := 50
	alpha := 0.5

	fastBars = false
	if got := lerpBar(prev, cur, alpha); got != 75 {
		t.Fatalf("expected interpolated value 75, got %d", got)
	}

	fastBars = true
	if got := lerpBar(prev, cur, alpha); got != cur {
		t.Fatalf("expected fast drop to %d, got %d", cur, got)
	}

	// Increases still interpolate when fastBars is enabled.
	if got := lerpBar(cur, prev, alpha); got != 75 {
		t.Fatalf("expected interpolated increase 75, got %d", got)
	}
	fastBars = false
}
