package main

import (
	"image"
	"testing"
)

func TestLayeredShadowCoverageGaps(t *testing.T) {
	resetLayeredCharacterShadows()
	defer resetLayeredCharacterShadows()
	left, right := image.Rect(-20, 0, -10, 10), image.Rect(10, 0, 20, 10)
	recordLayeredShadowCoverage(left)
	recordLayeredShadowCoverage(right)
	for _, tc := range []struct {
		box     image.Rectangle
		overlap bool
	}{
		{image.Rect(-5, 0, 5, 10), false},   // Inside union, outside both casters.
		{image.Rect(-10, 0, 10, 10), false}, // Touching is not overlap.
		{image.Rect(-11, 2, -9, 4), true},
		{image.Rect(19, 9, 30, 20), true},
		{image.Rect(-40, -40, 40, 40), true},
		{image.Rectangle{}, false},
	} {
		if got := overlapsLayeredShadowCoverage(tc.box); got != tc.overlap {
			t.Errorf("%v overlap=%v want %v", tc.box, got, tc.overlap)
		}
	}
	resetLayeredCharacterShadows()
	if overlapsLayeredShadowCoverage(left) {
		t.Fatal("coverage survived reset")
	}
}
