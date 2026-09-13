package main

import (
	"context"
	"net"
	"testing"
)

func TestSessionManagerStartsWithOnlyPrimarySession(t *testing.T) {
	primary := mustNewSession(primarySessionID)
	manager := newSessionManager(primary)

	selected := manager.selectedSession()
	if selected != primary {
		t.Fatalf("selected session = %p, want primary %p", selected, primary)
	}
	if _, ok := manager.session(primarySessionID); !ok {
		t.Fatal("primary session is missing")
	}
	if _, ok := manager.session(2); ok {
		t.Fatal("secondary session exists before multi-session is enabled")
	}
	if manager.selectSession(2) {
		t.Fatal("selected an inactive session slot")
	}
}

func TestSessionManagerEnablesFourStableSlots(t *testing.T) {
	primary := mustNewSession(primarySessionID)
	manager := newSessionManager(primary)
	slots := manager.enableMulti()

	if slots[0] != primary {
		t.Fatal("enabling multi-session replaced the primary session")
	}
	for slot, session := range slots {
		if session == nil {
			t.Fatalf("slot %d was not created", slot)
		}
		id, _ := sessionIDForSlot(slot)
		if session.ID() != id {
			t.Fatalf("slot %d has session ID %d, want %d", slot, session.ID(), id)
		}
	}

	if !manager.selectSession(3) || manager.selectedSession() != slots[2] {
		t.Fatal("selection did not switch to session three")
	}
	again := manager.enableMulti()
	for slot := range slots {
		if again[slot] != slots[slot] {
			t.Fatalf("enabling multi-session twice replaced slot %d", slot)
		}
	}
}

func TestSessionManagerDisconnectAllTargetsEveryActiveSlot(t *testing.T) {
	primary := mustNewSession(primarySessionID)
	manager := newSessionManager(primary)
	slots := manager.enableMulti()
	var peers []net.Conn
	for _, session := range slots[:2] {
		tcp, tcpPeer := net.Pipe()
		udp, udpPeer := net.Pipe()
		peers = append(peers, tcpPeer, udpPeer)
		if _, ok := session.transport.attach(tcp, udp); !ok {
			t.Fatal("could not attach test transport")
		}
	}
	t.Cleanup(func() {
		for _, peer := range peers {
			_ = peer.Close()
		}
	})

	if got := manager.disconnectAll(); got != 2 {
		t.Fatalf("disconnect count = %d, want 2", got)
	}
	for index, session := range slots[:2] {
		if session.transport.busy() {
			t.Fatalf("slot %d transport is still busy", index+1)
		}
	}
}

func TestSessionManagerRejectsInvalidLoginBeforeStarting(t *testing.T) {
	manager := newSessionManager(mustNewSession(primarySessionID))
	if _, err := manager.startLogin(context.Background(), 1, sessionLoginRequest{}, 1); err == nil {
		t.Fatal("empty login request started")
	}
	if manager.anyBusy() {
		t.Fatal("invalid login left the session busy")
	}
}

func TestSessionTransportCannotReconnectUntilDisconnectJoins(t *testing.T) {
	session := mustNewSession(2)
	if !session.transport.begin(func() {}) {
		t.Fatal("could not begin transport")
	}
	session.transport.mu.RLock()
	generation := session.transport.generation
	done := session.transport.done
	session.transport.mu.RUnlock()
	if !session.transport.disconnect() || !session.transport.busy() {
		t.Fatal("disconnect did not retain teardown ownership")
	}
	if session.transport.begin(func() {}) {
		t.Fatal("reconnected before old transport teardown completed")
	}
	if !session.transport.finish(generation) {
		t.Fatal("old transport could not finish after disconnect")
	}
	select {
	case <-done:
	default:
		t.Fatal("transport completion was not closed")
	}
	if session.transport.busy() || !session.transport.begin(func() {}) {
		t.Fatal("transport was not reusable after teardown")
	}
	session.transport.failConnect()
}
