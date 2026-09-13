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

type viewportLayout uint8

const (
	viewportLayoutSingle viewportLayout = iota
	viewportLayoutFreeform
	viewportLayoutTiled
)

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
	window             *eui.WindowData
	imageItem          *eui.ItemData
	image              *ebiten.Image
	imageBacking       *ebiten.Image
	inAspectResize     bool
	bubbleHistory      map[bubblePlacementHistoryKey]bubblePlacementHistoryEntry
	bubbleLayout       bubbleLayoutContext
	loginOverlay       *eui.ItemData
	loginForm          *eui.ItemData
	loginStatus        *eui.ItemData
	loginServerChoice  *eui.ItemData
	loginCharacterItem *eui.ItemData
	loginPasswordItem  *eui.ItemData
	loginRememberItem  *eui.ItemData
	loginAction        *eui.ItemData
	loginServer        string
	loginCharacter     string
	loginPassword      string
	loginRemember      bool
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
	mu     sync.RWMutex
	views  [maxSessions]Viewport
	layout viewportLayout
}

func newViewportManager() *viewportManager {
	m := &viewportManager{layout: viewportLayoutSingle}
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

func (m *viewportManager) enableMulti(layout viewportLayout) {
	if m == nil {
		return
	}
	if layout == viewportLayoutSingle {
		layout = viewportLayoutFreeform
	}
	m.mu.Lock()
	m.layout = layout
	for slot := range m.views {
		m.views[slot].Active = true
	}
	m.mu.Unlock()
}

func (m *viewportManager) disableMulti() {
	if m == nil {
		return
	}
	m.mu.Lock()
	m.layout = viewportLayoutSingle
	for slot := range m.views {
		m.views[slot].Active = slot == 0
		if slot != 0 {
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

func tiledViewportRects(area image.Rectangle) [maxSessions]image.Rectangle {
	var rects [maxSessions]image.Rectangle
	midX := area.Min.X + area.Dx()/2
	midY := area.Min.Y + area.Dy()/2
	rects[0] = image.Rect(area.Min.X, area.Min.Y, midX, midY)
	rects[1] = image.Rect(midX, area.Min.Y, area.Max.X, midY)
	rects[2] = image.Rect(area.Min.X, midY, midX, area.Max.Y)
	rects[3] = image.Rect(midX, midY, area.Max.X, area.Max.Y)
	return rects
}

func (m *viewportManager) tile(area image.Rectangle) {
	if m == nil {
		return
	}
	rects := tiledViewportRects(area)
	m.mu.Lock()
	m.layout = viewportLayoutTiled
	for slot := range m.views {
		m.views[slot].Active = true
		m.views[slot].Rect = rects[slot]
	}
	m.mu.Unlock()
}

var appViewports = newViewportManager()
