package main

import (
	"slices"
	"testing"
)

func TestCaptureDrawSnapshotReusesCrowdedBubbleStorage(t *testing.T) {
	s := mustNewSession(2)
	for i := range 32 {
		s.draw.current.bubbles = append(s.draw.current.bubbles, bubble{
			Index: uint8(i), DedupeID: uint16(300 + i), Far: true,
			Text: "hello", LifeFrames: 1000,
		})
	}
	var snap drawSnapshot
	captureSessionDrawSnapshot(s, &snap)
	if len(snap.bubbles) != 32 {
		t.Fatalf("captured %d bubbles, want 32", len(snap.bubbles))
	}
	if allocs := testing.AllocsPerRun(100, func() {
		captureSessionDrawSnapshot(s, &snap)
	}); allocs != 0 {
		t.Fatalf("crowded snapshot allocated %.1f times after warmup", allocs)
	}
}

func TestCaptureDrawSnapshotBubbleDeduplicationAndExpiry(t *testing.T) {
	s := mustNewSession(2)
	s.draw.frame = 100
	s.draw.current.bubbles = []bubble{
		{Index: 1, Far: true, Text: "old", CreatedFrame: 99, LifeFrames: 50},
		{Index: 2, Far: true, Text: "other", CreatedFrame: 99, LifeFrames: 50},
		{Index: 7, DedupeID: 600, Far: true, Text: "custom old", CreatedFrame: 99, LifeFrames: 50},
		{Index: 1, Far: true, Text: "latest", CreatedFrame: 99, LifeFrames: 50},
		{Index: 3, Far: true, Text: "expired", LifeFrames: 50},
		{Index: 8, DedupeID: 600, Far: true, Text: "custom latest", CreatedFrame: 99, LifeFrames: 50},
	}
	var snap drawSnapshot
	captureSessionDrawSnapshot(s, &snap)
	texts := func() []string {
		result := make([]string, 0, len(snap.bubbles))
		for _, b := range snap.bubbles {
			result = append(result, b.Text)
		}
		return result
	}
	if got := texts(); !slices.Equal(got, []string{"other", "latest", "custom latest"}) {
		t.Fatalf("deduplicated bubble order = %v", got)
	}
	// A later snapshot must use the new bubble positions, even for reused IDs.
	s.draw.current.bubbles = []bubble{
		{Index: 1, Far: true, Text: "replacement", CreatedFrame: 99, LifeFrames: 50},
		{Index: 2, Far: true, Text: "replacement other", CreatedFrame: 99, LifeFrames: 50},
	}
	captureSessionDrawSnapshot(s, &snap)
	if got := texts(); !slices.Equal(got, []string{"replacement", "replacement other"}) {
		t.Fatalf("refreshed bubbles = %v", got)
	}
	s.draw.frame = 149
	captureSessionDrawSnapshot(s, &snap)
	if len(snap.bubbles) != 0 {
		t.Fatalf("expired bubbles retained: %+v", snap.bubbles)
	}
}

func TestCaptureDrawSnapshotReusesStorage(t *testing.T) {
	primarySession.draw.mu.Lock()
	origState := primarySession.draw.current
	primarySession.draw.current = drawState{
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
	primarySession.draw.mu.Unlock()

	origMotionSmoothing := gs.MotionSmoothing
	origBlendMobiles := gs.BlendMobiles
	origObjectPinning := gs.ObjectPinning
	origFrameCounter := primarySession.draw.frame
	gs.MotionSmoothing = true
	gs.BlendMobiles = true
	gs.ObjectPinning = true
	primarySession.draw.frame = 1
	defer func() {
		primarySession.draw.mu.Lock()
		primarySession.draw.current = origState
		primarySession.draw.mu.Unlock()
		gs.MotionSmoothing = origMotionSmoothing
		gs.BlendMobiles = origBlendMobiles
		gs.ObjectPinning = origObjectPinning
		primarySession.draw.frame = origFrameCounter
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
	if captureDrawSnapshotIfChanged(&snap) {
		t.Fatal("unchanged snapshot was copied again")
	}
	markWorldStateChanged()
	if !captureDrawSnapshotIfChanged(&snap) {
		t.Fatal("changed world generation did not refresh snapshot")
	}
}

func TestCaptureDrawSnapshotRefreshesWhenSessionChanges(t *testing.T) {
	secondary := mustNewSession(2)
	originalGeneration := primarySession.draw.generation.Load()
	t.Cleanup(func() { primarySession.draw.generation.Store(originalGeneration) })
	primarySession.draw.generation.Store(7)
	secondary.draw.generation.Store(7)

	var snap drawSnapshot
	captureSessionDrawSnapshot(primarySession, &snap)
	if snap.source != primarySessionID {
		t.Fatalf("primary snapshot source = %d", snap.source)
	}
	if !captureSessionDrawSnapshotIfChanged(secondary, &snap) {
		t.Fatal("same world generation in another session reused the previous snapshot")
	}
	if snap.source != secondary.ID() {
		t.Fatalf("secondary snapshot source = %d", snap.source)
	}
}
