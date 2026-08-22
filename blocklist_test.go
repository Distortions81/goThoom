package main

import "testing"

func TestHandleBlockCommandToggle(t *testing.T) {
	withoutConsoleTimestamps(t)
	players = make(map[string]*Player)
	consoleLog = messageLog{max: maxMessages}
	handleBlockCommand("Bob")
	p := getPlayer("Bob")
	if !p.Blocked || p.Ignored || p.Friend || p.FriendLabel != 6 {
		t.Fatalf("expected Bob to be blocked only with label 6")
	}
	msgs := getConsoleMessages()
	if len(msgs) == 0 || msgs[len(msgs)-1] != "Blocking Bob." {
		t.Fatalf("unexpected message: %v", msgs)
	}
	handleBlockCommand("Bob")
	if p.Blocked || p.FriendLabel != 0 {
		t.Fatalf("expected Bob to be unblocked")
	}
	msgs = getConsoleMessages()
	if msgs[len(msgs)-1] != "No longer blocking Bob." {
		t.Fatalf("unexpected message: %v", msgs[len(msgs)-1])
	}
}

func TestHandleIgnoreCommandToggle(t *testing.T) {
	withoutConsoleTimestamps(t)
	players = make(map[string]*Player)
	consoleLog = messageLog{max: maxMessages}
	handleIgnoreCommand("Bob")
	p := getPlayer("Bob")
	if !p.Ignored || p.Blocked || p.Friend || p.FriendLabel != 7 {
		t.Fatalf("expected Bob to be ignored only with label 7")
	}
	msgs := getConsoleMessages()
	if len(msgs) == 0 || msgs[len(msgs)-1] != "Ignoring Bob." {
		t.Fatalf("unexpected message: %v", msgs)
	}
	handleIgnoreCommand("Bob")
	if p.Ignored || p.FriendLabel != 0 {
		t.Fatalf("expected Bob to be unignored")
	}
	msgs = getConsoleMessages()
	if msgs[len(msgs)-1] != "No longer ignoring Bob." {
		t.Fatalf("unexpected message: %v", msgs[len(msgs)-1])
	}
}

func TestHandleForgetCommand(t *testing.T) {
	withoutConsoleTimestamps(t)
	players = make(map[string]*Player)
	consoleLog = messageLog{max: maxMessages}
	p := getPlayer("Bob")
	p.GlobalLabel = 1
	applyPlayerLabel(p)
	handleForgetCommand("Bob")
	if p.Blocked || p.Ignored || p.Friend || p.FriendLabel != 0 {
		t.Fatalf("expected Bob to have no labels")
	}
	msgs := getConsoleMessages()
	if len(msgs) == 0 || msgs[len(msgs)-1] != "Removing label from Bob." {
		t.Fatalf("unexpected message: %v", msgs)
	}
}
