package eui

import (
	"math"
	"strings"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"gothoom/internal/inputkeys"
)

func TestTextEditorWordWrapPreservesSourceAndGraphemes(t *testing.T) {
	value := "\talpha  beta café e\u0301 👩‍💻 " + strings.Repeat("c#4/g8", 60) + "\n\nlast\tline  "
	item := editingFixture(t, value, true)
	item.SetWordWrap(true)
	for _, scale := range []float32{1, 1.5, 2} {
		uiScale = scale
		item.textDrawSize = point{X: 240 * scale, Y: 90 * scale}
		layout := item.editLayout()
		if len(layout.lines) <= 4 {
			t.Fatal("long tune was not wrapped")
		}
		stops := map[int]bool{}
		for _, stop := range graphemeStops(value) {
			stops[stop] = true
		}
		runes := []rune(value)
		next := 0
		var rebuilt strings.Builder
		for _, line := range layout.lines {
			if line.start != next || !stops[line.start] || !stops[line.start+line.length] {
				t.Fatalf("noncontiguous or split grapheme: %+v", line)
			}
			rebuilt.WriteString(string(runes[line.start : line.start+line.length]))
			next = line.start + line.length
			if !line.softBreak && next < len(runes) {
				if runes[next] != '\n' {
					t.Fatal("lost physical newline")
				}
				rebuilt.WriteRune('\n')
				next++
			}
			if line.width > layout.wrapWidth+0.01 && len(line.stops) > 2 {
				t.Fatalf("row exceeds width: %v > %v", line.width, layout.wrapWidth)
			}
			for i, b := range line.bytes[:len(line.bytes)-1] {
				if got := line.sourceRuneAtDisplayByte(b); got != line.start+i {
					t.Fatalf("highlight offset %d want %d", got, line.start+i)
				}
			}
		}
		if rebuilt.String() != value || item.Text != value || item.CanUndo() {
			t.Fatal("wrapping changed source or undo history")
		}
	}
}

func TestTextEditorWordWrapBreaksAtWordsAndRetainsWhitespace(t *testing.T) {
	item := editingFixture(t, "alpha beta gamma", true)
	layout := item.editLayout()
	width := float32(text.AdvanceAt("alpha be", len("alpha be"), layout.face))
	rows := wrapEditLine(layout.lines[0], item.Text, layout.face, width)
	if rows[0].display != "alpha " || rows[1].display != "beta " || rows[2].display != "gamma" {
		t.Fatalf("word boundaries: %+v", rows)
	}
}

func TestTextEditorWordWrapNavigationPointerAndSelection(t *testing.T) {
	item := editingFixture(t, strings.Repeat("alpha beta gamma ", 12), true)
	item.SetWordWrap(true)
	layout := item.editLayout()
	viewport, origin := item.editGeometry()
	// Every visible row boundary has two caret positions: previous row end and
	// next row start. Pointer placement must retain the requested visual row.
	for i, line := range layout.lines {
		if i > 2 {
			break
		}
		for _, end := range []bool{false, true} {
			pos, x := line.start, float32(0)
			if end {
				pos, x = line.start+line.length, line.width
			}
			item.clickEditableText(point{X: origin.X + x, Y: origin.Y + float32(i)*layout.lineHeight + 2}, false)
			row, cx := item.editCaret()
			if item.CursorPos != pos || row != i || math.Abs(float64(cx-x)) > 0.1 {
				t.Fatalf("click row %d: got %d,%d,%v", i, item.CursorPos, row, cx)
			}
			item.editor().lastClick = item.editor().lastClick.Add(-1000000000)
		}
	}
	item.editMove(layout.lines[1].start+2, false)
	item.editKey(ebiten.KeyEnd, inputkeys.Modifiers{}, false)
	if row, _ := item.editCaret(); row != 1 || item.CursorPos != layout.lines[1].start+layout.lines[1].length {
		t.Fatal("End jumped to another row")
	}
	item.editKey(ebiten.KeyHome, inputkeys.Modifiers{}, false)
	if item.CursorPos != layout.lines[1].start {
		t.Fatal("Home did not use visual row")
	}
	item.editKey(ebiten.KeyArrowDown, inputkeys.Modifiers{}, true)
	if item.CursorPos != layout.lines[2].start || item.SelectedText() != string([]rune(item.Text)[layout.lines[1].start:layout.lines[2].start]) {
		t.Fatal("vertical selection does not follow visual rows")
	}
	before := item.Text
	item.editInsert("replacement", "")
	item.Undo()
	if item.Text != before {
		t.Fatal("editing across wrap changed source")
	}
	item.editKey(ebiten.KeyEnd, inputkeys.Modifiers{Control: true}, false)
	item.followEditCaret(viewport)
	if item.CursorPos != len([]rune(item.Text)) || item.editor().scroll.Y <= 0 || item.editor().scroll.X != 0 {
		t.Fatal("document end not revealed vertically")
	}
}

func TestTextEditorWordWrapReflowsAndSearchesWithoutHorizontalScroll(t *testing.T) {
	item := editingFixture(t, strings.Repeat("a long tune phrase ", 40)+"needle", true)
	item.SetWordWrap(true)
	first := item.editLayout()
	if item.editLayout() != first {
		t.Fatal("layout not cached")
	}
	item.textDrawSize = point{X: 140, Y: 90}
	narrow := item.editLayout()
	if narrow == first || len(narrow.lines) <= len(first.lines) {
		t.Fatal("resize did not rewrap")
	}
	item.FontSize += 5
	large := item.editLayout()
	if large == narrow || len(large.lines) <= len(narrow.lines) {
		t.Fatal("font size did not rewrap")
	}
	if !item.FindText("phrase", 3, false) || item.SelectedText() != "phrase" {
		t.Fatal("search lost source offsets")
	}
	item.editSelect(0, len([]rune(item.Text)))
	if item.SelectedText() != item.Text {
		t.Fatal("copy contains soft newlines")
	}
	offset, size := item.editDrawGeometry()
	viewport, vertical, horizontal := item.editScrollGeometry(offset, size)
	item.editor().scroll = point{X: 999, Y: 99999}
	item.clampEditScroll(viewport)
	if horizontal != (rect{}) || vertical == (rect{}) || item.editor().scroll.X != 0 {
		t.Fatal("wrapped editor has horizontal scrolling")
	}
	item.editMove(0, false)
	item.FindText("needle", 0, false)
	item.followEditCaret(viewport)
	if item.editor().scroll.Y <= 0 || item.editor().scroll.X != 0 {
		t.Fatal("wrapped match not revealed")
	}
	before := item.editSnapshot()
	item.SetWordWrap(false)
	if item.editSnapshot() != before || item.CanUndo() {
		t.Fatal("toggle changed draft or history")
	}
	_, _, horizontal = item.editScrollGeometry(offset, size)
	if len(item.editLayout().lines) != 1 || horizontal == (rect{}) {
		t.Fatal("toggle did not restore horizontal scrolling")
	}
	item.Multiline = false
	item.SetWordWrap(true)
	if len(item.editLayout().lines) != 1 {
		t.Fatal("single-line input wrapped")
	}
}

func TestTextEditorWrappedColorSwatchKeepsSplitTextVisible(t *testing.T) {
	item := editingFixture(t, "abc "+strings.Repeat("1234567890", 40), true)
	item.SetWordWrap(true)
	span := TextColorSpan{Start: 4, End: 150, Color: ColorRed}
	if item.textSwatchVisible(span) || item.textSwatchCovers([]TextColorSpan{span}, 5) {
		t.Fatal("split swatch hid source text")
	}
}
