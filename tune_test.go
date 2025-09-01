//go:build integration
// +build integration

package main

import (
	"math"
	"testing"
	"time"
)

func TestParseClanLordTuneDurations(t *testing.T) {
	tests := []struct {
		input string
		want  []int
	}{
		{"c", []int{250}},     // lowercase uses durationBlack=2 half-beats
		{"C", []int{500}},     // uppercase uses durationWhite=4 half-beats
		{"c1", []int{125}},    // explicit duration 1 half-beat
		{"p", []int{250}},     // rest defaults to durationBlack
		{"[ce]", []int{250}},  // chord defaults to defaultChordDuration
		{"[ce]3", []int{375}}, // chord with explicit duration
	}
	for _, tt := range tests {
		pt := parseClanLordTuneWithTempo(tt.input, 120)
		if len(pt.events) != len(tt.want) {
			t.Fatalf("%q parsed to %d events, want %d", tt.input, len(pt.events), len(tt.want))
		}
		quarter := 60000 / 120
		for i, ev := range pt.events {
			got := int((ev.beats / 4) * float64(quarter))
			if got != tt.want[i] {
				t.Errorf("%q event %d duration = %d, want %d", tt.input, i, got, tt.want[i])
			}
		}
	}
}

func TestEventsToNotesDefaultGap(t *testing.T) {
	pt := parseClanLordTune("cd")
	inst := instrument{program: 0, octave: 0, chord: 100, melody: 100}
	notes := eventsToNotes(pt, inst, 100)
	if len(notes) != 2 {
		t.Fatalf("expected 2 notes, got %d", len(notes))
	}
	gap := time.Duration(int(math.Round(1500.0/120.0))) * time.Millisecond
	wantDur := 250*time.Millisecond - gap
	if notes[0].Duration != wantDur {
		t.Fatalf("first note duration = %v, want %v", notes[0].Duration, wantDur)
	}
	if notes[1].Start != 250*time.Millisecond {
		t.Fatalf("second note start = %v, want 250ms", notes[1].Start)
	}
	gotGap := notes[1].Start - notes[0].Start - notes[0].Duration
	if gotGap != gap {
		t.Fatalf("gap = %v, want %v", gotGap, gap)
	}
}

func TestRestDuration(t *testing.T) {
	pt := parseClanLordTune("cpd")
	inst := instrument{program: 0, octave: 0, chord: 100, melody: 100}
	notes := eventsToNotes(pt, inst, 100)
	if len(notes) != 2 {
		t.Fatalf("expected 2 notes, got %d", len(notes))
	}
	if notes[1].Start != 500*time.Millisecond {
		t.Fatalf("second note start = %v, want 500ms", notes[1].Start)
	}
	gapUnit := time.Duration(int(math.Round(1500.0/120.0))) * time.Millisecond
	gotGap := notes[1].Start - notes[0].Start - notes[0].Duration
	wantGap := gapUnit + 250*time.Millisecond
	if gotGap != wantGap {
		t.Fatalf("gap = %v, want %v", gotGap, wantGap)
	}
}

func TestInstrumentVelocityFactors(t *testing.T) {
	inst0 := instruments[0]
	if inst0.chord != 100 || inst0.melody != 100 {
		t.Fatalf("instrument 0 velocities = %d,%d; want 100,100", inst0.chord, inst0.melody)
	}
	inst1 := instruments[1]
	if inst1.chord != 100 || inst1.melody != 100 {
		t.Fatalf("instrument 1 velocities = %d,%d; want 100,100", inst1.chord, inst1.melody)
	}
}

func TestEventsToNotesVelocityFactors(t *testing.T) {
	inst := instrument{program: 0, octave: 0, chord: 50, melody: 100}
	pt := parsedTune{
		events: []noteEvent{
			{keys: []int{60, 64}, beats: 2, volume: 10},
			{keys: []int{67}, beats: 2, volume: 10},
		},
		tempo: 120,
	}
	notes := eventsToNotes(pt, inst, 100)
	if len(notes) != 3 {
		t.Fatalf("expected 3 notes, got %d", len(notes))
	}
	if notes[0].Velocity != 50 || notes[1].Velocity != 50 {
		t.Fatalf("chord note velocities = %d,%d; want 50", notes[0].Velocity, notes[1].Velocity)
	}
	if notes[2].Velocity != 100 {
		t.Fatalf("melody note velocity = %d; want 100", notes[2].Velocity)
	}
}

func TestLoopAndTempoAndVolume(t *testing.T) {
	// Loop: (cd)2 should produce 4 notes, then tempo change and volume change.
	pt := parseClanLordTuneWithTempo("(cd)2@+60e%5f", 120)
	inst := instrument{program: 0, octave: 0, chord: 100, melody: 100}
	notes := eventsToNotes(pt, inst, 100)
	if len(notes) != 6 {
		t.Fatalf("expected 6 notes, got %d", len(notes))
	}
	// After tempo change to 180 BPM, note 'e' should have shorter duration with gap applied.
	gap := time.Duration(int(math.Round(1500.0/180.0))) * time.Millisecond
	wantDur := 166*time.Millisecond - gap
	if notes[4].Duration != wantDur {
		t.Fatalf("tempo change not applied, got %v want %v", notes[4].Duration, wantDur)
	}
	// volume change should reduce velocity per square-root scaling (volume set to 5)
	if notes[5].Velocity != 71 {
		t.Fatalf("volume change not applied, got %d", notes[5].Velocity)
	}
}

// TestEventsToNotesLoop verifies that repeated loops terminate properly and
// produce the expected number of notes without getting stuck.
func TestEventsToNotesLoop(t *testing.T) {
	pt := parseClanLordTune("(c)2")
	inst := instrument{chord: 100, melody: 100}
	notes := eventsToNotes(pt, inst, 100)
	if len(notes) != 2 {
		t.Fatalf("expected 2 notes, got %d", len(notes))
	}
}

// TestLoopSeamlessRepeat ensures that looping a sequence starting with a note
// does not introduce an extra rest between iterations when no explicit rest is
// present at the loop boundary.
func TestLoopSeamlessRepeat(t *testing.T) {
	pt := parseClanLordTune("(c)2")
	inst := instrument{program: 0, octave: 0, chord: 100, melody: 100}
	notes := eventsToNotes(pt, inst, 100)
	if len(notes) != 2 {
		t.Fatalf("expected 2 notes, got %d", len(notes))
	}
	gap := notes[1].Start - notes[0].Start - notes[0].Duration
	wantGap := time.Duration(int(math.Round(1500.0/120.0))) * time.Millisecond
	if gap != wantGap {
		t.Fatalf("gap between loop iterations = %v, want %v", gap, wantGap)
	}
}

// TestAltEndingsSimple verifies that alternate endings within a loop select the
// appropriate tail segment per iteration and honor the default ending when a
// specific index is not present.
func TestAltEndingsSimple(t *testing.T) {
	// (c|1d|2e!f)3 => iterations produce: c d | c e | c f
	pt := parseClanLordTune("(c|1d|2e!f)3")
	inst := instrument{program: 0, octave: 0, chord: 100, melody: 100}
	notes := eventsToNotes(pt, inst, 100)
	if len(notes) != 6 {
		t.Fatalf("expected 6 notes, got %d", len(notes))
	}
	// Extract pitch classes modulo 12 to ignore octave
	got := []int{notes[0].Key % 12, notes[1].Key % 12, notes[2].Key % 12, notes[3].Key % 12, notes[4].Key % 12, notes[5].Key % 12}
	// c d c e c f => 0,2,0,4,0,5
	want := []int{0, 2, 0, 4, 0, 5}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("ending seq note %d = %d, want %d", i, got[i], want[i])
		}
	}
}

// TestLongChordSustain ensures that a long chord marked with '$' sustains until
// the next chord without consuming time at its own event.
func TestLongChordSustain(t *testing.T) {
	// [ce]$ a a [df]
	pt := parseClanLordTune("[ce]$ aa [df]")
	inst := instrument{program: 0, octave: 0, chord: 100, melody: 100, longChord: true}
	notes := eventsToNotes(pt, inst, 100)
	// Expect 2 chord notes + 2 melody a + 2 chord notes = 6
	if len(notes) != 6 {
		t.Fatalf("expected 6 notes, got %d", len(notes))
	}
	// First two are chord c and e, starting at 0, duration spans over the two 'a' notes
	// At 120 BPM, each lowercase note is 250ms minus 125ms gap = 125ms. But long chord
	// ignores gap and sustains: total until next chord start is 2*250ms = 500ms.
	if notes[0].Start != 0 || notes[1].Start != 0 {
		t.Fatalf("long chord should start at 0")
	}
	if notes[2].Start != 0 || notes[3].Start != 250*time.Millisecond {
		t.Fatalf("melody note starts wrong: got %v and %v", notes[2].Start, notes[3].Start)
	}
	if notes[0].Duration != 500*time.Millisecond || notes[1].Duration != 500*time.Millisecond {
		t.Fatalf("long chord duration = %v,%v want 500ms,500ms", notes[0].Duration, notes[1].Duration)
	}
}

func TestNoteDurationsWithTempoChange(t *testing.T) {
	tune := "c d1 @+60 E g2"
	pt := parseClanLordTuneWithTempo(tune, 120)
	inst := instrument{program: 0, octave: 0, chord: 100, melody: 100}
	notes := eventsToNotes(pt, inst, 100)
	if len(notes) != 4 {
		t.Fatalf("expected 4 notes, got %d", len(notes))
	}
	gap120 := time.Duration(int(math.Round(1500.0/120.0))) * time.Millisecond
	gap180 := time.Duration(int(math.Round(1500.0/180.0))) * time.Millisecond
	want := []time.Duration{
		250*time.Millisecond - gap120, // c: 1 beat at 120 BPM
		125*time.Millisecond - gap120, // d1: half beat at 120 BPM
		333*time.Millisecond - gap180, // E: 2 beats at 180 BPM
		166*time.Millisecond - gap180, // g2: 1 beat at 180 BPM
	}
	for i, n := range notes {
		if n.Duration != want[i] {
			t.Errorf("note %d duration = %v, want %v", i, n.Duration, want[i])
		}
	}
}

func TestNoteDurationsUncommonTempos(t *testing.T) {
	inst := instrument{program: 0, octave: 0, chord: 100, melody: 100}
	cases := []struct {
		tempo int
		want  time.Duration
	}{
		{95, 457 * time.Millisecond},
		{177, 246 * time.Millisecond},
	}
	for _, c := range cases {
		pt := parseClanLordTuneWithTempo("c3", c.tempo)
		notes := eventsToNotes(pt, inst, 100)
		if len(notes) != 1 {
			t.Fatalf("tempo %d: expected 1 note, got %d", c.tempo, len(notes))
		}
		if notes[0].Duration != c.want {
			t.Errorf("tempo %d: duration = %v, want %v", c.tempo, notes[0].Duration, c.want)
		}
	}
}

func TestParseNoteLowestCPreserved(t *testing.T) {
	pt := parseClanLordTune("\\----c")
	if len(pt.events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(pt.events))
	}
	if len(pt.events[0].keys) != 1 || pt.events[0].keys[0] != 0 {
		t.Fatalf("expected low C (0), got %+v", pt.events[0].keys)
	}
}

// TestTieMergeIdenticalNotes ensures that tied identical notes become a single
// sustained note without re-attack.
func TestTieMergeIdenticalNotes(t *testing.T) {
	pt := parseClanLordTune("c_c")
	inst := instrument{program: 0, octave: 0, chord: 100, melody: 100}
	notes := eventsToNotes(pt, inst, 100)
	if len(notes) != 1 {
		t.Fatalf("expected 1 merged note, got %d", len(notes))
	}
	// Two lowercase c notes at 120 BPM: each 250ms base. Tied merge => 500ms total.
	if notes[0].Duration != 500*time.Millisecond {
		t.Fatalf("merged tie duration = %v; want 500ms", notes[0].Duration)
	}
}
