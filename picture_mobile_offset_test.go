package main

import "testing"

func TestPictureMobileOffsetUsesPositionIndex(t *testing.T) {
	origPlayerIndex := playerIndex
	playerIndex = 1
	defer func() { playerIndex = origPlayerIndex }()

	mobiles := []frameMobile{{Index: 1, H: 10, V: 20}}
	prevMobiles := map[uint8]frameMobile{1: {Index: 1, H: 8, V: 18}}
	positions := map[picturePositionKey]struct{}{
		{pictID: 5, h: 10, v: 21}: {},
	}
	p := framePicture{PictID: 5, H: 12, V: 23}
	dx, dy, ok := pictureMobileOffset(p, mobiles, prevMobiles, positions, 0.5)
	if !ok || dx != -1 || dy != -1 {
		t.Fatalf("pictureMobileOffset = (%v, %v, %v), want (-1, -1, true)", dx, dy, ok)
	}
}

func TestHasPreviousPictureRejectsCoordinateOverflow(t *testing.T) {
	positions := map[picturePositionKey]struct{}{
		{pictID: 5, h: -32768, v: 0}: {},
	}
	if hasPreviousPicture(positions, 5, 32768, 0) {
		t.Fatal("overflowing coordinate wrapped to an indexed picture")
	}
}

func TestPictureMobileOffsetAllowsSmallSpriteJitter(t *testing.T) {
	origPlayerIndex := playerIndex
	origPictureSize := pictureSize
	pixelCountMu.Lock()
	origPixelCounts := pixelCountCache
	pixelCountCache = map[uint16]int{5: 2000}
	pixelCountMu.Unlock()
	playerIndex = 1
	pictureSize = func(uint16) (int, int) { return 64, 64 }
	defer func() {
		playerIndex = origPlayerIndex
		pictureSize = origPictureSize
		pixelCountMu.Lock()
		pixelCountCache = origPixelCounts
		pixelCountMu.Unlock()
	}()

	mobiles := []frameMobile{{Index: 1, H: 10, V: 20}}
	prevMobiles := map[uint8]frameMobile{1: {Index: 1, H: 8, V: 18}}
	positions := map[picturePositionKey]struct{}{
		{pictID: 5, h: 13, v: 19}: {}, // Four pixels from the expected position.
	}
	p := framePicture{PictID: 5, H: 12, V: 23}
	if _, _, ok := pictureMobileOffset(p, mobiles, prevMobiles, positions, 0.5); !ok {
		t.Fatal("small wandering picture was not pinned to the mobile")
	}
}

func TestPictureMobileOffsetRejectsLargeSpriteJitter(t *testing.T) {
	origPlayerIndex := playerIndex
	origPictureSize := pictureSize
	playerIndex = 1
	pictureSize = func(uint16) (int, int) { return 128, 128 }
	defer func() {
		playerIndex = origPlayerIndex
		pictureSize = origPictureSize
	}()

	mobiles := []frameMobile{{Index: 1, H: 10, V: 20}}
	prevMobiles := map[uint8]frameMobile{1: {Index: 1, H: 8, V: 18}}
	positions := map[picturePositionKey]struct{}{
		{pictID: 5, h: 13, v: 19}: {},
	}
	p := framePicture{PictID: 5, H: 12, V: 23}
	if _, _, ok := pictureMobileOffset(p, mobiles, prevMobiles, positions, 0.5); ok {
		t.Fatal("large picture was pinned despite offset jitter")
	}
}

func TestPictureCanWanderWithMobileRequiresTransparency(t *testing.T) {
	origPictureSize := pictureSize
	pixelCountMu.Lock()
	origPixelCounts := pixelCountCache
	pixelCountCache = map[uint16]int{5: 2500, 6: 2400}
	pixelCountMu.Unlock()
	pictureSize = func(uint16) (int, int) { return 64, 64 }
	defer func() {
		pictureSize = origPictureSize
		pixelCountMu.Lock()
		pixelCountCache = origPixelCounts
		pixelCountMu.Unlock()
	}()

	if pictureCanWanderWithMobile(5) {
		t.Fatal("sprite with less than 40% transparency can wander with a mobile")
	}
	if !pictureCanWanderWithMobile(6) {
		t.Fatal("sprite with more than 40% transparency cannot wander with a mobile")
	}
}
