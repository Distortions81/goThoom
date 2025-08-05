package main

import "testing"

func resetStateForTest() {
	stateMu.Lock()
	state = drawState{
		descriptors: make(map[uint8]frameDescriptor),
		mobiles:     make(map[uint8]frameMobile),
		prevMobiles: make(map[uint8]frameMobile),
		prevDescs:   make(map[uint8]frameDescriptor),
	}
	stateMu.Unlock()
	interp = false
	onion = false
	fastAnimation = true
}

func TestPictAgainStationaryPicture(t *testing.T) {
	resetStateForTest()
	pixelCountMu.Lock()
	pixelCountCache[1] = 1
	pixelCountMu.Unlock()

	frame1 := []byte{
		0,          // ackCmd
		0, 0, 0, 0, // ackFrame
		0, 0, 0, 0, // resendFrame
		0,                   // descriptor count
		0, 0, 0, 0, 0, 0, 0, // stats
		1,             // picture count
		0, 4, 0, 0, 0, // picture bits for id=1,h=0,v=0
		0,    // mobile count
		0, 4, // state length
		0, 0, 0, 0, // state data
	}
	if err := parseDrawState(frame1); err != nil {
		t.Fatalf("frame1 parse failed: %v", err)
	}

	frame2 := []byte{
		0,          // ackCmd
		0, 0, 0, 0, // ackFrame
		0, 0, 0, 0, // resendFrame
		0,                   // descriptor count
		0, 0, 0, 0, 0, 0, 0, // stats
		255, 1, 0, // pictCount=255, pictAgain=1, pictCount=0
		0,    // mobile count
		0, 4, // state length
		0, 0, 0, 0, // state data
	}
	if err := parseDrawState(frame2); err != nil {
		t.Fatalf("frame2 parse failed: %v", err)
	}

	if len(state.pictures) != 1 {
		t.Fatalf("expected 1 picture, got %d", len(state.pictures))
	}
	if state.pictures[0].Moving {
		t.Fatalf("expected Moving=false for reused picture")
	}
}
