package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
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
	defer func() { gs = origGS }()

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
	defer func() { gs = origGS }()

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

	if n := atomic.LoadInt32(&pendingTTS); n != 0 {
		t.Fatalf("pendingTTS = %d, want 0", n)
	}
}
