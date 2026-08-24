package main

import (
	"strconv"
	"strings"
	"sync"

	scriptapi "gt"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

var scriptInputSnapshot = struct {
	sync.RWMutex
	keys    map[ebiten.Key]bool
	buttons map[ebiten.MouseButton]bool
	wheelX  float64
	wheelY  float64
}{keys: map[ebiten.Key]bool{}, buttons: map[ebiten.MouseButton]bool{}}

func refreshScriptInputSnapshot() {
	keys := make(map[ebiten.Key]bool)
	for key := ebiten.Key(0); key <= ebiten.KeyMax; key++ {
		if inpututil.IsKeyJustPressed(key) {
			keys[key] = true
		}
	}
	buttons := make(map[ebiten.MouseButton]bool)
	for button := ebiten.MouseButton(0); button <= ebiten.MouseButtonMax; button++ {
		if inpututil.IsMouseButtonJustPressed(button) {
			buttons[button] = true
		}
	}
	wheelX, wheelY := ebiten.Wheel()
	scriptInputSnapshot.Lock()
	scriptInputSnapshot.keys = keys
	scriptInputSnapshot.buttons = buttons
	scriptInputSnapshot.wheelX = wheelX
	scriptInputSnapshot.wheelY = wheelY
	scriptInputSnapshot.Unlock()
}

var keyNameMap = func() map[string]ebiten.Key {
	m := make(map[string]ebiten.Key)
	for k := ebiten.Key(0); k <= ebiten.KeyMax; k++ {
		m[strings.ToLower(k.String())] = k
	}
	return m
}()

func keyFromName(name string) (ebiten.Key, bool) {
	k, ok := keyNameMap[strings.ToLower(name)]
	return k, ok
}

func mouseButtonFromName(name string) (ebiten.MouseButton, bool) {
	n := strings.ToLower(strings.TrimSpace(name))
	switch n {
	case "right", "rightclick":
		return ebiten.MouseButtonRight, true
	case "middle", "middleclick":
		return ebiten.MouseButtonMiddle, true
	}
	if strings.HasPrefix(n, "mouse") {
		numStr := strings.TrimSpace(strings.TrimPrefix(n, "mouse"))
		if numStr == "" {
			return 0, false
		}
		if num, err := strconv.Atoi(numStr); err == nil {
			b := ebiten.MouseButton(num)
			if b > ebiten.MouseButtonLeft && b <= ebiten.MouseButtonMax {
				return b, true
			}
		}
	}
	return 0, false
}

func scriptKeyJustPressed(name string) bool {
	if k, ok := keyFromName(name); ok {
		scriptInputSnapshot.RLock()
		pressed := scriptInputSnapshot.keys[k]
		scriptInputSnapshot.RUnlock()
		return pressed
	}
	return false
}

func scriptMouseJustPressed(name string) bool {
	if b, ok := mouseButtonFromName(name); ok {
		scriptInputSnapshot.RLock()
		pressed := scriptInputSnapshot.buttons[b]
		scriptInputSnapshot.RUnlock()
		return pressed
	}
	return false
}

func scriptMouseWheel() (float64, float64) {
	scriptInputSnapshot.RLock()
	x, y := scriptInputSnapshot.wheelX, scriptInputSnapshot.wheelY
	scriptInputSnapshot.RUnlock()
	return x, y
}

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
	stateMu.Lock()
	health, healthMax := state.hp, state.hpMax
	spirit, spiritMax := state.sp, state.spMax
	balance, balanceMax := state.balance, state.balanceMax
	stateMu.Unlock()
	scriptLocationMu.RLock()
	location := scriptLocation
	scriptLocationMu.RUnlock()
	equipment := scriptEquippedItems()
	return scriptapi.Character{
		Name:   playerName,
		Health: health, HealthMax: healthMax,
		Spirit: spirit, SpiritMax: spiritMax,
		Balance: balance, BalanceMax: balanceMax,
		Location:  location,
		Equipment: equipment,
	}
}

func scriptCurrentWorld() scriptapi.World {
	stateMu.Lock()
	mobiles := make([]scriptapi.Mobile, 0, len(state.liveMobs))
	for _, mobile := range state.liveMobs {
		descriptor, ok := state.descriptors[mobile.Index]
		if !ok {
			continue
		}
		mobiles = append(mobiles, scriptapi.Mobile{
			Index: mobile.Index, Name: descriptor.Name, H: mobile.H, V: mobile.V,
			PictID: descriptor.PictID, Colors: mobile.Colors, Player: descriptor.Type == kDescPlayer,
		})
	}
	stateMu.Unlock()
	scriptLocationMu.RLock()
	location := scriptLocation
	scriptLocationMu.RUnlock()
	return scriptapi.World{
		Width: gameAreaSizeX, Height: gameAreaSizeY, Location: location,
		Generation: worldStateGeneration.Load(), Mobiles: mobiles,
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
