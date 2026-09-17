package eui

import "github.com/hajimehoshi/ebiten/v2"

func (item *itemData) editScrollbar(part dragType, offset, size point) (track, thumb rect, limit float32) {
	viewport, vertical, horizontal := item.editScrollGeometry(offset, size)
	layout, state := item.editLayout(), item.editor()
	track = vertical
	length, content, scroll := viewport.Y1-viewport.Y0, float32(len(layout.lines))*layout.lineHeight, state.scroll.Y
	if part == PART_SCROLL_H {
		track = horizontal
		length, content, scroll = viewport.X1-viewport.X0, layout.width+2, state.scroll.X
	}
	if track.X1 <= track.X0 || track.Y1 <= track.Y0 {
		return track, rect{}, 0
	}
	limit = max(0, content-length)
	thumb = track
	thumbLength := scrollbarThumbLength(length, content, uiScale)
	position := float32(0)
	if limit > 0 {
		position = min(max(0, scroll), limit) / limit * (length - thumbLength)
	}
	if part == PART_SCROLL_V {
		thumb.Y0 += position
		thumb.Y1 = thumb.Y0 + thumbLength
	} else {
		thumb.X0 += position
		thumb.X1 = thumb.X0 + thumbLength
	}
	return
}

func (item *itemData) editScrollbarAt(pos point) dragType {
	if !itemHandlesTextEditing(item) || !item.Multiline || !item.DrawRect.containsPoint(pos) {
		return PART_NONE
	}
	offset, size := item.editDrawGeometry()
	for _, part := range []dragType{PART_SCROLL_V, PART_SCROLL_H} {
		track, _, limit := item.editScrollbar(part, offset, size)
		if limit > 0 && track.containsPoint(pos) {
			return part
		}
	}
	return PART_NONE
}

func (item *itemData) pressEditScrollbar(pos point, part dragType) {
	state := item.editor()
	offset, size := item.editDrawGeometry()
	track, thumb, _ := item.editScrollbar(part, offset, size)
	p, start, end, page := pos.Y, thumb.Y0, thumb.Y1, track.Y1-track.Y0
	if part == PART_SCROLL_H {
		p, start, end, page = pos.X, thumb.X0, thumb.X1, track.X1-track.X0
	}
	state.followCaret = false
	item.selecting = false
	if p >= start && p <= end {
		state.scrollDrag, state.scrollGrab = part, p-start
	} else {
		if p < start {
			page = -page
		}
		if part == PART_SCROLL_V {
			state.scroll.Y += page
		} else {
			state.scroll.X += page
		}
		viewport, _ := item.editGeometry()
		item.clampEditScroll(viewport)
	}
	item.markDirty()
}

func (item *itemData) dragEditScrollbar(pos point) {
	state := item.editor()
	if state.scrollDrag == PART_NONE {
		return
	}
	offset, size := item.editDrawGeometry()
	track, thumb, limit := item.editScrollbar(state.scrollDrag, offset, size)
	p, start, travel := pos.Y, track.Y0, track.Y1-track.Y0-(thumb.Y1-thumb.Y0)
	if state.scrollDrag == PART_SCROLL_H {
		p, start, travel = pos.X, track.X0, track.X1-track.X0-(thumb.X1-thumb.X0)
	}
	if travel <= 0 {
		return
	}
	scroll := min(max(0, (p-start-state.scrollGrab)/travel), 1) * limit
	if state.scrollDrag == PART_SCROLL_V {
		state.scroll.Y = scroll
	} else {
		state.scroll.X = scroll
	}
	state.followCaret = false
	item.markDirty()
}

func (item *itemData) editScrollbarColors() (track, thumb Color) {
	style := item.themeStyle()
	track = item.Color
	if track == (Color{}) {
		track = style.Color
	}
	backdrop := baseTheme.Window.BGColor
	if item.Theme != nil {
		backdrop = item.Theme.Window.BGColor
	}
	if item.ParentWindow != nil {
		backdrop = item.ParentWindow.backgroundColor()
	}
	track = colorOver(track, backdrop)
	track.A = 255
	thumb = colorOver(readableTextColor(style.ClickColor, track), track)
	return
}

func (item *itemData) drawEditScrollbars(dst *ebiten.Image, offset, size point, clip rect) {
	if !item.Multiline {
		return
	}
	trackColor, thumbColor := item.editScrollbarColors()
	for _, part := range []dragType{PART_SCROLL_V, PART_SCROLL_H} {
		track, thumb, limit := item.editScrollbar(part, offset, size)
		if limit <= 0 {
			continue
		}
		track, thumb = intersectRect(track, clip), intersectRect(thumb, clip)
		if track.X1 > track.X0 && track.Y1 > track.Y0 {
			drawFilledRect(dst, track.X0, track.Y0, track.X1-track.X0, track.Y1-track.Y0, trackColor, false)
		}
		if thumb.X1 > thumb.X0 && thumb.Y1 > thumb.Y0 {
			drawFilledRect(dst, thumb.X0, thumb.Y0, thumb.X1-thumb.X0, thumb.Y1-thumb.Y0, thumbColor, false)
		}
	}
}
