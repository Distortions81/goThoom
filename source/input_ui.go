package main

import (
	"gothoom/eui"
)

// uiOwnsPointerPress preserves ownership of a press after EUI changes the
// layout, such as when the press closes a window. The playable portion of the
// game window remains transparent to world input.
func uiOwnsPointerPress(overUI bool, pressWin *eui.WindowData, handled bool) bool {
	if overUI {
		return true
	}
	if pressWin != nil {
		// EUI reports the game image itself as handled. The playable part of
		// this window must still pass presses through for walking.
		return pressWin != gameWin
	}
	return handled
}

// pointInUI reports whether the given screen coordinate lies within any EUI window or overlay.
func pointInUI(x, y int) bool {
	fx, fy := float32(x), float32(y)

	windows := eui.Windows()
	for _, win := range windows {
		if !win.IsOpen() {
			continue
		}
		pos := win.GetPos()
		size := win.GetSize()
		s := windowInputScale(win)
		frame := (win.Margin + win.Border + win.BorderPad + win.Padding) * s
		title := win.GetTitleSize()
		x0, y0 := pos.X+1, pos.Y+1
		x1 := x0 + size.X + frame*2
		y1 := y0 + size.Y + frame*2 + title
		if win == gameWin {
			// Treat only the title bar of the game window as UI so
			// world clicks still pass through to the game.
			y1 = y0 + frame + title
		}
		if fx >= x0 && fx < x1 && fy >= y0 && fy < y1 {
			return true
		}
	}

	if gameWin != nil && gameWin.IsOpen() {
		if pointInItems(gameWin.Contents, fx, fy) {
			return true
		}
	}

	return false
}

func pointInAppScreen(x, y int) bool {
	w, h := eui.ScreenSize()
	return x >= 0 && y >= 0 && x < w && y < h
}

func windowInputScale(win *eui.WindowData) float32 {
	if win != nil && win.NoScale {
		return 1
	}
	return eui.UIScale()
}

// pointInGameWindow reports whether the given screen coordinate lies within the
// playable area of the game window.
func pointInGameWindow(x, y int) bool {
	if gameWin == nil || !gameWin.IsOpen() {
		return false
	}

	fx, fy := float32(x), float32(y)
	pos := gameWin.GetPos()
	size := gameWin.GetSize()
	s := windowInputScale(gameWin)
	frame := (gameWin.Margin + gameWin.Border + gameWin.BorderPad + gameWin.Padding) * s
	title := gameWin.GetTitleSize()
	x0 := pos.X + frame
	y0 := pos.Y + frame + title
	x1 := pos.X + size.X - frame
	y1 := pos.Y + size.Y - frame
	return fx >= x0 && fx < x1 && fy >= y0 && fy < y1
}

func pointInItems(items []*eui.ItemData, fx, fy float32) bool {
	for i := len(items) - 1; i >= 0; i-- {
		it := items[i]
		if it == nil || it.Invisible || it == gameImageItem {
			continue
		}
		if fx >= it.DrawRect.X0 && fx <= it.DrawRect.X1 && fy >= it.DrawRect.Y0 && fy <= it.DrawRect.Y1 {
			return true
		}
		if len(it.Contents) > 0 && pointInItems(it.Contents, fx, fy) {
			return true
		}
		if len(it.Tabs) > 0 {
			for _, tab := range it.Tabs {
				if pointInItems(tab.Contents, fx, fy) {
					return true
				}
			}
		}
	}
	return false
}

// typingInUI reports whether any EUI text input other than the console input bar
// currently has focus.
func typingInUI() bool {
	if eui.SearchActive() {
		return true
	}
	var inputItem *eui.ItemData
	if inputFlow != nil && len(inputFlow.Contents) > 0 {
		inputItem = inputFlow.Contents[0]
	}
	windows := eui.Windows()
	for _, win := range windows {
		if !win.IsOpen() {
			continue
		}
		if typingInItems(win.Contents, inputItem) {
			return true
		}
	}
	return false
}

func typingInItems(items []*eui.ItemData, exclude *eui.ItemData) bool {
	for _, it := range items {
		if it == nil {
			continue
		}
		if it.Focused && (it.ItemType == eui.ITEM_TEXT || it.ItemType == eui.ITEM_INPUT) && it != exclude {
			return true
		}
		if len(it.Contents) > 0 && typingInItems(it.Contents, exclude) {
			return true
		}
		if len(it.Tabs) > 0 {
			for _, tab := range it.Tabs {
				if typingInItems(tab.Contents, exclude) {
					return true
				}
			}
		}
	}
	return false
}
