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

	if playingMovie {
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

// ensureTextLog initializes the legacy Text Log path matching old_mac_client.
// Path: "Text Logs/<CharName>/CL Log YYYY/MM/DD HH.MM.SS.txt"
func ensureTextLog() {
	if isWASM {
		textLogPath = ""
		textLogChar = ""
		return
	}
	textLogMu.Lock()
	defer textLogMu.Unlock()

	if playingMovie {
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

	base := filepath.Join("Text Logs")
	charDir := filepath.Join(base, desired)

	now := time.Now()
	year := fmt.Sprintf("%04d", now.Year())
	month := fmt.Sprintf("%02d", int(now.Month()))
	day := fmt.Sprintf("%02d", now.Day())
	timeName := fmt.Sprintf("%s %02d.%02d.%02d.txt", day, now.Hour(), now.Minute(), now.Second())
	yearMonthDir := filepath.Join(charDir, "CL Log "+year, month)

	if err := os.MkdirAll(yearMonthDir, 0o755); err != nil {
		return
	}
	textLogPath = filepath.Join(yearMonthDir, timeName)
	textLogChar = desired

	// Optional session marker at rotation
	f, err := os.OpenFile(textLogPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err == nil {
		_, _ = f.WriteString(fmt.Sprintf("=== Session started %s as %s ===\n", now.Format(time.RFC3339), textLogChar))
		_ = f.Close()
	}
}
