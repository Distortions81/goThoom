package main

import (
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func withSessionRecordingTestState(t *testing.T) {
	t.Helper()
	preserveStoragePathTestState(t)
	originalSettings := gs
	originalMovie, originalPlaying, originalPCAP, originalFake := clmov, playingMovie, pcapPath, fake
	dataDirPath = t.TempDir()
	gs.LogsPath = ""
	gs.AutoRecord = false
	gs.PromptOnSaveRecording = false
	storagePathsActivated = false
	clmov, pcapPath = "", ""
	playingMovie, fake = false, false
	t.Cleanup(func() {
		gs = originalSettings
		clmov, playingMovie, pcapPath, fake = originalMovie, originalPlaying, originalPCAP, originalFake
	})
}

func TestArmedRecordingStartsOnlyForOwningSession(t *testing.T) {
	withSessionRecordingTestState(t)
	first := mustNewSession(2)
	second := mustNewSession(3)
	first.login.setRequest(sessionLoginRequest{character: "First"})
	second.login.setRequest(sessionLoginRequest{character: "Second"})
	first.recording.setArmed(true)
	second.recording.setArmed(true)
	draw := []byte{0, 2, 0, 0}
	if !recordSessionIncomingMovieMessageAt(first, draw, time.Now()) {
		t.Fatal("first armed draw was not consumed before snapshot")
	}
	firstActive, firstArmed, _ := sessionRecordingSnapshot(first)
	secondActive, secondArmed, _ := sessionRecordingSnapshot(second)
	if !firstActive || firstArmed {
		t.Fatalf("first recording state = active %t armed %t", firstActive, firstArmed)
	}
	if secondActive || !secondArmed {
		t.Fatalf("second recording changed before its draw = active %t armed %t", secondActive, secondArmed)
	}
	if !recordSessionIncomingMovieMessageAt(second, draw, time.Now()) {
		t.Fatal("second armed draw was not consumed before snapshot")
	}
	stopRecordingForSession(first)
	stopRecordingForSession(second)
}

func TestConcurrentSessionRecordingsUseIndependentFiles(t *testing.T) {
	withSessionRecordingTestState(t)
	first := mustNewSession(2)
	second := mustNewSession(3)
	first.login.setRequest(sessionLoginRequest{character: "Same Hero"})
	second.login.setRequest(sessionLoginRequest{character: "Same Hero"})
	if !startRecordingForSession(first) || !startRecordingForSession(second) {
		t.Fatal("could not start both session recorders")
	}
	_, _, firstPath := sessionRecordingSnapshot(first)
	_, _, secondPath := sessionRecordingSnapshot(second)
	if firstPath == "" || secondPath == "" || firstPath == secondPath {
		t.Fatalf("recording paths = %q and %q, want distinct files", firstPath, secondPath)
	}
	if !strings.Contains(filepath.Base(firstPath), "session-2") || !strings.Contains(filepath.Base(secondPath), "session-3") {
		t.Fatalf("recording filenames do not identify their slots: %q, %q", firstPath, secondPath)
	}

	const frames = 40
	var writes sync.WaitGroup
	for _, session := range []*Session{first, second} {
		session := session
		writes.Add(1)
		go func() {
			defer writes.Done()
			for range frames {
				recordSessionIncomingMovieMessageAt(session, []byte{0, 2, 0, 0}, time.Now())
			}
		}()
	}
	writes.Wait()
	stopRecordingForSession(first)
	stopRecordingForSession(second)
	for _, path := range []string{firstPath, secondPath} {
		movieFrames, err := parseMovie(path, clVersion)
		if err != nil {
			t.Fatalf("parse %s: %v", filepath.Base(path), err)
		}
		if len(movieFrames) != frames {
			t.Fatalf("%s frames = %d, want %d", filepath.Base(path), len(movieFrames), frames)
		}
	}
}

func TestSecondaryDisconnectFinalizesOnlyItsRecording(t *testing.T) {
	withSessionRecordingTestState(t)
	first := mustNewSession(2)
	second := mustNewSession(3)
	first.login.setRequest(sessionLoginRequest{character: "First"})
	second.login.setRequest(sessionLoginRequest{character: "Second"})
	if !startRecordingForSession(first) || !startRecordingForSession(second) {
		t.Fatal("could not start session recorders")
	}
	_, _, firstPath := sessionRecordingSnapshot(first)
	completeSessionDisconnect(first)
	firstActive, firstArmed, _ := sessionRecordingSnapshot(first)
	secondActive, _, _ := sessionRecordingSnapshot(second)
	if firstActive || firstArmed {
		t.Fatal("disconnected session retained its recorder")
	}
	if !secondActive {
		t.Fatal("disconnecting one session stopped another session's recorder")
	}
	if _, err := parseMovie(firstPath, clVersion); err != nil {
		t.Fatalf("finalized recording: %v", err)
	}
	stopRecordingForSession(second)
}
