package main

import (
	"sync"
	"sync/atomic"

	"github.com/hajimehoshi/ebiten/v2"
)

// Mobile represents basic info about a mobile clicked in the world.
type Mobile struct {
	Index  uint8
	Name   string
	H, V   int16
	PictID uint16
	Colors uint8
}

// ClickInfo describes the last click in the game world.
type ClickInfo struct {
	X, Y     int16
	OnMobile bool
	OnPlayer bool
	Mobile   Mobile
	// Button and modifiers at the time of the click.
	Button ebiten.MouseButton
	Ctrl   bool
	Alt    bool
	Shift  bool
}

var (
	lastClick   ClickInfo
	lastClickMu sync.Mutex

	// lastClickByButton keeps the most recent click info per mouse button.
	lastClickByButton   = map[ebiten.MouseButton]ClickInfo{}
	lastClickByButtonMu sync.Mutex

	lastHover   ClickInfo
	lastHoverMu sync.Mutex

	worldStateGeneration atomic.Uint64
	lastHoverGeneration  uint64
	lastHoverQueryValid  bool
)

func markWorldStateChanged() {
	worldStateGeneration.Add(1)
}

// worldInfoAt returns information about the world location including any
// mobile under the provided coordinates.
func worldInfoAt(x, y int16) ClickInfo {
	info, _ := worldInfoAtGeneration(x, y)
	return info
}

func worldInfoAtGeneration(x, y int16) (ClickInfo, uint64) {
	info := ClickInfo{X: x, Y: y}
	stateMu.Lock()
	generation := worldStateGeneration.Load()
	for _, m := range state.liveMobs {
		if d, ok := state.descriptors[m.Index]; ok {
			size := mobileSizeFunc(d.PictID)
			half := int16(size / 2)
			if x >= m.H-half && x < m.H+half && y >= m.V-half && y < m.V+half {
				info.OnMobile = true
				info.OnPlayer = d.Type == kDescPlayer
				info.Mobile = Mobile{
					Index:  m.Index,
					Name:   d.Name,
					H:      m.H,
					V:      m.V,
					PictID: d.PictID,
					Colors: m.Colors,
				}
				break
			}
		}
	}
	stateMu.Unlock()
	return info, generation
}

// handleWorldClick records a click in the game world and captures
// information about any mobile under the cursor.
func handleWorldClick(x, y int16, b ebiten.MouseButton) ClickInfo {
	info := worldInfoAt(x, y)
	// Snapshot modifier keys at the moment of click.
	mods := currentMods()
	for _, m := range mods {
		switch m {
		case "Ctrl":
			info.Ctrl = true
		case "Alt":
			info.Alt = true
		case "Shift":
			info.Shift = true
		}
	}
	info.Button = b

	lastClickMu.Lock()
	lastClick = info
	lastClickMu.Unlock()

	lastClickByButtonMu.Lock()
	lastClickByButton[b] = info
	lastClickByButtonMu.Unlock()

	return info
}

// updateWorldHover updates the last hovered world location and mobile.
func updateWorldHover(x, y int16) {
	generation := worldStateGeneration.Load()
	lastHoverMu.Lock()
	if lastHoverQueryValid && lastHoverGeneration == generation && lastHover.X == x && lastHover.Y == y {
		lastHoverMu.Unlock()
		return
	}
	lastHoverMu.Unlock()

	info, generation := worldInfoAtGeneration(x, y)
	lastHoverMu.Lock()
	lastHover = info
	lastHoverGeneration = generation
	lastHoverQueryValid = true
	lastHoverMu.Unlock()
}
