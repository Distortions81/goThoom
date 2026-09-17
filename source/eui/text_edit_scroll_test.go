package eui

import (
	"strings"
	"testing"
)

func TestTextEditorScrollbarContrastAcrossPalettes(t *testing.T) {
	isolateThemeTest(t)
	entries, err := embeddedThemes.ReadDir("themes/palettes")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		name := strings.TrimSuffix(entry.Name(), ".json")
		if name == "Example" {
			continue
		}
		t.Run(name, func(t *testing.T) {
			if err := LoadTheme(name); err != nil {
				t.Fatal(err)
			}
			item, _ := NewTextArea()
			for _, background := range []Color{{}, ColorBlack, ColorWhite} {
				item.Color = background
				track, thumb := item.editScrollbarColors()
				if track.A != 255 || thumb.A != 255 || textContrast(thumb, track) < 4.5 {
					t.Fatalf("scrollbar lacks opaque, contrasting colors: track %v, thumb %v", track, thumb)
				}
			}
		})
	}
}

func TestTextEditorScrollbarDraggingAndPaging(t *testing.T) {
	item := editingFixture(t, strings.Repeat(strings.Repeat("W", 90)+"\n", 40), true)
	item.editSelect(5, 9)
	for _, scale := range []float32{1, 1.5, 2} {
		uiScale = scale
		item.textDrawOrigin, item.textDrawSize = point{X: 40, Y: 60}, point{X: 240 * scale, Y: 90 * scale}
		item.DrawRect = rect{X0: 40, Y0: 60, X1: 40 + 240*scale, Y1: 60 + 90*scale}
		for _, part := range []dragType{PART_SCROLL_V, PART_SCROLL_H} {
			state := item.editor()
			state.scroll, state.scrollDrag = point{}, PART_NONE
			offset, size := item.editDrawGeometry()
			track, thumb, limit := item.editScrollbar(part, offset, size)
			if limit <= 0 {
				t.Fatalf("scale %v: missing scrollbar %v", scale, part)
			}
			p := point{X: (thumb.X0 + thumb.X1) / 2, Y: (thumb.Y0 + thumb.Y1) / 2}
			if item.editScrollbarAt(p) != part {
				t.Fatal("thumb hit test disagrees with rendering")
			}
			item.pressEditScrollbar(p, part)
			if part == PART_SCROLL_V {
				p.Y = track.Y1 + 100
			} else {
				p.X = track.X1 + 100
			}
			item.dragEditScrollbar(p)
			got := state.scroll.Y
			if part == PART_SCROLL_H {
				got = state.scroll.X
			}
			if got != limit || state.followCaret || item.SelectedText() != "WWWW" || item.CursorPos != 9 {
				t.Fatalf("drag: scroll %v, limit %v, selection %q", got, limit, item.SelectedText())
			}
			state.scrollDrag = PART_NONE
			// Clicking the track above/left of the thumb pages toward the start.
			item.pressEditScrollbar(point{X: track.X0 + 1, Y: track.Y0 + 1}, part)
			got = state.scroll.Y
			if part == PART_SCROLL_H {
				got = state.scroll.X
			}
			if got >= limit || state.scrollDrag != PART_NONE || item.CursorPos != 9 {
				t.Fatal("track click failed to page without moving the caret")
			}
		}
	}
}

func TestTextEditorScrollbarViewportAndDocumentShrink(t *testing.T) {
	item := editingFixture(t, strings.Repeat("wide line ", 100)+strings.Repeat("\nline", 30), true)
	offset, size := item.editDrawGeometry()
	viewport, vertical, horizontal := item.editScrollGeometry(offset, size)
	if viewport.X1 != vertical.X0 || viewport.Y1 != horizontal.Y0 || vertical.Y1 != viewport.Y1 || horizontal.X1 != viewport.X1 {
		t.Fatal("scrollbars overlap the text viewport or each other")
	}
	item.editor().scroll = point{X: 1000, Y: 1000}
	item.Text = "short"
	viewport, vertical, horizontal = item.editScrollGeometry(offset, size)
	item.clampEditScroll(viewport)
	if vertical != (rect{}) || horizontal != (rect{}) || item.editor().scroll != (point{}) {
		t.Fatal("short document retained scrollbars or out-of-range scroll")
	}
	item.Multiline = false
	item.Text = strings.Repeat("wide", 100)
	_, vertical, horizontal = item.editScrollGeometry(offset, size)
	if vertical != (rect{}) || horizontal != (rect{}) {
		t.Fatal("single-line field acquired scrollbars")
	}
}
