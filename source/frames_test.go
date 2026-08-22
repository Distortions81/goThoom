package main

import "testing"

func TestUpdateFrameCounters(t *testing.T) {
	lastAckFrame = 0
	numFrames = 0
	lostFrames = 0

	if dropped := updateFrameCounters(1); dropped != 0 {
		t.Fatalf("expected 0 dropped, got %d", dropped)
	}
	if numFrames != 1 || lostFrames != 0 || lastAckFrame != 1 {
		t.Fatalf("unexpected counters after first frame: num=%d lost=%d last=%d", numFrames, lostFrames, lastAckFrame)
	}

	if dropped := updateFrameCounters(3); dropped != 1 {
		t.Fatalf("expected 1 dropped, got %d", dropped)
	}
	if numFrames != 2 || lostFrames != 1 || lastAckFrame != 3 {
		t.Fatalf("unexpected counters after second frame: num=%d lost=%d last=%d", numFrames, lostFrames, lastAckFrame)
	}

	if dropped := updateFrameCounters(4); dropped != 0 {
		t.Fatalf("expected 0 dropped, got %d", dropped)
	}
	if numFrames != 3 || lostFrames != 1 || lastAckFrame != 4 {
		t.Fatalf("unexpected counters after third frame: num=%d lost=%d last=%d", numFrames, lostFrames, lastAckFrame)
	}
}
