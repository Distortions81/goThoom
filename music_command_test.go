package main

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestParseMusicCommandWithWho(t *testing.T) {
	// Ensure /music commands with a leading /who segment are parsed.
	if !parseMusicCommand("/music/who123/play/inst2/notesabc", nil) {
		t.Fatalf("parseMusicCommand failed to parse /music with /who prefix")
	}
}

func TestParseMusicCommandRawFallback(t *testing.T) {
	if !parseMusicCommand("", []byte("/music/play/inst1/notesabc")) {
		t.Fatalf("parseMusicCommand failed to parse raw payload")
	}
}

// TestParseMusicCommandFromMovie extracts a /music payload from the lore1.clMov
// sample and verifies that parseMusicCommand can decode it when debug logging
// is enabled.
func TestParseMusicCommandFromMovie(t *testing.T) {
	frames, err := parseMovie(movieFixturePath(t, "lore1.clMov"), baseVersion)
	if err != nil {
		t.Fatalf("parseMovie: %v", err)
	}
	var msg []byte
	for _, f := range frames {
		if idx := bytes.Index(f.data, []byte("/music/")); idx >= 0 {
			msg = f.data[idx:]
			if j := bytes.IndexByte(msg, 0); j >= 0 {
				msg = msg[:j]
			}
			break
		}
	}
	if len(msg) == 0 {
		t.Fatalf("no /music payload found in movie frames")
	}

	s := string(msg)
	inst := "0"
	if idx := strings.Index(s, "/inst"); idx >= 0 {
		v := s[idx+len("/inst"):]
		v = strings.TrimPrefix(v, "/")
		if j := strings.IndexByte(v, '/'); j >= 0 {
			v = v[:j]
		}
		inst = v
	}
	notes := ""
	if idx := strings.Index(s, "/notes"); idx >= 0 {
		notes = s[idx+len("/notes"):]
	} else if idx := strings.Index(s, "/N"); idx >= 0 {
		notes = s[idx+len("/N"):]
	} else {
		notes = s
	}
	notes = strings.Trim(notes, "/")
	expected := "/play " + inst + " " + notes

	consoleLog.entries = nil
	musicDebug = true
	defer func() { musicDebug = false }()
	if !parseMusicCommand("", msg) {
		t.Fatalf("parseMusicCommand failed to parse clMov payload")
	}
	found := false
	for _, m := range consoleLog.entries {
		if m.Text == expected {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected %q in console log, got %#v", expected, consoleLog.entries)
	}
}

func TestParseMusicCommandWithMalformedWith(t *testing.T) {
	cases := []string{
		"/music/play/inst1/with/with/notesabc",
		"/music/play/inst1/withabc/notesabc",
	}
	for _, cmd := range cases {
		done := make(chan struct{})
		go func(c string) {
			parseMusicCommand(c, nil)
			close(done)
		}(cmd)
		select {
		case <-done:
			// ok
		case <-time.After(100 * time.Millisecond):
			t.Fatalf("parseMusicCommand did not terminate for %q", cmd)
		}
	}
}
