package main

import (
	"strings"

	scriptapi "gt2"
)

func scriptLastClick() scriptapi.Click {
	lastClickMu.Lock()
	click := lastClick
	lastClickMu.Unlock()
	return scriptClickSnapshot(click, true)
}

func scriptHover() scriptapi.Click {
	lastHoverMu.Lock()
	hover := lastHover
	lastHoverMu.Unlock()
	return scriptClickSnapshot(hover, false)
}

func scriptClickSnapshot(info ClickInfo, includeButton bool) scriptapi.Click {
	button := ""
	if includeButton {
		button = mouseButtonName(info.Button)
	}
	return scriptapi.Click{
		X: info.X, Y: info.Y, OnMobile: info.OnMobile, OnPlayer: info.OnPlayer,
		Mobile: info.Mobile, Button: button,
		Ctrl: info.Ctrl, Alt: info.Alt, Shift: info.Shift, Meta: info.Meta,
	}
}

func scriptSelectedPlayer() (scriptapi.Player, bool) {
	name := strings.TrimSpace(selectedPlayerName)
	if name == "" {
		return scriptapi.Player{}, false
	}
	playersMu.RLock()
	player := players[name]
	if player == nil {
		for candidateName, candidate := range players {
			if strings.EqualFold(candidateName, name) {
				player = candidate
				break
			}
		}
	}
	if player == nil {
		playersMu.RUnlock()
		return scriptapi.Player{}, false
	}
	snapshot := scriptPlayerSnapshot(*player)
	playersMu.RUnlock()
	return snapshot, true
}

func scriptPlayerSnapshot(player Player) scriptapi.Player {
	return scriptapi.Player{
		Name: player.Name, Race: player.Race, Gender: player.Gender, Class: player.Class,
		PictID: player.PictID, Colors: append([]byte(nil), player.Colors...), IsNPC: player.IsNPC,
		Sharee: player.Sharee, Sharing: player.Sharing, Friend: player.Friend,
		FriendLabel: player.FriendLabel, LocalLabel: player.LocalLabel, GlobalLabel: player.GlobalLabel,
		Blocked: player.Blocked, Ignored: player.Ignored, Dead: player.Dead,
		FellWhere: player.FellWhere, FellTime: player.FellTime, KillerName: player.KillerName,
		Bard: player.Bard, SameClan: player.SameClan, Seen: player.Seen,
		LastSeen: player.LastSeen, LastOnScreen: player.LastOnScreen, Offline: player.Offline,
	}
}

func scriptSelectedItem() (scriptapi.Item, bool) {
	for _, item := range getInventory() {
		if item.ID == selectedInvID && item.IDIndex == selectedInvIdx {
			return item, true
		}
	}
	return scriptapi.Item{}, false
}

func scriptSelf() scriptapi.Character {
	// The primary client still has compatibility adapters backed by the
	// established global model. Keep this public wrapper on that model until
	// Phase 3 redirects all primary UI paths through Session.
	primarySession.draw.mu.Lock()
	health, healthMax := primarySession.draw.current.hp, primarySession.draw.current.hpMax
	spirit, spiritMax := primarySession.draw.current.sp, primarySession.draw.current.spMax
	balance, balanceMax := primarySession.draw.current.balance, primarySession.draw.current.balanceMax
	primarySession.draw.mu.Unlock()
	scriptLocationMu.RLock()
	location := scriptLocation
	scriptLocationMu.RUnlock()
	return scriptapi.Character{
		Name: playerName, Health: health, HealthMax: healthMax,
		Spirit: spirit, SpiritMax: spiritMax, Balance: balance, BalanceMax: balanceMax,
		Location: location, Equipment: scriptEquippedItems(),
	}
}

func scriptSelfForSession(session *Session) scriptapi.Character {
	if session == nil {
		return scriptapi.Character{}
	}
	session.draw.mu.Lock()
	health, healthMax := session.draw.current.hp, session.draw.current.hpMax
	spirit, spiritMax := session.draw.current.sp, session.draw.current.spMax
	balance, balanceMax := session.draw.current.balance, session.draw.current.balanceMax
	session.draw.mu.Unlock()
	equipment := scriptEquippedItemsForSession(session)
	return scriptapi.Character{
		Name:   session.characterName(),
		Health: health, HealthMax: healthMax,
		Spirit: spirit, SpiritMax: spiritMax,
		Balance: balance, BalanceMax: balanceMax,
		Location:  scriptLocationForSession(session),
		Equipment: equipment,
	}
}

func scriptEquippedItems() []InventoryItem {
	items := getInventory()
	res := make([]InventoryItem, 0, len(items))
	for _, it := range items {
		if it.Equipped {
			res = append(res, it)
		}
	}
	return res
}

func scriptEquippedItemsForSession(session *Session) []InventoryItem {
	if session == nil || session.inventory == nil {
		return nil
	}
	items := session.inventory.snapshot()
	res := make([]InventoryItem, 0, len(items))
	for _, it := range items {
		if it.Equipped {
			res = append(res, it)
		}
	}
	return res
}

func scriptPlayersForSession(session *Session) []scriptapi.Player {
	if session == nil || session.players == nil {
		return nil
	}
	players := session.players.snapshot()
	out := make([]scriptapi.Player, len(players))
	for index, player := range players {
		out[index] = scriptPlayerSnapshot(player)
	}
	return out
}

func scriptInventoryForSession(session *Session) []InventoryItem {
	if session == nil || session.inventory == nil {
		return nil
	}
	return session.inventory.snapshot()
}

func scriptFindItemExact(name string) (scriptapi.Item, bool) {
	name = strings.TrimSpace(name)
	for _, item := range getInventory() {
		if item.Name == name || item.Base == name {
			return item, true
		}
	}
	return scriptapi.Item{}, false
}

func scriptFindItem(name string) (scriptapi.Item, bool) {
	items := scriptFindItems(name)
	if len(items) == 0 {
		return scriptapi.Item{}, false
	}
	return items[0], true
}

func scriptFindItems(name string) []scriptapi.Item {
	name = normalizeInventoryName(name)
	if name == "" {
		return nil
	}
	var matches []scriptapi.Item
	for _, item := range getInventory() {
		if normalizeInventoryName(item.Name) == name || normalizeInventoryName(item.Base) == name {
			matches = append(matches, item)
		}
	}
	return matches
}

func scriptSearchItems(text string) []scriptapi.Item {
	text = normalizeInventoryName(text)
	if text == "" {
		return nil
	}
	var matches []scriptapi.Item
	for _, item := range getInventory() {
		if strings.Contains(normalizeInventoryName(item.Name), text) ||
			strings.Contains(normalizeInventoryName(item.Base), text) ||
			strings.Contains(normalizeInventoryName(item.Extra), text) {
			matches = append(matches, item)
		}
	}
	return matches
}

func scriptEquipped(slot string) (scriptapi.Item, bool) {
	slot = strings.ToLower(strings.TrimSpace(slot))
	for _, item := range getInventory() {
		if item.Equipped && item.Slot == slot {
			return item, true
		}
	}
	return scriptapi.Item{}, false
}

func scriptHasItem(name string) bool {
	n := strings.ToLower(name)
	for _, it := range getInventory() {
		if strings.ToLower(it.Name) == n {
			return true
		}
	}
	return false
}

// scriptIsEquipped reports whether any equipped item matches the given name.
func scriptIsEquipped(name string) bool {
	n := strings.ToLower(name)
	for _, it := range getInventory() {
		if it.Equipped && strings.ToLower(it.Name) == n {
			return true
		}
	}
	return false
}
