package eui

import (
	"image/color"
	"math"
	"strings"

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
	item.win = parent.win
	parent.Contents = append(parent.Contents, item)
	if parent.ItemType == ITEM_FLOW {
		parent.resizeFlow(parent.GetSize())
	}
	if parent.win != nil {
		parent.win.Dirty = true
	}
}

func (parent *windowData) addItemTo(item *itemData) {
	parent.Contents = append(parent.Contents, item)
	item.win = parent
	item.resizeFlow(parent.GetSize())
	parent.Dirty = true
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

func (win *windowData) setSize(size point) bool {
	orig := win.Size

	size = win.applyAspect(size, true)
	size, tooSmall := win.clampSize(size)
	size = win.applyAspect(size, false)
	size, tooSmall2 := win.clampSize(size)
	tooSmall = tooSmall || tooSmall2

	xc, yc := win.itemOverlap(size)
	if !xc {
		win.Size.X = size.X
	}
	if !yc {
		win.Size.Y = size.Y
	}
	if yc && xc {
		tooSmall = true
	}

	win.BringForward()
	win.resizeFlows()
	win.adjustScrollForResize()
	win.clampToScreen()

	if win.Size != orig {
		win.Dirty = true
	}

	return tooSmall
}

func (win *windowData) clampSize(size point) (point, bool) {
	tooSmall := false

	// Enforce minimum dimensions and prevent negatives.
	if size.X < minWinSizeX || size.X < 0 {
		size.X = minWinSizeX
		tooSmall = true
	}
	if size.Y < minWinSizeY || size.Y < 0 {
		size.Y = minWinSizeY
		tooSmall = true
	}

	return size, tooSmall
}

func (win *windowData) applyAspect(size point, useDelta bool) point {
	if !win.FixedRatio || win.AspectA <= 0 || win.AspectB <= 0 {
		return size
	}
	aspect := win.AspectA / win.AspectB
	title := win.TitleHeight
	if useDelta {
		dx := math.Abs(float64(size.X - win.Size.X))
		dy := math.Abs(float64(size.Y - win.Size.Y))
		if dx >= dy {
			contentH := size.X / aspect
			size.Y = contentH + title
		} else {
			contentH := size.Y - title
			if contentH < 0 {
				contentH = 0
			}
			size.X = contentH * aspect
			size.Y = contentH + title
		}
	} else {
		contentH := size.X / aspect
		size.Y = contentH + title
	}
	return size
}

func (win *windowData) adjustScrollForResize() {
	if win.NoScroll {
		return
	}

	pad := (win.Padding + win.BorderPad) * uiScale
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
}

// clampToScreen ensures the window remains within the visible screen area.
// When skipReposition is true the window's position is left unchanged. This is
// useful for auto-size operations where callers want to preserve the current
// location even if the new size would normally force the window to move.
func (win *windowData) clampToScreen(skipReposition ...bool) {
	if len(skipReposition) > 0 && skipReposition[0] {
		return
	}
	size := win.GetSize()
	m := win.Margin * uiScale

	switch win.PinTo {
	case PIN_TOP_LEFT, PIN_MID_LEFT, PIN_BOTTOM_LEFT:
		off := win.GetPos().X
		min := float32(0)
		max := float32(screenWidth) - size.X - m
		if off < min {
			win.Position.X = min / uiScale
		} else if off > max {
			win.Position.X = max / uiScale
		}
	case PIN_TOP_RIGHT, PIN_MID_RIGHT, PIN_BOTTOM_RIGHT:
		off := win.GetPos().X
		min := float32(0)
		max := float32(screenWidth) - size.X - m
		if off < min {
			win.Position.X = min / uiScale
		} else if off > max {
			win.Position.X = max / uiScale
		}
	case PIN_TOP_CENTER, PIN_MID_CENTER, PIN_BOTTOM_CENTER:
		off := win.GetPos().X
		max := float32(screenWidth)/2 - size.X/2 - m
		if off < -max {
			win.Position.X = -max / uiScale
		} else if off > max {
			win.Position.X = max / uiScale
		}
	}

	switch win.PinTo {
	case PIN_TOP_LEFT, PIN_TOP_CENTER, PIN_TOP_RIGHT:
		off := win.GetPos().Y
		min := float32(0)
		max := float32(screenHeight) - size.Y - m
		if off < min {
			win.Position.Y = min / uiScale
		} else if off > max {
			win.Position.Y = max / uiScale
		}
	case PIN_BOTTOM_LEFT, PIN_BOTTOM_CENTER, PIN_BOTTOM_RIGHT:
		off := win.GetPos().Y
		min := float32(0)
		max := float32(screenHeight) - size.Y - m
		if off < min {
			win.Position.Y = min / uiScale
		} else if off > max {
			win.Position.Y = max / uiScale
		}
	case PIN_MID_LEFT, PIN_MID_CENTER, PIN_MID_RIGHT:
		off := win.GetPos().Y
		max := float32(screenHeight)/2 - size.Y/2 - m
		if off < -max {
			win.Position.Y = -max / uiScale
		} else if off > max {
			win.Position.Y = max / uiScale

		}
	}
}

// dropdownOpenRect returns the rectangle used for drawing and input handling of
// an open dropdown menu. The rectangle is adjusted so it never extends off the
// screen while leaving room for overlay controls at the top and bottom equal to
// one option height. If there isn't enough room below the dropdown, the menu
// will open upward so it doesn't cover the button.
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

	bottomLimit := float32(screenHeight) - optionH*dropdownOverlayReserve
	topLimit := optionH * dropdownOverlayReserve
	startDown := offset.Y + maxSize.Y
	spaceBelow := bottomLimit - startDown
	spaceAbove := offset.Y - topLimit

	openH := optionH * float32(visible)

	if openH <= spaceBelow {
		r := rect{X0: offset.X, Y0: startDown, X1: offset.X + maxSize.X, Y1: startDown + openH}
		return r, visible
	}
	if openH <= spaceAbove {
		startUp := offset.Y - openH
		r := rect{X0: offset.X, Y0: startUp, X1: offset.X + maxSize.X, Y1: offset.Y}
		return r, visible
	}

	if spaceBelow >= spaceAbove {
		maxVis := int(spaceBelow / optionH)
		if maxVis < 1 {
			maxVis = 1
		}
		if visible > maxVis {
			visible = maxVis
			openH = optionH * float32(visible)
		}
		r := rect{X0: offset.X, Y0: startDown, X1: offset.X + maxSize.X, Y1: startDown + openH}
		return r, visible
	}

	maxVis := int(spaceAbove / optionH)
	if maxVis < 1 {
		maxVis = 1
	}
	if visible > maxVis {
		visible = maxVis
		openH = optionH * float32(visible)
	}
	startUp := offset.Y - openH
	r := rect{X0: offset.X, Y0: startUp, X1: offset.X + maxSize.X, Y1: offset.Y}
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

	t := scrollTolerance * uiScale
	ct := cornerTolerance * uiScale
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

	pad := (win.Padding + win.BorderPad) * uiScale
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
	size := (win.GetTitleSize()) / 2
	face := textFace(size)
	win.updateTitleCache(face, size)
	return point{X: float32(win.titleTextW), Y: float32(win.titleTextH)}
}

func (win *windowData) SetTitleSize(size float32) {
	win.TitleHeight = size / uiScale
	win.TitleHeightSet = true
	win.invalidateTitleCache()
	win.Dirty = true
	win.resizeFlows()
}

func (win *windowData) SetTitle(title string) {
	if win.Title != title {
		win.Title = title
		win.invalidateTitleCache()
		win.Dirty = true
		win.resizeFlows()
	}
}

func (win *windowData) NoTitlebar() {
	win.NoTitle = true
	win.NoTitleSet = true
	win.TitleHeight = 0
	win.TitleHeightSet = true
	win.ShowDragbar = false
	win.invalidateTitleCache()
	win.Dirty = true
	win.resizeFlows()
}

func (win *windowData) invalidateTitleCache() {
	win.titleRaw = ""
}

func (win *windowData) updateTitleCache(face text.Face, size float32) {
	if win.titleRaw != win.Title || win.titleTextSize != size {
		buf := strings.ReplaceAll(win.Title, "\n", "")
		buf = strings.ReplaceAll(buf, "\r", "")
		win.titleRaw = win.Title
		win.titleText = buf
		win.titleTextSize = size
		win.titleTextW, win.titleTextH = text.Measure(buf, face, 0)
	}
}

func SetUIScale(scale float32) {
	if scale < 1.0 {
		scale = 1.0
	}
	if scale > 2.5 {
		scale = 2.5
	}
	uiScale = scale
	for _, win := range windows {
		if win.AutoSize || win.AutoSizeOnScale {
			win.updateAutoSize()
		} else {
			win.resizeFlows()
		}
		win.adjustScrollForResize()
		win.clampToScreen()
	}
	for _, ov := range overlays {
		ov.resizeFlow(ov.GetSize())
	}
	markAllDirty()
}

func UIScale() float32 { return uiScale }

func (win *windowData) GetTitleSize() float32 {
	return win.TitleHeight * uiScale
}

func (win *windowData) GetSize() point {
	return point{X: win.Size.X * uiScale, Y: win.Size.Y * uiScale}
}

func (win *windowData) GetPos() point {
	return point{X: win.Position.X * uiScale, Y: win.Position.Y * uiScale}
}

func (item *itemData) labelHeight() float32 {
	var imgH, textH float32
	if item.LabelImage != nil {
		h := float32(item.LabelImage.Bounds().Dy())
		if item.LabelImageSize.Y > 0 {
			h = item.LabelImageSize.Y
		}
		imgH = h * uiScale
	}
	if item.Label != "" {
		textH = (item.FontSize * uiScale) + 2
	}
	if imgH < textH {
		imgH = textH
	}
	return imgH
}

func (item *itemData) GetSize() point {
	sz := point{X: item.Size.X * uiScale, Y: item.Size.Y * uiScale}
	lh := item.labelHeight()
	if lh > 0 {
		sz.Y += lh + currentStyle.TextPadding*uiScale
	}
	return sz
}

func (item *itemData) GetPos() point {
	return point{X: item.Position.X * uiScale, Y: item.Position.Y * uiScale}
}

func (item *itemData) GetTextPtr() *string {
	return &item.Text
}

func (item *itemData) markDirty() {
	if item != nil && item.ItemType != ITEM_FLOW {
		item.Dirty = true
		if item.win != nil {
			item.win.Dirty = true
		}
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
		for _, it := range win.Contents {
			markItemTreeDirty(it)
		}
	}
	for _, ov := range overlays {
		markItemTreeDirty(ov)
	}
}

func itemTreeDirty(it *itemData) bool {
	if it == nil {
		return false
	}
	if it.Dirty {
		return true
	}
	for _, child := range it.Contents {
		if itemTreeDirty(child) {
			return true
		}
	}
	for _, tab := range it.Tabs {
		if itemTreeDirty(tab) {
			return true
		}
	}
	return false
}

func (win *windowData) itemsDirty() bool {
	for _, it := range win.Contents {
		if itemTreeDirty(it) {
			return true
		}
	}
	return false
}

func (item *itemData) bounds(offset point) rect {
	m := item.Margin * uiScale
	var r rect
	if item.ItemType == ITEM_FLOW && !item.Fixed {
		// Unfixed flows should report bounds based solely on their content
		r = rect{X0: offset.X, Y0: offset.Y, X1: offset.X + m, Y1: offset.Y + m}
	} else {
		r = rect{
			X0: offset.X,
			Y0: offset.Y,
			X1: offset.X + item.GetSize().X + m,
			Y1: offset.Y + item.GetSize().Y + m,
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
			sm := sub.Margin * uiScale
			var off point
			if item.FlowType == FLOW_HORIZONTAL {
				off = pointAdd(offset, point{X: flowOffset.X + sub.GetPos().X + sm, Y: sub.GetPos().Y + sm})
			} else if item.FlowType == FLOW_VERTICAL {
				off = pointAdd(offset, point{X: sub.GetPos().X + sm, Y: flowOffset.Y + sub.GetPos().Y + sm})
			} else {
				off = pointAdd(offset, pointAdd(flowOffset, point{X: sub.GetPos().X + sm, Y: sub.GetPos().Y + sm}))
			}
			sr := sub.bounds(off)
			r = unionRect(r, sr)
			if item.FlowType == FLOW_HORIZONTAL {
				flowOffset.X += sub.GetSize().X + sub.GetPos().X + sm
			} else if item.FlowType == FLOW_VERTICAL {
				flowOffset.Y += sub.GetSize().Y + sub.GetPos().Y + sm
			}
		}
	} else {
		for _, sub := range item.Contents {
			sm := sub.Margin * uiScale
			off := pointAdd(offset, point{X: sub.GetPos().X + sm, Y: sub.GetPos().Y + sm})
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
		m := item.Margin * uiScale
		if item.ItemType == ITEM_FLOW {
			cb := item.contentBounds()
			r = rect{
				X0: base.X + item.GetPos().X + m,
				Y0: base.Y + item.GetPos().Y + m,
				X1: base.X + item.GetPos().X + cb.X + m,
				Y1: base.Y + item.GetPos().Y + cb.Y + m,
			}
		} else {
			off := pointAdd(base, point{X: item.GetPos().X + m, Y: item.GetPos().Y + m})
			r = item.bounds(off)
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

// updateAutoSize adjusts the window size to fit its contents. If
// skipReposition is true the window size is updated without clamping its
// position to the screen, allowing callers to preserve the current location
// during auto-size operations.
func (win *windowData) updateAutoSize(skipReposition ...bool) {
	req := win.contentBounds()
	pad := (win.Padding + win.BorderPad) * uiScale

	size := win.GetSize()
	needX := req.X + 2*pad
	if needX > size.X {
		size.X = needX
	}

	// Always include the titlebar height in the calculated size
	size.Y = req.Y + win.GetTitleSize() + 2*pad
	size = win.applyAspect(size, false)
	if size.X > float32(screenWidth) {
		size.X = float32(screenWidth)
	}
	if size.Y > float32(screenHeight) {
		size.Y = float32(screenHeight)
	}
	win.Size = point{X: size.X / uiScale, Y: size.Y / uiScale}
	win.resizeFlows()
	win.clampToScreen(skipReposition...)
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
		sm := sub.Margin * uiScale
		off := pointAdd(base, point{X: sub.GetPos().X + sm, Y: sub.GetPos().Y + sm})
		if item.ItemType == ITEM_FLOW {
			if item.FlowType == FLOW_HORIZONTAL {
				off = pointAdd(base, point{X: flowOffset.X + sub.GetPos().X + sm, Y: sub.GetPos().Y + sm})
			} else if item.FlowType == FLOW_VERTICAL {
				off = pointAdd(base, point{X: sub.GetPos().X + sm, Y: flowOffset.Y + sub.GetPos().Y + sm})
			} else {
				off = pointAdd(base, pointAdd(flowOffset, point{X: sub.GetPos().X + sm, Y: sub.GetPos().Y + sm}))
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
				flowOffset.X += sub.GetSize().X + sub.GetPos().X + sm
			} else if item.FlowType == FLOW_VERTICAL {
				flowOffset.Y += sub.GetSize().Y + sub.GetPos().Y + sm
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
