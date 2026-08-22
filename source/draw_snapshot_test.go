package main

import "testing"

func TestCaptureDrawSnapshotReusesStorage(t *testing.T) {
	stateMu.Lock()
	origState := state
	state = drawState{
		descriptors: map[uint8]frameDescriptor{
			1: {Index: 1, Name: "Bob", PictID: 100, Colors: []byte{1, 2, 3}},
		},
		prevPictures: []framePicture{{PictID: 10, H: 2, V: 3}},
		mobiles: map[uint8]frameMobile{
			1: {Index: 1, H: 4, V: 5},
		},
		prevMobiles: map[uint8]frameMobile{
			1: {Index: 1, H: 3, V: 4},
		},
		prevDescs: map[uint8]frameDescriptor{
			1: {Index: 1, Name: "Bob", PictID: 100, Colors: []byte{1, 2, 3}},
		},
		bubbles:      []bubble{{Index: 1, Text: "hello", LifeFrames: 1000}},
		picsNeg:      []framePicture{{PictID: 11, Plane: -1}},
		picsZero:     []framePicture{{PictID: 12}},
		picsPos:      []framePicture{{PictID: 13, Plane: 1}},
		liveMobs:     []frameMobile{{Index: 1, H: 4, V: 5}},
		deadMobs:     []frameMobile{{Index: 2, State: poseDead}},
		nameMobs:     []frameMobile{{Index: 1, H: 4, V: 5}},
		logicalFrame: 77,
	}
	stateMu.Unlock()

	origMotionSmoothing := gs.MotionSmoothing
	origBlendMobiles := gs.BlendMobiles
	origObjectPinning := gs.ObjectPinning
	origFrameCounter := frameCounter
	gs.MotionSmoothing = true
	gs.BlendMobiles = true
	gs.ObjectPinning = true
	frameCounter = 1
	defer func() {
		stateMu.Lock()
		state = origState
		stateMu.Unlock()
		gs.MotionSmoothing = origMotionSmoothing
		gs.BlendMobiles = origBlendMobiles
		gs.ObjectPinning = origObjectPinning
		frameCounter = origFrameCounter
	}()

	var snap drawSnapshot
	captureDrawSnapshot(&snap) // warm reusable maps and slices
	allocs := testing.AllocsPerRun(1000, func() {
		captureDrawSnapshot(&snap)
	})
	if allocs != 0 {
		t.Fatalf("captureDrawSnapshot allocated %.1f times per call after warmup", allocs)
	}
	if len(snap.descriptors) != 1 || len(snap.prevPicturePositions) != 1 || len(snap.picsNeg) != 1 || len(snap.liveMobs) != 1 {
		t.Fatalf("snapshot was not populated: %#v", snap)
	}
	if snap.logicalFrame != 77 {
		t.Fatalf("snapshot logical frame = %d, want 77", snap.logicalFrame)
	}
}
