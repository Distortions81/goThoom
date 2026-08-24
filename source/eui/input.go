package eui

import (
	"context"
	"math"
	"regexp"
	"strings"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	clipboard "golang.design/x/clipboard"
)

var (
	mposOld     point
	cursorShape ebiten.CursorShapeType

	dragPart   dragType
	dragWin    *windowData
	dragFlow   *itemData
	activeItem *itemData

	downWin             *windowData
	pointerPressHandled bool

	activeSearch     *windowData
	selectedTextItem *itemData
	updateNow        time.Time

	inputBuf []rune
)

var (
	ShiftPressed          bool
	CtrlPressed           bool
	CapsLockToggleHandler func()
)

// Update processes input and updates window state.
// Programs embedding the UI can call this from their Ebiten Update handler.
func Update() error {
	updateNow = time.Now()
	checkThemeStyleMods()
	pointerPressHandled = false

	shiftPressed := ebiten.IsKeyPressed(ebiten.KeyShift) || ebiten.IsKeyPressed(ebiten.KeyShiftLeft) || ebiten.IsKeyPressed(ebiten.KeyShiftRight)
	ShiftPressed = shiftPressed
	ctrlPressed := ebiten.IsKeyPressed(ebiten.KeyControl) || ebiten.IsKeyPressed(ebiten.KeyControlLeft) || ebiten.IsKeyPressed(ebiten.KeyControlRight)
	CtrlPressed = ctrlPressed
	altPressed := ebiten.IsKeyPressed(ebiten.KeyAlt) || ebiten.IsKeyPressed(ebiten.KeyAltLeft) || ebiten.IsKeyPressed(ebiten.KeyAltRight)
	metaPressed := ebiten.IsKeyPressed(ebiten.KeyMeta) || ebiten.IsKeyPressed(ebiten.KeyMetaLeft) || ebiten.IsKeyPressed(ebiten.KeyMetaRight)
	if inpututil.IsKeyJustPressed(ebiten.KeyCapsLock) {
		if CapsLockToggleHandler != nil {
			CapsLockToggleHandler()
		}
	}
	_ = altPressed

	if inpututil.IsKeyJustPressed(ebiten.KeyGraveAccent) && shiftPressed {
		_ = DumpTree()
	}

	prevHovered := hoveredItem
	hoveredItem = nil

	mx, my := PointerPosition()
	mpos := point{X: float32(mx), Y: float32(my)}

	click := pointerJustPressed()
	midClick := middleClickMove && inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonMiddle)
	if click || midClick {
		downWin = nil
		for i := len(windows) - 1; i >= 0; i-- {
			win := windows[i]
			if !win.Open {
				continue
			}
			if win.getWinRect().containsPoint(mpos) {
				downWin = win
				break
			}
		}
	}
	if click {
		clearTextSelection()
		if !dropdownOpenContainsAnywhere(mpos) {
			closeAllDropdowns()
		}
		// Close context menus when clicking anywhere outside them.
		if !contextMenuContainsAnywhere(mpos) {
			CloseContextMenus()
		}
		if focusedItem != nil {
			focusedItem.Focused = false
		}
		focusedItem = nil
	}
	clickTime := pointerPressDuration()
	clickDrag := clickTime > 1
	midClickTime := 0
	midPressed := false
	if middleClickMove {
		midClickTime = inpututil.MouseButtonPressDuration(ebiten.MouseButtonMiddle)
		midPressed = ebiten.IsMouseButtonPressed(ebiten.MouseButtonMiddle)
	}
	midClickDrag := midClickTime > 1

	if !pointerPressed() && !midPressed {
		if selectedTextItem != nil && selectedTextItem.selecting {
			selectedTextItem.selecting = false
			selectedTextItem.markDirty()
		}
		dragPart = PART_NONE
		dragWin = nil
		dragFlow = nil
		activeItem = nil
		downWin = nil
	}

	wx, wy := pointerWheel()
	wheelDelta := point{X: float32(wx), Y: float32(wy)}

	delta := pointSub(mpos, mposOld)
	c := ebiten.CursorShapeDefault

	chars := ebiten.AppendInputChars(inputBuf[:0])

	// First, give active context menus a chance to handle the click/hover.
	if handleContextMenus(mpos, click) {
		pointerPressHandled = click
		// If a context menu handled the interaction, avoid passing the click to windows.
		// Still continue with cursor updates and general housekeeping below.
		click = false
	}

	//Check all windows
	for i := len(windows) - 1; i >= 0; i-- {
		win := windows[i]
		if !win.Open {
			continue
		}

		s := win.scale()
		posCh := point{X: delta.X / s, Y: delta.Y / s}
		sizeCh := posCh

		var part dragType
		handled := false
		if dragPart != PART_NONE && dragWin == win {
			part = dragPart
		} else {
			localPos := point{X: mpos.X / s, Y: mpos.Y / s}
			part = win.getWindowPart(localPos, click)
			if part == PART_NONE && midClick && win.Movable && win.getWinRect().containsPoint(mpos) {
				part = PART_BAR
			}
		}

		if part != PART_NONE {

			if dragPart == PART_NONE && c == ebiten.CursorShapeDefault {
				switch part {
				case PART_BAR:
					c = ebiten.CursorShapeMove
				case PART_LEFT, PART_RIGHT:
					c = ebiten.CursorShapeEWResize
				case PART_TOP, PART_BOTTOM:
					c = ebiten.CursorShapeNSResize
				case PART_TOP_LEFT, PART_BOTTOM_RIGHT:
					c = ebiten.CursorShapeNWSEResize
				case PART_TOP_RIGHT, PART_BOTTOM_LEFT:
					c = ebiten.CursorShapeNESWResize
				case PART_SCROLL_V, PART_SCROLL_H, PART_SEARCH:
					c = ebiten.CursorShapePointer
				}
			}

			if click && dragPart == PART_NONE && downWin == win {
				pointerPressHandled = true
				if part == PART_CLOSE {
					win.Close()
					break
				}
				if part == PART_MAXIMIZE {
					if win.OnMaximize != nil {
						win.OnMaximize()
					} else {
						win.Maximize()
					}
					break
				}
				if part == PART_SEARCH {
					if win.searchOpen {
						win.closeSearch()
					} else {
						win.searchOpen = true
						win.SearchText = ""
						activeSearch = win
						if win.OnSearch != nil {
							win.OnSearch("")
						}
						win.markDirty()
					}
					break
				}
				dragPart = part
				dragWin = win
			} else if midClick && dragPart == PART_NONE && part == PART_BAR && downWin == win {
				dragPart = part
				dragWin = win
			} else if (clickDrag || midClickDrag) && dragPart != PART_NONE && dragWin == win {
				switch dragPart {
				case PART_BAR:
					dragWindowMove(win, posCh)
				case PART_TOP:
					posCh.X = 0
					sizeCh.X = 0
					if win.setSize(pointSub(win.Size, sizeCh)) && win.zone == nil {
						win.Position = pointAdd(win.Position, posCh)
					}

					if win.searchOpen {
						if part == PART_NONE {
							if win.searchBoxRect().containsPoint(mpos) {
								c = ebiten.CursorShapeText
							} else if win.searchCloseRect().containsPoint(mpos) {
								c = ebiten.CursorShapePointer
							}
						}
						if click && dragPart == PART_NONE && downWin == win {
							if win.searchCloseRect().containsPoint(mpos) {
								win.closeSearch()
								break
							}
							if win.searchBoxRect().containsPoint(mpos) {
								activeSearch = win
								break
							}
						}
					}
				case PART_BOTTOM:
					sizeCh.X = 0
					win.setSize(pointAdd(win.Size, sizeCh))
				case PART_LEFT:
					posCh.Y = 0
					sizeCh.Y = 0
					if win.setSize(pointSub(win.Size, sizeCh)) && win.zone == nil {
						win.Position = pointAdd(win.Position, posCh)
					}
				case PART_RIGHT:
					sizeCh.Y = 0
					win.setSize(pointAdd(win.Size, sizeCh))
				case PART_TOP_LEFT:
					if win.setSize(pointSub(win.Size, sizeCh)) && win.zone == nil {
						win.Position = pointAdd(win.Position, posCh)
					}
				case PART_TOP_RIGHT:
					tx := win.Size.X + sizeCh.X
					ty := win.Size.Y - sizeCh.Y
					if win.setSize(point{X: tx, Y: ty}) && win.zone == nil {
						win.Position.Y += posCh.Y
					}
				case PART_BOTTOM_RIGHT:
					tx := win.Size.X + sizeCh.X
					ty := win.Size.Y + sizeCh.Y
					win.setSize(point{X: tx, Y: ty})
				case PART_BOTTOM_LEFT:
					tx := win.Size.X - sizeCh.X
					ty := win.Size.Y + sizeCh.Y
					if win.setSize(point{X: tx, Y: ty}) && win.zone == nil {
						win.Position.X += posCh.X
					}
				case PART_SCROLL_V:
					if dragFlow != nil {
						dragFlowScroll(dragFlow, mpos, true)
					} else {
						dragWindowScroll(win, mpos, true)
					}
				case PART_SCROLL_H:
					if dragFlow != nil {
						dragFlowScroll(dragFlow, mpos, false)
					} else {
						dragWindowScroll(win, mpos, false)
					}
				}
				if dragPart != PART_BAR && dragPart != PART_SCROLL_V && dragPart != PART_SCROLL_H {
					if windowSnapping {
						snapResize(win, dragPart)
					}
				}
				win.clampToScreen()
				if windowSnapping && win.zone == nil {
					snapped := false
					if !win.snapAnchorActive {
						snapped = snapToCorner(win)
					}
					if !snapped && snapToWindow(win) {
						win.clampToScreen()
					}
				}
				break
			}
		} else if win.searchOpen {
			if dragPart == PART_NONE && c == ebiten.CursorShapeDefault {
				if win.searchBoxRect().containsPoint(mpos) {
					c = ebiten.CursorShapeText
				} else if win.searchCloseRect().containsPoint(mpos) {
					c = ebiten.CursorShapePointer
				}
			}
			if click && dragPart == PART_NONE && downWin == win {
				if win.searchCloseRect().containsPoint(mpos) {
					win.closeSearch()
					handled = true
				} else if win.searchBoxRect().containsPoint(mpos) {
					activeSearch = win
					handled = true
				}
			}
		}

		// Window items
		prevWinHovered := win.Hovered
		prevActiveWindow := activeWindow
		win.Hovered = false
		if !handled {
			handled = win.clickWindowItems(mpos, click)
		}
		if click && handled {
			pointerPressHandled = true
		}
		if win.Hovered != prevWinHovered {
			win.markDirty()
		}

		// Bring window forward on click if the cursor is over it or an
		// expanded dropdown. Break so windows behind don't receive the
		// event. The check includes clicks on dropdown menus which may
		// have closed during handling. Also consider context menus so a
		// right-click menu doesn't cause window activation behind it.
		if handled || win.getWinRect().containsPoint(mpos) || dropdownOpenContains(win.Contents, mpos) || contextMenuContainsAnywhere(mpos) {
			if click || midClick {
				if activeWindow == prevActiveWindow {
					if activeWindow != win || windows[len(windows)-1] != win {
						win.BringForward()
					}
				}
			}
			break
		}
	}

	if selectedTextItem != nil && selectedTextItem.selecting && pointerPressed() {
		idx := selectedTextItem.cursorIndexAt(mpos)
		if idx != selectedTextItem.SelectEnd {
			selectedTextItem.SelectEnd = idx
			if selectedTextItem.ItemType == ITEM_INPUT || (selectedTextItem.ItemType == ITEM_TEXT && selectedTextItem.Filled) {
				selectedTextItem.CursorPos = idx
			}
			selectedTextItem.markDirty()
		}
	}

	if hoveredItem != nil && hoveredItem.ItemType == ITEM_TEXT && hoveredItem.OnURLClick != nil && c == ebiten.CursorShapeDefault {
		if hoveredItem.urlAtChar(hoveredItem.cursorIndexAt(mpos)) != "" {
			c = ebiten.CursorShapePointer
		}
	}

	if cursorShape != c {
		ebiten.SetCursorShape(c)
		cursorShape = c
	}

	if (ctrlPressed || metaPressed) && inpututil.IsKeyJustPressed(ebiten.KeyC) && selectedTextItem != nil {
		if selected := selectedTextItem.SelectedText(); selected != "" {
			_, _ = clipboard.Write(context.Background(), clipboard.FmtText, []byte(selected))
		}
	}

	if focusedItem != nil {
		for _, r := range chars {
			if r >= 32 && r != 127 && r != '\r' && r != '\n' {
				if focusedItem.HideText {
					dispRunes := []rune(focusedItem.Text)
					secRunes := []rune(focusedItem.SecretText)
					pos := focusedItem.CursorPos
					dispRunes = append(dispRunes[:pos], append([]rune("*"), dispRunes[pos:]...)...)
					secRunes = append(secRunes[:pos], append([]rune(string(r)), secRunes[pos:]...)...)
					focusedItem.Text = string(dispRunes)
					focusedItem.SecretText = string(secRunes)
					if focusedItem.TextPtr != nil {
						*focusedItem.TextPtr = focusedItem.SecretText
					}
				} else {
					runes := []rune(focusedItem.Text)
					pos := focusedItem.CursorPos
					runes = append(runes[:pos], append([]rune(string(r)), runes[pos:]...)...)
					focusedItem.Text = string(runes)
					if focusedItem.TextPtr != nil {
						*focusedItem.TextPtr = focusedItem.Text
					}
				}
				focusedItem.CursorPos++
				focusedItem.markDirty()
				if focusedItem.Handler != nil {
					focusedItem.Handler.Emit(UIEvent{Item: focusedItem, Type: EventInputChanged, Text: focusedItem.Text})
				}
			}
		}

		if ctrlPressed && inpututil.IsKeyJustPressed(ebiten.KeyV) {
			if txt, err := clipboard.Read(context.Background(), clipboard.FmtText); err == nil && len(txt) > 0 {
				runes := []rune(string(txt))
				pos := focusedItem.CursorPos
				if focusedItem.HideText {
					dispRunes := []rune(focusedItem.Text)
					secRunes := []rune(focusedItem.SecretText)
					dispInsert := []rune(strings.Repeat("*", len(runes)))
					dispRunes = append(dispRunes[:pos], append(dispInsert, dispRunes[pos:]...)...)
					secRunes = append(secRunes[:pos], append(runes, secRunes[pos:]...)...)
					focusedItem.Text = string(dispRunes)
					focusedItem.SecretText = string(secRunes)
					if focusedItem.TextPtr != nil {
						*focusedItem.TextPtr = focusedItem.SecretText
					}
				} else {
					tRunes := []rune(focusedItem.Text)
					tRunes = append(tRunes[:pos], append(runes, tRunes[pos:]...)...)
					focusedItem.Text = string(tRunes)
					if focusedItem.TextPtr != nil {
						*focusedItem.TextPtr = focusedItem.Text
					}
				}
				focusedItem.CursorPos += len(runes)
				focusedItem.markDirty()
				if focusedItem.Handler != nil {
					focusedItem.Handler.Emit(UIEvent{Item: focusedItem, Type: EventInputChanged, Text: focusedItem.Text})
				}
			}
		}
		if (ctrlPressed || metaPressed) && inpututil.IsKeyJustPressed(ebiten.KeyC) {
			text := focusedItem.Text
			if focusedItem.HideText {
				text = focusedItem.SecretText
			}
			_, _ = clipboard.Write(context.Background(), clipboard.FmtText, []byte(text))
		}

		if inpututil.IsKeyJustPressed(ebiten.KeyBackspace) {
			pos := focusedItem.CursorPos
			runes := []rune(focusedItem.Text)
			if pos > 0 && len(runes) > 0 {
				if focusedItem.HideText {
					secRunes := []rune(focusedItem.SecretText)
					secRunes = append(secRunes[:pos-1], secRunes[pos:]...)
					focusedItem.SecretText = string(secRunes)
					dispRunes := append(runes[:pos-1], runes[pos:]...)
					focusedItem.Text = string(dispRunes)
					if focusedItem.TextPtr != nil {
						*focusedItem.TextPtr = focusedItem.SecretText
					}
				} else {
					runes = append(runes[:pos-1], runes[pos:]...)
					focusedItem.Text = string(runes)
					if focusedItem.TextPtr != nil {
						*focusedItem.TextPtr = focusedItem.Text
					}
				}
				focusedItem.CursorPos--
				focusedItem.markDirty()
				if focusedItem.Handler != nil {
					focusedItem.Handler.Emit(UIEvent{Item: focusedItem, Type: EventInputChanged, Text: focusedItem.Text})
				}
			}
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyArrowLeft) {
			if focusedItem.CursorPos > 0 {
				focusedItem.CursorPos--
				focusedItem.markDirty()
			}
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyArrowRight) {
			if focusedItem.CursorPos < len([]rune(focusedItem.Text)) {
				focusedItem.CursorPos++
				focusedItem.markDirty()
			}
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyArrowUp) {
			runes := []rune(focusedItem.Text)
			pos := focusedItem.CursorPos
			lineStart := pos
			for lineStart > 0 && runes[lineStart-1] != '\n' {
				lineStart--
			}
			if lineStart > 0 {
				col := pos - lineStart
				prevEnd := lineStart - 1
				prevStart := prevEnd
				for prevStart > 0 && runes[prevStart-1] != '\n' {
					prevStart--
				}
				prevLen := prevEnd - prevStart
				newPos := prevStart + col
				if col > prevLen {
					newPos = prevStart + prevLen
				}
				focusedItem.CursorPos = newPos
				focusedItem.markDirty()
			}
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyArrowDown) {
			runes := []rune(focusedItem.Text)
			pos := focusedItem.CursorPos
			lineStart := pos
			for lineStart > 0 && runes[lineStart-1] != '\n' {
				lineStart--
			}
			lineEnd := pos
			for lineEnd < len(runes) && runes[lineEnd] != '\n' {
				lineEnd++
			}
			if lineEnd < len(runes) {
				col := pos - lineStart
				nextStart := lineEnd + 1
				nextEnd := nextStart
				for nextEnd < len(runes) && runes[nextEnd] != '\n' {
					nextEnd++
				}
				nextLen := nextEnd - nextStart
				newPos := nextStart + col
				if col > nextLen {
					newPos = nextStart + nextLen
				}
				focusedItem.CursorPos = newPos
				focusedItem.markDirty()
			}
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
			focusedItem.Focused = false
			focusedItem.markDirty()
			focusedItem = nil
		}
	}

	if activeSearch != nil {
		for _, r := range chars {
			if r >= 32 && r != 127 && r != '\r' && r != '\n' {
				if len([]rune(activeSearch.SearchText)) < 64 {
					activeSearch.SearchText += string(r)
					if activeSearch.OnSearch != nil {
						activeSearch.OnSearch(activeSearch.SearchText)
					}
					activeSearch.markDirty()
				}
			}
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyBackspace) {
			runes := []rune(activeSearch.SearchText)
			if len(runes) > 0 {
				activeSearch.SearchText = string(runes[:len(runes)-1])
				if activeSearch.OnSearch != nil {
					activeSearch.OnSearch(activeSearch.SearchText)
				}
				activeSearch.markDirty()
			}
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
			activeSearch.searchOpen = false
			activeSearch.markDirty()
			activeSearch = nil
		}
	} else {
		if inpututil.IsKeyJustPressed(ebiten.KeyEnter) || inpututil.IsKeyJustPressed(ebiten.KeyKPEnter) {
			if activeWindow != nil && activeWindow.Open && activeWindow.DefaultButton != nil {
				btn := activeWindow.DefaultButton
				if !btn.Disabled && !btn.Invisible {
					activeItem = btn
					btn.Clicked = updateNow
					if btn.Handler != nil {
						btn.Handler.Emit(UIEvent{Item: btn, Type: EventClick})
					}
					btn.markDirty()
				}
			}
		}

		if inpututil.IsKeyJustPressed(ebiten.KeyTab) {
			if activeWindow != nil && activeWindow.Open {
				var inputs []*itemData
				collectInputs(activeWindow.Contents, &inputs)
				if len(inputs) > 0 {
					idx := -1
					for i, it := range inputs {
						if it == focusedItem {
							idx = i
							break
						}
					}
					next := 0
					if shiftPressed {
						if idx == -1 {
							next = len(inputs) - 1
						} else {
							next = (idx - 1 + len(inputs)) % len(inputs)
						}
					} else {
						if idx == -1 {
							next = 0
						} else {
							next = (idx + 1) % len(inputs)
						}
					}
					if focusedItem != nil {
						focusedItem.Focused = false
						focusedItem.markDirty()
					}
					focusedItem = inputs[next]
					focusedItem.CursorPos = len([]rune(focusedItem.Text))
					focusedItem.Focused = true
					focusedItem.markDirty()
				}
			}
		}
	}

	mposOld = mpos

	if wheelDelta.X != 0 || wheelDelta.Y != 0 {
		for i := len(windows) - 1; i >= 0; i-- {
			win := windows[i]
			if !win.Open {
				continue
			}
			// Give context menus first chance at scroll.
			if scrollContextMenus(mpos, wheelDelta) {
				break
			}
			if win.getMainRect().containsPoint(mpos) || dropdownOpenContains(win.Contents, mpos) {
				if scrollDropdown(win.Contents, mpos, wheelDelta) {
					break
				}
				if scrollFlow(win.Contents, mpos, wheelDelta) {
					break
				}
				if scrollWindow(win, wheelDelta) {
					break
				}
			}
		}
	}

	if hoveredItem != prevHovered {
		if prevHovered != nil {
			prevHovered.Hovered = false
			prevHovered.markDirty()
		}
	}

	// Refresh flow layouts only when needed. Constantly recalculating
	// layouts is expensive and can noticeably slow down the WebAssembly
	// build, especially on HiDPI screens. Windows and overlays handle their
	// own layout updates whenever sizes change, so avoid doing it every
	// frame here.

	for _, win := range windows {
		if win.Open {
			clearExpiredClicksAt(win.Contents, updateNow)
		}
	}

	return nil
}

func (win *windowData) clickWindowItems(mpos point, click bool) bool {
	// If the mouse isn't within the window or any open dropdown, just return
	if !win.getMainRect().containsPoint(mpos) && !dropdownOpenContains(win.Contents, mpos) {
		return false
	}
	if clickOpenDropdown(win.Contents, mpos, click) {
		return true
	}
	win.Hovered = true

	// Hit test in reverse draw order so later (visually-on-top) items
	// receive input before earlier ones when their rects overlap.
	for i := len(win.Contents) - 1; i >= 0; i-- {
		item := win.Contents[i]
		handled := false
		if item.ItemType == ITEM_FLOW {
			if part := item.getScrollbarPart(mpos); part != PART_NONE {
				if click && dragPart == PART_NONE && downWin == win {
					dragPart = part
					dragWin = win
					dragFlow = item
				}
				return true
			}
			handled = item.clickFlows(mpos, click)
		} else {
			handled = item.clickItem(mpos, click)
		}
		if handled {
			return true
		}
	}
	return false
}

func (item *itemData) clickFlows(mpos point, click bool) bool {
	if item.Disabled {
		return item.DrawRect.containsPoint(mpos)
	}
	if part := item.getScrollbarPart(mpos); part != PART_NONE {
		if click && dragPart == PART_NONE && downWin == item.ParentWindow {
			dragPart = part
			dragWin = item.ParentWindow
			dragFlow = item
		}
		return true
	}
	if len(item.Tabs) > 0 {
		if item.ActiveTab >= len(item.Tabs) {
			item.ActiveTab = 0
		}
		for i, tab := range item.Tabs {
			tab.Hovered = false
			if tab.DrawRect.containsPoint(mpos) {
				tab.Hovered = true
				hoveredItem = tab
				if click {
					activeItem = tab
					tab.Clicked = updateNow
					item.ActiveTab = i
				}
				return true
			}
		}
		for i := len(item.Tabs[item.ActiveTab].Contents) - 1; i >= 0; i-- {
			subItem := item.Tabs[item.ActiveTab].Contents[i]
			if subItem.ItemType == ITEM_FLOW {
				if subItem.clickFlows(mpos, click) {
					return true
				}
			} else {
				if subItem.clickItem(mpos, click) {
					return true
				}
			}
		}
	} else {
		for i := len(item.Contents) - 1; i >= 0; i-- {
			subItem := item.Contents[i]
			if subItem.ItemType == ITEM_FLOW {
				if subItem.clickFlows(mpos, click) {
					return true
				}
			} else {
				if subItem.clickItem(mpos, click) {
					return true
				}
			}
		}
	}
	return item.DrawRect.containsPoint(mpos)
}

func (item *itemData) clickItem(mpos point, click bool) bool {
	if item.Disabled {
		return item.DrawRect.containsPoint(mpos)
	}
	if pointerPressed() && activeItem != nil && activeItem != item {
		return false
	}
	// For dropdowns check the expanded option area as well
	if !item.DrawRect.containsPoint(mpos) {
		if !(item.ItemType == ITEM_DROPDOWN && item.Open && func() bool {
			r, _ := dropdownOpenRect(item, point{X: item.DrawRect.X0, Y: item.DrawRect.Y0})
			return r.containsPoint(mpos)
		}()) {
			return false
		}
	}

	if click {
		activeItem = item
		item.Clicked = updateNow
		if (item.ItemType == ITEM_TEXT || item.ItemType == ITEM_INPUT) && item.SelectableText {
			idx := item.cursorIndexAt(mpos)
			item.SelectStart = idx
			item.SelectEnd = idx
			item.selecting = true
			selectedTextItem = item
			if item.ItemType == ITEM_INPUT || item.Filled {
				focusedItem = item
				item.CursorPos = idx
				item.Focused = true
			}
			item.markDirty()
			if item.OnURLClick != nil {
				if url := item.urlAtChar(idx); url != "" {
					item.OnURLClick(url)
				}
			}
			return true
		}
		if item.ItemType == ITEM_SLIDER {
			// Record drag start state for precision-drag (Ctrl) behavior
			item.dragStart = mpos
			item.dragStartValue = item.Value
			item.dragStartInit = true
		}
		if item.ItemType == ITEM_BUTTON && item.Handler != nil {
			item.Handler.Emit(UIEvent{Item: item, Type: EventClick})
		}
		item.markDirty()
		if item.ItemType == ITEM_COLORWHEEL {
			if col, ok := item.colorAt(mpos); ok {
				item.WheelColor = col
				item.markDirty()
				if item.Handler != nil {
					item.Handler.Emit(UIEvent{Item: item, Type: EventColorChanged, Color: col})
				}
				if item.OnColorChange != nil {
					item.OnColorChange(col)
				} else {
					SetAccentColor(col)
				}
			}
		}
		if item.ItemType == ITEM_CHECKBOX {
			item.Checked = !item.Checked
			item.markDirty()
			if item.Handler != nil {
				item.Handler.Emit(UIEvent{Item: item, Type: EventCheckboxChanged, Checked: item.Checked})
			}
		} else if item.ItemType == ITEM_RADIO {
			item.Checked = true
			// uncheck others in group
			if item.RadioGroup != "" {
				uncheckRadioGroup(item.Parent, item.RadioGroup, item)
			}
			item.markDirty()
			if item.Handler != nil {
				item.Handler.Emit(UIEvent{Item: item, Type: EventRadioSelected, Checked: true})
			}
		} else if item.ItemType == ITEM_INPUT || (item.ItemType == ITEM_TEXT && item.Filled) {
			focusedItem = item
			focusedItem.CursorPos = item.cursorIndexAt(mpos)
			item.Focused = true
			item.markDirty()
		} else if item.ItemType == ITEM_DROPDOWN {
			if item.Open {
				optionH := item.GetSize().Y
				r, _ := dropdownOpenRect(item, point{X: item.DrawRect.X0, Y: item.DrawRect.Y0})
				startY := r.Y0
				if r.containsPoint(mpos) {
					idx := int((mpos.Y - startY + item.Scroll.Y) / optionH)
					if idx >= 0 && idx < len(item.Options) {
						item.Selected = idx
						item.Open = false
						item.markDirty()
						if item.Handler != nil {
							item.Handler.Emit(UIEvent{Item: item, Type: EventDropdownSelected, Index: idx})
						}
						if item.OnSelect != nil {
							item.OnSelect(idx)
						}
					}
				} else {
					item.Open = false
					item.markDirty()
				}
			} else {
				item.Open = true
				item.markDirty()
			}
		}
		if item.Action != nil {
			item.Action()
			return true
		}
	} else {
		if !item.Hovered {
			item.Hovered = true
			item.markDirty()
		}
		hoveredItem = item
		if item.ItemType == ITEM_COLORWHEEL && pointerPressed() && downWin == item.ParentWindow {
			if col, ok := item.colorAt(mpos); ok {
				item.WheelColor = col
				item.markDirty()
				if item.Handler != nil {
					item.Handler.Emit(UIEvent{Item: item, Type: EventColorChanged, Color: col})
				}
				if item.OnColorChange != nil {
					item.OnColorChange(col)
				} else {
					SetAccentColor(col)
				}
			}
		} else if item.ItemType == ITEM_DROPDOWN && item.Open {
			optionH := item.GetSize().Y
			r, _ := dropdownOpenRect(item, point{X: item.DrawRect.X0, Y: item.DrawRect.Y0})
			startY := r.Y0
			if r.containsPoint(mpos) {
				idx := int((mpos.Y - startY + item.Scroll.Y) / optionH)
				if idx >= 0 && idx < len(item.Options) {
					if idx != item.HoverIndex {
						item.HoverIndex = idx
						item.markDirty()
						if item.OnHover != nil {
							item.OnHover(idx)
						}
					}
				}
			} else {
				if item.HoverIndex != -1 {
					item.HoverIndex = -1
					item.markDirty()
					if item.OnHover != nil {
						item.OnHover(item.Selected)
					}
				}
			}
		}
		if item.ItemType == ITEM_SLIDER && pointerPressed() && downWin == item.ParentWindow {
			// Modifiers: Shift snaps to integer; Ctrl = fine adjust (1/8th).
			moveScale := float32(1)
			if CtrlPressed {
				moveScale = 1.0 / 8.0
			}
			// Only trigger Action when the value actually changes.
			old := item.Value
			item.setSliderValueWithModifiers(mpos, ShiftPressed, moveScale)
			item.markDirty()
			if item.Action != nil && item.Value != old {
				item.Action()
			}
		}
	}
	return true
}

var urlRe = regexp.MustCompile(`https?://[^\s<>"')\]]+`)

// urlRanges returns rune-indexed HTTP(S) URL spans in the item text.
func (item *itemData) urlRanges() []TextSpan {
	var spans []TextSpan
	for _, match := range urlRe.FindAllStringIndex(item.Text, -1) {
		start := len([]rune(item.Text[:match[0]]))
		end := start + len([]rune(item.Text[match[0]:match[1]]))
		spans = append(spans, TextSpan{Start: start, End: end})
	}
	return spans
}

func (item *itemData) urlAtChar(index int) string {
	for _, span := range item.urlRanges() {
		if index >= span.Start && index < span.End {
			return string([]rune(item.Text)[span.Start:span.End])
		}
	}
	return ""
}

// SelectedText returns the currently selected text range for this item.
func (item *itemData) SelectedText() string {
	if item == nil || item.SelectStart == item.SelectEnd {
		return ""
	}
	runes := []rune(item.Text)
	start, end := item.SelectStart, item.SelectEnd
	if start > end {
		start, end = end, start
	}
	if start < 0 {
		start = 0
	}
	if end > len(runes) {
		end = len(runes)
	}
	if start >= end {
		return ""
	}
	return string(runes[start:end])
}

func clearTextSelection() {
	if selectedTextItem == nil {
		return
	}
	selectedTextItem.SelectStart = 0
	selectedTextItem.SelectEnd = 0
	selectedTextItem.selecting = false
	selectedTextItem.markDirty()
	selectedTextItem = nil
}

func (item *itemData) cursorIndexAt(mpos point) int {
	textSize := (item.FontSize * uiScale) + 2
	face := itemFace(item, textSize)
	lines := strings.Split(item.Text, "\n")
	metrics := face.Metrics()
	lineHeight := float32(math.Ceil(metrics.HAscent + metrics.HDescent + 2))
	x := mpos.X - item.DrawRect.X0
	y := mpos.Y - item.DrawRect.Y0
	if x < 0 {
		x = 0
	}
	if y < 0 {
		y = 0
	}
	line := int(y / lineHeight)
	if line < 0 {
		line = 0
	}
	if line >= len(lines) {
		line = len(lines) - 1
	}
	pos := 0
	for i := 0; i < line; i++ {
		pos += len([]rune(lines[i]))
		pos++
	}
	runes := []rune(lines[line])
	advance := float32(0)
	for i, r := range runes {
		w, _ := text.Measure(string(r), face, 0)
		if x < advance+float32(w)/2 {
			return pos + i
		}
		advance += float32(w)
	}
	return pos + len(runes)
}

func uncheckRadioGroup(parent *itemData, group string, except *itemData) {
	if parent == nil {
		for _, win := range windows {
			subUncheckRadio(win.Contents, group, except)
		}
	} else {
		subUncheckRadio(parent.Contents, group, except)
	}
}

func subUncheckRadio(list []*itemData, group string, except *itemData) {
	for _, it := range list {
		if it.ItemType == ITEM_RADIO && it.RadioGroup == group && it != except {
			if it.Checked {
				it.Checked = false
				it.markDirty()
			}
		}
		if len(it.Tabs) > 0 {
			for _, tab := range it.Tabs {
				subUncheckRadio(tab.Contents, group, except)
			}
		}
		subUncheckRadio(it.Contents, group, except)
	}
}

func clearExpiredClicksAt(list []*itemData, now time.Time) {
	for _, it := range list {
		if !it.Clicked.IsZero() && now.Sub(it.Clicked) >= clickFlash {
			it.Clicked = time.Time{}
			it.markDirty()
		}
		for _, tab := range it.Tabs {
			if !tab.Clicked.IsZero() && now.Sub(tab.Clicked) >= clickFlash {
				tab.Clicked = time.Time{}
				tab.markDirty()
			}
			clearExpiredClicksAt(tab.Contents, now)
		}
		clearExpiredClicksAt(it.Contents, now)
	}
}

// setSliderValueWithModifiers adjusts slider value honoring Shift/Ctrl behavior.
// shiftSnap: when true, snap to nearest integer.
// moveScale: scales movement toward the pointer target (e.g., 1/8 for fine adjust).
func (item *itemData) setSliderValueWithModifiers(mpos point, shiftSnap bool, moveScale float32) {
	// Compute target from pointer using the base maths.
	old := item.Value
	// Recompute the target locally so we can adjust it before emitting events.
	if item.Vertical {
		knobH := item.AuxSize.Y * uiScale
		height := item.DrawRect.Y1 - item.DrawRect.Y0 - knobH
		if height <= 0 {
			return
		}
		start := item.DrawRect.Y0 + knobH/2
		end := start + height
		val := end - mpos.Y
		if val < 0 {
			val = 0
		}
		if val > height {
			val = height
		}
		ratio := val / height
		target := item.MinValue + ratio*(item.MaxValue-item.MinValue)
		// Fine adjust by moving fraction toward target
		if moveScale != 1 {
			target = old + (target-old)*moveScale
		}
		if shiftSnap {
			target = float32(int(target + 0.5))
		} else if item.IntOnly {
			target = float32(int(target + 0.5))
		}
		if target != item.Value {
			item.Value = target
			item.markDirty()
			if item.Handler != nil {
				item.Handler.Emit(UIEvent{Item: item, Type: EventSliderChanged, Value: item.Value})
			}
		}
		return
	}

	// Horizontal
	maxLabel := sliderMaxLabel
	textSize := (item.FontSize * uiScale) + 2
	face := textFace(textSize)
	maxW, _ := text.Measure(maxLabel, face, 0)
	knobW := item.AuxSize.X * uiScale
	gap := currentStyle.SliderValueGap
	width := item.DrawRect.X1 - item.DrawRect.X0 - knobW - gap - float32(maxW)
	if width < knobW {
		width = item.DrawRect.X1 - item.DrawRect.X0 - knobW
		if width < 0 {
			width = 0
		}
	}
	if width <= 0 {
		return
	}
	start := item.DrawRect.X0 + knobW/2
	val := (mpos.X - start)
	if val < 0 {
		val = 0
	}
	if val > width {
		val = width
	}
	ratio := val / width
	target := item.MinValue + ratio*(item.MaxValue-item.MinValue)
	if moveScale != 1 {
		target = old + (target-old)*moveScale
	}
	if shiftSnap {
		target = float32(int(target + 0.5))
	} else if item.IntOnly {
		target = float32(int(target + 0.5))
	}
	if target != item.Value {
		item.Value = target
		item.markDirty()
		if item.Handler != nil {
			item.Handler.Emit(UIEvent{Item: item, Type: EventSliderChanged, Value: item.Value})
		}
	}
}

func (item *itemData) colorAt(mpos point) (Color, bool) {
	size := point{X: item.Size.X * uiScale, Y: item.Size.Y * uiScale}
	offsetY := float32(0)
	if item.Label != "" {
		offsetY = (item.FontSize*uiScale + 2) + currentStyle.TextPadding*uiScale
	}
	wheelSize := size.Y
	if wheelSize > size.X {
		wheelSize = size.X
	}
	radius := wheelSize / 2
	cx := item.DrawRect.X0 + radius
	cy := item.DrawRect.Y0 + offsetY + radius
	dx := float64(mpos.X - cx)
	dy := float64(mpos.Y - cy)
	r := float64(radius)
	dist := math.Hypot(dx, dy)

	if !item.DrawRect.containsPoint(mpos) {
		return Color{}, false
	}
	if dist > r {
		dist = r
	}

	ang := math.Atan2(dy, dx) * 180 / math.Pi
	if ang < 0 {
		ang += 360
	}
	v := dist / r
	if v < 0 {
		v = 0
	} else if v > 1 {
		v = 1
	}
	col := hsvaToRGBA(ang, 1, v, 1)
	return Color(col), true
}

func scrollFlow(items []*itemData, mpos point, delta point) bool {
	for _, it := range items {
		if it.Disabled {
			continue
		}
		if it.ItemType == ITEM_FLOW {
			if it.DrawRect.containsPoint(mpos) {
				req := it.contentBounds()
				size := it.GetSize()
				old := it.Scroll
				if it.Scrollable {
					if it.FlowType == FLOW_VERTICAL && req.Y > size.Y {
						it.Scroll.Y -= delta.Y * 16
						if it.Scroll.Y < 0 {
							it.Scroll.Y = 0
						}
						slack := 4 * UIScale()
						max := req.Y - size.Y + slack
						if it.Scroll.Y > max {
							it.Scroll.Y = max
						}
						if it.Scroll != old && it.ParentWindow != nil {
							it.ParentWindow.markDirty()
						}
						return true
					} else if it.FlowType == FLOW_HORIZONTAL && req.X > size.X {
						it.Scroll.X -= delta.X * 16
						if it.Scroll.X < 0 {
							it.Scroll.X = 0
						}
						slack := 4 * UIScale()
						max := req.X - size.X + slack
						if it.Scroll.X > max {
							it.Scroll.X = max
						}
						if it.Scroll != old && it.ParentWindow != nil {
							it.ParentWindow.markDirty()
						}
						return true
					}
				} else {
					if req.Y <= size.Y {
						it.Scroll.Y = 0
					}
					if req.X <= size.X {
						it.Scroll.X = 0
					}
				}
			}
			var sub []*itemData
			if len(it.Tabs) > 0 {
				if it.ActiveTab >= len(it.Tabs) {
					it.ActiveTab = 0
				}
				sub = it.Tabs[it.ActiveTab].Contents
			} else {
				sub = it.Contents
			}
			if scrollFlow(sub, mpos, delta) {
				return true
			}
		}
	}
	return false
}

func scrollDropdown(items []*itemData, mpos point, delta point) bool {
	for _, it := range items {
		if it.Disabled {
			continue
		}
		if it.ItemType == ITEM_DROPDOWN && it.Open {
			optionH := it.GetSize().Y
			r, _ := dropdownOpenRect(it, point{X: it.DrawRect.X0, Y: it.DrawRect.Y0})
			openH := r.Y1 - r.Y0
			if r.containsPoint(mpos) {
				maxScroll := optionH*float32(len(it.Options)) - openH
				if maxScroll < 0 {
					maxScroll = 0
				}
				// Use the same scaling as window scrolling for a
				// consistent feel across widgets.
				old := it.Scroll
				it.Scroll.Y -= delta.Y * 16
				if it.Scroll.Y < 0 {
					it.Scroll.Y = 0
				}
				if it.Scroll.Y > maxScroll {
					it.Scroll.Y = maxScroll
				}
				if it.Scroll != old && it.ParentWindow != nil {
					it.ParentWindow.markDirty()
				}
				return true
			}
		}
		if len(it.Tabs) > 0 {
			if it.ActiveTab >= len(it.Tabs) {
				it.ActiveTab = 0
			}
			if scrollDropdown(it.Tabs[it.ActiveTab].Contents, mpos, delta) {
				return true
			}
		}
		if scrollDropdown(it.Contents, mpos, delta) {
			return true
		}
	}
	return false
}

func scrollWindow(win *windowData, delta point) bool {
	if win.NoScroll {
		return false
	}
	pad := (win.Padding + win.BorderPad) * win.scale()
	req := win.contentBounds()
	avail := point{
		X: win.GetSize().X - 2*pad,
		Y: win.GetSize().Y - win.GetTitleSize() - 2*pad,
	}
	old := win.Scroll
	handled := false
	if req.Y > avail.Y {
		win.Scroll.Y -= delta.Y * 16
		if win.Scroll.Y < 0 {
			win.Scroll.Y = 0
		}
		slack := 4 * UIScale()
		max := req.Y - avail.Y + slack
		if win.Scroll.Y > max {
			win.Scroll.Y = max
		}
		handled = true
	} else {
		win.Scroll.Y = 0
	}
	if req.X > avail.X {
		win.Scroll.X -= delta.X * 16
		if win.Scroll.X < 0 {
			win.Scroll.X = 0
		}
		slack := 4 * UIScale()
		max := req.X - avail.X + slack
		if win.Scroll.X > max {
			win.Scroll.X = max
		}
		handled = true
	} else {
		win.Scroll.X = 0
	}
	if handled || win.Scroll != old {
		win.markDirty()
	}
	return handled
}

func dragWindowMove(win *windowData, delta point) {
	if win.zone != nil && win.Movable {
		win.ClearZone()
	}
	if win.zone == nil {
		win.Position = pointAdd(win.Position, delta)
		if windowSnapping && win.snapAnchorActive {
			dx := float32(math.Abs(float64(win.Position.X - win.snapAnchor.X)))
			dy := float32(math.Abs(float64(win.Position.Y - win.snapAnchor.Y)))
			if dx > UnsnapThreshold || dy > UnsnapThreshold {
				win.snapAnchorActive = false
			}
		}
	}
}

func dragWindowScroll(win *windowData, mpos point, vert bool) {
	if win.NoScroll {
		return
	}
	old := win.Scroll
	pad := (win.Padding + win.BorderPad) * win.scale()
	req := win.contentBounds()
	avail := point{
		X: win.GetSize().X - 2*pad,
		Y: win.GetSize().Y - win.GetTitleSize() - 2*pad,
	}
	if vert && req.Y > avail.Y {
		barH := scrollbarThumbLength(avail.Y, req.Y, win.scale())
		slack := 4 * UIScale()
		maxScroll := req.Y - avail.Y + slack
		track := win.getPosition().Y + win.GetTitleSize() + win.BorderPad*win.scale()
		pos := mpos.Y - (track + barH/2)
		if pos < 0 {
			pos = 0
		}
		if pos > avail.Y-barH {
			pos = avail.Y - barH
		}
		if avail.Y-barH > 0 {
			win.Scroll.Y = (pos / (avail.Y - barH)) * maxScroll
		} else {
			win.Scroll.Y = 0
		}
	} else if vert {
		win.Scroll.Y = 0
	}
	if !vert && req.X > avail.X {
		barW := scrollbarThumbLength(avail.X, req.X, win.scale())
		slack := 4 * UIScale()
		maxScroll := req.X - avail.X + slack
		track := win.getPosition().X + win.BorderPad*win.scale()
		pos := mpos.X - (track + barW/2)
		if pos < 0 {
			pos = 0
		}
		if pos > avail.X-barW {
			pos = avail.X - barW
		}
		if avail.X-barW > 0 {
			win.Scroll.X = (pos / (avail.X - barW)) * maxScroll
		} else {
			win.Scroll.X = 0
		}
	} else if !vert {
		win.Scroll.X = 0
	}
	if win.Scroll != old {
		win.markDirty()
	}
}

func dragFlowScroll(flow *itemData, mpos point, vert bool) {
	if !flow.Scrollable {
		return
	}
	old := flow.Scroll
	req := flow.contentBounds()
	size := flow.GetSize()
	if vert && flow.FlowType == FLOW_VERTICAL && req.Y > size.Y {
		barH := scrollbarThumbLength(size.Y, req.Y, UIScale())
		slack := 4 * UIScale()
		maxScroll := req.Y - size.Y + slack
		track := flow.DrawRect.Y0
		pos := mpos.Y - (track + barH/2)
		if pos < 0 {
			pos = 0
		}
		if pos > size.Y-barH {
			pos = size.Y - barH
		}
		if size.Y-barH > 0 {
			flow.Scroll.Y = (pos / (size.Y - barH)) * maxScroll
		} else {
			flow.Scroll.Y = 0
		}
	} else if vert {
		flow.Scroll.Y = 0
	}
	if !vert && flow.FlowType == FLOW_HORIZONTAL && req.X > size.X {
		barW := scrollbarThumbLength(size.X, req.X, UIScale())
		slack := 4 * UIScale()
		maxScroll := req.X - size.X + slack
		track := flow.DrawRect.X0
		pos := mpos.X - (track + barW/2)
		if pos < 0 {
			pos = 0
		}
		if pos > size.X-barW {
			pos = size.X - barW
		}
		if size.X-barW > 0 {
			flow.Scroll.X = (pos / (size.X - barW)) * maxScroll
		} else {
			flow.Scroll.X = 0
		}
	} else if !vert {
		flow.Scroll.X = 0
	}
	if flow.Scroll != old && flow.ParentWindow != nil {
		flow.ParentWindow.markDirty()
	}
}
func dropdownOpenContains(items []*itemData, mpos point) bool {
	for _, it := range items {
		if it.ItemType == ITEM_DROPDOWN && it.Open {
			r, _ := dropdownOpenRect(it, point{X: it.DrawRect.X0, Y: it.DrawRect.Y0})
			if r.containsPoint(mpos) {
				return true
			}
		}
		if len(it.Tabs) > 0 {
			if it.ActiveTab >= len(it.Tabs) {
				it.ActiveTab = 0
			}
			if dropdownOpenContains(it.Tabs[it.ActiveTab].Contents, mpos) {
				return true
			}
		}
		if dropdownOpenContains(it.Contents, mpos) {
			return true
		}
	}
	return false
}

func clickOpenDropdown(items []*itemData, mpos point, click bool) bool {
	for _, it := range items {
		if it.ItemType == ITEM_DROPDOWN && it.Open {
			r, _ := dropdownOpenRect(it, point{X: it.DrawRect.X0, Y: it.DrawRect.Y0})
			if r.containsPoint(mpos) {
				it.clickItem(mpos, click)
				return true
			}
		}
		if len(it.Tabs) > 0 {
			if it.ActiveTab >= len(it.Tabs) {
				it.ActiveTab = 0
			}
			if clickOpenDropdown(it.Tabs[it.ActiveTab].Contents, mpos, click) {
				return true
			}
		}
		if clickOpenDropdown(it.Contents, mpos, click) {
			return true
		}
	}
	return false
}

func dropdownOpenContainsAnywhere(mpos point) bool {
	for _, win := range windows {
		if win.Open && dropdownOpenContains(win.Contents, mpos) {
			return true
		}
	}
	return false
}

func closeDropdowns(items []*itemData) {
	for _, it := range items {
		if it.ItemType == ITEM_DROPDOWN {
			if it.Open {
				it.Open = false
				it.markDirty()
			}
		}
		for _, tab := range it.Tabs {
			closeDropdowns(tab.Contents)
		}
		closeDropdowns(it.Contents)
	}
}

func closeAllDropdowns() {
	for _, win := range windows {
		if win.Open {
			closeDropdowns(win.Contents)
		}
	}
}

func collectInputs(items []*itemData, inputs *[]*itemData) {
	for _, it := range items {
		if it.ItemType == ITEM_INPUT && !it.Disabled && !it.Invisible {
			*inputs = append(*inputs, it)
		}
		if it.ItemType == ITEM_FLOW {
			if len(it.Tabs) > 0 {
				if it.ActiveTab >= len(it.Tabs) {
					it.ActiveTab = 0
				}
				collectInputs(it.Tabs[it.ActiveTab].Contents, inputs)
			} else {
				collectInputs(it.Contents, inputs)
			}
		} else {
			collectInputs(it.Contents, inputs)
		}
	}
}

// ---- Context menu helpers (overlay, screen-space) ----

// handleContextMenus processes hover and clicks for active context menus. Returns true if handled.
func handleContextMenus(mpos point, click bool) bool {
	handled := false
	for _, cm := range contextMenus {
		if cm == nil || !cm.Open {
			continue
		}
		// Compute the open rectangle at the stored origin.
		r, _ := dropdownOpenRect(cm, point{X: cm.DrawRect.X0, Y: cm.DrawRect.Y0})
		if r.containsPoint(mpos) {
			handled = true
			optionH := cm.GetSize().Y
			startY := r.Y0
			idx := int((mpos.Y - startY + cm.Scroll.Y) / optionH)
			if idx >= 0 && idx < len(cm.Options) {
				// Treat header rows as non-selectable: ignore hover highlight
				// and do not close or invoke selection callbacks.
				if idx < cm.HeaderCount {
					if !click {
						if cm.HoverIndex != -1 {
							cm.HoverIndex = -1
							cm.markDirty()
						}
					}
					// Do not close menus; simply consume the hover/click area.
					return true
				}
				if click {
					cm.Selected = idx
					cm.Open = false
					// Close the selected menu before invoking its callback so the
					// callback can open a follow-up menu without it being discarded.
					CloseContextMenus()
					if cm.OnSelect != nil {
						cm.OnSelect(idx)
					}
				} else {
					if idx != cm.HoverIndex {
						cm.HoverIndex = idx
						// markDirty not required for overlays, but keeps parity.
						cm.markDirty()
						if cm.OnHover != nil {
							cm.OnHover(idx)
						}
					}
				}
			}
		} else if !click {
			if cm.HoverIndex != -1 {
				cm.HoverIndex = -1
				cm.markDirty()
				if cm.OnHover != nil {
					cm.OnHover(cm.Selected)
				}
			}
		}
	}
	return handled
}

func contextMenuContainsAnywhere(mpos point) bool {
	for _, cm := range contextMenus {
		if cm == nil || !cm.Open {
			continue
		}
		r, _ := dropdownOpenRect(cm, point{X: cm.DrawRect.X0, Y: cm.DrawRect.Y0})
		if r.containsPoint(mpos) {
			return true
		}
	}
	return false
}

func scrollContextMenus(mpos point, delta point) bool {
	for _, cm := range contextMenus {
		if cm == nil || !cm.Open {
			continue
		}
		optionH := cm.GetSize().Y
		r, _ := dropdownOpenRect(cm, point{X: cm.DrawRect.X0, Y: cm.DrawRect.Y0})
		openH := r.Y1 - r.Y0
		if r.containsPoint(mpos) {
			maxScroll := optionH*float32(len(cm.Options)) - openH
			if maxScroll < 0 {
				maxScroll = 0
			}
			old := cm.Scroll
			cm.Scroll.Y -= delta.Y * 16
			if cm.Scroll.Y < 0 {
				cm.Scroll.Y = 0
			}
			if cm.Scroll.Y > maxScroll {
				cm.Scroll.Y = maxScroll
			}
			if cm.Scroll != old {
				cm.markDirty()
			}
			return true
		}
	}
	return false
}

// SearchActive reports whether a window search box currently has focus.
func SearchActive() bool {
	return activeSearch != nil
}
