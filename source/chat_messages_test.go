package main

import (
	"strings"
	"testing"

	"gothoom/eui"

	"golang.org/x/image/font/gofont/goregular"
)

func TestChatWindowDefersClosedUpdatesUntilOpen(t *testing.T) {
	if err := eui.EnsureFontSource(goregular.TTF); err != nil {
		t.Fatalf("load test font: %v", err)
	}
	originalWindow := chatWin
	originalList := chatList
	chatLog.mu.Lock()
	originalEntries := append([]timedMessage(nil), chatLog.entries...)
	originalMax := chatLog.max
	chatLog.mu.Unlock()
	originalMessagesToConsole := gs.MessagesToConsole
	t.Cleanup(func() {
		if chatWin != nil && chatWin != originalWindow {
			chatWin.RemoveWindow()
		}
		chatWin = originalWindow
		chatList = originalList
		chatLog = messageLog{entries: originalEntries, max: originalMax}
		gs.MessagesToConsole = originalMessagesToConsole
	})

	chatWin = nil
	chatList = nil
	chatLog = messageLog{max: maxChatMessages}
	gs.MessagesToConsole = false
	if err := makeChatWindow(); err != nil {
		t.Fatalf("make chat window: %v", err)
	}
	chatLog.Add("Bob says, hello")
	updateChatWindow()
	if len(chatList.Contents) != 0 {
		t.Fatal("closed Chat window rebuilt its message rows")
	}
	chatWin.MarkOpen()
	if len(chatList.Contents) != 1 || !strings.Contains(chatList.Contents[0].Text, "Bob says, hello") {
		t.Fatalf("Chat contents after opening = %#v", chatList.Contents)
	}
}

func TestIsSelfChatMessage(t *testing.T) {
	playerName = "Hero"
	cases := []struct {
		msg  string
		want bool
	}{
		{"Hero says, hello there", true},
		{"(Hero waves)", true},
		{"Hero yells, hey!", true},
		{"Bob says, hi", false},
		{"You are sharing experiences with Bob.", false},
		{"Hero has fallen", false},
	}
	for _, c := range cases {
		if got := isSelfChatMessage(c.msg); got != c.want {
			t.Errorf("isSelfChatMessage(%q) = %v; want %v", c.msg, got, c.want)
		}
	}
}

func TestChatMessageBlocked(t *testing.T) {
	players = make(map[string]*Player)
	chatLog = messageLog{max: maxChatMessages}
	p := getPlayer("Bob")
	playersMu.Lock()
	p.Blocked = true
	playersMu.Unlock()
	chatMessage("Bob says, hi")
	if len(getChatMessages()) != 0 {
		t.Fatalf("expected no messages")
	}
}

func TestChatMessageIgnored(t *testing.T) {
	players = make(map[string]*Player)
	chatLog = messageLog{max: maxChatMessages}
	p := getPlayer("Bob")
	playersMu.Lock()
	p.Ignored = true
	playersMu.Unlock()
	chatMessage("Bob says, hi")
	if len(getChatMessages()) != 0 {
		t.Fatalf("expected no messages")
	}
}

func TestChatSpeakerNPCWithDescriptor(t *testing.T) {
	cases := []struct {
		msg  string
		want string
	}{
		{"(Town Crier) says, hello", "Town Crier"},
		{"(Boat Seller) yells, boats", "Boat Seller"},
		{"Goblin says, hi", "Goblin"},
		{"Captain Barnac says Ah, Malcom.", "Captain Barnac"},
		{"High Priestess Aria whispers, hush.", "High Priestess Aria"},
	}
	for _, c := range cases {
		if got := chatSpeaker(c.msg); got != c.want {
			t.Fatalf("chatSpeaker(%q) = %q; want %q", c.msg, got, c.want)
		}
	}
}
