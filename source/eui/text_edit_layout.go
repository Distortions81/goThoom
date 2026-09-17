package eui

import (
	"math"
	"strings"
	"unicode/utf8"

	"gothoom/internal/inputkeys"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
)

type editTextLine struct {
	start, length int // source rune indices, excluding the newline
	display       string
	bytes         []int // source rune boundary -> display byte boundary (tabs expand)
	stops         []int // grapheme boundaries in the source line
	width         float32
}

type editTextLayout struct {
	value             string
	face              text.Face
	lines             []editTextLine
	lineHeight, width float32
}

func (line editTextLine) advance(pos int, face text.Face) float32 {
	pos = max(0, min(pos, line.length))
	return float32(text.AdvanceAt(line.display, line.bytes[pos], face))
}

func (line editTextLine) nearest(x float32, face text.Face) int {
	best, distance := 0, float32(math.Inf(1))
	for _, pos := range line.stops {
		d := float32(math.Abs(float64(x - line.advance(pos, face))))
		if d < distance {
			best, distance = pos, d
		}
	}
	return line.start + best
}

func (item *itemData) editLayout() *editTextLayout {
	s := item.editor()
	face := itemFace(item, item.FontSize*uiScale+2)
	if s.layout != nil && s.layout.value == item.Text && s.layout.face == face {
		return s.layout
	}
	metrics := face.Metrics()
	layout := &editTextLayout{value: item.Text, face: face, lineHeight: float32(math.Ceil(metrics.HAscent + metrics.HDescent + 2))}
	start := 0
	for _, raw := range strings.Split(item.Text, "\n") {
		line := editTextLine{start: start, length: utf8.RuneCountInString(raw), stops: graphemeStops(raw), bytes: []int{0}}
		var display strings.Builder
		column := 0
		for _, r := range raw {
			if r == '\t' {
				spaces := 4 - column%4
				display.WriteString(strings.Repeat(" ", spaces))
				column += spaces
			} else {
				display.WriteRune(r)
				column++
			}
			line.bytes = append(line.bytes, display.Len())
		}
		if item.HideText {
			// Mask glyphs still represent source rune offsets; pointer placement
			// must not split a composed character in the underlying password.
			line.stops = graphemeStops(item.SecretText)
		}
		line.display = display.String()
		line.width = float32(text.AdvanceAt(line.display, len(line.display), face))
		layout.width = max(layout.width, line.width)
		layout.lines = append(layout.lines, line)
		start += line.length + 1
	}
	s.layout = layout
	return layout
}

func (layout *editTextLayout) caret(pos int) (int, float32) {
	for i, line := range layout.lines {
		if pos <= line.start+line.length || i == len(layout.lines)-1 {
			return i, line.advance(pos-line.start, layout.face)
		}
	}
	return 0, 0
}

func (item *itemData) editViewport(offset, size point) rect {
	viewport, _, _ := item.editScrollGeometry(offset, size)
	return viewport
}

func (item *itemData) editScrollGeometry(offset, size point) (rect, rect, rect) {
	pad := (item.BorderPad + item.Padding + currentStyle.TextPadding) * uiScale
	// Small inputs must retain a full line even when a theme has generous
	// horizontal padding. Center that line in the available control height.
	padY := min(pad, max(0, (size.Y-item.editLayout().lineHeight)/2))
	viewport := rect{X0: offset.X + pad, Y0: offset.Y + padY,
		X1: max(offset.X+pad+1, offset.X+size.X-pad),
		Y1: max(offset.Y+padY+1, offset.Y+size.Y-padY)}
	var vertical, horizontal rect
	if item.Multiline {
		layout := item.editLayout()
		width := min(max(10*uiScale, ScrollbarWidth()), (viewport.X1-viewport.X0)/2, (viewport.Y1-viewport.Y0)/2)
		needV, needH := false, false
		// One scrollbar can make the other axis overflow.
		for range 2 {
			if !needV && float32(len(layout.lines))*layout.lineHeight > viewport.Y1-viewport.Y0 {
				needV = true
				viewport.X1 -= width
			}
			if !needH && layout.width+2 > viewport.X1-viewport.X0 {
				needH = true
				viewport.Y1 -= width
			}
		}
		if needV {
			vertical = rect{X0: viewport.X1, Y0: viewport.Y0, X1: viewport.X1 + width, Y1: viewport.Y1}
		}
		if needH {
			horizontal = rect{X0: viewport.X0, Y0: viewport.Y1, X1: viewport.X1, Y1: viewport.Y1 + width}
		}
	}
	return viewport, vertical, horizontal
}

func (item *itemData) editDrawGeometry() (point, point) {
	offset, size := item.textDrawOrigin, item.textDrawSize
	if size.X <= 0 || size.Y <= 0 {
		offset = point{X: item.DrawRect.X0, Y: item.DrawRect.Y0}
		size = point{X: item.DrawRect.X1 - item.DrawRect.X0, Y: item.DrawRect.Y1 - item.DrawRect.Y0}
	}
	return offset, size
}

func (item *itemData) editGeometry() (rect, point) {
	offset, size := item.editDrawGeometry()
	viewport := item.editViewport(offset, size)
	origin := point{X: viewport.X0 - item.editor().scroll.X, Y: viewport.Y0 - item.editor().scroll.Y}
	if !item.Multiline {
		origin.Y += max(0, (viewport.Y1-viewport.Y0-item.editLayout().lineHeight)/2)
	}
	return viewport, origin
}

func (item *itemData) editCursorAt(mpos point) int {
	layout := item.editLayout()
	_, origin := item.editGeometry()
	line := max(0, min(int(math.Floor(float64((mpos.Y-origin.Y)/layout.lineHeight))), len(layout.lines)-1))
	return layout.lines[line].nearest(mpos.X-origin.X, layout.face)
}

func (item *itemData) editVertical(direction int, page, extend bool) {
	layout, state := item.editLayout(), item.editor()
	line, x := layout.caret(item.CursorPos)
	if !state.hasPreferredX {
		state.preferredX, state.hasPreferredX = x, true
	}
	step := 1
	if page {
		viewport, _ := item.editGeometry()
		step = max(1, int((viewport.Y1-viewport.Y0)/layout.lineHeight)-1)
	}
	line = max(0, min(line+direction*step, len(layout.lines)-1))
	item.editMove(layout.lines[line].nearest(state.preferredX, layout.face), extend)
}

func (item *itemData) clampEditScroll(viewport rect) {
	layout, state := item.editLayout(), item.editor()
	state.scroll.X = max(0, min(state.scroll.X, layout.width+2-(viewport.X1-viewport.X0)))
	state.scroll.Y = max(0, min(state.scroll.Y, float32(len(layout.lines))*layout.lineHeight-(viewport.Y1-viewport.Y0)))
	if !item.Multiline {
		state.scroll.Y = 0
	}
}

func (item *itemData) followEditCaret(viewport rect) {
	layout, state := item.editLayout(), item.editor()
	if state.followCaret {
		line, x := layout.caret(item.CursorPos)
		y := float32(line) * layout.lineHeight
		w, h := viewport.X1-viewport.X0, viewport.Y1-viewport.Y0
		if x < state.scroll.X {
			state.scroll.X = x
		}
		if x+2 > state.scroll.X+w {
			state.scroll.X = x + 2 - w
		}
		// Keep a selected match fully visible when it fits on one line.
		anchorLine, anchorX := layout.caret(item.SelectStart)
		if item.SelectStart != item.SelectEnd && anchorLine == line && x >= anchorX && x-anchorX+2 <= w && anchorX < state.scroll.X {
			state.scroll.X = anchorX
		}
		if y < state.scroll.Y {
			state.scroll.Y = y
		}
		if y+layout.lineHeight > state.scroll.Y+h {
			state.scroll.Y = y + layout.lineHeight - h
		}
		state.followCaret = false
	}
	item.clampEditScroll(viewport)
}

func (item *itemData) drawEditableText(dst *ebiten.Image, offset, size point, clip rect, caption, selection Color) {
	layout := item.editLayout()
	viewport := item.editViewport(offset, size)
	item.followEditCaret(viewport)
	item.drawEditScrollbars(dst, offset, size, clip)
	clip = intersectRect(clip, viewport)
	if clip.X1 <= clip.X0 || clip.Y1 <= clip.Y0 {
		return
	}
	target := dst.RecyclableSubImage(clip.getRectangle())
	defer target.Recycle()
	state := item.editor()
	origin := point{X: viewport.X0 - state.scroll.X, Y: viewport.Y0 - state.scroll.Y}
	if !item.Multiline {
		origin.Y += max(0, (viewport.Y1-viewport.Y0-layout.lineHeight)/2)
	}
	start, end := item.editSelection()
	highlights := item.textHighlights()
	for i, line := range layout.lines {
		y := origin.Y + float32(i)*layout.lineHeight
		if y+layout.lineHeight <= clip.Y0 || y >= clip.Y1 {
			continue
		}
		op := &text.DrawOptions{}
		op.Filter = ebiten.FilterNearest
		op.GeoM.Translate(float64(origin.X), float64(y))
		op.ColorScale.ScaleWithColor(caption)
		if len(highlights) == 0 && len(item.textSwatches()) == 0 {
			text.Draw(target, line.display, layout.face, op)
		} else {
			item.drawHighlightedEditLine(target, line, layout.face, point{X: origin.X, Y: y}, highlights, caption)
		}
		item.drawTextSwatches(target, line, layout, origin, y)
		a, b := max(start, line.start), min(end, line.start+line.length)
		if start != end && (a < b || start <= line.start+line.length && end > line.start+line.length) {
			x0 := line.advance(a-line.start, layout.face)
			x1 := line.advance(b-line.start, layout.face)
			if end > line.start+line.length {
				x1 += max(3, layout.lineHeight/3)
			}
			r := intersectRect(rect{X0: origin.X + min(x0, x1), Y0: y, X1: origin.X + max(x0, x1), Y1: y + layout.lineHeight}, clip)
			if r.X1 > r.X0 && r.Y1 > r.Y0 {
				drawFilledRect(target, r.X0, r.Y0, r.X1-r.X0, r.Y1-r.Y0, selection, false)
				selected := target.RecyclableSubImage(r.getRectangle())
				op.ColorScale.Reset()
				op.ColorScale.ScaleWithColor(item.surfaceTextColor(caption, selection, true))
				text.Draw(selected, line.display, layout.face, op)
				selected.Recycle()
			}
		}
	}
	if item.Focused && state.caretOn && start == end {
		line, x := layout.caret(item.CursorPos)
		y := origin.Y + float32(line)*layout.lineHeight
		strokeLine(target, origin.X+x, y, origin.X+x, y+layout.lineHeight-2, max(1, uiScale), caption, false)
	}
}

func (item *itemData) editZoomWheel(delta point, mods inputkeys.Modifiers) bool {
	state := item.editor()
	if item.OnTextZoom == nil || !mods.Shortcut() {
		state.zoomWheel = 0
		return false
	}
	if delta.Y == 0 {
		return false
	}
	if state.zoomWheel*delta.Y < 0 {
		state.zoomWheel = 0
	}
	state.zoomWheel += delta.Y
	steps := int(state.zoomWheel)
	if steps != 0 {
		state.zoomWheel -= float32(steps)
		item.OnTextZoom(steps)
	}
	return true
}

func scrollEditable(items []*itemData, mpos, delta point, mods inputkeys.Modifiers) bool {
	for _, item := range items {
		if item.isInvisible() || !item.DrawRect.containsPoint(mpos) {
			continue
		}
		if itemHandlesTextEditing(item) {
			if !keyboardInputCaptured && item.editZoomWheel(delta, mods) {
				return true
			}
			state := item.editor()
			viewport, _ := item.editGeometry()
			if item.Multiline && !ShiftPressed {
				state.scroll.Y -= delta.Y * item.editLayout().lineHeight * 3
			} else {
				state.scroll.X -= delta.Y * item.editLayout().lineHeight * 3
			}
			state.scroll.X -= delta.X * item.editLayout().lineHeight * 3
			state.followCaret = false
			item.clampEditScroll(viewport)
			item.markDirty()
			return true
		}
		if len(item.Tabs) > 0 {
			if item.ActiveTab >= 0 && item.ActiveTab < len(item.Tabs) && scrollEditable(item.Tabs[item.ActiveTab].Contents, mpos, delta, mods) {
				return true
			}
		} else if scrollEditable(item.Contents, mpos, delta, mods) {
			return true
		}
	}
	return false
}
