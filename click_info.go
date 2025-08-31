package main

import (
    "sync"

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
)

// worldInfoAt returns information about the world location including any
// mobile under the provided coordinates.
func worldInfoAt(x, y int16) ClickInfo {
	info := ClickInfo{X: x, Y: y}
	stateMu.Lock()
	for _, m := range state.liveMobs {
		if d, ok := state.descriptors[m.Index]; ok {
			size := mobileSize(d.PictID)
			half := int16(size / 2)
			if x >= m.H-half && x < m.H+half && y >= m.V-half && y < m.V+half {
				info.OnMobile = true
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
	return info
}

// handleWorldClick records a click in the game world and captures
// information about any mobile under the cursor.
func handleWorldClick(x, y int16, b ebiten.MouseButton) {
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
}

// updateWorldHover updates the last hovered world location and mobile.
func updateWorldHover(x, y int16) {
	info := worldInfoAt(x, y)
	lastHoverMu.Lock()
	lastHover = info
	lastHoverMu.Unlock()
}
