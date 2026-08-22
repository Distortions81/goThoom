package main

import "testing"

func TestPictureMotionInterpolationEnabledForLargeSprites(t *testing.T) {
	originalSmoothMoving := gs.smoothMoving
	originalSemiTransparent := pictureSemiTransparent
	originalPictureSize := pictureSize
	pixelCountMu.Lock()
	originalCounts := pixelCountCache
	pixelCountCache = map[uint16]int{1: minMovingPicturePixels - 1, 2: minMovingPicturePixels, 3: minMovingPicturePixels}
	pixelCountMu.Unlock()
	t.Cleanup(func() {
		gs.smoothMoving = originalSmoothMoving
		pictureSemiTransparent = originalSemiTransparent
		pictureSize = originalPictureSize
		pixelCountMu.Lock()
		pixelCountCache = originalCounts
		pixelCountMu.Unlock()
	})

	gs.smoothMoving = false
	pictureSemiTransparent = func(id uint16) bool { return id == 3 }
	pictureSize = func(id uint16) (int, int) {
		if id == 3 {
			return 256, 128
		}
		return 32, 32
	}
	if pictureMotionInterpolationEnabled(1) {
		t.Fatal("small sprite enabled moving interpolation")
	}
	if pictureCloudMotionEnabled(2) {
		t.Fatal("large opaque sprite enabled moving interpolation")
	}
	if !pictureCloudMotionEnabled(3) {
		t.Fatal("large semi-transparent sprite did not enable moving interpolation")
	}
	if pictureExcludedFromShift(2) {
		t.Fatal("large opaque sprite excluded from camera-motion detection")
	}
	if !pictureExcludedFromShift(3) {
		t.Fatal("large semi-transparent sprite affected camera-motion detection")
	}
	gs.smoothMoving = true
	if !pictureMotionInterpolationEnabled(1) {
		t.Fatal("smooth-moving option did not enable interpolation")
	}
}

func TestMatchPicturePositionsAvoidsGreedyDuplicateSwap(t *testing.T) {
	prev := []framePicture{
		{PictID: 9, H: 0, V: 0},
		{PictID: 9, H: 20, V: 0},
	}
	cur := []framePicture{
		{PictID: 9, H: 12, V: 0},
		{PictID: 9, H: 10, V: 0},
	}
	matches := matchPicturePositions(prev, cur, 10, 0, maxInterpPixels, 0)
	if got := matches[0]; got != 1 {
		t.Fatalf("first current picture matched previous %d, want 1", got)
	}
	if got := matches[1]; got != 0 {
		t.Fatalf("second current picture matched previous %d, want 0", got)
	}
}
