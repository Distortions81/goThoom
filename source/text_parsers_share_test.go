//go:build integration
// +build integration

package main

import "testing"

func TestParseShareTextHeroShareUnshareOthers(t *testing.T) {
	oldName, oldPlayers := playerName, players
	oldDirty, oldPersistDirty := playersDirty, playersPersistDirty
	t.Cleanup(func() {
		playerName, players = oldName, oldPlayers
		playersDirty, playersPersistDirty = oldDirty, oldPersistDirty
	})
	playerName = "Hero"
	players = make(map[string]*Player)

	// Hero shares Bob
	shareRaw := append(pn("Hero"), []byte(" is sharing experiences with ")...)
	shareRaw = append(shareRaw, pn("Bob")...)
	shareRaw = append(shareRaw, '.')
	parseShareText(shareRaw, "Hero is sharing experiences with Bob.")
	if p, ok := players["Bob"]; !ok || !p.Sharee {
		t.Fatalf("Bob not marked sharee after share: %+v", p)
	}

	// Adding Carol keeps Bob in the recipient list.
	addRaw := append(pn("Hero"), []byte(" begins sharing experiences with ")...)
	addRaw = append(addRaw, pn("Carol")...)
	addRaw = append(addRaw, '.')
	parseShareText(addRaw, "Hero begins sharing experiences with Carol.")
	if !players["Bob"].Sharee || !players["Carol"].Sharee {
		t.Fatal("adding Carol cleared an existing share recipient")
	}

	// Hero unshares Bob
	unshareRaw := append(pn("Hero"), []byte(" is no longer sharing experiences with ")...)
	unshareRaw = append(unshareRaw, pn("Bob")...)
	unshareRaw = append(unshareRaw, '.')
	parseShareText(unshareRaw, "Hero is no longer sharing experiences with Bob.")
	if p, ok := players["Bob"]; ok && p.Sharee {
		t.Fatalf("Bob still marked sharee after unshare: %+v", p)
	}
	if !players["Carol"].Sharee {
		t.Fatal("unsharing Bob also cleared Carol")
	}

	// A full list still replaces existing recipients.
	parseShareText(shareRaw, "Hero is sharing experiences with Bob.")
	if !players["Bob"].Sharee || players["Carol"].Sharee {
		t.Fatal("full share list did not replace existing recipients")
	}
}
