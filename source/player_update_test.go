package main

import (
	"bytes"
	"testing"
	"time"
)

func TestUpdatePlayerAppearanceUnmarksDead(t *testing.T) {
	players = map[string]*Player{
		"Bob": {
			Name:       "Bob",
			Dead:       true,
			FellWhere:  "somewhere",
			KillerName: "killer",
			FellTime:   time.Unix(1, 0),
		},
	}
	updatePlayerAppearance("Bob", 1, nil, false)
	p := players["Bob"]
	if p.Dead || p.FellWhere != "" || p.KillerName != "" || !p.FellTime.IsZero() {
		t.Fatalf("expected Bob to be marked alive, got %#v", p)
	}
}

func TestUpdatePlayerAppearancePublishesPaletteChangesAndRemoval(t *testing.T) {
	playersMu.Lock()
	origPlayers := players
	players = make(map[string]*Player)
	playersMu.Unlock()
	origPlayerName := playerName
	origCharacters := characters
	origDataDirPath := dataDirPath
	origPlayersDirty := playersDirty
	origPlayersPersistDirty := playersPersistDirty
	playerName = "Bob"
	characters = []Character{{Name: "Bob"}}
	dataDirPath = t.TempDir()
	defer func() {
		playersMu.Lock()
		players = origPlayers
		playersMu.Unlock()
		playerName = origPlayerName
		characters = origCharacters
		dataDirPath = origDataDirPath
		playersDirty = origPlayersDirty
		playersPersistDirty = origPlayersPersistDirty
	}()

	updatePlayerAppearance("Bob", 1, []byte{1, 2, 3}, false)
	first := playerColorsForDescriptor(frameDescriptor{Name: "Bob"})
	playersPersistDirty = false
	updatePlayerAppearance("Bob", 1, []byte{4, 5, 6}, false)
	if !bytes.Equal(first, []byte{1, 2, 3}) {
		t.Fatalf("previously published colors were mutated: %v", first)
	}
	second := playerColorsForDescriptor(frameDescriptor{Name: "Bob"})
	if !bytes.Equal(second, []byte{4, 5, 6}) {
		t.Fatalf("updated colors = %v, want [4 5 6]", second)
	}
	if !playersPersistDirty {
		t.Fatal("palette change did not mark player persistence dirty")
	}

	playersPersistDirty = false
	updatePlayerAppearance("Bob", 1, nil, false)
	if got := playerColorsForDescriptor(frameDescriptor{Name: "Bob"}); len(got) != 0 {
		t.Fatalf("removed clothing retained colors: %v", got)
	}
	if len(characters[0].Colors) != 0 {
		t.Fatalf("saved character retained removed clothing colors: %v", characters[0].Colors)
	}
	if !bytes.Equal(second, []byte{4, 5, 6}) {
		t.Fatalf("removed palette mutated the prior published colors: %v", second)
	}
	if !playersPersistDirty {
		t.Fatal("clothing removal did not mark player persistence dirty")
	}
}

func TestPlayerColorsForDescriptorDoesNotAllocate(t *testing.T) {
	playersMu.Lock()
	origPlayers := players
	players = map[string]*Player{"Bob": {Name: "Bob", Colors: []byte{4, 5, 6}}}
	playersMu.Unlock()
	defer func() {
		playersMu.Lock()
		players = origPlayers
		playersMu.Unlock()
	}()

	d := frameDescriptor{Name: "Bob", Colors: []byte{1, 2, 3}}
	var got []byte
	allocs := testing.AllocsPerRun(1000, func() {
		got = playerColorsForDescriptor(d)
	})
	if allocs != 0 {
		t.Fatalf("playerColorsForDescriptor allocated %.1f times per call", allocs)
	}
	if !bytes.Equal(got, []byte{4, 5, 6}) {
		t.Fatalf("effective colors = %v", got)
	}
}
