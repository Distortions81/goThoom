package main

import "testing"

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
