package main

import "testing"

func TestPictureMotionInterpolationIsLimitedToSmallSpritesAndClouds(t *testing.T) {
	originalSmallMoving := gs.InterpolateSmallMovingPictures
	originalSemiTransparent := pictureSemiTransparent
	originalPictureSize := pictureSize
	originalPictureVisibleSize := pictureVisibleSize
	pixelCountMu.Lock()
	originalCounts := pixelCountCache
	pixelCountCache = map[uint16]int{1: minMovingPicturePixels - 1, 2: minMovingPicturePixels, 3: minMovingPicturePixels}
	pixelCountMu.Unlock()
	t.Cleanup(func() {
		gs.InterpolateSmallMovingPictures = originalSmallMoving
		pictureSemiTransparent = originalSemiTransparent
		pictureSize = originalPictureSize
		pictureVisibleSize = originalPictureVisibleSize
		pixelCountMu.Lock()
		pixelCountCache = originalCounts
		pixelCountMu.Unlock()
	})

	gs.InterpolateSmallMovingPictures = false
	pictureSemiTransparent = func(id uint16) bool { return id == 3 }
	pictureSize = func(id uint16) (int, int) {
		switch id {
		case 1:
			return 16, 20
		case 3:
			return 256, 128
		default:
			return maxInterpolatedMovingPictureDimension + 1, 16
		}
	}
	pictureVisibleSize = func(id uint16) (int, int) {
		switch id {
		case 1:
			return 16, 20
		case 3:
			return 22, 18
		default:
			return maxInterpolatedMovingPictureDimension + 1, 16
		}
	}
	small := framePicture{PictID: 1, Plane: 1}
	largeOpaque := framePicture{PictID: 2, Plane: 1}
	cloud := framePicture{PictID: 3, Plane: 1}
	if pictureMotionInterpolationEnabled(small) {
		t.Fatal("small sprite enabled moving interpolation")
	}
	if pictureCloudMotionEnabled(largeOpaque) {
		t.Fatal("large opaque sprite enabled moving interpolation")
	}
	if !pictureCloudMotionEnabled(cloud) {
		t.Fatal("large semi-transparent sprite did not enable moving interpolation")
	}
	if pictureCloudMotionEnabled(framePicture{PictID: 3, Plane: 0}) {
		t.Fatal("large semi-transparent sprite behind mobiles enabled cloud motion")
	}
	if pictureExcludedFromShift(largeOpaque) {
		t.Fatal("large opaque sprite excluded from camera-motion detection")
	}
	if !pictureExcludedFromShift(cloud) {
		t.Fatal("large semi-transparent sprite affected camera-motion detection")
	}
	gs.InterpolateSmallMovingPictures = true
	if !pictureMotionInterpolationEnabled(small) {
		t.Fatal("small-moving option did not enable a small sprite")
	}
	if pictureMotionInterpolationEnabled(largeOpaque) {
		t.Fatal("small-moving option enabled an oversized sprite")
	}
}

func TestPictureMotionInterpolationUsesVisibleFrameSize(t *testing.T) {
	originalSmallMoving := gs.InterpolateSmallMovingPictures
	originalPictureVisibleSize := pictureVisibleSize
	t.Cleanup(func() {
		gs.InterpolateSmallMovingPictures = originalSmallMoving
		pictureVisibleSize = originalPictureVisibleSize
	})

	gs.InterpolateSmallMovingPictures = true
	pictureVisibleSize = func(id uint16) (int, int) {
		switch id {
		case 1:
			// The source canvas can be much larger than the visible coin.
			return 12, 14
		case 2:
			return maxInterpolatedMovingPictureDimension, maxInterpolatedMovingPictureDimension
		}
		// One animation frame exceeds the small-sprite limit.
		return 12, maxInterpolatedMovingPictureDimension + 1
	}
	if !pictureMotionInterpolationEnabled(framePicture{PictID: 1}) {
		t.Fatal("transparent canvas padding prevented small-sprite interpolation")
	}
	if !pictureMotionInterpolationEnabled(framePicture{PictID: 2}) {
		t.Fatal("sprite at the visible-size limit was not interpolated")
	}
	if pictureMotionInterpolationEnabled(framePicture{PictID: 3}) {
		t.Fatal("oversized visible animation frame enabled small-sprite interpolation")
	}
}

func TestSmallPictureMotionInterpolationHasHardMovementLimit(t *testing.T) {
	previous := framePicture{H: 10, V: 20}
	if !smallPictureMotionWithinInterpolationLimit(framePicture{H: 74, V: 20}, previous, 0, 0) {
		t.Fatal("64-pixel movement was rejected")
	}
	if smallPictureMotionWithinInterpolationLimit(framePicture{H: 75, V: 20}, previous, 0, 0) {
		t.Fatal("65-pixel movement was accepted")
	}
	if smallPictureMotionWithinInterpolationLimit(framePicture{H: 74, V: 84}, previous, 0, 0) {
		t.Fatal("diagonal movement beyond 64 pixels was accepted")
	}
	if !smallPictureMotionWithinInterpolationLimit(framePicture{H: 84, V: 20}, previous, 10, 0) {
		t.Fatal("camera-adjusted 64-pixel movement was rejected")
	}
}

func TestMostlyOffscreenCloudKeepsMotion(t *testing.T) {
	originalImages := clImages
	originalSemiTransparent := pictureSemiTransparent
	originalPictureSize := pictureSize
	t.Cleanup(func() {
		clImages = originalImages
		pictureSemiTransparent = originalSemiTransparent
		pictureSize = originalPictureSize
	})

	const width, height = 256, 128
	clImages = mockCLImages(width, height)
	pictureSize = func(uint16) (int, int) { return width, height }
	pictureSemiTransparent = func(uint16) bool { return true }

	// Leave only 20% of the cloud inside the left edge of the field.
	cloud := framePicture{
		PictID: 1,
		Plane:  1,
		H:      int16(-fieldCenterX - width*3/10),
	}
	if !pictureOnEdge(cloud) {
		t.Fatal("test cloud was not mostly offscreen")
	}
	cloudMotion := pictureCloudMotionEnabled(cloud)
	if !cloudMotion {
		t.Fatal("mostly offscreen cloud was not detected")
	}
	if pictureMotionBlockedAtEdge(cloud, cloudMotion) {
		t.Fatal("mostly offscreen cloud motion was blocked by the edge rule")
	}
	if !pictureMotionBlockedAtEdge(cloud, false) {
		t.Fatal("ordinary mostly offscreen picture should still be blocked")
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
	scratch := matchPicturePositions(prev, cur, 10, 0, maxInterpPixels, 0)
	defer releasePicturePositionScratch(scratch)
	if got := scratch.matches[0]; got != 1 {
		t.Fatalf("first current picture matched previous %d, want 1", got)
	}
	if got := scratch.matches[1]; got != 0 {
		t.Fatalf("second current picture matched previous %d, want 0", got)
	}
}
