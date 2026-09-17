package eui

import (
	"cmp"
	"slices"
	"sort"
	"unicode/utf8"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
)

// TextColorSpan colors [Start, End) in source rune offsets, before tab expansion.
// Uncovered text uses the control's normal caption color. A shaped glyph uses
// the color at its starting rune, so coloring never splits a glyph or ligature.
type TextColorSpan struct {
	Start, End int
	Color      Color
}

// TextHighlighter supplies foreground colors for an editable control's text.
// It runs synchronously when drawing changed text; it should be fast and must
// not modify the control. EUI copies and caches the returned spans. Out-of-range
// endpoints are clamped; empty ranges are ignored. Earlier-starting spans win
// overlaps, with input order breaking ties. Password controls are never passed
// to a highlighter.
type TextHighlighter func(string) []TextColorSpan

// SetTextHighlighter enables optional coloring for NewInput, NewTextArea, and
// editable ITEM_TEXT controls. nil restores normal text. Colors do not affect
// layout, editing, or undo history. Selected text keeps the selection's readable
// foreground; disabled controls keep their disabled foreground.
func (item *itemData) SetTextHighlighter(highlighter TextHighlighter) {
	item.textHighlighter = highlighter
	item.RefreshTextHighlighting()
}

// RefreshTextHighlighting discards cached colors, for example after changing a
// palette. Text changes invalidate the colors automatically.
func (item *itemData) RefreshTextHighlighting() {
	if item.textEdit != nil {
		item.textEdit.highlightsValid = false
		item.textEdit.highlights = nil
	}
	item.markDirty()
}

func (item *itemData) textHighlights() []TextColorSpan {
	if item.textHighlighter == nil || item.HideText || item.Disabled {
		return nil
	}
	s := item.editor()
	if !s.highlightsValid || s.highlightText != item.Text {
		spans := slices.Clone(item.textHighlighter(item.Text))
		n := utf8.RuneCountInString(item.Text)
		for i := range spans {
			spans[i].Start = max(0, min(n, spans[i].Start))
			spans[i].End = max(0, min(n, spans[i].End))
		}
		slices.SortStableFunc(spans, func(a, b TextColorSpan) int { return cmp.Compare(a.Start, b.Start) })
		end, count := 0, 0
		for _, span := range spans {
			span.Start = max(end, span.Start)
			if span.Start >= span.End {
				continue
			}
			spans[count] = span
			end, count = span.End, count+1
		}
		s.highlights = spans[:count]
		s.highlightText, s.highlightsValid = item.Text, true
	}
	return s.highlights
}

func textHighlightColor(spans []TextColorSpan, pos int, fallback Color) Color {
	i := sort.Search(len(spans), func(i int) bool { return spans[i].End > pos })
	if i < len(spans) && spans[i].Start <= pos {
		return spans[i].Color
	}
	return fallback
}

func (line editTextLine) sourceRuneAtDisplayByte(pos int) int {
	i := sort.Search(len(line.bytes), func(i int) bool { return line.bytes[i] > pos }) - 1
	return line.start + max(0, min(line.length, i))
}

type editGlyphDraw struct {
	image   *ebiten.Image
	x, y    float64
	color   Color
	colored bool
}

// Shape the whole line, just as the plain renderer does. Only its glyph colors
// change; splitting strings into separate draws would change kerning/shaping.
func (item *itemData) drawHighlightedEditLine(dst *ebiten.Image, line editTextLine, face text.Face, origin point, spans []TextColorSpan, caption Color) {
	s := item.editor()
	s.highlightGlyphs = text.AppendLazyGlyphs(s.highlightGlyphs[:0], line.display, face, nil)
	swatches := item.textSwatches()
	bounds := dst.Bounds()
	// Realize all visible images before drawing to preserve atlas batching.
	for _, glyph := range s.highlightGlyphs {
		if item.textSwatchCovers(swatches, line.sourceRuneAtDisplayByte(glyph.StartIndexInBytes)) {
			continue
		}
		r := glyph.ImageBounds
		if r.Empty() || float32(r.Max.X)+origin.X <= float32(bounds.Min.X) || float32(r.Min.X)+origin.X >= float32(bounds.Max.X) ||
			float32(r.Max.Y)+origin.Y <= float32(bounds.Min.Y) || float32(r.Min.Y)+origin.Y >= float32(bounds.Max.Y) {
			continue
		}
		img := glyph.Image()
		if img == nil {
			continue
		}
		color := textHighlightColor(spans, line.sourceRuneAtDisplayByte(glyph.StartIndexInBytes), caption)
		s.highlightDraws = append(s.highlightDraws, editGlyphDraw{img, float64(origin.X) + float64(r.Min.X), float64(origin.Y) + float64(r.Min.Y), color, glyph.Colored()})
	}
	for _, glyph := range s.highlightDraws {
		op := &ebiten.DrawImageOptions{Filter: ebiten.FilterNearest}
		op.GeoM.Translate(glyph.x, glyph.y)
		if glyph.colored {
			op.ColorScale.ScaleAlpha(float32(glyph.color.A) / 255)
		} else {
			op.ColorScale.ScaleWithColor(glyph.color)
		}
		dst.DrawImage(glyph.image, op)
	}
	// Keep scratch capacity without retaining glyph images from old documents.
	s.highlightGlyphs = slices.Delete(s.highlightGlyphs, 0, len(s.highlightGlyphs))
	s.highlightDraws = slices.Delete(s.highlightDraws, 0, len(s.highlightDraws))
}
