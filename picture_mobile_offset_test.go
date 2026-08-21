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
