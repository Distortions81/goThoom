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
	playersRowRefs = make(map[*eui.ItemData]playerRef)
	rows := make([]*eui.ItemData, 10)
	for i := range rows {
		name := string(rune('A' + i))
		row := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_HORIZONTAL, Fixed: true}
		row.Size = eui.Point{X: 100, Y: 10}
		rows[i] = row
		playersRowRefs[row] = playerRef{session: primarySessionID, name: name}
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

func TestPlayersWindowBindsRowsAndLabelsToSelectedSession(t *testing.T) {
	if err := eui.EnsureFontSource(goregular.TTF); err != nil {
		t.Fatalf("load test font: %v", err)
	}

	originalSessions := appSessions
	originalPlayers := players
	originalCharacters := characters
	originalWindow, originalList := playersWin, playersList
	originalRows, originalHeaders := cachedPlayerRows, cachedPlayerHeaders
	originalRefs, originalGroupHeaders := playersRowRefs, playersGroupHeaders
	originalRenderSession := playersRenderSession
	originalSelected := selectedPlayerName
	originalLastName, originalLastSession, originalLastTime := lastPlayerClickName, lastPlayerClickSession, lastPlayerClickTime
	t.Cleanup(func() {
		if playersWin != nil && playersWin != originalWindow {
			playersWin.RemoveWindow()
		}
		appSessions = originalSessions
		players = originalPlayers
		characters = originalCharacters
		playersWin, playersList = originalWindow, originalList
		cachedPlayerRows, cachedPlayerHeaders = originalRows, originalHeaders
		playersRowRefs, playersGroupHeaders = originalRefs, originalGroupHeaders
		playersRenderSession = originalRenderSession
		selectedPlayerName = originalSelected
		lastPlayerClickName, lastPlayerClickSession, lastPlayerClickTime = originalLastName, originalLastSession, originalLastTime
	})

	manager := newSessionManager(primarySession)
	slots := manager.enableMulti()
	secondary := slots[1]
	secondary.setCharacterName("Second Hero")
	secondary.players.observeAppearance("Alice", 101, nil, false)
	secondary.players.observeAppearance("Bob", 102, nil, false)
	players = map[string]*Player{
		"Alice": {Name: "Alice", Race: "Human", GlobalLabel: 2, Offline: false},
	}
	characters = []Character{{Name: "Second Hero", Labels: map[string]int{"Bob": 4}}}
	appSessions = manager
	lastPlayerClickName, lastPlayerClickSession, lastPlayerClickTime = "", 0, time.Time{}
	if !manager.selectSession(secondary.ID()) {
		t.Fatal("select secondary session")
	}

	playersWin, playersList = nil, nil
	cachedPlayerRows = map[string]cachedPlayerRow{}
	cachedPlayerHeaders = map[string]cachedPlayerHeader{}
	makePlayersWindow()
	playersWin.MarkOpen()
	updatePlayersWindow()

	alice, ok := playerSnapshotForSession(secondary, "Alice")
	if !ok || alice.Race != "Human" || alice.FriendLabel != 2 {
		t.Fatalf("secondary global profile = %+v, %v", alice, ok)
	}
	bob, ok := playerSnapshotForSession(secondary, "Bob")
	if !ok || bob.LocalLabel != 4 || bob.FriendLabel != 4 {
		t.Fatalf("secondary local label = %+v, %v", bob, ok)
	}
	bobRow := playerWindowTestRow(t, "Bob")
	if ref := playersRowRefs[bobRow]; ref.session != secondary.ID() {
		t.Fatalf("Bob row session = %d, want %d", ref.session, secondary.ID())
	}
	bobRow.Action()
	if got := secondary.selectedPlayerSnapshot(); got != "Bob" {
		t.Fatalf("secondary selected player = %q, want Bob", got)
	}
	if selectedPlayerName != originalSelected {
		t.Fatalf("secondary row changed primary selection to %q", selectedPlayerName)
	}
	primarySession.commands.mu.Lock()
	primaryPendingBefore := primarySession.commands.pending
	primarySession.commands.mu.Unlock()
	menu := openPlayersContextMenu(playersRowRefs[bobRow], eui.Point{})
	if menu == nil {
		t.Fatal("secondary player context menu did not open")
	}
	thankIndex := -1
	for i, option := range menu.Options {
		if option == "Thank" {
			thankIndex = i
			break
		}
	}
	if thankIndex < 0 {
		t.Fatalf("secondary player context options = %v", menu.Options)
	}
	menu.OnSelect(thankIndex)
	secondary.commands.mu.Lock()
	secondaryPending := secondary.commands.pending
	secondary.commands.mu.Unlock()
	primarySession.commands.mu.Lock()
	primaryPendingAfter := primarySession.commands.pending
	primarySession.commands.mu.Unlock()
	if secondaryPending != "/thank Bob" || primaryPendingAfter != primaryPendingBefore {
		t.Fatalf("secondary context command routed incorrectly: secondary=%q primary before=%q after=%q", secondaryPending, primaryPendingBefore, primaryPendingAfter)
	}
	secondary.commands.clear()
	eui.CloseContextMenus()
}

func playerWindowTestRow(t *testing.T, name string) *eui.ItemData {
	t.Helper()
	for row, ref := range playersRowRefs {
		if ref.name == name {
			return row
		}
	}
	t.Fatalf("player row %q not found", name)
	return nil
}
