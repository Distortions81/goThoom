package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"
)

type classicReferenceNote struct {
	Key, Velocity, Start, Duration int
}

type classicReferenceCase struct {
	Instrument  int
	Tune, Error string
	Notes       []classicReferenceNote
}

func checkClassicReferenceNotes(t *testing.T, got []Note, want []classicReferenceNote) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("notes = %+v, want %d classic events: %+v", got, len(want), want)
	}
	for i, note := range got {
		start := time.Duration(want[i].Start) * time.Second / 600
		duration := time.Duration(want[i].Duration) * time.Second / 600
		if note.Key != want[i].Key || note.Velocity != want[i].Velocity ||
			absDuration(note.Start-start) > time.Nanosecond || absDuration(note.Duration-duration) > time.Nanosecond {
			t.Errorf("note %d = %+v, want key=%d velocity=%d start=%v duration=%v", i, note, want[i].Key, want[i].Velocity, start, duration)
		}
	}
}

func absDuration(d time.Duration) time.Duration {
	if d < 0 {
		return -d
	}
	return d
}

//go:embed testdata/music/classic_parser.json
var classicReferenceData []byte

func TestClassicParserReference(t *testing.T) {
	var cases []classicReferenceCase
	if err := json.Unmarshal(classicReferenceData, &cases); err != nil {
		t.Fatal(err)
	}
	if len(cases) == 0 {
		t.Fatal("empty classic reference corpus")
	}
	for _, test := range cases {
		t.Run(fmt.Sprintf("%d_%s", test.Instrument, test.Tune), func(t *testing.T) {
			notes, parseErr := parseClassicTune(test.Tune, instruments[test.Instrument], 120, 100)
			code := "none"
			if parseErr != nil {
				code = string(parseErr.Code)
			}
			if code != test.Error {
				t.Fatalf("parse error = %v, want %s", parseErr, test.Error)
			}
			if parseErr != nil {
				if parseErr.Position < 0 || parseErr.Position >= len(test.Tune) {
					t.Fatalf("invalid source position: %v", parseErr)
				}
				if _, err := validateBardTune(test.Tune, test.Instrument); err == nil {
					t.Fatal("Bard accepted notation rejected by classic")
				}
				return
			}
			checkClassicReferenceNotes(t, notes, test.Notes)
			if len(test.Notes) == 0 {
				return
			} // Bard requires an audible event.
			edited, err := validateBardTune(test.Tune, test.Instrument)
			if err != nil {
				t.Fatalf("Bard rejected classic notation: %v", err)
			}
			checkClassicReferenceNotes(t, edited, test.Notes)
			commands, err := bardTuneCommands(test.Tune)
			if err != nil {
				t.Fatal(err)
			}
			var parts []string
			for _, command := range commands {
				parts = append(parts, strings.TrimPrefix(strings.TrimPrefix(command, "/use "), "/part "))
			}
			sent, parseErr := parseClassicTune(strings.Join(parts, " "), instruments[test.Instrument], 120, 100)
			if parseErr != nil {
				t.Fatal(parseErr)
			}
			checkClassicReferenceNotes(t, sent, test.Notes)
		})
	}
}

func TestTuneErrorPositionsUseOriginalText(t *testing.T) {
	for _, test := range []struct {
		tune, marker string
		code         tuneParseErrorCode
	}{
		{"<雪>\n(c)2 \\ c .", ".", tuneErrorToneOverflow},
		{"<tempo> [c]8 p2 @ 60 d", "@", tuneErrorInvalidTempoChange},
		{"<title> (c|1d|1e)2", "|1e", tuneErrorDuplicateEnding},
		{"<title> c ? d", "?", tuneErrorInvalidNote},
		{"c <unfinished", "<", tuneErrorUnterminatedComment},
	} {
		_, err := parseClassicTune(test.tune, instruments[2], 120, 100)
		if err == nil || err.Code != test.code || err.Position != strings.Index(test.tune, test.marker) {
			t.Errorf("%q: error=%+v, want %s at %d", test.tune, err, test.code, strings.Index(test.tune, test.marker))
		}
	}
}

func TestTuneDurationIncludesFiniteChordTails(t *testing.T) {
	for _, test := range []struct {
		tune  string
		ticks int
	}{
		{"[ceg]", 292}, {"c[eg]", 442}, {"[c]8e", 592}, {"[c]8p8p8", 1200},
	} {
		duration, err := bardPartDuration(bardPart{Instrument: 2, Text: test.tune})
		want := time.Duration(test.ticks) * time.Second / 600
		if err != nil || absDuration(duration-want) > time.Nanosecond {
			t.Errorf("%q: duration=%v error=%v, want %v", test.tune, duration, err, want)
		}
	}
}

func TestBardSpacedModifiersSurviveMultipartCommands(t *testing.T) {
	phrase := "(c 4 # d 2 _ [e g] 2 p 4) 2 "
	value := "@ 90 " + strings.Repeat(phrase, 70)
	want, err := validateBardTune(value, 2)
	if err != nil {
		t.Fatal(err)
	}
	commands, err := bardEnsembleCommands(value, []string{"Blue"})
	if err != nil || len(commands) < 2 {
		t.Fatalf("commands=%d error=%v", len(commands), err)
	}
	var parts []string
	for _, cmd := range commands {
		if len(cmd) > 511 {
			t.Fatalf("command length=%d", len(cmd))
		}
		part := strings.TrimPrefix(strings.TrimPrefix(cmd, "/use "), "/part ")
		parts = append(parts, strings.TrimPrefix(part, "/with Blue "))
	}
	got, err := validateBardTune(strings.Join(parts, " "), 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != len(want) {
		t.Fatalf("round trip changed note count: %d vs %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("round trip changed note %d", i)
		}
	}
}

func TestIncomingTuneExpansionLimit(t *testing.T) {
	for _, value := range []string{strings.Repeat("c", maxTuneTextBytes+1), "(" + strings.Repeat("c", 20000) + ")9", "((((((c)9)9)9)9)9)9"} {
		_, err := parseClassicTune(value, instruments[2], 120, 100)
		if err == nil || err.Code != tuneErrorTooLarge {
			t.Errorf("oversized tune error=%v", err)
		}
	}
}

func FuzzClassicTuneNotation(f *testing.F) {
	for _, seed := range []string{"[=cc]2e", "(c|1d|2e!f)3", "[c]$p8[c]$", "c <unterminated", "[c@60e]", "c4#", "@ 1 2 0 c", "(c)2_", "[cc]$p2"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, text string) {
		notes, duration, err := parseClassicTuneTimeline(text, instruments[7], 120, 100)
		if err != nil {
			if err.Position < 0 || err.Position > len(text) {
				t.Fatalf("error position %d outside input", err.Position)
			}
			return
		}
		for _, note := range notes {
			if note.Start < 0 || note.Duration <= 0 || note.Start+note.Duration > duration {
				t.Fatalf("invalid event %+v in duration %v", note, duration)
			}
		}
	})
}
