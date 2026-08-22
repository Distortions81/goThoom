package main

import "testing"

// helper to build BEPP line with specified prefix
func presenceLine(prefix string, msg []byte) []byte {
	b := []byte{0xC2}
	b = append(b, prefix[0], prefix[1])
	b = append(b, msg...)
	b = append(b, 0)
	return b
}

func TestDecodeLoginBEPP(t *testing.T) {
	players = make(map[string]*Player)
	players["Bob"] = &Player{Name: "Bob", Offline: true}
	msg := append(pnTag("Bob"), []byte(" has logged on")...)
	raw := presenceLine("lg", msg)
	if got := decodeBEPP(raw); got != "Bob has logged on" {
		t.Fatalf("decodeBEPP returned %q", got)
	}
	playersMu.RLock()
	offline := players["Bob"].Offline
	playersMu.RUnlock()
	if offline {
		t.Errorf("player still offline")
	}
}

func TestDecodeLogoutBEPP(t *testing.T) {
	players = make(map[string]*Player)
	players["Bob"] = &Player{Name: "Bob", Offline: false}
	msg := append(pnTag("Bob"), []byte(" has left the lands")...)
	raw := presenceLine("lf", msg)
	if got := decodeBEPP(raw); got != "Bob has left the lands" {
		t.Fatalf("decodeBEPP returned %q", got)
	}
	playersMu.RLock()
	offline := players["Bob"].Offline
	playersMu.RUnlock()
	if !offline {
		t.Errorf("player not marked offline")
	}
}
