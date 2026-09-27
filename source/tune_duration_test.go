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
	// Two sixteenths at 120 BPM: 150 ticks, with an 8-tick gap.
	ns := classicNotesFromTune("c", instruments[0], 120, 100)
	if len(ns) != 1 {
		t.Fatalf("expected 1 note, got %d", len(ns))
	}
	if ns[0].Duration != classicTestDuration(142) {
		t.Fatalf("lowercase note duration = %v, want 142 ticks", ns[0].Duration)
	}
}

func TestNoteDurations_DefaultUppercase(t *testing.T) {
	// Four sixteenths at 120 BPM: 300 ticks, with an 8-tick gap.
	ns := classicNotesFromTune("C", instruments[0], 120, 100)
	if len(ns) != 1 {
		t.Fatalf("expected 1 note, got %d", len(ns))
	}
	if ns[0].Duration != classicTestDuration(292) {
		t.Fatalf("uppercase note duration = %v, want 292 ticks", ns[0].Duration)
	}
}

func TestRestAdvancesTimeline(t *testing.T) {
	// Sequence c p c at 120 BPM:
	// Each event durMS=250, so second note starts at 500ms
	ns := classicNotesFromTune("c p c", instruments[0], 120, 100)
	if len(ns) != 2 {
		t.Fatalf("expected 2 notes, got %d", len(ns))
	}
	if ns[1].Start != 500*time.Millisecond {
		t.Fatalf("second note start = %v, want 500ms", ns[1].Start)
	}
}

func TestLinkRemovesFirstGap(t *testing.T) {
	notes := classicNotesFromTune("c_d", instruments[0], 120, 100)
	if len(notes) != 2 || notes[0].Start+notes[0].Duration != notes[1].Start {
		t.Fatalf("linked notes have a gap: %+v", notes)
	}
}

func TestTempoAffectsDuration(t *testing.T) {
	// At 60 BPM, lowercase default: durMS=(2/4)*1000=500, gap=25 => 475ms
	ns := classicNotesFromTune("c", instruments[0], 60, 100)
	if len(ns) != 1 {
		t.Fatalf("expected 1 note, got %d", len(ns))
	}
	if ns[0].Duration != 475*time.Millisecond {
		t.Fatalf("lowercase @60BPM duration = %v, want 475ms", ns[0].Duration)
	}
}

func TestTotalSongEndTime(t *testing.T) {
	// c p C at 120 BPM: timeline advances 250 + 250 + 500 = 1s total
	ns := classicNotesFromTune("c p C", instruments[0], 120, 100)
	end := noteEndTime(ns)
	want := classicTestDuration(592)
	if end != want {
		t.Fatalf("song end = %v, want %v", end, want)
	}
}
