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

func TestChatTTSMessageTypeOptions(t *testing.T) {
	original := gs
	t.Cleanup(func() { gs = original })
	gs.ChatTTSSay = false
	gs.ChatTTSWhisper = false
	gs.ChatTTSYell = false
	gs.ChatTTSThink = false
	gs.ChatTTSAction = false
	gs.ChatTTSPonder = false
	gs.ChatTTSMonster = false

	tests := []struct {
		messageType string
		enable      func()
	}{
		{messageTextTypeSay, func() { gs.ChatTTSSay = true }},
		{messageTextTypeWhisper, func() { gs.ChatTTSWhisper = true }},
		{messageTextTypeYell, func() { gs.ChatTTSYell = true }},
		{messageTextTypeThink, func() { gs.ChatTTSThink = true }},
		{messageTextTypeAction, func() { gs.ChatTTSAction = true }},
		{messageTextTypePonder, func() { gs.ChatTTSPonder = true }},
		{messageTextTypeMonster, func() { gs.ChatTTSMonster = true }},
	}
	for _, test := range tests {
		if chatTTSMessageTypeEnabled(test.messageType) {
			t.Errorf("%s enabled before its option", test.messageType)
		}
		test.enable()
		if !chatTTSMessageTypeEnabled(test.messageType) {
			t.Errorf("%s remains disabled after its option", test.messageType)
		}
	}
	if !chatTTSMessageTypeEnabled(messageTextTypeSystem) {
		t.Fatal("unclassified direct chat messages lost compatibility TTS")
	}
}

func TestChatTTSMessageDefaults(t *testing.T) {
	if !gsdef.ChatTTSSay || !gsdef.ChatTTSWhisper || !gsdef.ChatTTSYell ||
		!gsdef.ChatTTSThink || !gsdef.ChatTTSAction || !gsdef.ChatTTSPonder ||
		!gsdef.ChatTTSMonster || gsdef.ChatTTSSelf || gsdef.ChatTTSNotifications {
		t.Fatal("incoming chat TTS should default on while own messages and notifications default off")
	}
}

func TestCombinedChatRoutesThroughTTSOptions(t *testing.T) {
	originalSettings := gs
	originalFocusMuted, originalBlockTTS := focusMuted, blockTTS
	originalPlayerName := playerName
	originalSpeaker, originalSpeakerTime := lastTTSSpeaker, lastTTSTime
	originalFunc := playChatTTSFunc
	gs = gsdef
	gs.MessagesToConsole = true
	gs.ChatTTS = true
	gs.Mute = false
	focusMuted = false
	blockTTS = false
	playerName = "Hero"
	lastTTSSpeaker, lastTTSTime = "", time.Time{}
	stopAllTTS()
	spoken := make(chan string, 2)
	playChatTTSFunc = func(_ context.Context, text string) { spoken <- text }
	t.Cleanup(func() {
		stopAllTTS()
		playChatTTSFunc = originalFunc
		gs = originalSettings
		focusMuted, blockTTS = originalFocusMuted, originalBlockTTS
		playerName = originalPlayerName
		lastTTSSpeaker, lastTTSTime = originalSpeaker, originalSpeakerTime
	})

	displayChatMessageTyped("Alice says, hello", messageTextTypeSay)
	select {
	case got := <-spoken:
		if got != "Alice says, hello" {
			t.Fatalf("combined chat spoke %q", got)
		}
	case <-time.After(time.Second):
		t.Fatal("combined chat did not reach TTS")
	}

	gs.ChatTTSSay = false
	displayChatMessageTyped("Alice says, this should stay quiet", messageTextTypeSay)
	select {
	case got := <-spoken:
		t.Fatalf("disabled speech option spoke %q", got)
	case <-time.After(350 * time.Millisecond):
	}
}

func TestChatTTSSelfOption(t *testing.T) {
	originalSettings, originalPlayerName := gs, playerName
	t.Cleanup(func() { gs, playerName = originalSettings, originalPlayerName })
	gs = gsdef
	gs.ChatTTS = true
	playerName = "Hero"

	if gs.ChatTTSSelf {
		t.Fatal("own-message TTS should default off")
	}
	selfMessages := []struct {
		message     string
		messageType string
	}{
		{"Hero says, hello", messageTextTypeSay},
		{"Hero thinks, hmm", messageTextTypeThink},
		{"Hero ponders, perhaps", messageTextTypePonder},
		{"(Hero waves)", messageTextTypeAction},
	}
	for _, selfMessage := range selfMessages {
		if chatTTSMessageSelected(selfMessage.message, selfMessage.messageType) {
			t.Fatalf("own message %q selected while own-message TTS is disabled", selfMessage.message)
		}
	}
	gs.ChatTTSSelf = true
	for _, selfMessage := range selfMessages {
		if !chatTTSMessageSelected(selfMessage.message, selfMessage.messageType) {
			t.Fatalf("own message %q remains unselected after enabling own-message TTS", selfMessage.message)
		}
	}
}
