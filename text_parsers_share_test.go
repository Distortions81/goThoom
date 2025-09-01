//go:build integration
// +build integration

package main

import "testing"

func TestParseShareTextHeroShareUnshareOthers(t *testing.T) {
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

	// Hero unshares Bob
	unshareRaw := append(pn("Hero"), []byte(" is no longer sharing experiences with ")...)
	unshareRaw = append(unshareRaw, pn("Bob")...)
	unshareRaw = append(unshareRaw, '.')
	parseShareText(unshareRaw, "Hero is no longer sharing experiences with Bob.")
	if p, ok := players["Bob"]; ok && p.Sharee {
		t.Fatalf("Bob still marked sharee after unshare: %+v", p)
	}
}
