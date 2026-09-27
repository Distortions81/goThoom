package main

import (
	"testing"
	"time"
)

func TestInlineTempoIncrease(t *testing.T) {
	// Start at 120 BPM, then +60 -> 180 BPM
	ns := classicNotesFromTune("c @+60 c", instruments[0], 120, 100)
	if len(ns) != 2 {
		t.Fatalf("expected 2 notes, got %d", len(ns))
	}
	// First note at 120 BPM: 142 ticks.
	if ns[0].Duration != classicTestDuration(142) {
		t.Fatalf("first note dur=%v want 142 ticks", ns[0].Duration)
	}
	// Second note at 180 BPM: 100 ticks, with a 5-tick gap.
	if ns[1].Duration != classicTestDuration(95) {
		t.Fatalf("second note dur=%v want 95 ticks", ns[1].Duration)
	}
	// Start times: second note begins after first event's durMS=250ms
	if ns[1].Start != 250*time.Millisecond {
		t.Fatalf("second note start=%v want 250ms", ns[1].Start)
	}
}

func TestInlineTempoAbsolute(t *testing.T) {
	// Set absolute tempo to 60 BPM mid-song
	ns := classicNotesFromTune("c @60 c", instruments[0], 120, 100)
	if len(ns) != 2 {
		t.Fatalf("expected 2 notes, got %d", len(ns))
	}
	// First note at 120 BPM: 142 ticks.
	if ns[0].Duration != classicTestDuration(142) {
		t.Fatalf("first note dur=%v want 142 ticks", ns[0].Duration)
	}
	// Second note at 60 BPM: durMS=500ms, gap=25ms => 475ms
	if ns[1].Duration != 475*time.Millisecond {
		t.Fatalf("second note dur=%v want 475ms", ns[1].Duration)
	}
}

func TestInlineTempoResetDefault(t *testing.T) {
	// @ with no value resets to 120 BPM per parser logic
	ns := classicNotesFromTune("@60 c @ c", instruments[0], 200, 100)
	if len(ns) != 2 {
		t.Fatalf("expected 2 notes, got %d", len(ns))
	}
	// First note at 60 BPM: 475ms
	if ns[0].Duration != 475*time.Millisecond {
		t.Fatalf("first note dur=%v want 475ms", ns[0].Duration)
	}
	// Second note after reset to 120 BPM: 142 ticks.
	if ns[1].Duration != classicTestDuration(142) {
		t.Fatalf("second note dur=%v want 142 ticks", ns[1].Duration)
	}
}
