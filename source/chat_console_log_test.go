package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTextLogsSuppressedForPrimaryMovieSession(t *testing.T) {
	preserveStoragePathTestState(t)
	originalTextLog := primarySession.textLog
	originalMovie, originalMode, originalPlaying := clmov, movieMode, playingMovie
	t.Cleanup(func() {
		primarySession.textLog = originalTextLog
		clmov, movieMode, playingMovie = originalMovie, originalMode, originalPlaying
	})
	for _, tc := range []struct {
		name, moviePath string
		mode, playing   bool
	}{
		{name: "loading", moviePath: "example.clmov"},
		{name: "finished", mode: true},
		{name: "playing", playing: true},
		{name: "live"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dataDirPath = t.TempDir()
			gs.LogsPath = ""
			storagePathsActivated = false
			primarySession.textLog = newSessionTextLogState()
			primarySession.login.setRequest(sessionLoginRequest{character: "Hero"})
			clmov, movieMode, playingMovie = tc.moviePath, tc.mode, tc.playing
			existingPath := filepath.Join(dataDirPath, "existing.txt")
			primarySession.textLog.path = existingPath
			primarySession.textLog.character = "Hero"
			const original = "existing live session\n"
			if err := os.WriteFile(existingPath, []byte(original), 0o644); err != nil {
				t.Fatal(err)
			}
			appendTextLogForSession(primarySession, "chat line")
			appendTextLogForSession(primarySession, "console line")
			contents, err := os.ReadFile(existingPath)
			if err != nil {
				t.Fatal(err)
			}
			suppressed := tc.name != "live"
			if suppressed && string(contents) != original {
				t.Fatal("movie session appended to an existing text log")
			}
			if !suppressed && (!strings.Contains(string(contents), "chat line") || !strings.Contains(string(contents), "console line")) {
				t.Fatal("live session did not append chat and console lines")
			}
			primarySession.textLog = newSessionTextLogState()
			appendTextLogForSession(primarySession, "new session")
			ensureTextLogForSession(primarySession)
			path, _ := sessionTextLogSnapshot(primarySession)
			if suppressed {
				if path != "" {
					t.Fatal("movie session initialized a text log")
				}
				if _, err := os.Stat(textLogsDirPath()); !os.IsNotExist(err) {
					t.Fatalf("movie session created Text Logs: %v", err)
				}
			} else if path == "" {
				t.Fatal("live session did not initialize a text log")
			}
		})
	}
}

func TestTextLogsRemainSeparateForDuplicateCharacterSessions(t *testing.T) {
	preserveStoragePathTestState(t)
	dataDirPath = t.TempDir()
	gs.LogsPath = ""
	storagePathsActivated = false
	first := mustNewSession(2)
	second := mustNewSession(3)
	first.login.setRequest(sessionLoginRequest{character: "Same Hero"})
	second.login.setRequest(sessionLoginRequest{character: "Same Hero"})
	appendTextLogForSession(first, "first session line")
	appendTextLogForSession(second, "second session line")
	firstPath, _ := sessionTextLogSnapshot(first)
	secondPath, _ := sessionTextLogSnapshot(second)
	if firstPath == "" || secondPath == "" || firstPath == secondPath {
		t.Fatalf("session log paths = %q and %q, want distinct files", firstPath, secondPath)
	}
	firstContents, err := os.ReadFile(firstPath)
	if err != nil {
		t.Fatal(err)
	}
	secondContents, err := os.ReadFile(secondPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(firstContents), "second session line") || strings.Contains(string(secondContents), "first session line") {
		t.Fatal("session text logs contain another session's message")
	}
}
