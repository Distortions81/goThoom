package main

import (
	"testing"

	"gothoom/climg"

	"github.com/hajimehoshi/ebiten/v2"
)

func TestEffectiveCharacterUsesSelectedProfileWhileOffline(t *testing.T) {
	originalPlayer := playerName
	originalLast := gs.LastCharacter
	originalConn := tcpConn
	primarySession.transport.mu.Lock()
	originalTransportStatus := primarySession.transport.status
	primarySession.transport.status = sessionDisconnected
	primarySession.transport.mu.Unlock()
	t.Cleanup(func() {
		playerName = originalPlayer
		gs.LastCharacter = originalLast
		tcpConn = originalConn
		primarySession.transport.mu.Lock()
		primarySession.transport.status = originalTransportStatus
		primarySession.transport.mu.Unlock()
	})

	playerName = "Previous"
	gs.LastCharacter = "Selected"
	tcpConn = nil
	if got := effectiveCharacterName(); got != "Selected" {
		t.Fatalf("offline effective character = %q, want Selected", got)
	}

	primarySession.transport.mu.Lock()
	primarySession.transport.status = sessionConnecting
	primarySession.transport.mu.Unlock()
	if got := effectiveCharacterName(); got != "Previous" {
		t.Fatalf("connecting effective character = %q, want Previous", got)
	}
}

func TestScriptHoverAndSelectionSnapshots(t *testing.T) {
	withScriptInventoryTestState(t)
	playersMu.Lock()
	originalPlayers := players
	players = map[string]*Player{
		"Example": {Name: "Example", Race: "Sylvan", Colors: []byte{1, 2, 3}, Seen: true},
	}
	playersMu.Unlock()

	setScriptInventoryTestItems([]InventoryItem{{ID: 42, IDIndex: 3, Name: "Test Blade", Base: "Test Blade", Quantity: 1}})

	lastHoverMu.Lock()
	originalHover := lastHover
	lastHover = ClickInfo{X: 12, Y: -8, OnMobile: true, OnPlayer: true, Mobile: Mobile{Name: "Example"}}
	lastHoverMu.Unlock()

	originalSelectedPlayer, originalSelectedID, originalSelectedIndex := selectedPlayerName, selectedInvID, selectedInvIdx
	selectedPlayerName, selectedInvID, selectedInvIdx = "example", 42, 3
	t.Cleanup(func() {
		playersMu.Lock()
		players = originalPlayers
		playersMu.Unlock()
		lastHoverMu.Lock()
		lastHover = originalHover
		lastHoverMu.Unlock()
		selectedPlayerName, selectedInvID, selectedInvIdx = originalSelectedPlayer, originalSelectedID, originalSelectedIndex
	})

	hover := scriptHover()
	if !hover.OnPlayer || hover.Mobile.Name != "Example" || hover.X != 12 || hover.Y != -8 || hover.Button != "" {
		t.Fatalf("hover snapshot = %+v", hover)
	}
	player, ok := scriptSelectedPlayer()
	if !ok || player.Name != "Example" || player.Race != "Sylvan" {
		t.Fatalf("selected player = %+v, %v", player, ok)
	}
	player.Colors[0] = 99
	playersMu.RLock()
	internalColor := players["Example"].Colors[0]
	playersMu.RUnlock()
	if internalColor != 1 {
		t.Fatal("mutating a player snapshot changed client-owned state")
	}
	item, ok := scriptSelectedItem()
	if !ok || item.ID != 42 || item.IDIndex != 3 || item.Name != "Test Blade" {
		t.Fatalf("selected item = %+v, %v", item, ok)
	}
}

func TestScriptLastClickUsesReadableButtonName(t *testing.T) {
	lastClickMu.Lock()
	original := lastClick
	lastClick = ClickInfo{X: 1, Y: 2, Button: ebiten.MouseButtonRight, Ctrl: true, Meta: true}
	lastClickMu.Unlock()
	t.Cleanup(func() {
		lastClickMu.Lock()
		lastClick = original
		lastClickMu.Unlock()
	})

	click := scriptLastClick()
	if click.Button != "RightClick" || !click.Ctrl || !click.Meta {
		t.Fatalf("click snapshot = %+v", click)
	}
}

func TestScriptSelfSnapshot(t *testing.T) {
	withScriptInventoryTestState(t)
	originalName := playerName
	playerName = "Hero"
	primarySession.draw.mu.Lock()
	originalVitals := [6]int{primarySession.draw.current.hp, primarySession.draw.current.hpMax, primarySession.draw.current.sp, primarySession.draw.current.spMax, primarySession.draw.current.balance, primarySession.draw.current.balanceMax}
	primarySession.draw.current.hp, primarySession.draw.current.hpMax, primarySession.draw.current.sp, primarySession.draw.current.spMax, primarySession.draw.current.balance, primarySession.draw.current.balanceMax = 7, 10, 8, 11, 9, 12
	primarySession.draw.mu.Unlock()
	scriptLocationMu.Lock()
	originalLocation := scriptLocation
	scriptLocation = "Town Square"
	scriptLocationMu.Unlock()
	setScriptInventoryTestItems([]InventoryItem{{ID: 100, Name: "Shadow Bell", Base: "Shadow Bell", Equipped: true, Quantity: 1}})
	originalImages := clImages
	clImages = testCLImages(map[uint32]*climg.ClientItem{100: {Name: "Shadow Bell", Slot: kItemSlotRightHand}})
	t.Cleanup(func() {
		playerName = originalName
		primarySession.draw.mu.Lock()
		primarySession.draw.current.hp, primarySession.draw.current.hpMax, primarySession.draw.current.sp, primarySession.draw.current.spMax, primarySession.draw.current.balance, primarySession.draw.current.balanceMax =
			originalVitals[0], originalVitals[1], originalVitals[2], originalVitals[3], originalVitals[4], originalVitals[5]
		primarySession.draw.mu.Unlock()
		scriptLocationMu.Lock()
		scriptLocation = originalLocation
		scriptLocationMu.Unlock()
		clImages = originalImages
	})

	self := scriptSelf()
	if self.Name != "Hero" || self.Health != 7 || self.HealthMax != 10 ||
		self.Spirit != 8 || self.SpiritMax != 11 || self.Balance != 9 || self.BalanceMax != 12 || self.Location != "Town Square" {
		t.Fatalf("self snapshot = %+v", self)
	}
	if len(self.Equipment) != 1 || self.Equipment[0].Name != "Shadow Bell" || self.Equipment[0].Slot != "right-hand" {
		t.Fatalf("equipment snapshot = %+v", self.Equipment)
	}
}

func TestScriptWorldSnapshot(t *testing.T) {
	primarySession.draw.mu.Lock()
	originalMobiles, originalDescriptors := primarySession.draw.current.liveMobs, primarySession.draw.current.descriptors
	primarySession.draw.current.liveMobs = []frameMobile{{Index: 7, H: 14, V: -3, Colors: 2}}
	primarySession.draw.current.descriptors = map[uint8]frameDescriptor{7: {Index: 7, Type: kDescPlayer, Name: "Traveler", PictID: 99}}
	primarySession.draw.mu.Unlock()
	scriptLocationMu.Lock()
	originalLocation := scriptLocation
	scriptLocation = "Forest"
	scriptLocationMu.Unlock()
	markWorldStateChanged()
	t.Cleanup(func() {
		primarySession.draw.mu.Lock()
		primarySession.draw.current.liveMobs, primarySession.draw.current.descriptors = originalMobiles, originalDescriptors
		primarySession.draw.mu.Unlock()
		scriptLocationMu.Lock()
		scriptLocation = originalLocation
		scriptLocationMu.Unlock()
	})

	world := scriptCurrentWorld()
	if world.Width != gameAreaSizeX || world.Height != gameAreaSizeY || world.Location != "Forest" || world.Generation == 0 {
		t.Fatalf("world snapshot = %+v", world)
	}
	if len(world.Mobiles) != 1 || world.Mobiles[0].Name != "Traveler" || !world.Mobiles[0].Player || world.Mobiles[0].H != 14 {
		t.Fatalf("world mobiles = %+v", world.Mobiles)
	}
	world.Mobiles[0].Name = "Changed"
	primarySession.draw.mu.Lock()
	internalName := primarySession.draw.current.descriptors[7].Name
	primarySession.draw.mu.Unlock()
	if internalName != "Traveler" {
		t.Fatal("mutating a world snapshot changed client-owned state")
	}
}
