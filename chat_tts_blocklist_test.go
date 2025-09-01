package main

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestNoTTSBlocklist(t *testing.T) {
	origGS := gs
	gs.ChatTTS = true
	gs.Mute = false
	blockTTS = false
	gs.ChatTTSBlocklist = []string{"blocked"}
	syncTTSBlocklist()
	defer func() { gs = origGS; syncTTSBlocklist() }()

	stopAllTTS()

	var mu sync.Mutex
	var got []string
	origFunc := playChatTTSFunc
	playChatTTSFunc = func(ctx context.Context, text string) {
		mu.Lock()
		got = append(got, text)
		mu.Unlock()
	}
	defer func() { playChatTTSFunc = origFunc }()

	chatMessage("Blocked hello")
	chatMessage("Allowed hi")
	time.Sleep(500 * time.Millisecond)

	mu.Lock()
	msgs := append([]string(nil), got...)
	mu.Unlock()
	if len(msgs) != 1 || msgs[0] != "Allowed hi" {
		t.Fatalf("got %v", msgs)
	}
}

func TestHandleNoTTSCommand(t *testing.T) {
	orig := gs.ChatTTSBlocklist
	gs.ChatTTSBlocklist = []string{}
	syncTTSBlocklist()
	handleNoTTSCommand("add foo")
	if !isTTSBlocked("foo") {
		t.Fatalf("foo not added to blocklist")
	}
	handleNoTTSCommand("remove foo")
	if isTTSBlocked("foo") {
		t.Fatalf("foo not removed from blocklist")
	}
	gs.ChatTTSBlocklist = orig
	syncTTSBlocklist()
}

func TestHandleNoTTSList(t *testing.T) {
	origList := gs.ChatTTSBlocklist
	gs.ChatTTSBlocklist = []string{"foo", "bar"}
	syncTTSBlocklist()
	origEntries, origMax := consoleLog.entries, consoleLog.max
	consoleLog.entries = nil
	consoleLog.max = maxMessages
	handleNoTTSCommand("list")
	msgs := getConsoleMessages()
	if len(msgs) != 1 || msgs[0] != "TTS blocklist: foo, bar" {
		t.Fatalf("got %v", msgs)
	}
	consoleLog.entries = origEntries
	consoleLog.max = origMax
	gs.ChatTTSBlocklist = origList
	syncTTSBlocklist()
}
