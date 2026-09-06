package eui

import (
	"image/color"
	"math"
	"strings"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

var (
	strokeLineFn = vector.StrokeLine
	strokeRectFn = vector.StrokeRect
)

func (item *itemData) themeStyle() *itemData {
	if item == nil {
		return nil
	}

	th := item.Theme
	if th == nil {
		th = currentTheme
	}
	if th == nil {
		return nil
	}

	switch item.ItemType {
	case ITEM_BUTTON:
		return &th.Button
	case ITEM_TEXT:
		return &th.Text
	case ITEM_CHECKBOX:
		return &th.Checkbox
	case ITEM_RADIO:
		return &th.Radio
	case ITEM_INPUT:
		return &th.Input
	case ITEM_SLIDER:
		return &th.Slider
	case ITEM_DROPDOWN:
		return &th.Dropdown
	case ITEM_PROGRESS:
		return &th.Progress
	case ITEM_FLOW:
		if len(item.Tabs) > 0 {
			return &th.Tab
		}
	}
	return nil
}

// disabledStyle returns a copy of style with all visual colors replaced by the
// style's DisabledColor. When a widget is disabled, using this helps ensure it
// renders in a consistent "grayed out" appearance regardless of hover or
// active states.
func disabledStyle(style *itemData) *itemData {
	if style == nil {
		return nil
	}
	ds := *style
	ds.Color = style.DisabledColor
	ds.HoverColor = style.DisabledColor
	ds.ClickColor = style.DisabledColor
	ds.OutlineColor = style.DisabledColor
	ds.TextColor = style.DisabledColor
	ds.SelectedColor = style.DisabledColor
	return &ds
}

func (win *windowData) getWinRect() rect {
	pos := win.GetPos()
	size := win.GetSize()
	return rect{
		X0: pos.X,
		Y0: pos.Y,
		X1: pos.X + size.X,
		Y1: pos.Y + size.Y,
	}
}

func (parent *itemData) addItemTo(item *itemData) {
	item.Parent = parent
	if item.Theme == nil {
		item.Theme = parent.Theme
	}
	item.setParentWindow(parent.ParentWindow)
	parent.Contents = append(parent.Contents, item)
	if parent.ItemType == ITEM_FLOW {
		parent.resizeFlow(parent.GetSize())
	}
	if parent.ParentWindow != nil {
		parent.ParentWindow.updateHasIndeterminate()
	}
}

func (parent *itemData) prependItemTo(item *itemData) {
	item.Parent = parent
	if item.Theme == nil {
		item.Theme = parent.Theme
	}
	item.setParentWindow(parent.ParentWindow)
	parent.Contents = append([]*itemData{item}, parent.Contents...)
	if parent.ItemType == ITEM_FLOW {
		parent.resizeFlow(parent.GetSize())
	}
	if parent.ParentWindow != nil {
		parent.ParentWindow.updateHasIndeterminate()
	}
}

func (parent *windowData) addItemTo(item *itemData) {
	if item.Theme == nil {
		item.Theme = parent.Theme
	}
	parent.Contents = append(parent.Contents, item)
	item.setParentWindow(parent)
	parent.updateHasIndeterminate()
	item.resizeFlow(parent.GetSize())
	parent.markDirty()
}

func (parent *windowData) prependItemTo(item *itemData) {
	if item.Theme == nil {
		item.Theme = parent.Theme
	}
	parent.Contents = append([]*itemData{item}, parent.Contents...)
	item.setParentWindow(parent)
	parent.updateHasIndeterminate()
	item.resizeFlow(parent.GetSize())
	parent.markDirty()
}

func (parent *itemData) removeItem(child *itemData) {
	if child == nil {
		return
	}
	for i, it := range parent.Contents {
		if it == child {
			parent.Contents = append(parent.Contents[:i], parent.Contents[i+1:]...)
			break
		}
	}
	child.setParentWindow(nil)
	if parent.ItemType == ITEM_FLOW {
		parent.resizeFlow(parent.GetSize())
	}
	if parent.ParentWindow != nil {
		parent.ParentWindow.updateHasIndeterminate()
		parent.ParentWindow.markDirty()
	}
}

func (parent *itemData) setItems(children []*itemData) {
	if parent == nil {
		return
	}
	oldContents := parent.Contents
	for _, child := range oldContents {
		if child != nil && child.Parent == parent {
			child.Parent = nil
			child.setParentWindow(nil)
		}
	}
	parent.Contents = append(oldContents[:0], children...)
	for i := len(parent.Contents); i < len(oldContents); i++ {
		oldContents[i] = nil
	}
	for _, child := range parent.Contents {
		if child == nil {
			continue
		}
		child.Parent = parent
		if child.Theme == nil {
			child.Theme = parent.Theme
		}
		child.setParentWindow(parent.ParentWindow)
	}
	if parent.ItemType == ITEM_FLOW {
		parent.resizeFlow(parent.GetSize())
	}
	if parent.ParentWindow != nil {
		parent.ParentWindow.updateHasIndeterminate()
		parent.ParentWindow.markDirty()
	}
}

func (parent *windowData) removeItem(child *itemData) {
	if child == nil {
		return
	}
	for i, it := range parent.Contents {
		if it == child {
			parent.Contents = append(parent.Contents[:i], parent.Contents[i+1:]...)
			break
		}
	}
	child.setParentWindow(nil)
	parent.updateHasIndeterminate()
	parent.markDirty()
}

func (parent *itemData) insertItem(idx int, child *itemData) {
	if parent == nil || child == nil {
		return
	}
	if idx < 0 {
		idx = 0
	}
	if idx > len(parent.Contents) {
		idx = len(parent.Contents)
	}
	child.Parent = parent
	if child.Theme == nil {
		child.Theme = parent.Theme
	}
	child.setParentWindow(parent.ParentWindow)
	parent.Contents = append(parent.Contents, nil)
	copy(parent.Contents[idx+1:], parent.Contents[idx:])
	parent.Contents[idx] = child
	if parent.ItemType == ITEM_FLOW {
		child.resizeFlow(parent.GetSize())
	}
	if parent.ParentWindow != nil {
		parent.ParentWindow.updateHasIndeterminate()
		parent.ParentWindow.markDirty()
	}
}

func (parent *windowData) insertItem(idx int, child *itemData) {
	if parent == nil || child == nil {
		return
	}
	if child.Theme == nil {
		child.Theme = parent.Theme
	}
	if idx < 0 {
		idx = 0
	}
	if idx > len(parent.Contents) {
		idx = len(parent.Contents)
	}
	parent.Contents = append(parent.Contents, nil)
	copy(parent.Contents[idx+1:], parent.Contents[idx:])
	parent.Contents[idx] = child
	child.setParentWindow(parent)
	parent.updateHasIndeterminate()
	parent.markDirty()
}

func (parent *itemData) replaceItem(idx int, child *itemData) {
	if parent == nil || child == nil || idx < 0 || idx >= len(parent.Contents) {
		return
	}
	old := parent.Contents[idx]
	child.Parent = parent
	if child.Theme == nil {
		child.Theme = parent.Theme
	}
	child.setParentWindow(parent.ParentWindow)
	parent.Contents[idx] = child
	if parent.ItemType == ITEM_FLOW {
		child.resizeFlow(parent.GetSize())
	}
	if old != nil {
		old.setParentWindow(nil)
	}
	if parent.ParentWindow != nil {
		parent.ParentWindow.updateHasIndeterminate()
		parent.ParentWindow.markDirty()
	}
}

func (parent *windowData) replaceItem(idx int, child *itemData) {
	if parent == nil || child == nil || idx < 0 || idx >= len(parent.Contents) {
		return
	}
	old := parent.Contents[idx]
	if child.Theme == nil {
		child.Theme = parent.Theme
	}
	parent.Contents[idx] = child
	child.setParentWindow(parent)
	if old != nil {
		old.setParentWindow(nil)
	}
	parent.updateHasIndeterminate()
	parent.markDirty()
}

func (item *itemData) updateText(text string) {
	if item == nil || item.Text == text {
		return
	}
	item.Text = text
	item.tooltipW = 0
	item.tooltipH = 0
	item.markDirty()
}

func (win *windowData) getMainRect() rect {
	pos := win.getPosition()
	size := win.GetSize()
	title := win.GetTitleSize()
	return rect{
		X0: pos.X,
		Y0: pos.Y + title,
		X1: pos.X + size.X,
		Y1: pos.Y + size.Y,
	}
}

func (win *windowData) getTitleRect() rect {
	if win.TitleHeight <= 0 {
		return rect{}
	}
	pos := win.GetPos()
	size := win.GetSize()
	title := win.GetTitleSize()
	return rect{
		X0: pos.X, Y0: pos.Y,
		X1: pos.X + size.X,
		Y1: pos.Y + title,
	}
}

func (win *windowData) xRect() rect {
	if win.TitleHeight <= 0 || !win.Closable {
		return rect{}
	}

	var xpad float32 = win.Border * win.scale()
	pos := win.GetPos()
	size := win.GetSize()
	title := win.GetTitleSize()
	return rect{
		X0: pos.X + size.X - title + xpad,
		Y0: pos.Y + xpad,

		X1: pos.X + size.X - xpad,
		Y1: pos.Y + title - xpad,
	}
}

func (win *windowData) maxRect() rect {
	if win.TitleHeight <= 0 || !win.Maximizable {
		return rect{}
	}
	var xpad float32 = win.Border * win.scale()
	size := win.GetTitleSize()
	pos := win.GetPos()
	x1 := pos.X + win.GetSize().X - xpad
	if win.Closable {
		x1 -= size
	}
	x0 := x1 - size
	return rect{
		X0: x0,
		Y0: pos.Y + xpad,
		X1: x1,
		Y1: pos.Y + size - xpad,
	}
}

func (win *windowData) searchRect() rect {
	if win.TitleHeight <= 0 || !win.Searchable {
		return rect{}
	}

	var xpad float32 = win.Border * win.scale()
	size := win.GetTitleSize()
	pos := win.GetPos()
	x1 := pos.X + win.GetSize().X - xpad
	if win.Closable {
		x1 -= size
	}
	if win.Maximizable {
		x1 -= size
	}
	x0 := x1 - size
	return rect{
		X0: x0,
		Y0: pos.Y + xpad,
		X1: x1,
		Y1: pos.Y + size - xpad,
	}
}

func (win *windowData) searchBoxRect() rect {
	if win.TitleHeight <= 0 || !win.searchOpen {
		return rect{}
	}
	var xpad float32 = win.Border * win.scale()
	size := win.GetTitleSize()
	textSize := size / 2
	face := textFace(textSize)
	w, _ := text.Measure(win.SearchText, face, 0)
	maxW, _ := text.Measure(strings.Repeat("W", 64), face, 0)
	width := float32(w) + size
	if float64(width) > maxW+float64(size) {
		width = float32(maxW) + size
	}
	if width < size*2 {
		width = size * 2
	}
	sr := win.searchRect()
	x1 := sr.X0 - xpad
	x0 := x1 - width
	y0 := win.getPosition().Y + xpad
	y1 := y0 + size - xpad*2
	return rect{X0: x0, Y0: y0, X1: x1, Y1: y1}
}

func (win *windowData) searchCloseRect() rect {
	sb := win.searchBoxRect()
	if sb.X0 == 0 && sb.X1 == 0 {
		return rect{}
	}
	h := sb.Y1 - sb.Y0
	return rect{X0: sb.X1 - h, Y0: sb.Y0, X1: sb.X1, Y1: sb.Y1}
}

// closeSearch resets the search state and clears highlights.
func (win *windowData) closeSearch() {
	win.searchOpen = false
	if activeSearch == win {
		activeSearch = nil
	}
	win.SearchText = ""
	if win.OnSearch != nil {
		win.OnSearch("")
	}
	win.markDirty()
}

func (win *windowData) dragbarRect() rect {
	if win.TitleHeight <= 0 && !win.Resizable {
		return rect{}
	}
	pos := win.GetPos()
	right := pos.X + win.GetSize().X
	for _, control := range []rect{win.xRect(), win.maxRect(), win.searchRect()} {
		if control.X0 > pos.X && control.X0 < right {
			right = control.X0
		}
	}
	return rect{
		X0: pos.X,
		Y0: pos.Y,
		X1: right,
		Y1: pos.Y + win.GetTitleSize(),
	}
}

func captureScroll(items []*itemData) map[*itemData]point {
	saved := make(map[*itemData]point)
	var walk func([]*itemData)
	walk = func(list []*itemData) {
		for _, it := range list {
			if it == nil {
				continue
			}
			if it.ItemType == ITEM_FLOW {
				saved[it] = it.Scroll
			}
			if len(it.Tabs) > 0 {
				for _, tab := range it.Tabs {
					walk(tab.Contents)
				}
			}
			if len(it.Contents) > 0 {
				walk(it.Contents)
			}
		}
	}
	walk(items)
	return saved
}

func restoreScroll(items []*itemData, saved map[*itemData]point) {
	var walk func([]*itemData)
	walk = func(list []*itemData) {
		for _, it := range list {
			if it == nil {
				continue
			}
			if it.ItemType == ITEM_FLOW {
				if sc, ok := saved[it]; ok {
					it.Scroll = sc
					clampFlowScroll(it)
				}
			}
			if len(it.Tabs) > 0 {
				for _, tab := range it.Tabs {
					walk(tab.Contents)
				}
			}
			if len(it.Contents) > 0 {
				walk(it.Contents)
			}
		}
	}
	walk(items)
}

func clampFlowScroll(item *itemData) {
	req := item.contentBounds()
	size := item.GetSize()
	if req.Y <= size.Y {
		item.Scroll.Y = 0
	} else {
		max := req.Y - size.Y
		if item.Scroll.Y > max {
			item.Scroll.Y = max
		}
	}
	if req.X <= size.X {
		item.Scroll.X = 0
	} else {
		max := req.X - size.X
		if item.Scroll.X > max {
			item.Scroll.X = max
		}
	}
}

const scrollBottomSlop float32 = 1

func scrolledToBottom(scrollY, contentH, viewH float32) bool {
	if contentH <= viewH {
		return true
	}
	max := contentH - viewH
	return scrollY >= max-scrollBottomSlop
}

func (item *itemData) ScrollAtBottom() bool {
	if item == nil {
		return true
	}
	req := item.contentBounds()
	size := item.GetSize()
	return scrolledToBottom(item.Scroll.Y, req.Y, size.Y)
}

func (win *windowData) ScrollAtBottom() bool {
	if win.NoScroll {
		return true
	}
	pad := (win.Padding + win.BorderPad) * win.scale()
	req := win.contentBounds()
	availY := win.GetSize().Y - win.GetTitleSize() - 2*pad
	return scrolledToBottom(win.Scroll.Y, req.Y, availY)
}

func (win *windowData) Refresh() {
	win.RefreshWithReason("layout")
}

// RefreshWithReason refreshes layout and labels the next cached repaint for
// diagnostics. Like Refresh, this is called on the UI thread.
func (win *windowData) RefreshWithReason(reason string) {
	if !win.IsOpen() {
		return
	}

	savedWinScroll := win.Scroll
	savedFlows := captureScroll(win.Contents)

	win.resizeFlows()
	if win.AutoSize {
		win.updateAutoSize()
		if win.zone != nil {
			// After autosizing, re-apply zone centering so initial
			// creation and subsequent Refresh keep the window centered
			// on its selected zone rather than appearing pinned by
			// the top-left corner.
			win.updateZonePosition()
		}
	}

	win.Scroll = savedWinScroll
	restoreScroll(win.Contents, savedFlows)
	win.adjustScrollForResize()
	win.markDirty()
	win.repaintReason = reason
}

func (win *windowData) IsOpen() bool {
	return win.Open
}

func (win *windowData) setSize(size point) bool {
	if size.X < 1 || size.Y < 1 {
		return false
	}
	if size.X < MinWindowSize {
		size.X = MinWindowSize
	}
	if size.Y < MinWindowSize {
		size.Y = MinWindowSize
	}

	old := win.Size
	win.Size = size
	if old != size {
		win.markDirty()
	}

	win.BringForward()
	win.resizeFlows()
	win.adjustScrollForResize()
	if win.zone != nil {
		win.updateZonePosition()
	}
	win.clampToScreen()

	if old != size && win.OnResize != nil {
		win.OnResize()
	}

	return true
}

func (win *windowData) adjustScrollForResize() {
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
	if req.Y <= avail.Y {
		win.Scroll.Y = 0
	} else {
		max := req.Y - avail.Y
		if win.Scroll.Y > max {
			win.Scroll.Y = max
		}
	}
	if req.X <= avail.X {
		win.Scroll.X = 0
	} else {
		max := req.X - avail.X
		if win.Scroll.X > max {
			win.Scroll.X = max
		}
	}
	if win.Scroll != old {
		win.markDirty()
	}
}

func (win *windowData) clampToScreen() {
	pos := win.getPosition()
	size := win.GetSize()
	old := win.Position
	s := win.scale()

	// Ensure window size never exceeds the screen bounds. If it does,
	// shrink the window so it can be fully clamped on-screen.
	sizeChanged := false
	if size.X > float32(screenWidth) {
		win.Size.X = float32(screenWidth) / s
		size.X = float32(screenWidth)
		sizeChanged = true
	}
	if size.Y > float32(screenHeight) {
		win.Size.Y = float32(screenHeight) / s
		size.Y = float32(screenHeight)
		sizeChanged = true
	}

	if pos.X < 0 {
		win.Position.X -= pos.X / s
		pos.X = 0
	}
	if pos.Y < 0 {
		win.Position.Y -= pos.Y / s
		pos.Y = 0
	}

	overX := pos.X + size.X - float32(screenWidth)
	if overX > 0 {
		win.Position.X -= overX / s
	}
	overY := pos.Y + size.Y - float32(screenHeight)
	if overY > 0 {
		win.Position.Y -= overY / s
	}
	if sizeChanged {
		win.resizeFlows()
		win.adjustScrollForResize()
		if win.OnResize != nil {
			win.OnResize()
		}
	}
	if win.Position != old {
		//win.markDirty()
	}
}

// dropdownOpenRect returns the rectangle used for drawing and input handling of
// an open dropdown menu. The rectangle is adjusted so it never extends off the
// screen while leaving room for overlay controls at the top and bottom equal to
// one option height.
func dropdownOpenRect(item *itemData, offset point) (rect, int) {
	maxSize := item.GetSize()
	optionH := maxSize.Y
	visible := item.MaxVisible
	if visible <= 0 {
		visible = len(item.Options)
	}
	if visible > len(item.Options) {
		visible = len(item.Options)
	}

	maxVisible := int((float32(screenHeight) - optionH*dropdownOverlayReserve*2) / optionH)
	if maxVisible < 1 {
		maxVisible = 1
	}
	if visible > maxVisible {
		visible = maxVisible
	}

	startY := offset.Y + maxSize.Y
	openH := optionH * float32(visible)
	r := rect{X0: offset.X, Y0: startY, X1: offset.X + maxSize.X, Y1: startY + openH}

	bottomLimit := float32(screenHeight) - optionH*dropdownOverlayReserve
	if r.Y1 > bottomLimit {
		diff := r.Y1 - bottomLimit
		r.Y0 -= diff
		r.Y1 -= diff
	}
	topLimit := optionH * dropdownOverlayReserve
	if r.Y0 < topLimit {
		diff := topLimit - r.Y0
		r.Y0 += diff
		r.Y1 += diff
	}

	return r, visible
}

func (win *windowData) getWindowPart(mpos point, click bool) dragType {
	s := win.scale()
	mpos = point{X: mpos.X * s, Y: mpos.Y * s}
	if win.Docked {
		// Docked panes retain title-bar search and content scrollbars, but the
		// workspace owns their move, resize, close, and maximize interactions.
		if win.TitleHeight > 0 && win.getTitleRect().containsPoint(mpos) && win.Searchable && win.searchRect().containsPoint(mpos) {
			win.HoverSearch = true
			return PART_SEARCH
		}
		return win.getScrollbarPart(mpos)
	}
	if part := win.getTitlebarPart(mpos); part != PART_NONE {
		return part
	}
	if part := win.getResizePart(mpos); part != PART_NONE {
		return part
	}
	if !win.Resizable && win.Movable {
		ct := cornerTolerance * s
		winRect := win.getWinRect()
		inCorner := func(x0, y0 float32) bool {
			return mpos.X >= x0-ct && mpos.X <= x0+ct && mpos.Y >= y0-ct && mpos.Y <= y0+ct
		}
		if inCorner(winRect.X0, winRect.Y0) || inCorner(winRect.X1, winRect.Y0) || inCorner(winRect.X0, winRect.Y1) || inCorner(winRect.X1, winRect.Y1) {
			return PART_BAR
		}
	}
	return win.getScrollbarPart(mpos)
}

// SetDocked switches a window between standalone and workspace-owned input
// and rendering behavior.
func (win *windowData) SetDocked(docked bool) {
	if win.Docked == docked {
		return
	}
	win.Docked = docked
	win.Dirty = true
}

func (win *windowData) getTitlebarPart(mpos point) dragType {
	if win.TitleHeight <= 0 {
		return PART_NONE
	}
	if win.getTitleRect().containsPoint(mpos) {
		if win.searchOpen {
			if win.searchCloseRect().containsPoint(mpos) || win.searchBoxRect().containsPoint(mpos) {
				return PART_NONE
			}
		}
		if win.Closable && win.xRect().containsPoint(mpos) {
			win.HoverClose = true
			return PART_CLOSE
		}
		if win.Maximizable && win.maxRect().containsPoint(mpos) {
			win.HoverMax = true
			return PART_MAXIMIZE
		}
		if win.Searchable && win.searchRect().containsPoint(mpos) {
			win.HoverSearch = true
			return PART_SEARCH
		}
		if win.Movable && win.dragbarRect().containsPoint(mpos) {
			win.HoverDragbar = true
			return PART_BAR
		}
	}
	return PART_NONE
}

func (win *windowData) getResizePart(mpos point) dragType {
	if !win.Resizable {
		return PART_NONE
	}

	s := win.scale()
	t := scrollTolerance * s
	ct := cornerTolerance * s
	winRect := win.getWinRect()
	// Check enlarged corner areas first
	if mpos.X >= winRect.X0-ct && mpos.X <= winRect.X0+ct && mpos.Y >= winRect.Y0-ct && mpos.Y <= winRect.Y0+ct {
		return PART_TOP_LEFT
	}
	if mpos.X >= winRect.X1-ct && mpos.X <= winRect.X1+ct && mpos.Y >= winRect.Y0-ct && mpos.Y <= winRect.Y0+ct {
		return PART_TOP_RIGHT
	}
	if mpos.X >= winRect.X0-ct && mpos.X <= winRect.X0+ct && mpos.Y >= winRect.Y1-ct && mpos.Y <= winRect.Y1+ct {
		return PART_BOTTOM_LEFT
	}
	if mpos.X >= winRect.X1-ct && mpos.X <= winRect.X1+ct && mpos.Y >= winRect.Y1-ct && mpos.Y <= winRect.Y1+ct {
		return PART_BOTTOM_RIGHT
	}
	outRect := winRect
	outRect.X0 -= t
	outRect.X1 += t
	outRect.Y0 -= t
	outRect.Y1 += t

	inRect := winRect
	inRect.X0 += t
	inRect.X1 -= t
	inRect.Y0 += t
	inRect.Y1 -= t

	if outRect.containsPoint(mpos) && !inRect.containsPoint(mpos) {
		top := mpos.Y < inRect.Y0
		bottom := mpos.Y > inRect.Y1
		left := mpos.X < inRect.X0
		right := mpos.X > inRect.X1

		switch {
		case top && left:
			return PART_TOP_LEFT
		case top && right:
			return PART_TOP_RIGHT
		case bottom && left:
			return PART_BOTTOM_LEFT
		case bottom && right:
			return PART_BOTTOM_RIGHT
		case top:
			return PART_TOP
		case bottom:
			return PART_BOTTOM
		case left:
			return PART_LEFT
		case right:
			return PART_RIGHT
		}
	}
	return PART_NONE
}

func (win *windowData) getScrollbarPart(mpos point) dragType {
	if win.NoScroll {
		return PART_NONE
	}

	pad := (win.Padding + win.BorderPad) * win.scale()
	req := win.contentBounds()
	avail := point{
		X: win.GetSize().X - 2*pad,
		Y: win.GetSize().Y - win.GetTitleSize() - 2*pad,
	}
	if req.Y > avail.Y {
		barH := scrollbarThumbLength(avail.Y, req.Y, win.scale())
		maxScroll := req.Y - avail.Y
		pos := float32(0)
		if maxScroll > 0 {
			pos = (win.Scroll.Y / maxScroll) * (avail.Y - barH)
		}
		sbW := currentStyle.BorderPad.Slider * 2
		r := rect{
			X0: win.getPosition().X + win.GetSize().X - win.BorderPad - sbW,
			Y0: win.getPosition().Y + win.GetTitleSize() + win.BorderPad + pos,
			X1: win.getPosition().X + win.GetSize().X - win.BorderPad,
			Y1: win.getPosition().Y + win.GetTitleSize() + win.BorderPad + pos + barH,
		}
		if r.containsPoint(mpos) {
			return PART_SCROLL_V
		}
	}
	if req.X > avail.X {
		barW := scrollbarThumbLength(avail.X, req.X, win.scale())
		maxScroll := req.X - avail.X
		pos := float32(0)
		if maxScroll > 0 {
			pos = (win.Scroll.X / maxScroll) * (avail.X - barW)
		}
		sbW := currentStyle.BorderPad.Slider * 2
		r := rect{
			X0: win.getPosition().X + win.BorderPad + pos,
			Y0: win.getPosition().Y + win.GetSize().Y - win.BorderPad - sbW,
			X1: win.getPosition().X + win.BorderPad + pos + barW,
			Y1: win.getPosition().Y + win.GetSize().Y - win.BorderPad,
		}
		if r.containsPoint(mpos) {
			return PART_SCROLL_H
		}
	}
	return PART_NONE
}

func (item *itemData) getScrollbarPart(mpos point) dragType {
	if !item.Scrollable {
		return PART_NONE
	}

	req := item.contentBounds()
	size := item.GetSize()
	if item.FlowType == FLOW_VERTICAL && req.Y > size.Y {
		barH := scrollbarThumbLength(size.Y, req.Y, UIScale())
		maxScroll := req.Y - size.Y
		pos := float32(0)
		if maxScroll > 0 {
			pos = (item.Scroll.Y / maxScroll) * (size.Y - barH)
		}
		sbW := currentStyle.BorderPad.Slider * 2
		r := rect{
			X0: item.DrawRect.X1 - sbW,
			Y0: item.DrawRect.Y0 + pos,
			X1: item.DrawRect.X1,
			Y1: item.DrawRect.Y0 + pos + barH,
		}
		if r.containsPoint(mpos) {
			return PART_SCROLL_V
		}
	}
	if item.FlowType == FLOW_HORIZONTAL && req.X > size.X {
		barW := scrollbarThumbLength(size.X, req.X, UIScale())
		maxScroll := req.X - size.X
		pos := float32(0)
		if maxScroll > 0 {
			pos = (item.Scroll.X / maxScroll) * (size.X - barW)
		}
		sbW := currentStyle.BorderPad.Slider * 2
		r := rect{
			X0: item.DrawRect.X0 + pos,
			Y0: item.DrawRect.Y1 - sbW,
			X1: item.DrawRect.X0 + pos + barW,
			Y1: item.DrawRect.Y1,
		}
		if r.containsPoint(mpos) {
			return PART_SCROLL_H
		}
	}
	return PART_NONE
}

func scrollbarThumbLength(track, content, scale float32) float32 {
	if track <= 0 {
		return 0
	}
	if content <= track {
		return track
	}
	length := track * track / content
	minimum := minScrollbarThumbSize * scale
	if minimum > track {
		minimum = track
	}
	if length < minimum {
		length = minimum
	}
	return length
}

func (win *windowData) SetTitleSize(size float32) {
	win.TitleHeight = size / win.scale()
}

func SetUIScale(scale float32) {
	// Clamp to a sane, supported range so very small or large values don't
	// break hit testing or rendering. The effective limit is higher than the
	// user preference limit so a 4x preference can still work on a 2x Retina
	// display.
	if scale < 0.5 {
		scale = 0.5
	} else if scale > 8.0 {
		scale = 8.0
	}
	if uiScale == scale {
		return
	}
	uiScale = scale
	for _, win := range windows {
		if win.AutoSize {
			win.updateAutoSize()
		} else {
			win.resizeFlows()
		}
		if win.zone != nil {
			win.updateZonePosition()
		}
		win.clampToScreen()
	}
	updateAllTooltipBounds()
	markAllDirty()
}

// SetUserUIScale sets the user's preferred UI scale. The effective scale also
// includes the active display's device scale factor, keeping controls and text
// physically readable on Retina and other HiDPI displays.
func SetUserUIScale(scale float32) {
	if scale < 0.5 {
		scale = 0.5
	} else if scale > 4.0 {
		scale = 4.0
	}
	userUIScale = scale
	deviceScale := float32(1)
	if AutoHiDPI {
		deviceScale = float32(lastDeviceScale)
	}
	SetUIScale(userUIScale * deviceScale)
}

// UserUIScale reports the cross-display UI preference before automatic HiDPI
// scaling is applied.
func UserUIScale() float32 { return userUIScale }

func UIScale() float32 { return uiScale }

func (win *windowData) scale() float32 {
	if win.NoScale {
		return 1
	}
	return uiScale
}

func (win *windowData) GetRawTitleSize() float32 { return win.TitleHeight }

func (win *windowData) GetTitleSize() float32 {
	return win.TitleHeight * win.scale()
}

func (win *windowData) GetSize() Point {
	s := win.scale()
	return Point{X: win.Size.X * s, Y: win.Size.Y * s}
}

// RequiresScroll reports whether the window content exceeds its visible area.
// It is useful to callers that need to guarantee a fixed settings layout fits
// without introducing a scrollbar.
func (win *windowData) RequiresScroll() (horizontal, vertical bool) {
	if win.NoScroll {
		return false, false
	}
	pad := (win.Padding + win.BorderPad) * win.scale()
	req := win.contentBounds()
	avail := point{
		X: win.GetSize().X - 2*pad,
		Y: win.GetSize().Y - win.GetTitleSize() - 2*pad,
	}
	return req.X > avail.X, req.Y > avail.Y
}

func (win *windowData) GetPos() Point {
	s := win.scale()
	return Point{X: win.Position.X * s, Y: win.Position.Y * s}
}

func (win *windowData) SetPos(pos Point) bool {
	if win.zone != nil {
		return false
	}
	s := win.scale()
	win.Position = point{X: pos.X / s, Y: pos.Y / s}
	win.clampToScreen()
	return true
}

func (win *windowData) SetSize(size Point) bool {
	if !win.Resizable {
		return false
	}
	s := win.scale()
	return win.setSize(point{X: size.X / s, Y: size.Y / s})
}

func (win *windowData) GetRawSize() Point { return win.Size }

func (win *windowData) GetRawPos() Point { return win.Position }

func (item *itemData) GetSize() Point {
	// Start with the explicitly set size (scaled to pixels).
	sz := Point{X: item.Size.X * uiScale, Y: item.Size.Y * uiScale}

	// Auto-size when a dimension is missing or zero.
	if sz.X <= 0 || sz.Y <= 0 {
		// 1) If this is an image and no size is provided, use the image bounds.
		if item.Image != nil {
			b := item.Image.Bounds()
			w, h := b.Dx(), b.Dy()
			if sz.X <= 0 {
				sz.X = float32(w)
			}
			if sz.Y <= 0 {
				sz.Y = float32(h)
			}
		}

		// 2) If this is a text item, derive size from its text content and font metrics.
		if item.ItemType == ITEM_TEXT && (sz.X <= 0 || sz.Y <= 0) {
			// Choose an effective font size.
			effFont := item.FontSize
			if effFont <= 0 {
				if st := item.themeStyle(); st != nil && st.FontSize > 0 {
					effFont = st.FontSize
				} else {
					effFont = 12
				}
			}
			textSize := (effFont * uiScale) + 2
			face := textFace(textSize)
			// Measure lines and compute bounding box.
			lineCount := 0
			maxW := float64(0)
			for ln := range strings.SplitSeq(item.Text, "\n") {
				lineCount++
				if w, _ := text.Measure(ln, face, 0); w > maxW {
					maxW = w
				}
			}
			metrics := face.Metrics()
			linePx := math.Ceil(metrics.HAscent + metrics.HDescent + 2)
			totalH := float32(linePx) * float32(lineCount)
			if sz.X <= 0 {
				if maxW <= 0 {
					sz.X = textSize // minimal width
				} else {
					sz.X = float32(maxW)
				}
			}
			if sz.Y <= 0 {
				sz.Y = totalH
			}
		}

		// 3) For flows with unspecified size, adopt their content bounds for missing dimensions.
		if item.ItemType == ITEM_FLOW && (sz.X <= 0 || sz.Y <= 0) {
			cb := item.contentBounds()
			if sz.X <= 0 {
				sz.X = cb.X
			}
			if sz.Y <= 0 {
				sz.Y = cb.Y
			}
		}

		// 4) Fall back to theme defaults for remaining unspecified dimensions.
		if (sz.X <= 0 || sz.Y <= 0) && currentTheme != nil {
			if st := item.themeStyle(); st != nil {
				def := st.GetSize()
				if sz.X <= 0 {
					sz.X = def.X
				}
				if sz.Y <= 0 {
					sz.Y = def.Y
				}
			}
		}
		// Ensure size is at least 1px to participate in layout.
		if sz.X <= 0 {
			sz.X = 1
		}
		if sz.Y <= 0 {
			sz.Y = 1
		}
	}

	if item.Size.X <= 0 && item.ItemType == ITEM_TEXT && item.Tooltip != "" && item.ParentWindow != nil && item.ParentWindow.ShowTooltipIndicators {
		sz.X += 18 * uiScale
	}

	// Account for label text below an item if set.
	if item.Label != "" {
		textSize := (item.FontSize * uiScale) + 2
		sz.Y += textSize + currentStyle.TextPadding*uiScale
	}
	return sz
}

func (item *itemData) GetPos() Point {
	return Point{X: item.Position.X * uiScale, Y: item.Position.Y * uiScale}
}

func (item *itemData) GetTextPtr() *string {
	return &item.Text
}

func itemAcceptsTextEditing(item *itemData) bool {
	return item != nil && (item.ItemType == ITEM_INPUT || (item.ItemType == ITEM_TEXT && item.EditableText))
}

const (
	tooltipMaxRunes = 256
	tooltipMaxWidth = 340
	tooltipPadding  = 4
)

// SetTooltip assigns compact, wrapped tooltip text and caches its measured size.
func (item *itemData) SetTooltip(tip string) {
	item.tooltipRaw = truncateTooltip(tip)
	item.updateTooltipBounds()
}

// updateTooltipBounds recalculates the cached tooltip size.
func (item *itemData) updateTooltipBounds() {
	if item == nil {
		return
	}
	if item.tooltipRaw == "" {
		item.Tooltip = ""
		item.tooltipW, item.tooltipH = 0, 0
		return
	}
	faceSize := float32(12) * uiScale
	face := textFace(faceSize)
	item.Tooltip = wrapTooltip(item.tooltipRaw, face, tooltipWrapWidth())
	w, h := text.Measure(item.Tooltip, face, tooltipLineSpacing(face))
	item.tooltipW = float32(w)
	item.tooltipH = float32(h)
}

func tooltipWrapWidth() float64 {
	maxWidth := tooltipMaxWidth * float64(uiScale)
	available := float64(screenWidth) - 2*tooltipPadding*float64(uiScale)
	if available < maxWidth {
		return math.Max(float64(uiScale), available)
	}
	return maxWidth
}

func tooltipLineSpacing(face text.Face) float64 {
	metrics := face.Metrics()
	return math.Ceil(metrics.HAscent + metrics.HDescent + 2*float64(uiScale))
}

func truncateTooltip(tip string) string {
	tip = strings.Join(strings.Fields(tip), " ")
	runes := []rune(tip)
	if len(runes) <= tooltipMaxRunes {
		return tip
	}
	const suffix = " ..."
	limit := tooltipMaxRunes - len([]rune(suffix))
	short := strings.TrimSpace(string(runes[:limit]))
	if split := strings.LastIndexByte(short, ' '); split >= limit/2 {
		short = strings.TrimSpace(short[:split])
	}
	return short + suffix
}

func wrapTooltip(tip string, face text.Face, maxWidth float64) string {
	words := strings.Fields(tip)
	if len(words) == 0 || maxWidth <= 0 {
		return ""
	}
	lines := make([]string, 0, 3)
	line := ""
	for _, word := range words {
		candidate := word
		if line != "" {
			candidate = line + " " + word
		}
		if width, _ := text.Measure(candidate, face, 0); width <= maxWidth {
			line = candidate
			continue
		}
		if line != "" {
			lines = append(lines, line)
			line = ""
		}
		for len(word) > 0 {
			chunk, rest := tooltipWordChunk(word, face, maxWidth)
			if rest == "" {
				line = chunk
				break
			}
			lines = append(lines, chunk)
			word = rest
		}
	}
	if line != "" {
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

func tooltipWordChunk(word string, face text.Face, maxWidth float64) (chunk, rest string) {
	runes := []rune(word)
	for end := len(runes); end > 0; end-- {
		candidate := string(runes[:end])
		if width, _ := text.Measure(candidate, face, 0); width <= maxWidth || end == 1 {
			return candidate, string(runes[end:])
		}
	}
	return word, ""
}

func (win *windowData) markDirty() {
	if win == nil {
		return
	}
	if win.repaintRequested.IsZero() {
		win.repaintRequested = time.Now()
	}
	if win.refreshInterval > 0 && win.Render != nil && !win.lastRefresh.IsZero() && time.Since(win.lastRefresh) < win.refreshInterval {
		win.refreshPending = true
		return
	}
	win.Dirty = true
}

func (item *itemData) markDirty() {
	if item != nil && item.ItemType != ITEM_FLOW {
		item.Dirty = true
		if item.ParentWindow != nil {
			item.ParentWindow.markDirty()
			if item.ParentWindow.repaintReason == "" {
				item.ParentWindow.repaintReason = "item"
			}
		}
	}
}

// UpdateImage replaces the item's image and adjusts size if needed.
func (item *itemData) UpdateImage(img *ebiten.Image) {
	if item == nil {
		return
	}
	item.Image = img
	if img != nil {
		b := img.Bounds()
		w, h := b.Dx(), b.Dy()
		if item.Size.X != float32(w) || item.Size.Y != float32(h) {
			item.Size = point{X: float32(w), Y: float32(h)}
		}
	}
	item.markDirty()
}

func (item *itemData) setParentWindow(win *windowData) {
	item.ParentWindow = win
	for _, child := range item.Contents {
		child.setParentWindow(win)
	}
	for _, tab := range item.Tabs {
		tab.setParentWindow(win)
	}
}

// itemsHaveIndeterminate reports whether any of the provided items or their
// children contain an indeterminate progress bar.
func itemsHaveIndeterminate(items []*itemData) bool {
	for _, it := range items {
		if it.ItemType == ITEM_PROGRESS && it.Indeterminate {
			return true
		}
		if len(it.Tabs) > 0 {
			if it.ActiveTab >= len(it.Tabs) {
				if len(it.Tabs) > 0 {
					it.ActiveTab = 0
				}
			}
			if it.ActiveTab >= 0 && it.ActiveTab < len(it.Tabs) {
				if itemsHaveIndeterminate(it.Tabs[it.ActiveTab].Contents) {
					return true
				}
			}
		}
		if itemsHaveIndeterminate(it.Contents) {
			return true
		}
	}
	return false
}

// updateHasIndeterminate refreshes the window's HasIndeterminate flag based on
// its current contents.
func (win *windowData) updateHasIndeterminate() {
	if win == nil {
		return
	}
	win.HasIndeterminate = itemsHaveIndeterminate(win.Contents)
}

func markItemTreeDirty(it *itemData) {
	if it == nil {
		return
	}
	it.markDirty()
	for _, child := range it.Contents {
		markItemTreeDirty(child)
	}
	for _, tab := range it.Tabs {
		markItemTreeDirty(tab)
	}
}

// updateItemTooltipTree walks items and updates cached tooltip bounds.
func updateItemTooltipTree(it *itemData) {
	if it == nil {
		return
	}
	it.updateTooltipBounds()
	for _, child := range it.Contents {
		updateItemTooltipTree(child)
	}
	for _, tab := range it.Tabs {
		updateItemTooltipTree(tab)
	}
}

func updateAllTooltipBounds() {
	for _, win := range windows {
		for _, it := range win.Contents {
			updateItemTooltipTree(it)
		}
	}
}

func markAllDirty() {
	for _, win := range windows {
		win.markDirty()
		for _, it := range win.Contents {
			markItemTreeDirty(it)
		}
	}
}

func (item *itemData) bounds(offset point) rect {
	var r rect
	if item.ItemType == ITEM_FLOW && !item.Fixed {
		// Unfixed flows should report bounds based solely on their content
		r = rect{X0: offset.X, Y0: offset.Y, X1: offset.X, Y1: offset.Y}
	} else {
		size := item.GetSize()
		r = rect{
			X0: offset.X,
			Y0: offset.Y,
			X1: offset.X + size.X,
			Y1: offset.Y + size.Y,
		}
	}
	if item.ItemType == ITEM_FLOW {
		// For fixed flows, the bounds should be limited to the flow's
		// own rectangle so autosize and hit-testing don't expand to the
		// content beyond the fixed size. The content is visible via
		// scrolling/clipping as needed.
		if item.Fixed {
			return r
		}
		var flowOffset point
		var subItems []*itemData
		tabHeight := float32(0)
		if len(item.Tabs) > 0 {
			if item.ActiveTab >= len(item.Tabs) {
				item.ActiveTab = 0
			}
			subItems = item.Tabs[item.ActiveTab].Contents
			tabHeight = tabStripHeight(item, item.themeStyle())
			r.Y1 = offset.Y + tabHeight
			r.X1 = offset.X + staggeredTabStripWidth(item)
		} else {
			subItems = item.Contents
		}
		for _, sub := range subItems {
			var off point
			if item.FlowType == FLOW_HORIZONTAL {
				off = pointAdd(offset, point{X: flowOffset.X + sub.getPosition(item.ParentWindow).X, Y: tabHeight + sub.getPosition(item.ParentWindow).Y})
			} else if item.FlowType == FLOW_VERTICAL {
				off = pointAdd(offset, point{X: sub.getPosition(item.ParentWindow).X, Y: tabHeight + flowOffset.Y + sub.getPosition(item.ParentWindow).Y})
			} else {
				off = pointAdd(offset, pointAdd(point{X: flowOffset.X, Y: tabHeight + flowOffset.Y}, sub.getPosition(item.ParentWindow)))
			}
			sr := sub.bounds(off)
			r = unionRect(r, sr)
			if item.FlowType == FLOW_HORIZONTAL {
				flowOffset.X += sub.GetSize().X + sub.getPosition(item.ParentWindow).X
			} else if item.FlowType == FLOW_VERTICAL {
				flowOffset.Y += sub.GetSize().Y + sub.getPosition(item.ParentWindow).Y
			}
		}
	} else {
		for _, sub := range item.Contents {
			off := pointAdd(offset, sub.getPosition(item.ParentWindow))
			r = unionRect(r, sub.bounds(off))
		}
	}
	return r
}

func (win *windowData) contentBounds() point {
	if len(win.Contents) == 0 {
		return point{}
	}

	base := point{X: 0, Y: win.GetTitleSize()}
	first := true
	var b rect

	for _, item := range win.Contents {
		// Use the generic bounds calculation for all items, including flows.
		// This ensures fixed flows contribute their set size to autosize,
		// while unfixed flows contribute based on their content.
		r := item.bounds(pointAdd(base, item.getPosition(win)))
		if first {
			b = r
			first = false
		} else {
			b = unionRect(b, r)
		}
	}

	if first {
		return point{}
	}
	return point{X: b.X1 - base.X, Y: b.Y1 - base.Y}
}

func (win *windowData) updateAutoSize() {
	// Resolve nested flows before measuring them. In particular, a UI-scale
	// change can leave a flow with its previous dimensions until it is laid out
	// again; measuring that stale size made an auto-sized window one layout pass
	// too short and caused an unnecessary scrollbar.
	win.resizeFlows()
	req := win.contentBounds()
	pad := (win.Padding + win.BorderPad) * win.scale()

	size := win.GetSize()
	needX := float32(math.Ceil(float64(req.X + 2*pad)))
	if needX > size.X {
		size.X = needX
	}

	// Always include the titlebar height in the calculated size
	// Round outward to whole pixels so fractional UI scales cannot leave the
	// viewport slightly smaller than its content after converting to UI units.
	size.Y = float32(math.Ceil(float64(req.Y + win.GetTitleSize() + 2*pad)))
	if size.X > float32(screenWidth) {
		size.X = float32(screenWidth)
	}
	if size.Y > float32(screenHeight) {
		size.Y = float32(screenHeight)
	}
	s := win.scale()
	win.Size = point{X: size.X / s, Y: size.Y / s}
	win.resizeFlows()

	// A parent flow can grow after it receives its final width. Re-measure once
	// so the window tracks that resolved content rather than leaving a stray
	// scroll range after increasing UI scale.
	resolved := win.contentBounds()
	resolvedY := float32(math.Ceil(float64(resolved.Y + win.GetTitleSize() + 2*pad)))
	if resolvedY > size.Y && resolvedY <= float32(screenHeight) {
		win.Size.Y = resolvedY / s
		win.resizeFlows()
	}
	win.clampToScreen()
}

func (item *itemData) contentBounds() point {
	list := item.Contents
	tabHeight := float32(0)
	if len(item.Tabs) > 0 {
		if item.ActiveTab >= len(item.Tabs) {
			item.ActiveTab = 0
		}
		list = item.Tabs[item.ActiveTab].Contents
		tabHeight = tabStripHeight(item, item.themeStyle())
	}
	if len(list) == 0 {
		return point{Y: tabHeight}
	}

	base := point{}
	first := true
	var b rect
	var flowOffset point

	for _, sub := range list {
		if sub == nil {
			continue
		}
		off := pointAdd(base, sub.getPosition(item.ParentWindow))
		if item.ItemType == ITEM_FLOW {
			if item.FlowType == FLOW_HORIZONTAL {
				off = pointAdd(base, point{X: flowOffset.X + sub.getPosition(item.ParentWindow).X, Y: sub.getPosition(item.ParentWindow).Y})
			} else if item.FlowType == FLOW_VERTICAL {
				off = pointAdd(base, point{X: sub.getPosition(item.ParentWindow).X, Y: flowOffset.Y + sub.getPosition(item.ParentWindow).Y})
			} else {
				off = pointAdd(base, pointAdd(flowOffset, sub.getPosition(item.ParentWindow)))
			}
		}

		r := sub.bounds(off)
		if first {
			b = r
			first = false
		} else {
			b = unionRect(b, r)
		}

		if item.ItemType == ITEM_FLOW {
			if item.FlowType == FLOW_HORIZONTAL {
				flowOffset.X += sub.GetSize().X + sub.getPosition(item.ParentWindow).X
			} else if item.FlowType == FLOW_VERTICAL {
				flowOffset.Y += sub.GetSize().Y + sub.getPosition(item.ParentWindow).Y
			}
		}
	}

	if first {
		return point{Y: tabHeight}
	}
	return point{X: max(b.X1-base.X, staggeredTabStripWidth(item)), Y: b.Y1 - base.Y + tabHeight}
}

func (item *itemData) resizeFlow(parentSize point) {
	available := parentSize

	if item.ItemType == ITEM_FLOW {
		size := available
		if item.Fixed {
			size = item.GetSize()
			// If a fixed flow has unspecified dimensions, expand missing ones to content bounds.
			if size.X <= 1 || size.Y <= 1 {
				cb := item.contentBounds()
				if size.X <= 1 {
					size.X = cb.X
				}
				if size.Y <= 1 {
					size.Y = cb.Y
				}
			}
		} else if !item.Scrollable {
			// Unfixed, non-scrollable flows should size to their content
			size = item.contentBounds()
		}

		if !item.Scrollable {
			// Ensure the flow is large enough to contain its children
			req := item.contentBounds()
			if req.X > size.X {
				size.X = req.X
			}
			if req.Y > size.Y {
				size.Y = req.Y
			}
		}

		item.Size = point{X: size.X / uiScale, Y: size.Y / uiScale}
		available = item.GetSize()
	} else {
		available = item.GetSize()
	}

	var list []*itemData
	if len(item.Tabs) > 0 {
		if item.ActiveTab >= len(item.Tabs) {
			item.ActiveTab = 0
		}
		list = item.Tabs[item.ActiveTab].Contents
	} else {
		list = item.Contents
	}
	for _, sub := range list {
		if sub == nil {
			continue
		}
		sub.resizeFlow(available)
	}

	if item.ItemType == ITEM_FLOW {
		req := item.contentBounds()
		size := item.GetSize()
		if req.Y <= size.Y {
			item.Scroll.Y = 0
		} else {
			max := req.Y - size.Y
			if item.Scroll.Y > max {
				item.Scroll.Y = max
			}
		}
		if req.X <= size.X {
			item.Scroll.X = 0
		} else {
			max := req.X - size.X
			if item.Scroll.X > max {
				item.Scroll.X = max
			}
		}
	}
}

func (win *windowData) resizeFlows() {
	for _, item := range win.Contents {
		item.resizeFlow(win.GetSize())
	}
}

func pixelOffset(width float32) float32 {
	if int(math.Round(float64(width)))%2 == 0 {
		return 0
	}
	return 0.5
}

func strokeLine(dst *ebiten.Image, x0, y0, x1, y1, width float32, col color.Color, aa bool) {
	width = float32(math.Round(float64(width)))
	off := pixelOffset(width)
	x0 = float32(math.Round(float64(x0))) + off
	y0 = float32(math.Round(float64(y0))) + off
	x1 = float32(math.Round(float64(x1))) + off
	y1 = float32(math.Round(float64(y1))) + off
	strokeLineFn(dst, x0, y0, x1, y1, width, col, aa)
}

func strokeRect(dst *ebiten.Image, x, y, w, h, width float32, col color.Color, aa bool) {
	width = float32(math.Round(float64(width)))
	off := pixelOffset(width)
	x = float32(math.Round(float64(x))) + off
	y = float32(math.Round(float64(y))) + off
	w = float32(math.Round(float64(w)))
	h = float32(math.Round(float64(h)))
	strokeRectFn(dst, x, y, w, h, width, col, aa)
}

func drawFilledRect(dst *ebiten.Image, x, y, w, h float32, col color.Color, _ bool) {
	x = float32(math.Round(float64(x)))
	y = float32(math.Round(float64(y)))
	w = float32(math.Round(float64(w)))
	h = float32(math.Round(float64(h)))
	// All edges are on pixel boundaries, so antialiasing adds no coverage.
	// The non-AA path batches an image quad instead of building a stencil
	// and filling it, which is especially costly during full-pane repaints.
	vector.FillRect(dst, x, y, w, h, col, false)
}
