package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

var (
	textLogPath string
	textLogChar string
	textLogMu   sync.Mutex
)

// appendChatLog appends a chat line to the legacy-style Text Logs file.
func appendChatLog(msg string) { appendTextLog(msg) }

// appendConsoleLog appends a console line to the legacy-style Text Logs file.
func appendConsoleLog(msg string) { appendTextLog(msg) }

func appendTextLog(msg string) {
	if msg == "" {
		return
	}
	if isWASM {
		return
	}

	// The selected path covers loading; movieMode remains set at end of playback.
	if clmov != "" || movieMode || playingMovie {
		return
	}

	ensureTextLog()
	if textLogPath == "" {
		return
	}

	// Old client timestamp format: M/D/YY H:MM:SSa (no leading zeros for M/D/H)
	now := time.Now()
	hour := now.Hour()
	ampm := byte('a')
	if hour >= 12 {
		ampm = 'p'
	}
	hour12 := hour % 12
	if hour12 == 0 {
		hour12 = 12
	}
	ts := fmt.Sprintf("%d/%d/%.2d %d:%.2d:%.2d%c ",
		int(now.Month()), now.Day(), now.Year()%100,
		hour12, now.Minute(), now.Second(), ampm,
	)

	// Convert any CR to LF similar to SwapLineEndings before writing.
	line := strings.ReplaceAll(msg, "\r", "\n")
	line = strings.TrimRight(line, "\n")
	// One entry per line
	out := ts + line + "\n"

	// Append
	f, err := os.OpenFile(textLogPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return
	}
	_, _ = f.WriteString(out)
	_ = f.Close()
}

// ensureTextLog initializes the Text Log path matching the classic Windows client.
// Path: "Text Logs/<CharName>/CL Log YYYY-MM-DD HH.MM.SS.txt"
func ensureTextLog() {
	if isWASM {
		textLogPath = ""
		textLogChar = ""
		return
	}
	textLogMu.Lock()
	defer textLogMu.Unlock()

	if clmov != "" || movieMode || playingMovie {
		textLogPath = ""
		textLogChar = ""
		return
	}

	// Determine the preferred character name for logging.
	desired := strings.TrimSpace(playerName)
	if desired == "" {
		desired = strings.TrimSpace(gs.LastCharacter)
	}

	// If we already have a log file and either no desired name yet or the same
	// character, keep using the current file.
	if textLogPath != "" && (desired == "" || desired == textLogChar) {
		return
	}

	// If we don't have a desired character yet and no file exists, defer until later.
	if textLogPath == "" && desired == "" {
		return
	}

	// Rotate or initialize the log file for the new character.
	if desired == "" {
		// No new character yet; keep existing file.
		return
	}

	base := textLogsDirPath()
	charDir := filepath.Join(base, desired)

	now := time.Now()
	timeName := now.Format("CL Log 2006-01-02 15.04.05.txt")

	if err := os.MkdirAll(charDir, 0o755); err != nil {
		return
	}
	textLogPath = filepath.Join(charDir, timeName)
	textLogChar = desired

	// Optional session marker at rotation
	f, err := os.OpenFile(textLogPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err == nil {
		_, _ = f.WriteString(fmt.Sprintf("=== Session started %s as %s ===\n", now.Format(time.RFC3339), textLogChar))
		_ = f.Close()
	}
}
