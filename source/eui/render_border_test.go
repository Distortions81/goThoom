package eui

import "testing"

func TestInsideRectBorderRectsKeepAllEdgesInsideBounds(t *testing.T) {
	got := insideRectBorderRects(0, 0, 10, 10, 1)
	want := [4]rect{
		{X0: 0, Y0: 0, X1: 10, Y1: 1},
		{X0: 0, Y0: 9, X1: 10, Y1: 10},
		{X0: 0, Y0: 0, X1: 1, Y1: 10},
		{X0: 9, Y0: 0, X1: 10, Y1: 10},
	}
	if got != want {
		t.Fatalf("inside border rects got %+v, want %+v", got, want)
	}
}

func TestRenderImageSizeMatchesRoundedDrawSize(t *testing.T) {
	w, h := renderImageSize(point{X: 100.6, Y: 40.6})
	if w != 101 || h != 41 {
		t.Fatalf("rounded image size got %dx%d, want 101x41", w, h)
	}

	w, h = renderImageSize(point{X: 100.4, Y: 40.4})
	if w != 100 || h != 40 {
		t.Fatalf("rounded image size got %dx%d, want 100x40", w, h)
	}
}
