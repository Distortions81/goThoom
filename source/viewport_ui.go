package main

import (
	"fmt"
	"image"
	"math"

	"gothoom/eui"

	"github.com/hajimehoshi/ebiten/v2"
)

func viewportTitle(session *Session) string {
	if session == nil {
		return "Session"
	}
	name := session.characterName()
	if session == primarySession {
		name = playerName
	}
	if name == "" {
		return fmt.Sprintf("Session %d", session.ID())
	}
	return fmt.Sprintf("Session %d -- %s", session.ID(), name)
}

func refreshViewportTitles() {
	views := appViewports.snapshot()
	sessions := appSessions.snapshot()
	for slot, view := range views {
		if view.render != nil && view.render.window != nil && sessions[slot] != nil {
			title := viewportTitle(sessions[slot])
			if appSessions.multiEnabled() && appSessions.selectedID() == sessions[slot].ID() {
				title += " [Selected]"
			}
			view.render.window.Title = title
			view.render.window.Dirty = true
		}
	}
}

func bindPrimaryViewportWindow() {
	state := appViewports.renderStateForViewport(1)
	if state == nil {
		return
	}
	state.window = gameWin
	state.imageItem = gameImageItem
	state.image = gameImage
	state.imageBacking = gameImageBacking
}

func syncPrimaryViewportAliases(state *viewportRenderState) {
	if state == nil || state.window != gameWin {
		return
	}
	gameImageItem = state.imageItem
	gameImage = state.image
	gameImageBacking = state.imageBacking
}

func viewportStateForWindow(win *eui.WindowData) *viewportRenderState {
	if win == nil {
		return nil
	}
	for _, view := range appViewports.snapshot() {
		if view.render != nil && view.render.window == win {
			return view.render
		}
	}
	return nil
}

func isViewportWindow(win *eui.WindowData) bool {
	return win != nil && (win == gameWin || viewportStateForWindow(win) != nil)
}

func isViewportImageItem(item *eui.ItemData) bool {
	if item == nil {
		return false
	}
	if item == gameImageItem {
		return true
	}
	for _, view := range appViewports.snapshot() {
		if view.render != nil && view.render.imageItem == item {
			return true
		}
	}
	return false
}

func updateViewportImageSize(state *viewportRenderState, tiled bool) {
	if state == nil || state.window == nil {
		return
	}
	win := state.window
	size := win.GetSize()
	pad := float64(2 * win.Padding)
	title := float64(win.GetTitleSize())
	pixelW := int(size.X) &^ 1
	pixelH := int(size.Y) &^ 1
	edgeInset := 2
	if tiled {
		pixelW = int(math.Round(float64(size.X)))
		pixelH = int(math.Round(float64(size.Y)))
		edgeInset = 0
	}
	w := int(float64(pixelW)-pad) - 2*edgeInset
	h := int(float64(pixelH)-pad-title) - 2*edgeInset
	if w <= 0 || h <= 0 {
		return
	}
	s := eui.UIScale()
	if state.imageItem == nil {
		item, backing := eui.NewImageFastItem(w, h)
		state.imageItem = item
		state.imageBacking = backing
		state.image = backing
		item.Image = state.image
		item.Size = eui.Point{X: float32(w) / s, Y: float32(h) / s}
		item.Position = eui.Point{X: float32(edgeInset) / s, Y: float32(edgeInset) / s}
		win.AddItem(item)
		syncPrimaryViewportAliases(state)
		return
	}
	iw, ih := 0, 0
	if state.imageBacking != nil {
		bounds := state.imageBacking.Bounds()
		iw, ih = bounds.Dx(), bounds.Dy()
	}
	if state.imageBacking == nil || iw < w || ih < h {
		_, replacement := eui.NewImageFastItem(w, h)
		if state.imageBacking != nil {
			state.imageBacking.Deallocate()
		}
		state.imageBacking = replacement
		win.Dirty = true
	}
	state.image = state.imageBacking.SubImage(image.Rect(0, 0, w, h)).(*ebiten.Image)
	state.imageItem.Image = state.image
	state.imageItem.Size = eui.Point{X: float32(w) / s, Y: float32(h) / s}
	state.imageItem.Position = eui.Point{X: float32(edgeInset) / s, Y: float32(edgeInset) / s}
	syncPrimaryViewportAliases(state)
}

func resizeViewportWindow(state *viewportRenderState) {
	if state == nil || state.window == nil {
		return
	}
	win := state.window
	if state.inAspectResize {
		updateViewportImageSize(state, false)
		return
	}
	size := win.GetSize()
	if size.X <= 0 || size.Y <= 0 {
		return
	}
	pad := float64(2 * win.Padding)
	title := float64(win.GetTitleSize())
	availW := float64(int(size.X)&^1) - pad
	availH := float64(int(size.Y)&^1) - pad - title
	if availW <= 0 || availH <= 0 {
		updateViewportImageSize(state, false)
		return
	}
	scale := math.Min(availW/float64(gameAreaSizeX), availH/float64(gameAreaSizeY))
	if scale < 0.25 {
		scale = 0.25
	}
	newSize := eui.Point{
		X: float32(math.Round(float64(gameAreaSizeX)*scale + pad)),
		Y: float32(math.Round(float64(gameAreaSizeY)*scale + pad + title)),
	}
	if math.Abs(float64(size.X-newSize.X)) > 0.5 || math.Abs(float64(size.Y-newSize.Y)) > 0.5 {
		state.inAspectResize = true
		_ = win.SetSize(newSize)
		state.inAspectResize = false
	}
	updateViewportImageSize(state, false)
}

func configureSecondaryViewportWindow(view Viewport, session *Session) {
	state := view.render
	if state == nil || state.window != nil {
		return
	}
	win := newGameRenderWindow()
	win.Title = viewportTitle(session)
	win.Closable = false
	win.Resizable = true
	win.Movable = true
	win.Maximizable = false
	state.window = win
	win.OnResize = func() { resizeViewportWindow(state) }

	screenW, screenH := eui.ScreenSize()
	width := max(320, min(640, screenW/2-24))
	height := max(220, min(420, screenH/2-24))
	win.Size = eui.Point{X: float32(width), Y: float32(height)}
	col := (int(view.ID) - 1) % 2
	row := (int(view.ID) - 1) / 2
	_ = win.SetPos(eui.Point{X: float32(12 + col*(width+12)), Y: float32(12 + row*(height+12))})
	updateViewportImageSize(state, false)
	win.MarkOpen()
}

func refreshViewportWorkspace() {
	bindPrimaryViewportWindow()
	views := appViewports.snapshot()
	sessions := appSessions.snapshot()
	for slot, view := range views {
		state := view.render
		if state == nil {
			continue
		}
		if slot == 0 {
			continue
		}
		if view.Active && appSessions.multiEnabled() {
			configureSecondaryViewportWindow(view, sessions[slot])
			if state.window != nil && !state.window.IsOpen() {
				state.window.MarkOpen()
			}
			continue
		}
		if state.window != nil {
			state.window.RemoveWindow()
			if state.imageBacking != nil {
				state.imageBacking.Deallocate()
			}
			state.window = nil
			state.imageItem = nil
			state.image = nil
			state.imageBacking = nil
			state.worldRenderValid = false
		}
	}
	refreshViewportTitles()
}

func refreshViewportRectsFromDrawRects() {
	for _, view := range appViewports.snapshot() {
		state := view.render
		if !view.Active || state == nil || state.window == nil || !state.window.IsOpen() || state.imageItem == nil {
			appViewports.setRect(view.ID, image.Rectangle{})
			continue
		}
		r := state.imageItem.DrawRect
		rect := image.Rect(int(math.Round(float64(r.X0))), int(math.Round(float64(r.Y0))), int(math.Round(float64(r.X1))), int(math.Round(float64(r.Y1))))
		appViewports.setRect(view.ID, rect)
	}
}

func sessionViewportWorldAt(session *Session, point image.Point) (int16, int16, bool) {
	if session == nil {
		return 0, 0, false
	}
	for _, view := range appViewports.snapshot() {
		if view.SessionID != session.ID() {
			continue
		}
		return view.worldAt(point)
	}
	return 0, 0, false
}

func worldDrawInfoForSession(session *Session) (int, int, float64) {
	if session != nil {
		for _, view := range appViewports.snapshot() {
			if view.SessionID != session.ID() || !view.Active || view.Rect.Empty() {
				continue
			}
			viewRect, scale := fittedWorldView(view.Rect.Dx(), view.Rect.Dy())
			return view.Rect.Min.X + viewRect.Min.X, view.Rect.Min.Y + viewRect.Min.Y, scale
		}
	}
	return worldDrawInfo()
}
