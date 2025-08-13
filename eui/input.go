package eui

import (
	"math"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
)

var (
	mposOld     point
	cursorShape ebiten.CursorShapeType

	dragPart   dragType
	dragWin    *windowData
	activeItem *itemData

	downPos point
	downWin *windowData
)

// Update processes input and updates window state.
// Programs embedding the UI can call this from their Ebiten Update handler.
func Update() error {
	w, h := ebiten.WindowSize()
	Layout(w, h)

	checkThemeStyleMods()

	if inpututil.IsKeyJustPressed(ebiten.KeyGraveAccent) &&
		(ebiten.IsKeyPressed(ebiten.KeyShift) || ebiten.IsKeyPressed(ebiten.KeyShiftLeft) || ebiten.IsKeyPressed(ebiten.KeyShiftRight)) {
		_ = DumpTree()
	}

	prevHovered := hoveredItem
	hoveredItem = nil

	mx, my := pointerPosition()
	mpos := point{X: float32(mx), Y: float32(my)}

	click := pointerJustPressed()
	midClick := inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonMiddle)
	if click || midClick {
		downPos = mpos
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
		if !dropdownOpenContainsAnywhere(mpos) {
			closeAllDropdowns()
		}
		if focusedItem != nil {
			focusedItem.Focused = false
		}
		focusedItem = nil
	}
	clickTime := pointerPressDuration()
	clickDrag := clickTime > 1
	midClickTime := inpututil.MouseButtonPressDuration(ebiten.MouseButtonMiddle)
	midClickDrag := midClickTime > 1
	midPressed := ebiten.IsMouseButtonPressed(ebiten.MouseButtonMiddle)

	if !pointerPressed() && !midPressed {
		dragPart = PART_NONE
		dragWin = nil
		activeItem = nil
		downWin = nil
	}

	wx, wy := pointerWheel()
	wheelDelta := point{X: float32(wx), Y: float32(wy)}

	delta := pointSub(mpos, mposOld)
	c := ebiten.CursorShapeDefault

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
				case PART_SCROLL_V, PART_SCROLL_H, PART_PIN:
					c = ebiten.CursorShapePointer
				}
			}

			if click && dragPart == PART_NONE && downWin == win {
				if part == PART_CLOSE {
					win.Open = false
					//win.RemoveWindow()
					continue
				}
				if part == PART_PIN {
					if win.zone != nil {
						win.ClearZone()
						win.clampToScreen()
					} else {
						win.PinToClosestZone()
					}
					win.markDirty()
					continue
				}
				dragPart = part
				dragWin = win
			} else if midClick && dragPart == PART_NONE && part == PART_BAR && downWin == win {
				dragPart = part
				dragWin = win
			} else if (clickDrag || midClickDrag) && dragPart != PART_NONE && dragWin == win {
				switch dragPart {
				case PART_BAR:
					if win.zone == nil {
						win.Position = pointAdd(win.Position, posCh)
					}
				case PART_TOP:
					posCh.X = 0
					sizeCh.X = 0
					if !win.setSize(pointSub(win.Size, sizeCh)) {
						if win.zone == nil {
							win.Position = pointAdd(win.Position, posCh)
						}
					}
				case PART_BOTTOM:
					sizeCh.X = 0
					win.setSize(pointAdd(win.Size, sizeCh))
				case PART_LEFT:
					posCh.Y = 0
					sizeCh.Y = 0
					if !win.setSize(pointSub(win.Size, sizeCh)) {
						if win.zone == nil {
							win.Position = pointAdd(win.Position, posCh)
						}
					}
				case PART_RIGHT:
					sizeCh.Y = 0
					win.setSize(pointAdd(win.Size, sizeCh))
				case PART_TOP_LEFT:
					if !win.setSize(pointSub(win.Size, sizeCh)) {
						if win.zone == nil {
							win.Position = pointAdd(win.Position, posCh)
						}
					}
				case PART_TOP_RIGHT:
					tx := win.Size.X + sizeCh.X
					ty := win.Size.Y - sizeCh.Y
					if !win.setSize(point{X: tx, Y: ty}) {
						if win.zone == nil {
							win.Position.Y += posCh.Y
						}
					}
				case PART_BOTTOM_RIGHT:
					tx := win.Size.X + sizeCh.X
					ty := win.Size.Y + sizeCh.Y
					win.setSize(point{X: tx, Y: ty})
				case PART_BOTTOM_LEFT:
					tx := win.Size.X - sizeCh.X
					ty := win.Size.Y + sizeCh.Y
					if !win.setSize(point{X: tx, Y: ty}) {
						if win.zone == nil {
							win.Position.X += posCh.X
						}
					}
				case PART_SCROLL_V:
					dragWindowScroll(win, mpos, true)
				case PART_SCROLL_H:
					dragWindowScroll(win, mpos, false)
				}
				win.clampToScreen()
				if win.zone == nil {
					if !snapToCorner(win) {
						if snapToWindow(win) {
							win.clampToScreen()
						}
					}
				}
				break
			}
		}

		// Window items
		prevWinHovered := win.Hovered
		prevActiveWindow := activeWindow
		win.Hovered = false
		win.clickWindowItems(mpos, click)
		if win.Hovered != prevWinHovered {
			win.markDirty()
		}

		// Bring window forward on click if the cursor is over it or an
		// expanded dropdown. Break so windows behind don't receive the
		// event.
		if win.getWinRect().containsPoint(mpos) || dropdownOpenContains(win.Contents, mpos) {
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

	if cursorShape != c {
		ebiten.SetCursorShape(c)
		cursorShape = c
	}

	if focusedItem != nil {
		for _, r := range ebiten.AppendInputChars(nil) {
			if r >= 32 && r != 127 && r != '\r' && r != '\n' {
				focusedItem.Text += string(r)
				if focusedItem.TextPtr != nil {
					*focusedItem.TextPtr = focusedItem.Text
				}
				focusedItem.markDirty()
				if focusedItem.Handler != nil {
					focusedItem.Handler.Emit(UIEvent{Item: focusedItem, Type: EventInputChanged, Text: focusedItem.Text})
				}
			}
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyBackspace) {
			runes := []rune(focusedItem.Text)
			if len(runes) > 0 {
				focusedItem.Text = string(runes[:len(runes)-1])
				if focusedItem.TextPtr != nil {
					*focusedItem.TextPtr = focusedItem.Text
				}
				focusedItem.markDirty()
				if focusedItem.Handler != nil {
					focusedItem.Handler.Emit(UIEvent{Item: focusedItem, Type: EventInputChanged, Text: focusedItem.Text})
				}
			}
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
			focusedItem.Focused = false
			focusedItem.markDirty()
			focusedItem = nil
		}
	}

	mposOld = mpos

	if wheelDelta.X != 0 || wheelDelta.Y != 0 {
		for i := len(windows) - 1; i >= 0; i-- {
			win := windows[i]
			if !win.Open {
				continue
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
			clearExpiredClicks(win.Contents)
		}
	}

	return nil
}

func (win *windowData) clickWindowItems(mpos point, click bool) {
	// If the mouse isn't within the window or any open dropdown, just return
	if !win.getMainRect().containsPoint(mpos) && !dropdownOpenContains(win.Contents, mpos) {
		return
	}
	if clickOpenDropdown(win.Contents, mpos, click) {
		return
	}
	win.Hovered = true

	for _, item := range win.Contents {
		handled := false
		if item.ItemType == ITEM_FLOW {
			handled = item.clickFlows(mpos, click)
		} else {
			handled = item.clickItem(mpos, click)
		}
		if handled {
			return
		}
	}
}

func (item *itemData) clickFlows(mpos point, click bool) bool {
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
					tab.Clicked = time.Now()
					item.ActiveTab = i
				}
				return true
			}
		}
		for _, subItem := range item.Tabs[item.ActiveTab].Contents {
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
		for _, subItem := range item.Contents {
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
		item.Clicked = time.Now()
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
		} else if item.ItemType == ITEM_INPUT {
			focusedItem = item
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
			item.setSliderValue(mpos)
			item.markDirty()
			if item.Action != nil {
				item.Action()
			}
		}
	}
	return true
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

func clearExpiredClicks(list []*itemData) {
	for _, it := range list {
		if !it.Clicked.IsZero() && time.Since(it.Clicked) >= clickFlash {
			it.Clicked = time.Time{}
			it.markDirty()
		}
		for _, tab := range it.Tabs {
			if !tab.Clicked.IsZero() && time.Since(tab.Clicked) >= clickFlash {
				tab.Clicked = time.Time{}
				tab.markDirty()
			}
			clearExpiredClicks(tab.Contents)
		}
		clearExpiredClicks(it.Contents)
	}
}

func (item *itemData) setSliderValue(mpos point) {
	// Determine the width of the slider track accounting for the
	// displayed value text to the right of the knob.
	// Measure against a consistent label width so sliders with
	// different ranges have identical track lengths.
	maxLabel := sliderMaxLabel
	textSize := (item.FontSize * uiScale) + 2
	face := textFace(textSize)
	maxW, _ := text.Measure(maxLabel, face, 0)

	knobW := item.AuxSize.X * uiScale
	width := item.DrawRect.X1 - item.DrawRect.X0 - knobW - currentStyle.SliderValueGap - float32(maxW)
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
	item.Value = item.MinValue + ratio*(item.MaxValue-item.MinValue)
	if item.IntOnly {
		item.Value = float32(int(item.Value + 0.5))
	}
	item.markDirty()
	if item.Handler != nil {
		item.Handler.Emit(UIEvent{Item: item, Type: EventSliderChanged, Value: item.Value})
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
		if it.ItemType == ITEM_FLOW {
			if it.DrawRect.containsPoint(mpos) {
				req := it.contentBounds()
				size := it.GetSize()
				if it.Scrollable {
					if it.FlowType == FLOW_VERTICAL && req.Y > size.Y {
						it.Scroll.Y -= delta.Y * 16
						if it.Scroll.Y < 0 {
							it.Scroll.Y = 0
						}
						max := req.Y - size.Y
						if it.Scroll.Y > max {
							it.Scroll.Y = max
						}
						return true
					} else if it.FlowType == FLOW_HORIZONTAL && req.X > size.X {
						it.Scroll.X -= delta.X * 16
						if it.Scroll.X < 0 {
							it.Scroll.X = 0
						}
						max := req.X - size.X
						if it.Scroll.X > max {
							it.Scroll.X = max
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
				it.Scroll.Y -= delta.Y * 16
				if it.Scroll.Y < 0 {
					it.Scroll.Y = 0
				}
				if it.Scroll.Y > maxScroll {
					it.Scroll.Y = maxScroll
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
		max := req.Y - avail.Y
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
		max := req.X - avail.X
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
		barH := avail.Y * avail.Y / req.Y
		maxScroll := req.Y - avail.Y
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
		barW := avail.X * avail.X / req.X
		maxScroll := req.X - avail.X
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
