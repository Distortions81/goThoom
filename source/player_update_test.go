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
	if !p.LastOnScreen.IsZero() {
		t.Fatal("descriptor-only appearance incorrectly recorded an on-screen timestamp")
	}
}

func TestUnchangedPlayerAppearanceDoesNotDirtyList(t *testing.T) {
	originalPlayers := players
	originalDirty := playersDirty
	originalPlayerName := playerName
	t.Cleanup(func() {
		players = originalPlayers
		playersDirty = originalDirty
		playerName = originalPlayerName
	})
	playerName = ""
	players = map[string]*Player{
		"Bob": {Name: "Bob", PictID: 7, Colors: []byte{1, 2}, Seen: true},
	}
	playersDirty = false
	updatePlayerAppearance("Bob", 7, []byte{1, 2}, false)
	if playersDirty {
		t.Fatal("unchanged descriptor dirtied the Players list")
	}
}

func TestPlayerGroupUsesTwoMinuteOnScreenWindow(t *testing.T) {
	now := time.Unix(1000, 0)
	recent := Player{Name: "Recent", LastOnScreen: now.Add(-recentPlayerWindow + time.Second)}
	if got := playerGroup(recent, now, true); got != playerGroupRecent {
		t.Fatalf("recent player group = %v", got)
	}
	if got := playerGroup(recent, now, false); got != playerGroupOnline {
		t.Fatalf("recent player with section disabled = %v", got)
	}

	expired := Player{Name: "Expired", LastOnScreen: now.Add(-recentPlayerWindow)}
	if got := playerGroup(expired, now, true); got != playerGroupOnline {
		t.Fatalf("expired player group = %v", got)
	}

	offline := recent
	offline.Offline = true
	if got := playerGroup(offline, now, true); got != playerGroupOffline {
		t.Fatalf("offline recent player group = %v", got)
	}
}

func TestMarkPlayersOnScreenTracksCurrentSnellWithoutConstantRefresh(t *testing.T) {
	originalPlayers := players
	originalDirty := playersDirty
	originalMobileSize := mobileSizeFunc
	defer func() {
		players = originalPlayers
		playersDirty = originalDirty
		mobileSizeFunc = originalMobileSize
	}()
	mobileSizeFunc = func(uint16) int { return 20 }

	now := time.Unix(2000, 0)
	players = map[string]*Player{"Bob": {Name: "Bob", Offline: true}}
	mobiles := []frameMobile{{Index: 7, Colors: styleBoldItalic}}
	descriptors := map[uint8]frameDescriptor{7: {Index: 7, Name: "Bob", Colors: []byte{1}}}

	playersDirty = false
	markPlayersOnScreen(mobiles, descriptors, now)
	if players["Bob"].Offline || players["Bob"].LastOnScreen != now || !playersDirty {
		t.Fatalf("first snell observation was not recorded: player=%+v dirty=%v", players["Bob"], playersDirty)
	}
	if !players["Bob"].Sharing || !players["Bob"].SameClan {
		t.Fatalf("mobile style hints did not update Players-list flags: %+v", players["Bob"])
	}

	playersDirty = false
	markPlayersOnScreen(mobiles, descriptors, now.Add(time.Second))
	if playersDirty {
		t.Fatal("continuous presence dirtied the Players list")
	}
	if got := players["Bob"].LastOnScreen; got != now.Add(time.Second) {
		t.Fatalf("continuous presence timestamp = %v", got)
	}
}

func TestPlayerListNameStyleMatchesMobileNameStyle(t *testing.T) {
	p := Player{Sharing: true, SameClan: true, Sharee: true}
	if got, want := playerListNameStyle(p), styleBoldItalic|styleUnderline; got != want {
		t.Fatalf("player-list style = %#x, want %#x", got, want)
	}
}

func TestMobileWithoutColorDescriptorDoesNotClearPlayerListStyle(t *testing.T) {
	originalPlayers := players
	originalDirty := playersDirty
	defer func() {
		players = originalPlayers
		playersDirty = originalDirty
	}()

	players = map[string]*Player{"Bob": {Name: "Bob", Sharing: true, SameClan: true}}
	markPlayersOnScreen(
		[]frameMobile{{Index: 7}},
		map[uint8]frameDescriptor{7: {Index: 7, Name: "Bob"}},
		time.Unix(4000, 0),
	)
	if !players["Bob"].Sharing || !players["Bob"].SameClan {
		t.Fatalf("colorless descriptor cleared classic style flags: %+v", players["Bob"])
	}
}

func TestMarkPlayersOnScreenNeverTreatsLocalMobileAsSharing(t *testing.T) {
	originalPlayers := players
	originalDirty := playersDirty
	originalPlayerName := playerName
	defer func() {
		players = originalPlayers
		playersDirty = originalDirty
		playerName = originalPlayerName
	}()

	playerName = "Warawonda"
	players = map[string]*Player{
		"warawonda": {Name: "warawonda", Sharing: true, Sharee: true},
	}
	markPlayersOnScreen(
		[]frameMobile{{Index: 7, Colors: styleBold}},
		map[uint8]frameDescriptor{7: {Index: 7, Name: "warawonda", Colors: []byte{1}}},
		time.Unix(4500, 0),
	)

	self := players["warawonda"]
	if self.Sharing || self.Sharee {
		t.Fatalf("local player retained a self-sharing relationship: %+v", self)
	}
}

func TestBackendShareRefreshIgnoresLocalPlayer(t *testing.T) {
	originalPlayers := players
	originalDirty := playersDirty
	originalPlayerName := playerName
	defer func() {
		players = originalPlayers
		playersDirty = originalDirty
		playerName = originalPlayerName
	}()

	taggedName := func(name string) []byte {
		data := []byte{0xC2, 'p', 'n'}
		data = append(data, name...)
		return append(data, 0xC2, 'p', 'n')
	}
	playerName = "Warawonda"
	players = map[string]*Player{
		"Warawonda": {Name: "Warawonda"},
	}
	payload := append(taggedName("Warawonda"), '\t')
	payload = append(payload, taggedName("Warawonda")...)
	payload = append(payload, []byte(" and ")...)
	payload = append(payload, taggedName("Bob")...)
	parseBackendShare(payload)

	self := players["Warawonda"]
	if self.Sharing || self.Sharee {
		t.Fatalf("backend refresh created a self-sharing relationship: %+v", self)
	}
	if bob := players["Bob"]; bob == nil || !bob.Sharing {
		t.Fatalf("backend refresh lost real incoming sharer: %+v", bob)
	}
}

func TestMarkPlayersOnScreenExcludesOffscreenAndPersistedMobiles(t *testing.T) {
	originalPlayers := players
	originalDirty := playersDirty
	originalMobileSize := mobileSizeFunc
	defer func() {
		players = originalPlayers
		playersDirty = originalDirty
		mobileSizeFunc = originalMobileSize
	}()
	mobileSizeFunc = func(uint16) int { return 20 }

	now := time.Unix(3000, 0)
	players = map[string]*Player{
		"Visible":   {Name: "Visible"},
		"Offscreen": {Name: "Offscreen"},
		"Persisted": {Name: "Persisted"},
	}
	mobiles := []frameMobile{
		{Index: 1, H: 0, V: 0},
		{Index: 2, H: int16(fieldCenterX + 20), V: 0},
		{Index: 3, H: 0, V: 0, Persist: true},
	}
	descriptors := map[uint8]frameDescriptor{
		1: {Index: 1, Type: kDescPlayer, PictID: 1, Name: "Visible"},
		2: {Index: 2, Type: kDescPlayer, PictID: 1, Name: "Offscreen"},
		3: {Index: 3, Type: kDescPlayer, PictID: 1, Name: "Persisted"},
	}
	markPlayersOnScreen(mobiles, descriptors, now)

	if players["Visible"].LastOnScreen != now {
		t.Fatal("visible mobile was not recorded")
	}
	if !players["Offscreen"].LastOnScreen.IsZero() || !players["Persisted"].LastOnScreen.IsZero() {
		t.Fatalf("non-visible mobiles recorded: offscreen=%v persisted=%v", players["Offscreen"].LastOnScreen, players["Persisted"].LastOnScreen)
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
