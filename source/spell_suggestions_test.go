//go:build integration
// +build integration

package main

import (
	"math"
	"reflect"
	"testing"

	"gothoom/eui"

	ebiten "github.com/hajimehoshi/ebiten/v2"
	text "github.com/hajimehoshi/ebiten/v2/text/v2"
)

func TestHoverSpellSuggestionsMatchesContext(t *testing.T) {
	win := eui.NewWindow()
	win.MarkOpen()
	txt, _ := eui.NewText()
	txt.Text = "helo"
	txt.Underlines = findMisspellings(txt.Text)
	if len(txt.Underlines) == 0 {
		t.Fatalf("expected misspelling to be underlined")
	}
	win.AddItem(txt)
	txt.Focused = false

	metrics := txt.Face.Metrics()
	lineHeight := float32(math.Ceil(metrics.HAscent + metrics.HDescent + 2))
	w, _ := text.Measure(txt.Text, txt.Face, 0)
	txt.DrawRect.X0 = 0
	txt.DrawRect.Y0 = 0
	txt.DrawRect.X1 = float32(w)
	txt.DrawRect.Y1 = lineHeight

	cx := float32(w) / 2
	cy := lineHeight / 2
	cursorPosition = func() (int, int) { return int(cx), int(cy) }
	defer func() { cursorPosition = ebiten.CursorPosition }()

	var got []string
	showContextMenu = func(opts []string, x, y float32, onSelect func(int)) *eui.ItemData {
		got = append([]string(nil), opts...)
		return nil
	}
	defer func() { showContextMenu = eui.ShowContextMenu }()

	showSpellSuggestions(txt)

	expected := suggestCorrections("helo", 5)
	if !reflect.DeepEqual(got, expected) {
		t.Fatalf("expected %v, got %v", expected, got)
	}
}
