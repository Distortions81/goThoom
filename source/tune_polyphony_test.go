package main

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestClassicInstrumentTable(t *testing.T) {
	want := []instrument{
		classicInstrument(47, 1, 100, 100, false, true, true, 6, 0),
		classicInstrument(73, 1, 100, 100, false, false, true, 0, 0),
		classicInstrument(47, 0, 100, 100, false, true, true, 10, 0),
		classicInstrument(106, 0, 100, 100, false, true, true, 6, 0),
		classicInstrument(13, 0, 100, 100, false, true, true, 6, 0),
		classicInstrument(25, 0, 100, 100, false, true, true, 6, 0),
		classicInstrument(76, 1, 100, 100, false, false, true, 0, 0),
		classicInstrument(17, -1, 100, 100, true, true, true, 10, 0),
		classicInstrument(94, -1, 100, 100, true, true, true, 1, 0),
		classicInstrument(80, 1, 100, 100, false, false, true, 0, 0),
		classicInstrument(77, 1, 100, 100, true, true, true, 6, 0),
		classicInstrument(12, 0, 100, 100, false, true, true, 6, 0),
		classicInstrument(59, -1, 100, 100, false, false, true, 0, 0),
		classicInstrument(110, 0, 100, 100, true, true, true, 3, 0),
		classicInstrument(117, -1, 100, 100, false, false, true, 0, orgaDrumNoteClasses),
		classicInstrument(115, 0, 100, 100, false, true, true, 4, 0),
		classicInstrument(41, 1, 100, 100, false, true, true, 2, 0),
		classicInstrument(78, 1, 100, 100, false, false, true, 0, 0),
		classicInstrument(22, -1, 100, 100, true, true, true, 6, 0),
		classicInstrument(108, -1, 100, 100, false, true, true, 3, 0),
		classicInstrument(44, -2, 100, 100, false, true, true, 2, 0),
		classicInstrument(33, -2, 100, 100, false, false, true, 0, 0),
		classicInstrument(77, 0, 100, 100, false, false, true, 1, 0),
	}

	if got := instruments[:classicInstrumentCount]; !reflect.DeepEqual(got, want) {
		t.Fatalf("classic instrument table =\n%+v\nwant:\n%+v", got, want)
	}
	if len(instruments) != classicInstrumentCount {
		t.Fatalf("instrument table has %d entries, want only %d classic instruments", len(instruments), classicInstrumentCount)
	}
}

func TestClassicInstrumentChordPolyphony(t *testing.T) {
	tests := []struct {
		instrument int
		want       int
	}{
		{2, 10},
		{7, 10},
		{8, 1},
		{13, 3},
		{15, 4},
		{16, 2},
		{19, 3},
		{20, 2},
	}
	chordNotes := []string{"c", "c#", "d", "d#", "e", "f", "f#", "g", "g#", "a", "a#"}

	for _, test := range tests {
		inst := instruments[test.instrument]
		if inst.polyphony != test.want {
			t.Errorf("instrument %d polyphony = %d, want %d", test.instrument, inst.polyphony, test.want)
			continue
		}

		for _, count := range []int{test.want, test.want + 1} {
			tune := "c [" + strings.Join(chordNotes[:count], "") + "] c"
			notes, parseErr := parseClassicTune(tune, inst, 120, 100)
			if count == test.want {
				if parseErr != nil {
					t.Errorf("instrument %d rejected boundary chord: %v", test.instrument, parseErr)
				} else if got := len(notes) - 2; got != test.want {
					t.Errorf("instrument %d with %d chord notes rendered %d, want %d", test.instrument, count, got, test.want)
				}
				continue
			}
			if parseErr == nil || parseErr.Code != tuneErrorInvalidChord {
				t.Errorf("instrument %d accepted %d chord notes: error = %v, want %s", test.instrument, count, parseErr, tuneErrorInvalidChord)
			}
			if len(notes) != 1 {
				t.Errorf("instrument %d overflow rendered %d partial notes, want preceding melody only", test.instrument, len(notes))
			}
		}
	}
}

func TestClassicTuneChordDiagnostics(t *testing.T) {
	tests := []struct {
		name       string
		instrument int
		tune       string
		code       tuneParseErrorCode
		wantNotes  int
	}{
		{"melody-only chord", 1, "c [e] d", tuneErrorInvalidChord, 1},
		{"unsupported long chord", 0, "c [e]$ d", tuneErrorUnsupportedInstrument, 1},
		{"occupied chord voices", 8, "c [c][d] c", tuneErrorPolyphonyOverflow, 2},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			notes, parseErr := parseClassicTune(test.tune, instruments[test.instrument], 120, 100)
			if parseErr == nil || parseErr.Code != test.code {
				t.Fatalf("parse error = %v, want %s", parseErr, test.code)
			}
			if parseErr.Position < 0 || parseErr.Position >= len(test.tune) {
				t.Errorf("parse error position = %d, tune length %d", parseErr.Position, len(test.tune))
			}
			if len(notes) != test.wantNotes {
				t.Errorf("partial notes = %d, want %d", len(notes), test.wantNotes)
			}
		})
	}
}

func TestOrgaDrumRestrictionIsInstrumentMetadata(t *testing.T) {
	inst := instruments[14]
	inst.program = 0
	for _, test := range []struct {
		key  int
		want bool
	}{
		{55, true},
		{59, true},
		{57, false},
	} {
		if got := allowedNoteForInst(inst, test.key); got != test.want {
			t.Errorf("allowedNoteForInst(Orga Drum, %d) = %v, want %v", test.key, got, test.want)
		}
	}
}

func TestChordOctaveChangePersistsIntoMelody(t *testing.T) {
	notes, parseErr := parseClassicTune("\\c [=e] c", instruments[0], 120, 100)
	if parseErr != nil {
		t.Fatal(parseErr)
	}
	if len(notes) != 3 {
		t.Fatalf("notes = %d, want 3", len(notes))
	}
	if got := notes[2].Key - notes[0].Key; got != 12 {
		t.Fatalf("melody octave change after chord = %d semitones, want 12", got)
	}
}

func TestClassicTuneRepeatedChordNotes(t *testing.T) {
	// CTuneBuilder::StuffChord (classic client 6ba334c) reuses the pitch's
	// bookkeeping slot but appends another finite note event. Only long notes
	// have their previously written duration changed by a repeated pitch.
	const twoUnits = 250 * time.Millisecond
	tests := []struct {
		name string
		tune string
		want []Note
	}{
		{
			name: "double chord with melody",
			tune: "[=cc]2e",
			want: []Note{
				{Key: 48, Velocity: 100, Duration: 236666667 * time.Nanosecond},
				{Key: 48, Velocity: 100, Duration: 236666667 * time.Nanosecond},
				{Key: 52, Velocity: 100, Duration: 236666667 * time.Nanosecond},
			},
		},
		{
			name: "adjacent chords",
			tune: "[c]2[c]4p4",
			want: []Note{
				{Key: 48, Velocity: 100, Duration: 236666667 * time.Nanosecond},
				{Key: 48, Velocity: 100, Duration: 486666667 * time.Nanosecond},
			},
		},
		{
			name: "overlapping chords",
			tune: "[c]8p2[c]2p8",
			want: []Note{
				{Key: 48, Velocity: 100, Duration: 986666667 * time.Nanosecond},
				{Key: 48, Velocity: 100, Start: twoUnits, Duration: 236666667 * time.Nanosecond},
			},
		},
		{
			name: "finite chord followed by sustained chord",
			tune: "[c]8p2[c]$p2[c]$p8",
			want: []Note{
				{Key: 48, Velocity: 100, Duration: 986666667 * time.Nanosecond},
				{Key: 48, Velocity: 100, Start: twoUnits, Duration: twoUnits},
			},
		},
		{
			name: "sustained chord toggled off",
			tune: "[c]$p8[c]$p2",
			want: []Note{
				{Key: 48, Velocity: 100, Duration: time.Second},
			},
		},
		{
			name: "sustained chord followed by finite chord",
			tune: "[c]$p8[cc]2p2",
			want: []Note{
				{Key: 48, Velocity: 100, Duration: time.Second},
				{Key: 48, Velocity: 100, Start: time.Second, Duration: 236666667 * time.Nanosecond},
				{Key: 48, Velocity: 100, Start: time.Second, Duration: 236666667 * time.Nanosecond},
			},
		},
		{
			name: "duplicate sustained pitches toggle off immediately",
			tune: "[cc]$p2",
			want: []Note{},
		},
		{
			name: "third sustained pitch toggles back on",
			tune: "[ccc]$p2",
			want: []Note{
				{Key: 48, Velocity: 100, Duration: twoUnits},
			},
		},
		{
			name: "late sustained chord ends with song",
			tune: "p8p8[c]$p2",
			want: []Note{
				{Key: 48, Velocity: 100, Start: 2 * time.Second, Duration: twoUnits},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			notes, err := parseClassicTune(test.tune, instruments[7], 120, 100)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(notes, test.want) {
				t.Fatalf("%q notes =\n%+v\nwant:\n%+v", test.tune, notes, test.want)
			}
		})
	}
}

func TestClassicTuneDuplicateChordPolyphony(t *testing.T) {
	for index, inst := range instruments {
		if !inst.hasChords {
			continue
		}
		for _, count := range []int{inst.polyphony, inst.polyphony + 1} {
			tune := "[" + strings.Repeat("c", count) + "]2p2"
			notes, err := parseClassicTune(tune, inst, 120, 100)
			if count > inst.polyphony {
				if err == nil || err.Code != tuneErrorInvalidChord {
					t.Errorf("instrument %d accepted %d repeated notes: %v", index, count, err)
				}
			} else if err != nil || len(notes) != count {
				t.Errorf("instrument %d with %d repeated notes: got %d notes, error %v", index, count, len(notes), err)
			}
		}
	}

	// Repeating a pitch reuses its classic bookkeeping slot, even while an
	// earlier finite event for that pitch is still audible.
	notes, err := parseClassicTune("[c]8p2[c]2p2[d]2p8", instruments[8], 120, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(notes) != 3 || notes[0].Duration != 986666667*time.Nanosecond || notes[2].Start != 500*time.Millisecond {
		t.Fatalf("reused chord slot: got %+v", notes)
	}
}
