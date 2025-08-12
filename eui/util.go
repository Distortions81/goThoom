package eui

import (
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

var (
	strokeLineFn = vector.StrokeLine
	strokeRectFn = vector.StrokeRect
)

func (item *itemData) themeStyle() *itemData {
	if item == nil || item.Theme == nil {
		return nil
	}
	switch item.ItemType {
	case ITEM_BUTTON:
		return &item.Theme.Button
	case ITEM_TEXT:
		return &item.Theme.Text
	case ITEM_CHECKBOX:
		return &item.Theme.Checkbox
	case ITEM_RADIO:
		return &item.Theme.Radio
	case ITEM_INPUT:
		return &item.Theme.Input
	case ITEM_SLIDER:
		return &item.Theme.Slider
	case ITEM_DROPDOWN:
		return &item.Theme.Dropdown
	case ITEM_FLOW:
		if len(item.Tabs) > 0 {
			return &item.Theme.Tab
		}
	}
	return nil
}

func (win *windowData) getWinRect() rect {
	winPos := win.getPosition()
	return rect{
		X0: winPos.X,
		Y0: winPos.Y,
		X1: winPos.X + win.GetSize().X,
		Y1: winPos.Y + win.GetSize().Y,
	}
}

func (item *itemData) getItemRect(win *windowData) rect {
	return rect{
		X0: win.getPosition().X + (item.getPosition(win).X),
		Y0: win.getPosition().Y + (item.getPosition(win).Y),
		X1: win.getPosition().X + (item.getPosition(win).X) + (item.GetSize().X),
		Y1: win.getPosition().Y + (item.getPosition(win).Y) + (item.GetSize().Y),
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
}

func (parent *windowData) addItemTo(item *itemData) {
	if item.Theme == nil {
		item.Theme = parent.Theme
	}
	parent.Contents = append(parent.Contents, item)
	item.setParentWindow(parent)
	item.resizeFlow(parent.GetSize())
	parent.markDirty()
}

func (win *windowData) getMainRect() rect {
	return rect{
		X0: win.getPosition().X,
		Y0: win.getPosition().Y + win.GetTitleSize(),
		X1: win.getPosition().X + win.GetSize().X,
		Y1: win.getPosition().Y + win.GetSize().Y,
	}
}

func (win *windowData) getTitleRect() rect {
	if win.TitleHeight <= 0 {
		return rect{}
	}
	return rect{
		X0: win.getPosition().X, Y0: win.getPosition().Y,
		X1: win.getPosition().X + win.GetSize().X,
		Y1: win.getPosition().Y + (win.GetTitleSize()),
	}
}

func (win *windowData) xRect() rect {
	if win.TitleHeight <= 0 || !win.Closable {
		return rect{}
	}

	var xpad float32 = win.Border
	return rect{
		X0: win.getPosition().X + win.GetSize().X - (win.GetTitleSize()) + xpad,
		Y0: win.getPosition().Y + xpad,

		X1: win.getPosition().X + win.GetSize().X - xpad,
		Y1: win.getPosition().Y + (win.GetTitleSize()) - xpad,
	}
}

func (win *windowData) dragbarRect() rect {
	if win.TitleHeight <= 0 && !win.Resizable {
		return rect{}
	}
	textSize := win.titleTextWidth()
	xRect := win.xRect()
	buttonsWidth := xRect.X1 - xRect.X0 + 3

	dpad := (win.GetTitleSize()) / 5
	xStart := textSize.X + float32((win.GetTitleSize())/1.5)
	xEnd := (win.GetSize().X - buttonsWidth)
	return rect{
		X0: win.getPosition().X + xStart, Y0: win.getPosition().Y + dpad,
		X1: win.getPosition().X + xEnd, Y1: win.getPosition().Y + (win.GetTitleSize()) - dpad,
	}
}

func (win *windowData) Refresh() {
	win.resizeFlows()
	win.updateAutoSize()
	win.markDirty()
}

func (win *windowData) IsOpen() bool {
	return win.Open
}

func (win *windowData) setSize(size point) bool {
	if size.X < 1 || size.Y < 1 {
		return false
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
	if win.zone != nil {
		return
	}
	pos := win.getPosition()
	size := win.GetSize()
	old := win.Position
	s := win.scale()

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
		visible = 5
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
	if part := win.getTitlebarPart(mpos); part != PART_NONE {
		return part
	}
	if part := win.getResizePart(mpos); part != PART_NONE {
		return part
	}
	return win.getScrollbarPart(mpos)
}

func (win *windowData) getTitlebarPart(mpos point) dragType {
	if win.TitleHeight <= 0 {
		return PART_NONE
	}
	if win.getTitleRect().containsPoint(mpos) {
		if win.Closable && win.xRect().containsPoint(mpos) {
			win.HoverClose = true
			return PART_CLOSE
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
		barH := avail.Y * avail.Y / req.Y
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
		barW := avail.X * avail.X / req.X
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

func (win *windowData) titleTextWidth() point {
	if win.TitleHeight <= 0 {
		return point{}
	}
	textSize := ((win.GetTitleSize()) / 1.5)
	face := textFace(textSize)
	textWidth, textHeight := text.Measure(win.Title, face, 0)
	return point{X: float32(textWidth), Y: float32(textHeight)}
}

func (win *windowData) SetTitleSize(size float32) {
	win.TitleHeight = size / win.scale()
}

func SetUIScale(scale float32) {
	uiScale = scale
	for _, win := range windows {
		if win.AutoSize {
			win.updateAutoSize()
		} else {
			win.resizeFlows()
		}
	}
	markAllDirty()
}

// SyncHiDPIScale adjusts the UI scale when the device scale factor changes.
// It preserves the current UI scale relative to the previous factor so the
// interface keeps the same on-screen size.
func SyncHiDPIScale() {
	ds := ebiten.Monitor().DeviceScaleFactor()
	if ds <= 0 {
		ds = 1
	}
	if ds != lastDeviceScale {
		SetUIScale(uiScale * float32(ds/lastDeviceScale))
		lastDeviceScale = ds
	}
}

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
	sz := Point{X: item.Size.X * uiScale, Y: item.Size.Y * uiScale}
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

func (win *windowData) markDirty() {
	if win != nil {
		win.Dirty = true
	}
}

func (item *itemData) markDirty() {
	if item != nil && item.ItemType != ITEM_FLOW {
		item.Dirty = true
		if item.ParentWindow != nil {
			item.ParentWindow.markDirty()
		}
	}
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
		r = rect{
			X0: offset.X,
			Y0: offset.Y,
			X1: offset.X + item.GetSize().X,
			Y1: offset.Y + item.GetSize().Y,
		}
	}
	if item.ItemType == ITEM_FLOW {
		var flowOffset point
		var subItems []*itemData
		if len(item.Tabs) > 0 {
			if item.ActiveTab >= len(item.Tabs) {
				item.ActiveTab = 0
			}
			subItems = item.Tabs[item.ActiveTab].Contents
		} else {
			subItems = item.Contents
		}
		for _, sub := range subItems {
			var off point
			if item.FlowType == FLOW_HORIZONTAL {
				off = pointAdd(offset, point{X: flowOffset.X + sub.GetPos().X, Y: sub.GetPos().Y})
			} else if item.FlowType == FLOW_VERTICAL {
				off = pointAdd(offset, point{X: sub.GetPos().X, Y: flowOffset.Y + sub.GetPos().Y})
			} else {
				off = pointAdd(offset, pointAdd(flowOffset, sub.GetPos()))
			}
			sr := sub.bounds(off)
			r = unionRect(r, sr)
			if item.FlowType == FLOW_HORIZONTAL {
				flowOffset.X += sub.GetSize().X + sub.GetPos().X
			} else if item.FlowType == FLOW_VERTICAL {
				flowOffset.Y += sub.GetSize().Y + sub.GetPos().Y
			}
		}
	} else {
		for _, sub := range item.Contents {
			off := pointAdd(offset, sub.GetPos())
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
		var r rect
		if item.ItemType == ITEM_FLOW {
			cb := item.contentBounds()
			r = rect{
				X0: base.X + item.GetPos().X,
				Y0: base.Y + item.GetPos().Y,
				X1: base.X + item.GetPos().X + cb.X,
				Y1: base.Y + item.GetPos().Y + cb.Y,
			}
		} else {
			r = item.bounds(pointAdd(base, item.GetPos()))
		}
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
	req := win.contentBounds()
	pad := (win.Padding + win.BorderPad) * win.scale()

	size := win.GetSize()
	needX := req.X + 2*pad
	if needX > size.X {
		size.X = needX
	}

	// Always include the titlebar height in the calculated size
	size.Y = req.Y + win.GetTitleSize() + 2*pad
	if size.X > float32(screenWidth) {
		size.X = float32(screenWidth)
	}
	if size.Y > float32(screenHeight) {
		size.Y = float32(screenHeight)
	}
	s := win.scale()
	win.Size = point{X: size.X / s, Y: size.Y / s}
	win.resizeFlows()
	win.clampToScreen()
}

func (item *itemData) contentBounds() point {
	list := item.Contents
	if len(item.Tabs) > 0 {
		if item.ActiveTab >= len(item.Tabs) {
			item.ActiveTab = 0
		}
		list = item.Tabs[item.ActiveTab].Contents
	}
	if len(list) == 0 {
		return point{}
	}

	base := point{}
	first := true
	var b rect
	var flowOffset point

	for _, sub := range list {
		off := pointAdd(base, sub.GetPos())
		if item.ItemType == ITEM_FLOW {
			if item.FlowType == FLOW_HORIZONTAL {
				off = pointAdd(base, point{X: flowOffset.X + sub.GetPos().X, Y: sub.GetPos().Y})
			} else if item.FlowType == FLOW_VERTICAL {
				off = pointAdd(base, point{X: sub.GetPos().X, Y: flowOffset.Y + sub.GetPos().Y})
			} else {
				off = pointAdd(base, pointAdd(flowOffset, sub.GetPos()))
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
				flowOffset.X += sub.GetSize().X + sub.GetPos().X
			} else if item.FlowType == FLOW_VERTICAL {
				flowOffset.Y += sub.GetSize().Y + sub.GetPos().Y
			}
		}
	}

	if first {
		return point{}
	}
	return point{X: b.X1 - base.X, Y: b.Y1 - base.Y}
}

func (item *itemData) resizeFlow(parentSize point) {
	available := parentSize

	if item.ItemType == ITEM_FLOW {
		size := available
		if item.Fixed {
			size = item.GetSize()
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

func drawFilledRect(dst *ebiten.Image, x, y, w, h float32, col color.Color, aa bool) {
	x = float32(math.Round(float64(x)))
	y = float32(math.Round(float64(y)))
	w = float32(math.Round(float64(w)))
	h = float32(math.Round(float64(h)))
	vector.DrawFilledRect(dst, x, y, w, h, col, aa)
}
