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

func TestPictureMobileOffsetRequiresExactMatch(t *testing.T) {
	origPlayerIndex := playerIndex
	playerIndex = 1
	defer func() { playerIndex = origPlayerIndex }()

	mobiles := []frameMobile{{Index: 1, H: 10, V: 20}}
	prevMobiles := map[uint8]frameMobile{1: {Index: 1, H: 8, V: 18}}
	positions := map[picturePositionKey]struct{}{
		{pictID: 5, h: 13, v: 19}: {}, // Four pixels from the expected position.
	}
	p := framePicture{PictID: 5, H: 12, V: 23}
	if _, _, ok := pictureMobileOffset(p, mobiles, prevMobiles, positions, 0.5); ok {
		t.Fatal("picture with a changed mobile offset was pinned")
	}
}

func TestPictureFollowingBackgroundIsNotPinned(t *testing.T) {
	originalPinning := gs.ObjectPinning
	originalSmoothing := gs.MotionSmoothing
	gs.ObjectPinning = true
	gs.MotionSmoothing = true
	defer func() {
		gs.ObjectPinning = originalPinning
		gs.MotionSmoothing = originalSmoothing
	}()

	if pictureCanPinToMobile(framePicture{Moving: true, Background: true}, 64, 64) {
		t.Fatal("background picture must not be pinned")
	}
	if pictureCanPinToMobile(framePicture{}, 64, 64) {
		t.Fatal("non-moving picture must not be pinned")
	}
	if !pictureCanPinToMobile(framePicture{Moving: true}, 64, 64) {
		t.Fatal("independently moving picture should be eligible for pinning")
	}
}
