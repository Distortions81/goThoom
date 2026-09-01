package main

import (
	"testing"
	"time"
)

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

func TestBeWhoScanKeepsExistingPresenceUntilCompletion(t *testing.T) {
	originalWhoActive := whoActive
	originalWhoScanStarted := whoScanStarted
	originalPlayersDirty := playersDirty
	t.Cleanup(func() {
		whoActive = originalWhoActive
		whoScanStarted = originalWhoScanStarted
		playersDirty = originalPlayersDirty
	})

	oldSeen := time.Now().Add(-time.Minute)
	players = map[string]*Player{
		"Alice": {Name: "Alice", beWho: true, LastSeen: oldSeen},
		"Bob":   {Name: "Bob", beWho: true, Offline: true},
	}
	beginBeWhoScan()
	if players["Alice"].Offline {
		t.Fatal("online player became offline while /be-who pages were pending")
	}
	for name, p := range players {
		if p.beWho {
			t.Errorf("%s retained stale staged /be-who membership", name)
		}
	}

	considerNextWhoBatch(20)
	if players["Alice"].Offline || !whoActive {
		t.Fatal("full /be-who page committed presence before pagination completed")
	}
	considerNextWhoBatch(0)
	if !players["Alice"].Offline || whoActive {
		t.Fatal("final /be-who page did not commit an omitted player offline")
	}
}

func TestBeWhoScanPreservesNewerPresence(t *testing.T) {
	originalWhoScanStarted := whoScanStarted
	t.Cleanup(func() { whoScanStarted = originalWhoScanStarted })

	players = map[string]*Player{"Alice": {Name: "Alice", LastSeen: time.Now().Add(-time.Minute)}}
	beginBeWhoScan()
	players["Alice"].LastSeen = whoScanStarted.Add(time.Millisecond)
	finishBeWhoScan()
	if players["Alice"].Offline {
		t.Fatal("activity observed during /be-who scan was overwritten at completion")
	}
}

func TestUnchangedBeWhoScanDoesNotDirtyPlayers(t *testing.T) {
	originalPlayers := players
	originalWhoActive := whoActive
	originalWhoScanStarted := whoScanStarted
	originalPlayersDirty := playersDirty
	originalPlayersPersistDirty := playersPersistDirty
	t.Cleanup(func() {
		players = originalPlayers
		whoActive = originalWhoActive
		whoScanStarted = originalWhoScanStarted
		playersDirty = originalPlayersDirty
		playersPersistDirty = originalPlayersPersistDirty
	})

	players = map[string]*Player{
		"Alice": {
			Name:     "Alice",
			Race:     "Human",
			Gender:   "Female",
			Class:    "Fighter",
			clan:     "No Clan",
			Seen:     true,
			beWho:    true,
			Offline:  false,
			LastSeen: time.Now().Add(-time.Minute),
		},
	}
	whoActive = false
	playersDirty = false
	playersPersistDirty = false
	payload := append(pnTag("Alice"), []byte(",Alice,0\t")...)

	parseBackendWho(payload)

	if playersDirty {
		t.Fatal("unchanged /be-who result dirtied the Players window")
	}
	if playersPersistDirty {
		t.Fatal("unchanged /be-who result dirtied player persistence")
	}
}

func TestBeWhoOnlineTransitionDirtiesPlayers(t *testing.T) {
	originalPlayers := players
	originalWhoActive := whoActive
	originalWhoScanStarted := whoScanStarted
	originalPlayersDirty := playersDirty
	originalPlayersPersistDirty := playersPersistDirty
	t.Cleanup(func() {
		players = originalPlayers
		whoActive = originalWhoActive
		whoScanStarted = originalWhoScanStarted
		playersDirty = originalPlayersDirty
		playersPersistDirty = originalPlayersPersistDirty
	})

	players = map[string]*Player{
		"Alice": {
			Name:    "Alice",
			Race:    "Human",
			Gender:  "Female",
			Class:   "Fighter",
			clan:    "No Clan",
			Seen:    true,
			beWho:   true,
			Offline: true,
		},
	}
	whoActive = false
	playersDirty = false
	playersPersistDirty = false
	payload := append(pnTag("Alice"), []byte(",Alice,0\t")...)

	parseBackendWho(payload)

	if !playersDirty {
		t.Fatal("online transition did not dirty the Players window")
	}
	if !playersPersistDirty {
		t.Fatal("online transition did not dirty player persistence")
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
