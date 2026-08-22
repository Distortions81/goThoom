package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestListPiperVoices(t *testing.T) {
	dir := t.TempDir()
	orig := dataDirPath
	dataDirPath = dir
	defer func() { dataDirPath = orig }()

	voicesDir := filepath.Join(dir, "piper", "voices")
	if err := os.MkdirAll(voicesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// voice stored directly in voices directory
	if err := os.WriteFile(filepath.Join(voicesDir, "rootvoice.onnx"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(voicesDir, "rootvoice.onnx.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	// voice stored inside a matching subdirectory
	sub := filepath.Join(voicesDir, "dirvoice")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "dirvoice.onnx"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "dirvoice.onnx.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	// voice stored inside a mismatching subdirectory
	mis := filepath.Join(voicesDir, "mismatch")
	if err := os.MkdirAll(mis, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(mis, "othervoice.onnx"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(mis, "othervoice.onnx.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}

	voices, err := listPiperVoices()
	if err != nil {
		t.Fatalf("listPiperVoices: %v", err)
	}
	want := []string{"dirvoice", "othervoice", "rootvoice"}
	if !reflect.DeepEqual(voices, want) {
		t.Fatalf("voices = %v, want %v", voices, want)
	}
}

func TestChatTTSPendingLimit(t *testing.T) {
	origGS := gs
	gs.ChatTTS = true
	gs.Mute = false
	blockTTS = false
	defer func() {
		gs = origGS
		setHighQualityResamplingEnabled(gs.HighQualityResampling)
	}()

	stopAllTTS()

	var mu sync.Mutex
	total := 0
	origFunc := playChatTTSFunc
	playChatTTSFunc = func(ctx context.Context, text string) {
		mu.Lock()
		total += len(strings.Split(text, ". "))
		mu.Unlock()
	}
	defer func() { playChatTTSFunc = origFunc }()

	for i := 0; i < 25; i++ {
		speakChatMessage(fmt.Sprintf("m%d", i))
	}

	time.Sleep(500 * time.Millisecond)

	mu.Lock()
	got := total
	mu.Unlock()
	if got > 10 {
		t.Fatalf("synthesized %d messages, want at most 10", got)
	}
}

func TestChatTTSDisableDropsQueued(t *testing.T) {
	origGS := gs
	gs.ChatTTS = true
	gs.Mute = false
	blockTTS = false
	defer func() {
		gs = origGS
		setHighQualityResamplingEnabled(gs.HighQualityResampling)
	}()

	stopAllTTS()

	var mu sync.Mutex
	called := false
	origFunc := playChatTTSFunc
	playChatTTSFunc = func(ctx context.Context, text string) {
		mu.Lock()
		called = true
		mu.Unlock()
	}
	defer func() { playChatTTSFunc = origFunc }()

	speakChatMessage("hello")
	speakChatMessage("world")
	disableTTS()
	time.Sleep(500 * time.Millisecond)

	mu.Lock()
	wasCalled := called
	mu.Unlock()
	if wasCalled {
		t.Fatalf("playChatTTS called after disabling")
	}

	pendingTTSMu.Lock()
	n := pendingTTS
	pendingTTSMu.Unlock()
	if n != 0 {
		t.Fatalf("pendingTTS = %d, want 0", n)
	}
}

func TestSubstituteTTS(t *testing.T) {
	dir := t.TempDir()
	origDir := dataDirPath
	dataDirPath = dir
	defer func() { dataDirPath = origDir }()

	// Ensure file is created from embedded default
	loadTTSSubstitutions()
	path := filepath.Join(dir, ttsSubstituteFile)
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("substitute file not created: %v", err)
	}
	// Write custom substitutions and reload
	if err := os.WriteFile(path, []byte("foo=bar"), 0o644); err != nil {
		t.Fatal(err)
	}
	loadTTSSubstitutions()
	got := substituteTTS("foo baz foo")
	want := "bar baz bar"
	if got != want {
		t.Fatalf("substituteTTS = %q, want %q", got, want)
	}
}

func TestChatTTSSameSpeakerCondenses(t *testing.T) {
	origGS := gs
	gs.ChatTTS = true
	gs.Mute = false
	blockTTS = false
	defer func() {
		gs = origGS
		setHighQualityResamplingEnabled(gs.HighQualityResampling)
	}()

	stopAllTTS()
	lastTTSSpeaker = ""
	lastTTSTime = time.Time{}

	var mu sync.Mutex
	var outs []string
	origFunc := playChatTTSFunc
	playChatTTSFunc = func(ctx context.Context, text string) {
		mu.Lock()
		outs = append(outs, text)
		mu.Unlock()
	}
	defer func() { playChatTTSFunc = origFunc }()

	speakChatMessage("Alice says, hello")
	time.Sleep(250 * time.Millisecond)
	speakChatMessage("Alice says, how are you?")
	time.Sleep(300 * time.Millisecond)

	mu.Lock()
	got := append([]string(nil), outs...)
	mu.Unlock()
	if len(got) != 2 {
		t.Fatalf("got %d messages, want 2", len(got))
	}
	if got[0] != "Alice says, hello" {
		t.Fatalf("first = %q, want %q", got[0], "Alice says, hello")
	}
	if got[1] != "and then said how are you?" {
		t.Fatalf("second = %q, want %q", got[1], "and then said how are you?")
	}
}
