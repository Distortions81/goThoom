package main

import "testing"

func TestUpdateFrameCounters(t *testing.T) {
	primarySession.frames.lastAck = 0
	primarySession.frames.received = 0
	primarySession.frames.lost = 0

	if dropped := updateFrameCounters(1); dropped != 0 {
		t.Fatalf("expected 0 dropped, got %d", dropped)
	}
	if primarySession.frames.received != 1 || primarySession.frames.lost != 0 || primarySession.frames.lastAck != 1 {
		t.Fatalf("unexpected counters after first frame: num=%d lost=%d last=%d", primarySession.frames.received, primarySession.frames.lost, primarySession.frames.lastAck)
	}

	if dropped := updateFrameCounters(3); dropped != 1 {
		t.Fatalf("expected 1 dropped, got %d", dropped)
	}
	if primarySession.frames.received != 2 || primarySession.frames.lost != 1 || primarySession.frames.lastAck != 3 {
		t.Fatalf("unexpected counters after second frame: num=%d lost=%d last=%d", primarySession.frames.received, primarySession.frames.lost, primarySession.frames.lastAck)
	}

	if dropped := updateFrameCounters(4); dropped != 0 {
		t.Fatalf("expected 0 dropped, got %d", dropped)
	}
	if primarySession.frames.received != 3 || primarySession.frames.lost != 1 || primarySession.frames.lastAck != 4 {
		t.Fatalf("unexpected counters after third frame: num=%d lost=%d last=%d", primarySession.frames.received, primarySession.frames.lost, primarySession.frames.lastAck)
	}
}
