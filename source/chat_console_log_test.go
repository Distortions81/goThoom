package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTextLogsSuppressedForMovieSession(t *testing.T) {
	preserveStoragePathTestState(t)
	originalPath, originalChar, originalPlayer := textLogPath, textLogChar, playerName
	originalMovie, originalMode, originalPlaying := clmov, movieMode, playingMovie
	t.Cleanup(func() {
		textLogPath, textLogChar, playerName = originalPath, originalChar, originalPlayer
		clmov, movieMode, playingMovie = originalMovie, originalMode, originalPlaying
	})
	for _, tc := range []struct {
		name, path    string
		mode, playing bool
	}{
		{name: "loading", path: "example.clmov"},
		{name: "finished", mode: true},
		{name: "playing", playing: true},
		{name: "live"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dataDirPath = t.TempDir()
			gs.LogsPath = ""
			storagePathsActivated = false
			playerName = "Hero"
			clmov, movieMode, playingMovie = tc.path, tc.mode, tc.playing
			textLogPath = filepath.Join(dataDirPath, "existing.txt")
			textLogChar = playerName
			const original = "existing live session\n"
			if err := os.WriteFile(textLogPath, []byte(original), 0o644); err != nil {
				t.Fatal(err)
			}
			existingPath := textLogPath
			appendChatLog("chat line")
			appendConsoleLog("console line")
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
			textLogPath, textLogChar = "", ""
			appendChatLog("new session")
			ensureTextLog()
			if suppressed {
				if textLogPath != "" {
					t.Fatal("movie session initialized a text log")
				}
				if _, err := os.Stat(textLogsDirPath()); !os.IsNotExist(err) {
					t.Fatalf("movie session created Text Logs: %v", err)
				}
			} else if textLogPath == "" {
				t.Fatal("live session did not initialize a text log")
			}
		})
	}
}
