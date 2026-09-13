package main

import (
	"reflect"
	"strings"
	"testing"
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
