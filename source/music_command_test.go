package main

import (
	"bytes"
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/hajimehoshi/ebiten/v2/audio"
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

func TestParseMusicCommandIgnoredWhileMovieSeeking(t *testing.T) {
	oldBlockMusic := blockMusic
	blockMusic = true
	t.Cleanup(func() { blockMusic = oldBlockMusic })

	if !parseMusicCommand("/music/play/inst1/notesabc", nil) {
		t.Fatal("recognized music command should remain handled while seeking")
	}
}

func TestMusicStopLeavesMixerEnabled(t *testing.T) {
	oldSettings := gs
	gs = gsdef
	gs.Music = true
	t.Cleanup(func() { gs = oldSettings })

	handleMusicParams(MusicParams{Stop: true})
	if !gs.Music {
		t.Fatal("a song stop must not disable the Music mixer setting")
	}
}

func TestMusicStopTokenRecognition(t *testing.T) {
	tests := []struct {
		command string
		want    bool
	}{
		{command: "W123/S", want: true},
		{command: "who123/stop", want: true},
		{command: "W123/S/P/inst0/Nc", want: true},
		{command: "W123/Sustain/P/inst0/Nc", want: false},
		{command: "W123/P/inst0/Nc/S", want: false},
	}
	for _, tt := range tests {
		if got := musicCommandHasStop(tt.command); got != tt.want {
			t.Errorf("musicCommandHasStop(%q) = %v, want %v", tt.command, got, tt.want)
		}
	}
}

func TestMusicCommandWho(t *testing.T) {
	for _, command := range []string{"W123/S", "who123/stop", "other/W123/S/P"} {
		if got := musicCommandWho(command); got != 123 {
			t.Errorf("musicCommandWho(%q) = %d, want 123", command, got)
		}
	}
}

func TestScopedMusicStopOnlyClosesMatchingBard(t *testing.T) {
	first, err := audioContext.NewPlayer(bytes.NewReader([]byte{0, 0, 0, 0}))
	if err != nil {
		t.Fatalf("first player: %v", err)
	}
	second, err := audioContext.NewPlayer(bytes.NewReader([]byte{0, 0, 0, 0}))
	if err != nil {
		_ = first.Close()
		t.Fatalf("second player: %v", err)
	}
	newStream := func() *musicStream {
		return &musicStream{done: make(chan struct{})}
	}

	musicPlayersMu.Lock()
	originalPlayers := musicPlayers
	musicPlayers = map[*audio.Player]musicTrack{
		first:  {stream: newStream(), whos: map[int]struct{}{123: {}}},
		second: {stream: newStream(), whos: map[int]struct{}{456: {}}},
	}
	musicPlayersMu.Unlock()
	t.Cleanup(func() {
		musicPlayersMu.Lock()
		musicPlayers = originalPlayers
		musicPlayersMu.Unlock()
		_ = first.Close()
		_ = second.Close()
	})

	if !parseMusicCommand("/music/W123/S", nil) {
		t.Fatal("scoped stop command was not handled")
	}
	if err := first.Close(); err == nil {
		t.Fatal("matching bard player remained open")
	}
	if err := second.Close(); err != nil {
		t.Fatalf("unrelated bard player was closed: %v", err)
	}
}

func TestVolumeRefreshKeepsPrebufferedMusicPlayer(t *testing.T) {
	p, err := audioContext.NewPlayer(bytes.NewReader([]byte{0, 0, 0, 0}))
	if err != nil {
		t.Fatalf("NewPlayer: %v", err)
	}
	musicPlayersMu.Lock()
	musicPlayers[p] = musicTrack{}
	musicPlayersMu.Unlock()
	t.Cleanup(func() {
		musicPlayersMu.Lock()
		delete(musicPlayers, p)
		musicPlayersMu.Unlock()
		_ = p.Close()
	})

	// The player has deliberately not been started yet, just like a /with
	// track that is still rendering its five-second buffer.
	updateSoundVolume()
	if err := p.Close(); err != nil {
		t.Fatalf("volume refresh closed a valid prebuffered music player: %v", err)
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

// TestConcertTrioAtThirteenThirty verifies the three-way /H group near 13:30
// (frames 4071-4141 at 5 FPS) in the
// concert recording.  The player must recognize each complete track: Aurelie
// on viol, Xepel on conch, and Coriakin on lucky lyra.
func TestConcertTrioAtThirteenThirty(t *testing.T) {
	frames, err := parseMovie(movieFixturePath(t, "concert1.clMov"), baseVersion)
	if err != nil {
		t.Fatalf("parseMovie: %v", err)
	}

	type track struct {
		who, inst int
		notes     string
		frame     int32
	}
	want := map[int]int{
		227574197: 16, // Aurelie: viol
		227864012: 8,  // Xepel: conch
		227630059: 0,  // Coriakin: lucky lyra
	}
	got := make(map[int]track)
	for _, frame := range frames {
		if frame.index < 4050 || frame.index > 4200 {
			continue
		}
		idx := bytes.Index(frame.data, []byte("/music/"))
		if idx < 0 {
			continue
		}
		s := string(frame.data[idx:])
		if end := strings.IndexByte(s, 0); end >= 0 {
			s = s[:end]
		}
		// The first two messages per bard carry /M (more follows). The final
		// message is the one that starts the assembled track.
		if strings.Contains(s, "/M/") {
			continue
		}
		var who, inst int
		if _, err := fmt.Sscanf(s, "/music/W%d/P/", &who); err != nil {
			continue
		}
		if expectedInst, ok := want[who]; ok {
			if _, err := fmt.Sscanf(s[strings.Index(s, "/inst"):], "/inst%d", &inst); err != nil {
				t.Fatalf("frame %d: instrument: %v", frame.index, err)
			}
			noteStart := strings.Index(s, "/N")
			if noteStart < 0 {
				t.Fatalf("frame %d: no notes", frame.index)
			}
			got[who] = track{who: who, inst: inst, notes: s[noteStart+2:], frame: frame.index}
			if inst != expectedInst {
				t.Errorf("frame %d: bard %d instrument = %d, want %d", frame.index, who, inst, expectedInst)
			}
		}
	}
	if len(got) != len(want) {
		t.Fatalf("found %d complete trio tracks, want %d", len(got), len(want))
	}
	for who, tr := range got {
		if ns := classicNotesFromTune(tr.notes, instruments[tr.inst], 120, 127); len(ns) == 0 {
			t.Errorf("frame %d: bard %d has no playable notes", tr.frame, who)
		}
	}
}

func TestConcertTrioCommandsAssembleInMovieOrder(t *testing.T) {
	frames, err := parseMovie(movieFixturePath(t, "concert1.clMov"), baseVersion)
	if err != nil {
		t.Fatalf("parseMovie: %v", err)
	}
	oldSettings, oldBlock := gs, blockMusic
	oldDataDir, oldFont, oldSynthSettings := dataDirPath, sfntCached, synthSettings
	_, testFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("find test source path")
	}
	dataDirPath = filepath.Join(filepath.Dir(testFile), "data")
	setupSynthOnce = sync.Once{}
	sfntCached, synthSettings = nil, nil
	gs = gsdef
	gs.Music = true
	blockMusic = false
	pendingMu.Lock()
	pendingByID = make(map[int]*pendingSong)
	pendingMu.Unlock()
	t.Cleanup(func() {
		stopAllMusic()
		gs, blockMusic = oldSettings, oldBlock
		dataDirPath = oldDataDir
		setupSynthOnce = sync.Once{}
		sfntCached, synthSettings = oldFont, oldSynthSettings
		pendingMu.Lock()
		pendingByID = make(map[int]*pendingSong)
		pendingMu.Unlock()
	})

	for _, frame := range frames {
		if frame.index < 4071 || frame.index > 4141 {
			continue
		}
		if idx := bytes.Index(frame.data, []byte("/music/")); idx >= 0 {
			if !parseMusicCommand("", frame.data[idx:]) {
				t.Fatalf("frame %d: music command was not handled", frame.index)
			}
		}
	}

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		musicPlayersMu.Lock()
		for _, track := range musicPlayers {
			if len(track.whos) == 3 {
				musicPlayersMu.Unlock()
				return
			}
		}
		musicPlayersMu.Unlock()
		time.Sleep(10 * time.Millisecond)
	}
	pendingMu.Lock()
	pending := make(map[int]int, len(pendingByID))
	for who, song := range pendingByID {
		pending[who] = len(song.notes)
	}
	pendingMu.Unlock()
	t.Fatalf("ordered concert trio never started a mixed music player; pending=%v", pending)
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
