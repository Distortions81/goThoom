//go:build integration
// +build integration

package main

import (
	"testing"
	"time"
)

// helper to collect durations and timeline from a notes slice
func noteEndTime(ns []Note) time.Duration {
	var end time.Duration
	for _, n := range ns {
		if e := n.Start + n.Duration; e > end {
			end = e
		}
	}
	return end
}

func TestNoteDurations_DefaultLowercase(t *testing.T) {
	// At 120 BPM, lowercase default (beats=2) -> durMS=250, gap=13 => 237ms
	pt := parseClanLordTuneWithTempo("c", 120)
	ns := eventsToNotes(pt, instruments[0], 100)
	if len(ns) != 1 {
		t.Fatalf("expected 1 note, got %d", len(ns))
	}
	if ns[0].Duration != 237*time.Millisecond {
		t.Fatalf("lowercase note duration = %v, want 237ms", ns[0].Duration)
	}
}

func TestNoteDurations_DefaultUppercase(t *testing.T) {
	// Uppercase default beats=4 -> durMS=500, gap=13 => 487ms
	pt := parseClanLordTuneWithTempo("C", 120)
	ns := eventsToNotes(pt, instruments[0], 100)
	if len(ns) != 1 {
		t.Fatalf("expected 1 note, got %d", len(ns))
	}
	if ns[0].Duration != 487*time.Millisecond {
		t.Fatalf("uppercase note duration = %v, want 487ms", ns[0].Duration)
	}
}

func TestRestAdvancesTimeline(t *testing.T) {
	// Sequence c p c at 120 BPM:
	// Each event durMS=250, so second note starts at 500ms
	pt := parseClanLordTuneWithTempo("c p c", 120)
	ns := eventsToNotes(pt, instruments[0], 100)
	if len(ns) != 2 {
		t.Fatalf("expected 2 notes, got %d", len(ns))
	}
	if ns[1].Start != 500*time.Millisecond {
		t.Fatalf("second note start = %v, want 500ms", ns[1].Start)
	}
}

func TestTieMergesDuration(t *testing.T) {
	// c_c at 120 BPM should merge to a single note of 500ms total
	pt := parseClanLordTuneWithTempo("c_c", 120)
	ns := eventsToNotes(pt, instruments[0], 100)
	if len(ns) != 1 {
		t.Fatalf("expected 1 merged note, got %d", len(ns))
	}
	// Two events of 250ms each, tie removes gap and extends by full 250ms
	if ns[0].Duration != 500*time.Millisecond {
		t.Fatalf("tied note duration = %v, want 500ms", ns[0].Duration)
	}
}

func TestTempoAffectsDuration(t *testing.T) {
	// At 60 BPM, lowercase default: durMS=(2/4)*1000=500, gap=25 => 475ms
	pt := parseClanLordTuneWithTempo("c", 60)
	ns := eventsToNotes(pt, instruments[0], 100)
	if len(ns) != 1 {
		t.Fatalf("expected 1 note, got %d", len(ns))
	}
	if ns[0].Duration != 475*time.Millisecond {
		t.Fatalf("lowercase @60BPM duration = %v, want 475ms", ns[0].Duration)
	}
}

func TestTotalSongEndTime(t *testing.T) {
	// c p C at 120 BPM: timeline advances 250 + 250 + 500 = 1s total
	pt := parseClanLordTuneWithTempo("c p C", 120)
	ns := eventsToNotes(pt, instruments[0], 100)
	end := noteEndTime(ns)
	if end != 1000*time.Millisecond {
		t.Fatalf("song end = %v, want 1000ms", end)
	}
}
