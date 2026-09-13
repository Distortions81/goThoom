package main

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// sessionRecordingState owns the recorder and pre-connect recording request
// for exactly one game session.
type sessionRecordingState struct {
	mu       sync.Mutex
	recorder *movieRecorder
	armed    bool
	path     string
}

func newSessionRecordingState() *sessionRecordingState {
	return &sessionRecordingState{}
}

func (r *sessionRecordingState) setArmed(armed bool) {
	if r == nil {
		return
	}
	r.mu.Lock()
	r.armed = armed
	r.mu.Unlock()
}

func sessionRecordingSnapshot(session *Session) (active, armed bool, path string) {
	if session == nil || session.recording == nil {
		return false, false, ""
	}
	session.recording.mu.Lock()
	active = session.recording.recorder != nil
	armed = session.recording.armed
	path = session.recording.path
	session.recording.mu.Unlock()
	return
}

func sessionRecordingRequested(session *Session) bool {
	active, armed, _ := sessionRecordingSnapshot(session)
	return active || armed
}

func recordingCharacterName(session *Session) string {
	if session == nil {
		return "movie"
	}
	name := strings.TrimSpace(session.characterName())
	if name == "" {
		name = strings.TrimSpace(session.login.requestSnapshot().character)
	}
	name = strings.Map(func(r rune) rune {
		switch r {
		case '/', '\\', ':', '*', '?', '"', '<', '>', '|':
			return '_'
		default:
			return r
		}
	}, name)
	if name == "" || name == "." {
		return "movie"
	}
	return name
}

func recordingPathForSession(session *Session, now time.Time) string {
	base := recordingCharacterName(session)
	if session != nil && session.ID() != primarySessionID {
		base = fmt.Sprintf("%s_session-%d", base, session.ID())
	}
	dir := filepath.Join(dataDirPath, "Movies")
	stem := fmt.Sprintf("%s__%s", base, now.Format("2006-01-02-15-04-05"))
	for sequence := 0; ; sequence++ {
		name := stem + ".clMov"
		if sequence > 0 {
			name = fmt.Sprintf("%s_%d.clMov", stem, sequence+1)
		}
		candidate := filepath.Join(dir, name)
		if _, err := os.Stat(candidate); err != nil {
			return candidate
		}
	}
}

func startRecordingForSession(session *Session) bool {
	return startSessionRecorder(session, false)
}

func startArmedRecordingForSession(session *Session) bool {
	return startSessionRecorder(session, true)
}

func startSessionRecorder(session *Session, requireArmed bool) bool {
	if session == nil || session.recording == nil {
		return false
	}
	r := session.recording
	r.mu.Lock()
	if r.recorder != nil {
		r.armed = false
		r.mu.Unlock()
		return true
	}
	if requireArmed && !r.armed {
		r.mu.Unlock()
		return false
	}
	if isWASM {
		r.mu.Unlock()
		session.publishClientConsole("movie recording unavailable in browser build", messageTextTypeSystem)
		return false
	}
	dir := filepath.Join(dataDirPath, "Movies")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		r.mu.Unlock()
		logError("record movie for session %d: %v", session.ID(), err)
		return false
	}
	path := recordingPathForSession(session, time.Now())
	mr, err := newMovieRecorder(path, clVersion, int(movieRevision))
	if err != nil {
		r.mu.Unlock()
		logError("record movie for session %d: %v", session.ID(), err)
		return false
	}
	session.draw.mu.Lock()
	snapshot := cloneDrawState(session.draw.current)
	session.draw.mu.Unlock()
	mr.AddStateSnapshot(snapshot, uint16(clVersion), captureMovieNightStateForSession(session))
	r.recorder = mr
	r.path = path
	r.armed = false
	r.mu.Unlock()
	session.publishClientConsole(fmt.Sprintf("recording to %s", filepath.Base(path)), messageTextTypeSystem)
	updateRecordButton()
	return true
}

func stopRecordingForSession(session *Session) bool {
	if session == nil || session.recording == nil {
		return false
	}
	r := session.recording
	r.mu.Lock()
	mr, saved := r.recorder, r.path
	r.recorder = nil
	r.path = ""
	r.armed = false
	r.mu.Unlock()
	if mr == nil {
		return false
	}
	if err := mr.Close(); err != nil {
		logError("record movie for session %d: %v", session.ID(), err)
	}
	if saved != "" {
		session.publishClientConsole(fmt.Sprintf("saved movie: %s", filepath.Base(saved)), messageTextTypeSystem)
		if gs.AutoRecord {
			go compressSessionRecording(session, saved)
		} else if gs.PromptOnSaveRecording {
			dispatchMainThread(func() { showRecordingSaveDialog(saved) })
		}
	}
	updateRecordButton()
	return true
}

func compressSessionRecording(session *Session, source string) {
	outName := filepath.Base(source) + ".zip"
	destination := filepath.Join(filepath.Dir(source), outName)
	if err := compressZip(source, destination); err != nil {
		logError("zip recording for session %d: %v", session.ID(), err)
		session.publishClientConsole("compress failed: "+err.Error(), messageTextTypeSystem)
		return
	}
	session.publishClientConsole("compressed: "+outName, messageTextTypeSystem)
	_ = os.Remove(source)
}

func toggleRecordingForSession(session *Session) {
	if session == nil {
		return
	}
	active, _, _ := sessionRecordingSnapshot(session)
	if active {
		stopRecordingForSession(session)
		return
	}
	if clmov != "" || playingMovie || pcapPath != "" || fake {
		session.publishClientConsole("cannot record during playback or replay", messageTextTypeSystem)
		return
	}
	if !session.transport.connected() {
		session.recording.setArmed(true)
		session.publishClientConsole("recording will start on connect", messageTextTypeSystem)
		updateRecordButton()
		return
	}
	startRecordingForSession(session)
}

// recordSessionIncomingMovieMessageAt is shared by TCP and UDP so both
// transports use the same clMov state-block encoding for their own session.
func recordSessionIncomingMovieMessageAt(session *Session, message []byte, receivedAt time.Time) bool {
	if session == nil || len(message) < 2 {
		return false
	}
	tag := binary.BigEndian.Uint16(message[:2])
	active, armed, _ := sessionRecordingSnapshot(session)
	if !active && armed && tag == 2 {
		// Apply the first complete draw before taking the initial snapshot.
		// This avoids an empty baseline when recording was armed pre-login.
		processSessionServerMessageAt(session, message, receivedAt)
		startArmedRecordingForSession(session)
		return true
	}
	if !active {
		return false
	}
	r := session.recording
	r.mu.Lock()
	mr := r.recorder
	var err error
	if mr != nil {
		err = mr.WriteNetworkMessage(message, frameFlags(message))
	}
	r.mu.Unlock()
	if err != nil {
		logError("record frame for session %d: %v", session.ID(), err)
	}
	return false
}

func stopAllSessionRecordings() {
	if appSessions == nil {
		return
	}
	for _, session := range appSessions.snapshot() {
		stopRecordingForSession(session)
	}
}
