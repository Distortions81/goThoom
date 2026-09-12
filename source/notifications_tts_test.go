package main

import (
	"context"
	"testing"
	"time"

	"gothoom/eui"
)

func TestNotificationTTSOption(t *testing.T) {
	initFont()
	originalSettings := gs
	originalWindow := gameWin
	originalNotifications := notifications
	originalFocusMuted, originalBlockTTS := focusMuted, blockTTS
	originalSpeaker, originalSpeakerTime := lastTTSSpeaker, lastTTSTime
	originalFunc := playChatTTSFunc
	gs = gsdef
	gs.Notifications = true
	gs.NotificationBeep = false
	gs.ChatTTS = true
	gs.Mute = false
	focusMuted = false
	blockTTS = false
	gameWin = eui.NewWindow()
	notifications = nil
	lastTTSSpeaker, lastTTSTime = "Hero", time.Now()
	stopAllTTS()
	spoken := make(chan string, 1)
	playChatTTSFunc = func(_ context.Context, text string) { spoken <- text }
	t.Cleanup(func() {
		stopAllTTS()
		playChatTTSFunc = originalFunc
		gs = originalSettings
		gameWin = originalWindow
		notifications = originalNotifications
		focusMuted, blockTTS = originalFocusMuted, originalBlockTTS
		lastTTSSpeaker, lastTTSTime = originalSpeaker, originalSpeakerTime
	})

	showNotification("Hero is online")
	select {
	case got := <-spoken:
		t.Fatalf("notification TTS spoke while disabled: %q", got)
	case <-time.After(350 * time.Millisecond):
	}

	gs.ChatTTSNotifications = true
	showNotification("Hero is online")
	select {
	case got := <-spoken:
		if got != "Hero is online" {
			t.Fatalf("notification TTS spoke %q, want uncondensed notification text", got)
		}
	case <-time.After(time.Second):
		t.Fatal("enabled notification TTS did not speak")
	}
}
