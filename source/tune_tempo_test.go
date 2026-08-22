//go:build integration
// +build integration

package main

import (
	"testing"
	"time"
)

func TestInlineTempoIncrease(t *testing.T) {
	// Start at 120 BPM, then +60 -> 180 BPM
	pt := parseClanLordTuneWithTempo("c @+60 c", 120)
	ns := eventsToNotes(pt, instruments[0], 100)
	if len(ns) != 2 {
		t.Fatalf("expected 2 notes, got %d", len(ns))
	}
	// First note at 120 BPM: 237ms
	if ns[0].Duration != 237*time.Millisecond {
		t.Fatalf("first note dur=%v want 237ms", ns[0].Duration)
	}
	// Second note at 180 BPM: durMS=(2/4)*(60000/180)=166ms, gap=round(1500/180)=8ms => 158ms
	if ns[1].Duration != 158*time.Millisecond {
		t.Fatalf("second note dur=%v want 158ms", ns[1].Duration)
	}
	// Start times: second note begins after first event's durMS=250ms
	if ns[1].Start != 250*time.Millisecond {
		t.Fatalf("second note start=%v want 250ms", ns[1].Start)
	}
}

func TestInlineTempoAbsolute(t *testing.T) {
	// Set absolute tempo to 60 BPM mid-song
	pt := parseClanLordTuneWithTempo("c @60 c", 120)
	ns := eventsToNotes(pt, instruments[0], 100)
	if len(ns) != 2 {
		t.Fatalf("expected 2 notes, got %d", len(ns))
	}
	// First note at 120 BPM: 237ms
	if ns[0].Duration != 237*time.Millisecond {
		t.Fatalf("first note dur=%v want 237ms", ns[0].Duration)
	}
	// Second note at 60 BPM: durMS=500ms, gap=25ms => 475ms
	if ns[1].Duration != 475*time.Millisecond {
		t.Fatalf("second note dur=%v want 475ms", ns[1].Duration)
	}
}

func TestInlineTempoResetDefault(t *testing.T) {
	// @ with no value resets to 120 BPM per parser logic
	pt := parseClanLordTuneWithTempo("@60 c @ c", 200)
	ns := eventsToNotes(pt, instruments[0], 100)
	if len(ns) != 2 {
		t.Fatalf("expected 2 notes, got %d", len(ns))
	}
	// First note at 60 BPM: 475ms
	if ns[0].Duration != 475*time.Millisecond {
		t.Fatalf("first note dur=%v want 475ms", ns[0].Duration)
	}
	// Second note after reset to default 120 BPM: 237ms
	if ns[1].Duration != 237*time.Millisecond {
		t.Fatalf("second note dur=%v want 237ms", ns[1].Duration)
	}
}
