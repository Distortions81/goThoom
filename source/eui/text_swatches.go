package eui

import (
	"cmp"
	"slices"
	"sort"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
)

// SetTextColorSwatches displays source ranges as clickable color previews.
// Source text, layout, copying, and undo history remain unchanged. A caret or
// selection inside a range reveals the original text for keyboard editing.
// Providers must be fast and must not modify the control. Ranges use rune
// offsets, must fit on one line, and must not overlap. nil removes the previews.
func (item *itemData) SetTextColorSwatches(provider TextHighlighter, click func(TextColorSpan)) {
	item.textSwatchProvider, item.textSwatchClick = provider, click
	if item.textEdit != nil {
		item.textEdit.swatchesValid = false
	}
	item.markDirty()
}

func (item *itemData) textSwatches() []TextColorSpan {
	if item.textSwatchProvider == nil || item.Disabled || item.HideText {
		return nil
	}
	state := item.editor()
	if !state.swatchesValid || state.swatchText != item.Text {
		spans := slices.Clone(item.textSwatchProvider(item.Text))
		slices.SortStableFunc(spans, func(a, b TextColorSpan) int { return cmp.Compare(a.Start, b.Start) })
		runes := []rune(item.Text)
		end, count := 0, 0
		for _, span := range spans {
			if span.Start < end || span.Start < 0 || span.End <= span.Start || span.End > len(runes) || strings.ContainsAny(string(runes[span.Start:span.End]), "\r\n\t") {
				continue
			}
			spans[count] = span
			end, count = span.End, count+1
		}
		state.swatches, state.swatchText, state.swatchesValid = spans[:count], item.Text, true
	}
	return state.swatches
}

func (item *itemData) textSwatchVisible(span TextColorSpan) bool {
	if item.WordWrap && item.Multiline {
		layout := item.editLayout()
		first, _ := layout.caret(span.Start)
		last, _ := layout.caretAt(span.End, true)
		// A swatch must fit on one visual row; otherwise show its editable text.
		if first != last {
			return false
		}
	}
	start, end := min(item.SelectStart, item.SelectEnd), max(item.SelectStart, item.SelectEnd)
	if start != end && start < span.End && end > span.Start {
		return false
	}
	return !item.Focused || item.CursorPos < span.Start || item.CursorPos > span.End
}

func (item *itemData) textSwatchCovers(spans []TextColorSpan, pos int) bool {
	i := sort.Search(len(spans), func(i int) bool { return spans[i].End > pos })
	return i < len(spans) && spans[i].Start <= pos && item.textSwatchVisible(spans[i])
}

func textSwatchRect(span TextColorSpan, line editTextLine, layout *editTextLayout, origin point, y float32) rect {
	x0, x1 := line.advance(span.Start-line.start, layout.face), line.advance(span.End-line.start, layout.face)
	return rect{X0: origin.X + min(x0, x1), Y0: y + 1, X1: origin.X + max(x0, x1), Y1: y + layout.lineHeight - 1}
}

func (item *itemData) textSwatchAt(mpos point) (TextColorSpan, bool) {
	if !itemHandlesTextEditing(item) || item.textSwatchClick == nil {
		return TextColorSpan{}, false
	}
	viewport, origin := item.editGeometry()
	if !viewport.containsPoint(mpos) || !item.DrawRect.containsPoint(mpos) {
		return TextColorSpan{}, false
	}
	layout := item.editLayout()
	lineIndex := int((mpos.Y - origin.Y) / layout.lineHeight)
	if lineIndex < 0 || lineIndex >= len(layout.lines) {
		return TextColorSpan{}, false
	}
	line := layout.lines[lineIndex]
	for _, span := range item.textSwatches() {
		if span.Start < line.start || span.End > line.start+line.length || !item.textSwatchVisible(span) {
			continue
		}
		if textSwatchRect(span, line, layout, origin, origin.Y+float32(lineIndex)*layout.lineHeight).containsPoint(mpos) {
			return span, true
		}
	}
	return TextColorSpan{}, false
}

func (item *itemData) drawTextSwatches(dst *ebiten.Image, line editTextLine, layout *editTextLayout, origin point, y float32) {
	for _, span := range item.textSwatches() {
		if span.Start < line.start || span.End > line.start+line.length || !item.textSwatchVisible(span) {
			continue
		}
		r := textSwatchRect(span, line, layout, origin, y)
		cell := max(3, layout.lineHeight/3)
		// Composite against opaque checks so alpha remains visible and the
		// source glyphs cannot show through a transparent color.
		for row, top := 0, r.Y0; top < r.Y1; row, top = row+1, top+cell {
			for col, left := 0, r.X0; left < r.X1; col, left = col+1, left+cell {
				grey := uint32(200)
				if (row+col)%2 != 0 {
					grey = 110
				}
				a := uint32(span.Color.A)
				blend := func(c uint8) uint8 { return uint8((uint32(c)*a + grey*(255-a)) / 255) }
				color := NewColor(blend(span.Color.R), blend(span.Color.G), blend(span.Color.B), 255)
				drawFilledRect(dst, left, top, min(cell, r.X1-left), min(cell, r.Y1-top), color, false)
			}
		}
		strokeRect(dst, r.X0, r.Y0, r.X1-r.X0, r.Y1-r.Y0, max(1, uiScale), NewColor(140, 140, 140, 255), false)
	}
}
