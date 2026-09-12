package main

import (
	"image"
	"testing"
)

func TestGameOverlayLayoutStacksReservationsAtEveryAnchor(t *testing.T) {
	bounds := image.Rect(10, 20, 310, 220)
	layout := newGameOverlayLayout(bounds)

	topLeftA := layout.Add(gameOverlayTopLeft, image.Pt(50, 20))
	topLeftB := layout.Add(gameOverlayTopLeft, image.Pt(70, 10))
	topRight := layout.Add(gameOverlayTopRight, image.Pt(54, 16))
	top := layout.Add(gameOverlayTop, image.Pt(40, 12))
	left := layout.Add(gameOverlayLeft, image.Pt(30, 14))
	right := layout.Add(gameOverlayRight, image.Pt(32, 16))
	bottom := layout.Add(gameOverlayBottom, image.Pt(44, 18))
	bottomLeft := layout.Add(gameOverlayBottomLeft, image.Pt(58, 20))
	bottomRight := layout.Add(gameOverlayBottomRight, image.Pt(60, 22))

	rects := layout.Rects()
	wants := map[int]image.Rectangle{
		topLeftA:    image.Rect(18, 28, 68, 48),
		topLeftB:    image.Rect(18, 52, 88, 62),
		topRight:    image.Rect(248, 28, 302, 44),
		top:         image.Rect(140, 28, 180, 40),
		left:        image.Rect(18, 113, 48, 127),
		right:       image.Rect(270, 112, 302, 128),
		bottom:      image.Rect(138, 194, 182, 212),
		bottomLeft:  image.Rect(18, 192, 76, 212),
		bottomRight: image.Rect(242, 190, 302, 212),
	}
	for handle, want := range wants {
		if got := rects[handle]; got != want {
			t.Errorf("reservation %d = %v, want %v", handle, got, want)
		}
	}
	if rects[topLeftA].Overlaps(rects[topLeftB]) {
		t.Fatalf("top-left reservations overlap: %v and %v", rects[topLeftA], rects[topLeftB])
	}
}

func TestGameOverlayLayoutNormalizesConflictingEdges(t *testing.T) {
	layout := newGameOverlayLayout(image.Rect(0, 0, 100, 100))
	handle := layout.Add(gameOverlayTop|gameOverlayBottom|gameOverlayLeft|gameOverlayRight, image.Pt(20, 10))
	if got, want := layout.Rects()[handle], image.Rect(40, 45, 60, 55); got != want {
		t.Fatalf("conflicting anchors = %v, want centered %v", got, want)
	}
}
