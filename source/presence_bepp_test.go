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

func TestDecodeLoginBEPPAddsOnlinePlayer(t *testing.T) {
	players = make(map[string]*Player)
	msg := append(pnTag("Newcomer"), []byte(" has logged on")...)
	decodeBEPP(presenceLine("lg", msg))
	p, ok := players["Newcomer"]
	if !ok || p.Offline {
		t.Fatal("explicit login did not add an online player")
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

func TestBeginBeWhoScanMarksExistingPlayersOffline(t *testing.T) {
	players = map[string]*Player{
		"Alice": {Name: "Alice", beWho: true},
		"Bob":   {Name: "Bob", beWho: true, Offline: true},
	}
	beginBeWhoScan()
	for name, p := range players {
		if !p.Offline {
			t.Errorf("%s remained online at start of /be-who scan", name)
		}
		if p.beWho {
			t.Errorf("%s retained stale /be-who membership", name)
		}
	}
}

func TestBackendInfoDoesNotChangeOfflinePresence(t *testing.T) {
	players = map[string]*Player{"Bob": {Name: "Bob", Offline: true}}
	payload := append(pnTag("Bob"), []byte("\tHuman\tMale\tFighter\tNo Clan")...)
	parseBackendInfo(payload)
	if !players["Bob"].Offline {
		t.Fatal("/be-info metadata incorrectly marked Bob online")
	}
}
