package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type sessionTextLogState struct {
	mu        sync.Mutex
	path      string
	character string
}

func newSessionTextLogState() *sessionTextLogState {
	return &sessionTextLogState{}
}

func textLogSuppressedForSession(session *Session) bool {
	return session == primarySession && (clmov != "" || movieMode || playingMovie)
}

func textLogCharacter(session *Session) string {
	if session == nil {
		return ""
	}
	character := strings.TrimSpace(session.characterName())
	if character == "" {
		character = strings.TrimSpace(session.login.requestSnapshot().character)
	}
	return character
}

func appendTextLogForSession(session *Session, message string) {
	if session == nil || session.textLog == nil || message == "" || isWASM {
		return
	}
	ensureTextLogForSession(session)
	state := session.textLog
	state.mu.Lock()
	defer state.mu.Unlock()
	if state.path == "" || textLogSuppressedForSession(session) {
		return
	}

	// Old client timestamp format: M/D/YY H:MM:SSa (no leading zeros for M/D/H).
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
	timestamp := fmt.Sprintf("%d/%d/%.2d %d:%.2d:%.2d%c ",
		int(now.Month()), now.Day(), now.Year()%100,
		hour12, now.Minute(), now.Second(), ampm,
	)
	line := strings.TrimRight(strings.ReplaceAll(message, "\r", "\n"), "\n")
	file, err := os.OpenFile(state.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return
	}
	_, _ = file.WriteString(timestamp + line + "\n")
	_ = file.Close()
}

// ensureTextLogForSession initializes one persistent log per live session.
// Primary logs retain the classic filename; additional slots include their
// stable session ID so duplicate character sessions never share a writer.
func ensureTextLogForSession(session *Session) {
	if session == nil || session.textLog == nil {
		return
	}
	state := session.textLog
	state.mu.Lock()
	defer state.mu.Unlock()
	if isWASM || textLogSuppressedForSession(session) {
		state.path = ""
		state.character = ""
		return
	}
	desired := textLogCharacter(session)
	if state.path != "" && (desired == "" || desired == state.character) {
		return
	}
	if desired == "" {
		return
	}
	characterDir := filepath.Join(textLogsDirPath(), desired)
	now := time.Now()
	filename := now.Format("CL Log 2006-01-02 15.04.05")
	if session.ID() != primarySessionID {
		filename += fmt.Sprintf(" Session %d", session.ID())
	}
	if err := os.MkdirAll(characterDir, 0o755); err != nil {
		return
	}
	state.path = filepath.Join(characterDir, filename+".txt")
	state.character = desired
	file, err := os.OpenFile(state.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err == nil {
		_, _ = file.WriteString(fmt.Sprintf("=== Session started %s as %s ===\n", now.Format(time.RFC3339), state.character))
		_ = file.Close()
	}
}

func sessionTextLogSnapshot(session *Session) (path, character string) {
	if session == nil || session.textLog == nil {
		return "", ""
	}
	session.textLog.mu.Lock()
	path, character = session.textLog.path, session.textLog.character
	session.textLog.mu.Unlock()
	return
}
