//go:build integration
// +build integration

package main

import (
	"testing"
	"time"
)

func classicTestDuration(ticks int) time.Duration {
	return time.Duration((int64(ticks)*int64(time.Second) + 300) / 600)
}

func TestParseClanLordTuneDurations(t *testing.T) {
	tests := []struct {
		input        string
		wantDuration time.Duration
	}{
		{"c", 142 * time.Second / 600},
		{"C", 292 * time.Second / 600},
		{"c1", 67 * time.Second / 600},
	}
	for _, tt := range tests {
		notes := classicNotesFromTune(tt.input, instruments[0], 120, 100)
		if len(notes) != 1 {
			t.Fatalf("%q parsed to %d notes, want 1", tt.input, len(notes))
		}
		if notes[0].Duration != tt.wantDuration {
			t.Errorf("%q duration = %v, want %v", tt.input, notes[0].Duration, tt.wantDuration)
		}
	}
}

func TestEventsToNotesDefaultGap(t *testing.T) {
	inst := instruments[0]
	notes := classicNotesFromTune("cd", inst, 120, 100)
	if len(notes) != 2 {
		t.Fatalf("expected 2 notes, got %d", len(notes))
	}
	gap := classicTestDuration(8)
	wantDur := classicTestDuration(142)
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
	inst := instruments[0]
	notes := classicNotesFromTune("cpd", inst, 120, 100)
	if len(notes) != 2 {
		t.Fatalf("expected 2 notes, got %d", len(notes))
	}
	if notes[1].Start != 500*time.Millisecond {
		t.Fatalf("second note start = %v, want 500ms", notes[1].Start)
	}
	gotGap := notes[1].Start - notes[0].Start - notes[0].Duration
	wantGap := classicTestDuration(158)
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
	inst := instruments[0]
	inst.chord = 50
	notes := classicNotesFromTune("[ce]g", inst, 120, 100)
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
	inst := instruments[0]
	notes := classicNotesFromTune("(cd)2@+60e%5f", inst, 120, 100)
	if len(notes) != 6 {
		t.Fatalf("expected 6 notes, got %d", len(notes))
	}
	// After tempo change to 180 BPM, note 'e' should have shorter duration with gap applied.
	wantDur := classicTestDuration(95)
	if notes[4].Duration != wantDur {
		t.Fatalf("tempo change not applied, got %v want %v", notes[4].Duration, wantDur)
	}
	// Classic volume steps are linear, so volume 5 is half velocity.
	if notes[5].Velocity != 50 {
		t.Fatalf("volume change not applied, got %d", notes[5].Velocity)
	}
}

// TestEventsToNotesLoop verifies that repeated loops terminate properly and
// produce the expected number of notes without getting stuck.
func TestEventsToNotesLoop(t *testing.T) {
	inst := instruments[0]
	notes := classicNotesFromTune("(c)2", inst, 120, 100)
	if len(notes) != 2 {
		t.Fatalf("expected 2 notes, got %d", len(notes))
	}
}

// TestLoopSeamlessRepeat ensures that looping a sequence starting with a note
// does not introduce an extra rest between iterations when no explicit rest is
// present at the loop boundary.
func TestLoopSeamlessRepeat(t *testing.T) {
	inst := instruments[0]
	notes := classicNotesFromTune("(c)2", inst, 120, 100)
	if len(notes) != 2 {
		t.Fatalf("expected 2 notes, got %d", len(notes))
	}
	gap := notes[1].Start - notes[0].Start - notes[0].Duration
	wantGap := classicTestDuration(8)
	if gap != wantGap {
		t.Fatalf("gap between loop iterations = %v, want %v", gap, wantGap)
	}
}

// TestAltEndingsSimple verifies that alternate endings within a loop select the
// appropriate tail segment per iteration and honor the default ending when a
// specific index is not present.
func TestAltEndingsSimple(t *testing.T) {
	// (c|1d|2e!f)3 => iterations produce: c d | c e | c f
	inst := instruments[0]
	notes := classicNotesFromTune("(c|1d|2e!f)3", inst, 120, 100)
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
	// Start the melody before the chord because classic chord scheduling is
	// gated until a melody timeline exists.
	inst := instruments[0]
	inst.longChord = true
	notes := classicNotesFromTune("a [ce]$ aa [df]", inst, 120, 100)
	// The final chord marks the boundary but starts at the end of the melody,
	// so classic gating drops its zero-duration notes.
	if len(notes) != 5 {
		t.Fatalf("expected 5 notes, got %d", len(notes))
	}
	if notes[1].Start != 250*time.Millisecond || notes[2].Start != 250*time.Millisecond {
		t.Fatalf("long chord should start at 250ms")
	}
	if notes[3].Start != 250*time.Millisecond || notes[4].Start != 500*time.Millisecond {
		t.Fatalf("melody note starts wrong: got %v and %v", notes[3].Start, notes[4].Start)
	}
	if notes[1].Duration != 500*time.Millisecond || notes[2].Duration != 500*time.Millisecond {
		t.Fatalf("long chord duration = %v,%v want 500ms,500ms", notes[1].Duration, notes[2].Duration)
	}
}

func TestNoteDurationsWithTempoChange(t *testing.T) {
	tune := "c d1 @+60 E g2"
	inst := instruments[0]
	notes := classicNotesFromTune(tune, inst, 120, 100)
	if len(notes) != 4 {
		t.Fatalf("expected 4 notes, got %d", len(notes))
	}
	want := []time.Duration{
		classicTestDuration(142), // c at 120 BPM
		classicTestDuration(67),  // d1 at 120 BPM
		classicTestDuration(195), // E at 180 BPM
		classicTestDuration(95),  // g2 at 180 BPM
	}
	for i, n := range notes {
		if n.Duration != want[i] {
			t.Errorf("note %d duration = %v, want %v", i, n.Duration, want[i])
		}
	}
}

func TestNoteDurationsUncommonTempos(t *testing.T) {
	inst := instruments[0]
	cases := []struct {
		tempo int
		want  time.Duration
	}{
		{95, 457 * time.Millisecond},
		{177, 246 * time.Millisecond},
	}
	for _, c := range cases {
		notes := classicNotesFromTune("c3", inst, c.tempo, 100)
		if len(notes) != 1 {
			t.Fatalf("tempo %d: expected 1 note, got %d", c.tempo, len(notes))
		}
		if notes[0].Duration != c.want {
			t.Errorf("tempo %d: duration = %v, want %v", c.tempo, notes[0].Duration, c.want)
		}
	}
}

func TestParseNoteLowestClassicOctavePreserved(t *testing.T) {
	notes := classicNotesFromTune("\\----c", instruments[0], 120, 100)
	if len(notes) != 1 {
		t.Fatalf("expected 1 note, got %d", len(notes))
	}
	if notes[0].Key != 60 {
		t.Fatalf("expected central C (60), got %d", notes[0].Key)
	}
}

// TestTieMergeIdenticalNotes ensures that tied identical notes become a single
// sustained note without re-attack.
func TestTieMergeIdenticalNotes(t *testing.T) {
	inst := instruments[0]
	notes := classicNotesFromTune("c_c", inst, 120, 100)
	if len(notes) != 1 {
		t.Fatalf("expected 1 merged note, got %d", len(notes))
	}
	// Two lowercase c notes at 120 BPM: each 250ms base. Tied merge => 500ms total.
	if notes[0].Duration != 500*time.Millisecond {
		t.Fatalf("merged tie duration = %v; want 500ms", notes[0].Duration)
	}
}
