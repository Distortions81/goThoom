package main

import (
	"testing"

	"gothoom/climg"
)

// Encode just the draw-state tables, exercising the same picture matching and
// state transfer used by live frames without requiring a game asset archive.
func lightingMotionPacket(pictures []framePicture, again int) []byte {
	data := make([]byte, 17) // acknowledgement, descriptors, vitals, lighting
	data = append(data, 255, byte(again), byte(len(pictures)))
	packed := make([]byte, (len(pictures)*36+7)/8)
	bit := 0
	write := func(value uint32, count int) {
		for shift := count - 1; shift >= 0; shift-- {
			packed[bit/8] |= byte((value>>shift)&1) << (7 - bit%8)
			bit++
		}
	}
	for _, p := range pictures {
		write(uint32(p.PictID), 14)
		write(uint32(uint16(p.H))&2047, 11)
		write(uint32(uint16(p.V))&2047, 11)
	}
	data = append(data, packed...)
	return append(data, 0) // mobiles
}

func TestPictureLightFlickerFollowsCameraAndPictureMotion(t *testing.T) {
	oldState, oldSettings, oldImages := state, gs, clImages
	oldMovie, oldVersion, oldSeeking, oldFrame := movieMode, movieVersion, seekingMov, frameCounter
	oldNight := gNight
	oldCounts := pixelCountCache
	t.Cleanup(func() {
		state, gs, clImages = oldState, oldSettings, oldImages
		movieMode, movieVersion, seekingMov, frameCounter = oldMovie, oldVersion, oldSeeking, oldFrame
		gNight = oldNight
		pixelCountCache = oldCounts
	})
	clImages = nil
	movieMode, movieVersion, seekingMov = true, 367, false
	pixelCountCache = map[uint16]int{1: 100, 2: 10000, 3: 10000}
	gs.FadeObscuringPictures = false
	gs.GameScale = 1
	gs.FloatingPointSpriteCoords = true
	for _, smoothing := range []bool{true, false} {
		gs.MotionSmoothing = smoothing
		resetState()
		frameCounter = 0
		previous := []framePicture{{PictID: 1, H: -80, V: 10}, {PictID: 1, H: 80, V: 20}, {PictID: 2, H: -120, V: 100}, {PictID: 3, H: 120, V: 100}}
		parse := func(pictures []framePicture, again int) []framePicture {
			t.Helper()
			if _, _, err := parseDrawStateWithStateData(lightingMotionPacket(pictures, again), false, false); err != nil {
				t.Fatal(err)
			}
			return append([]framePicture(nil), state.pictures...)
		}
		previous = parse(previous, 0)
		if previous[0].lightKey == 0 || previous[0].lightKey == previous[1].lightKey {
			t.Fatal("distinct light instances shared a key")
		}
		for step := 0; step < 4; step++ {
			// Reorder identical lights while panning; the first light also moves
			// independently on alternating updates.
			const dx, dy = -12, 5
			current := append([]framePicture(nil), previous...)
			for i := range current {
				current[i].H += dx
				current[i].V += dy
			}
			if step%2 == 0 {
				current[0].H += 3
			}
			current[0], current[1] = current[1], current[0]
			current = parse(current, 0)
			for i := 0; i < 2; i++ {
				old := previous[1-i]
				if current[i].lightKey != old.lightKey {
					t.Fatalf("smoothing=%t step=%d: light %d lost its phase", smoothing, step, i)
				}
				end := flameLightFlicker(climg.PictDefFlagLightFlicker, uint32(old.PictID), pictureLightInstanceKey(old), state.logicalFrame-1, 1, 1)
				start := flameLightFlicker(climg.PictDefFlagLightFlicker, uint32(current[i].PictID), pictureLightInstanceKey(current[i]), state.logicalFrame, 0, 1)
				if end != start {
					t.Fatalf("flicker jumped at moving frame boundary: %+v -> %+v", end, start)
				}
				if smoothing && step%2 == 1 {
					x, y := pictureScreenPositionFloat(0, 0, current[i], 0, nil, nil, nil, dx, dy, 20, 20)
					if x != float64(int(old.H)+fieldCenterX) || y != float64(int(old.V)+fieldCenterY) {
						t.Fatalf("light did not start at previous sprite position: (%v,%v)", x, y)
					}
				}
			}
			previous = current
		}
		retained := parse(nil, len(previous))
		fresh := append([]framePicture(nil), retained...)
		for i := range fresh {
			fresh[i].H -= 10
		}
		fresh = parse(fresh, 0)
		for i := range fresh {
			if fresh[i].lightKey != retained[i].lightKey {
				t.Fatal("retained picture lost flicker state when refreshed")
			}
		}
		// An unrelated scene must not inherit old identities through stale buffers.
		fresh = parse([]framePicture{{PictID: 4, H: 300, V: 300}}, 0)
		if fresh[0].lightKey != pictureLightInstanceKey(framePicture{H: 300, V: 300}) {
			t.Fatal("new scene retained an unrelated light identity")
		}
	}
}

func TestPictureMatchingReservesRetainedPrefix(t *testing.T) {
	previous := []framePicture{{PictID: 1, H: 0}, {PictID: 1, H: 20}}
	current := []framePicture{{PictID: 1, H: 0}, {PictID: 1, H: 1}}
	scratch := matchPicturePositions(previous, current, 0, 0, maxInterpPixels, 1)
	defer releasePicturePositionScratch(scratch)
	if scratch.matches[1] != 1 {
		t.Fatal("fresh picture stole retained picture's identity")
	}
}
