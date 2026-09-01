package main

import (
	"testing"

	"gothoom/eui"

	"golang.org/x/image/font/gofont/goregular"
)

func TestPlayersWindowDefersClosedUpdatesAndReusesRows(t *testing.T) {
	if err := eui.EnsureFontSource(goregular.TTF); err != nil {
		t.Fatalf("load test font: %v", err)
	}

	originalPlayers := players
	originalWindow := playersWin
	originalList := playersList
	originalDirty := playersDirty
	originalRows := cachedPlayerRows
	originalHeaders := cachedPlayerHeaders
	originalRowRefs := playersRowRefs
	originalGroupHeaders := playersGroupHeaders
	originalPlayerName := playerName
	originalShareIcons := gs.PlayerShareIcons
	t.Cleanup(func() {
		if playersWin != nil && playersWin != originalWindow {
			playersWin.RemoveWindow()
		}
		players = originalPlayers
		playersWin = originalWindow
		playersList = originalList
		playersDirty = originalDirty
		cachedPlayerRows = originalRows
		cachedPlayerHeaders = originalHeaders
		playersRowRefs = originalRowRefs
		playersGroupHeaders = originalGroupHeaders
		playerName = originalPlayerName
		gs.PlayerShareIcons = originalShareIcons
	})

	players = map[string]*Player{
		"Bob": {Name: "Bob", Class: "Fighter", Offline: false},
	}
	playersWin = nil
	playersList = nil
	playerName = "Hero"
	gs.PlayerShareIcons = false
	makePlayersWindow()

	if len(playersList.Contents) != 0 {
		t.Fatalf("closed Players window built %d items", len(playersList.Contents))
	}
	playersWin.MarkOpen()
	bob := playerWindowTestRow(t, "Bob")
	updatePlayersWindow()
	if got := playerWindowTestRow(t, "Bob"); got != bob {
		t.Fatal("unchanged player row was not reused")
	}

	playersWin.Close()
	playersMu.Lock()
	players["Alice"] = &Player{Name: "Alice", Class: "Healer"}
	playersMu.Unlock()
	before := len(playersList.Contents)
	updatePlayersWindow()
	if len(playersList.Contents) != before {
		t.Fatal("closed Players window contents changed")
	}
	playersWin.MarkOpen()
	_ = playerWindowTestRow(t, "Alice")
	if got := playerWindowTestRow(t, "Bob"); got != bob {
		t.Fatal("unchanged player row was not reused when stale window reopened")
	}
	playersMu.Lock()
	players["Bob"].Offline = true
	playersMu.Unlock()
	updatePlayersWindow()
	if got := playerWindowTestRow(t, "Bob"); got == bob {
		t.Fatal("changed player incorrectly reused its old row")
	}
}

func playerWindowTestRow(t *testing.T, name string) *eui.ItemData {
	t.Helper()
	for row, rowName := range playersRowRefs {
		if rowName == name {
			return row
		}
	}
	t.Fatalf("player row %q not found", name)
	return nil
}
