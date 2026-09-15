package main

import (
	"sync/atomic"

	scriptapi "gt2"

	"github.com/hajimehoshi/ebiten/v2"
)

type Mobile = scriptapi.Mobile

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
	Meta   bool
}

var worldRenderGeneration atomic.Uint64

func markWorldStateChanged() {
	markSessionWorldStateChanged(primarySession)
}

func markSessionWorldStateChanged(session *Session) {
	if session == nil || session.draw == nil {
		return
	}
	session.draw.markChanged()
	markWorldRenderChanged()
}

// markWorldRenderChanged invalidates the cached world image for visual state
// that is not part of a server draw-state packet, such as script overlays.
func markWorldRenderChanged() {
	worldRenderGeneration.Add(1)
}

// worldInfoAt returns information about the world location including any
// mobile under the provided coordinates.
func worldInfoAt(x, y int16) ClickInfo {
	info, _ := worldInfoAtGeneration(x, y)
	return info
}

func worldInfoAtGeneration(x, y int16) (ClickInfo, uint64) {
	return worldInfoAtSessionGeneration(primarySession, x, y)
}

func worldInfoAtSession(session *Session, x, y int16) ClickInfo {
	info, _ := worldInfoAtSessionGeneration(session, x, y)
	return info
}

func worldInfoAtSessionGeneration(session *Session, x, y int16) (ClickInfo, uint64) {
	info := ClickInfo{X: x, Y: y}
	if session == nil || session.draw == nil {
		return info, 0
	}
	selfIndex := session.playerIndexSnapshot()
	if session == primarySession {
		selfIndex = playerIndex
	}
	session.draw.mu.Lock()
	generation := session.draw.generation.Load()
	for _, m := range session.draw.current.liveMobs {
		if d, ok := session.draw.current.descriptors[m.Index]; ok {
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
					Player: d.Type == kDescPlayer,
					State:  m.State, Plane: d.Plane, Size: size, Self: m.Index == selfIndex,
					Dead: m.State == poseDead, Stale: m.Persist,
				}
				break
			}
		}
	}
	session.draw.mu.Unlock()
	return info, generation
}

// handleWorldClick records a click in the game world and captures
// information about any mobile under the cursor.
func handleWorldClick(x, y int16, b ebiten.MouseButton) ClickInfo {
	return handleSessionWorldClick(primarySession, x, y, b)
}

func handleSessionWorldClick(session *Session, x, y int16, b ebiten.MouseButton) ClickInfo {
	if session == nil {
		session = primarySession
	}
	info := worldInfoAtSession(session, x, y)
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
		case "Meta":
			info.Meta = true
		}
	}
	info.Button = b

	session.input.storeClick(info)

	return info
}

// updateWorldHover updates the last hovered world location and mobile.
func updateWorldHover(x, y int16) {
	updateSessionWorldHover(primarySession, x, y)
}

func updateSessionWorldHover(session *Session, x, y int16) {
	if session == nil {
		session = primarySession
	}
	generation := session.draw.generation.Load()
	if _, ok := session.input.cachedHover(generation, x, y); ok {
		return
	}
	info, generation := worldInfoAtSessionGeneration(session, x, y)
	hoveredMobileChanged := session.input.storeHover(info, generation)
	if gs.NameTagsOnHoverOnly && hoveredMobileChanged {
		markWorldRenderChanged()
	}
}

func updateSessionWorldHoverForPointer(session *Session, x, y int16, insideWorld, focused bool) {
	if !insideWorld || !focused {
		if session == nil {
			session = primarySession
		}
		if session.input.clearHover() && gs.NameTagsOnHoverOnly {
			markWorldRenderChanged()
		}
		return
	}
	updateSessionWorldHover(session, x, y)
}

func sessionHoverSnapshot(session *Session) ClickInfo {
	if session == nil {
		return ClickInfo{}
	}
	return session.input.hoverSnapshot()
}

func sessionHoverSnapshotForID(id SessionID) ClickInfo {
	session, ok := appSessions.session(id)
	if !ok {
		session = primarySession
	}
	return sessionHoverSnapshot(session)
}
