package main

import (
	"testing"
	"time"

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

func TestVisiblePlayerArtworkLoadsOnlyViewportRows(t *testing.T) {
	originalWindow := playersWin
	originalList := playersList
	originalRows := cachedPlayerRows
	originalRefs := playersRowRefs
	originalViewport := playerArtworkViewport
	t.Cleanup(func() {
		if playersWin != nil && playersWin != originalWindow {
			playersWin.RemoveWindow()
		}
		playersWin = originalWindow
		playersList = originalList
		cachedPlayerRows = originalRows
		playersRowRefs = originalRefs
		playerArtworkViewport = originalViewport
	})

	playersWin = eui.NewWindow()
	playersList = &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL, Fixed: true, Scrollable: true}
	playersList.Size = eui.Point{X: 100, Y: 20}
	playersWin.AddItem(playersList)
	playersWin.MarkOpen()
	cachedPlayerRows = make(map[string]cachedPlayerRow)
	playersRowRefs = make(map[*eui.ItemData]string)
	rows := make([]*eui.ItemData, 10)
	for i := range rows {
		name := string(rune('A' + i))
		row := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_HORIZONTAL, Fixed: true}
		row.Size = eui.Point{X: 100, Y: 10}
		rows[i] = row
		playersRowRefs[row] = name
		cachedPlayerRows[name] = cachedPlayerRow{
			row:        row,
			profession: eui.NewImageReferenceItem(10, 10),
			avatar:     eui.NewImageReferenceItem(10, 10),
			signature:  playerRowSignature{name: name, rowUnits: 10},
		}
	}
	playersList.SetItems(rows)
	playerArtworkViewport.valid = false
	loadVisiblePlayerArtwork(false)
	if !cachedPlayerRows["A"].artworkLoaded {
		t.Fatal("first visible player row was not materialized")
	}
	if cachedPlayerRows["J"].artworkLoaded {
		t.Fatal("off-screen player row was materialized")
	}

	playersList.Scroll.Y = 80
	loadVisiblePlayerArtwork(false)
	if !cachedPlayerRows["J"].artworkLoaded {
		t.Fatal("newly visible player row was not materialized after scrolling")
	}
}

func TestRecentPlayerExpiryCheckIsThrottled(t *testing.T) {
	original := lastRecentPlayerExpiryCheck
	t.Cleanup(func() { lastRecentPlayerExpiryCheck = original })
	lastRecentPlayerExpiryCheck = time.Time{}
	now := time.Unix(5000, 0)
	if !shouldCheckRecentPlayerExpiry(now) {
		t.Fatal("initial recent-player expiry check was skipped")
	}
	if shouldCheckRecentPlayerExpiry(now.Add(9 * time.Second)) {
		t.Fatal("recent-player expiry check ran before ten seconds")
	}
	if !shouldCheckRecentPlayerExpiry(now.Add(10 * time.Second)) {
		t.Fatal("recent-player expiry check did not run at ten seconds")
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
