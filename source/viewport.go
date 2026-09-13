package main

import (
	"gothoom/eui"
	"image"
	"sync"

	"github.com/hajimehoshi/ebiten/v2"
)

// ViewportID is the stable identity of one visual session surface. It is kept
// separate from SessionID even though the first workspace maps them one-to-one.
type ViewportID uint8

// Viewport owns only presentation and pointer-mapping state. Protocol and
// character state remain on its assigned Session.
type Viewport struct {
	ID        ViewportID
	SessionID SessionID
	Rect      image.Rectangle
	Active    bool
	render    *viewportRenderState
}

type viewportRenderState struct {
	drawSnapshot       drawSnapshot
	lastWorldRenderKey worldRenderKey
	worldRenderValid   bool
	lighting           viewportLightingFrame
	window             *eui.WindowData
	imageItem          *eui.ItemData
	image              *ebiten.Image
	imageBacking       *ebiten.Image
	lightingTmp        *ebiten.Image
	nightTransition    nightTransitionState
	bubbleHistory      map[bubblePlacementHistoryKey]bubblePlacementHistoryEntry
	bubbleLayout       bubbleLayoutContext
	loginOverlay       *eui.ItemData
	loginForm          *eui.ItemData
	loginStatus        *eui.ItemData
	loginServerChoice  *eui.ItemData
	loginCharacters    *eui.ItemData
	loginAdd           *eui.ItemData
	loginEdit          *eui.ItemData
	loginDelete        *eui.ItemData
	loginAction        *eui.ItemData
	sessionLogout      *eui.ItemData
	loginServer        string
	loginCharacter     string
	loginDemoLookup    bool
}

func (v Viewport) worldAt(point image.Point) (int16, int16, bool) {
	if !v.Active || !point.In(v.Rect) || v.Rect.Empty() {
		return 0, 0, false
	}
	viewRect, scale := fittedWorldView(v.Rect.Dx(), v.Rect.Dy())
	local := point.Sub(v.Rect.Min)
	if !local.In(viewRect) || scale <= 0 {
		return 0, 0, false
	}
	x := int16(float64(local.X-viewRect.Min.X)/scale - float64(fieldCenterX))
	y := int16(float64(local.Y-viewRect.Min.Y)/scale - float64(fieldCenterY))
	return x, y, true
}

type viewportManager struct {
	mu    sync.RWMutex
	views [maxSessions]Viewport
}

func newViewportManager() *viewportManager {
	m := &viewportManager{}
	for slot := range m.views {
		id, _ := sessionIDForSlot(slot)
		m.views[slot] = Viewport{
			ID: ViewportID(slot + 1), SessionID: id, Active: slot == 0,
			render: &viewportRenderState{},
		}
	}
	return m
}

func (m *viewportManager) renderStateForSession(id SessionID) *viewportRenderState {
	slot, ok := id.Slot()
	if m == nil || !ok {
		return nil
	}
	m.mu.RLock()
	state := m.views[slot].render
	m.mu.RUnlock()
	return state
}

func (m *viewportManager) renderStateForViewport(id ViewportID) *viewportRenderState {
	slot := int(id) - 1
	if m == nil || slot < 0 || slot >= len(m.views) {
		return nil
	}
	m.mu.RLock()
	state := m.views[slot].render
	m.mu.RUnlock()
	return state
}

func (m *viewportManager) snapshot() [maxSessions]Viewport {
	if m == nil {
		return [maxSessions]Viewport{}
	}
	m.mu.RLock()
	views := m.views
	m.mu.RUnlock()
	return views
}

func (m *viewportManager) showSession(id SessionID) {
	if m == nil {
		return
	}
	m.mu.Lock()
	for slot := range m.views {
		m.views[slot].Active = m.views[slot].SessionID == id
		if !m.views[slot].Active {
			m.views[slot].Rect = image.Rectangle{}
		}
	}
	m.mu.Unlock()
}

func (m *viewportManager) setRect(id ViewportID, rect image.Rectangle) bool {
	slot := int(id) - 1
	if m == nil || slot < 0 || slot >= len(m.views) {
		return false
	}
	m.mu.Lock()
	m.views[slot].Rect = rect
	m.mu.Unlock()
	return true
}

func (m *viewportManager) hitTest(point image.Point) (Viewport, bool) {
	if m == nil {
		return Viewport{}, false
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	// Follow EUI's current window order when freeform playfields overlap.
	windows := eui.Windows()
	for index := len(windows) - 1; index >= 0; index-- {
		for slot := range m.views {
			view := m.views[slot]
			if view.Active && view.render != nil && view.render.window == windows[index] && point.In(view.Rect) {
				return view, true
			}
		}
	}
	// Unit tests and pre-draw setup can have rectangles before windows exist.
	for slot := len(m.views) - 1; slot >= 0; slot-- {
		view := m.views[slot]
		if view.Active && point.In(view.Rect) {
			return view, true
		}
	}
	return Viewport{}, false
}

func (m *viewportManager) selectAt(point image.Point, sessions *sessionManager) (Viewport, int16, int16, bool, bool) {
	view, hit := m.hitTest(point)
	if !hit || sessions == nil || !sessions.selectSession(view.SessionID) {
		return Viewport{}, 0, 0, false, false
	}
	x, y, inWorld := view.worldAt(point)
	return view, x, y, inWorld, true
}

var appViewports = newViewportManager()
