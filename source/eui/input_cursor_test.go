package eui

import (
	"golang.org/x/image/font/gofont/goregular"
	"testing"

	text "github.com/hajimehoshi/ebiten/v2/text/v2"
)

func TestCursorIndexAtInput(t *testing.T) {
	if err := EnsureFontSource(goregular.TTF); err != nil {
		t.Fatal(err)
	}
	oldScale, oldFocus, oldSelection, oldActive := uiScale, focusedItem, selectedTextItem, activeItem
	t.Cleanup(func() {
		uiScale, focusedItem, selectedTextItem, activeItem = oldScale, oldFocus, oldSelection, oldActive
	})
	uiScale = 1
	item, _ := NewInput()
	item.DrawRect = rect{X0: 0, Y0: 0, X1: 200, Y1: 20}
	item.FontSize = 12
	item.Text = "hello"
	face := itemFace(item, (item.FontSize*uiScale)+2)
	w, _ := text.Measure("hel", face, 0)
	pad := (item.BorderPad + item.Padding + currentStyle.TextPadding) * uiScale
	mpos := point{X: pad + float32(w), Y: 10}
	item.clickItem(mpos, true)
	if item.CursorPos != 3 {
		t.Fatalf("cursor pos = %d want 3", item.CursorPos)
	}
}

func TestCursorIndexAtText(t *testing.T) {
	if err := EnsureFontSource(goregular.TTF); err != nil {
		t.Fatal(err)
	}
	oldScale, oldFocus, oldSelection, oldActive := uiScale, focusedItem, selectedTextItem, activeItem
	t.Cleanup(func() {
		uiScale, focusedItem, selectedTextItem, activeItem = oldScale, oldFocus, oldSelection, oldActive
	})
	uiScale = 1
	item, _ := NewText()
	item.Filled = true
	item.EditableText = true
	item.Multiline = true
	item.DrawRect = rect{X0: 0, Y0: 0, X1: 200, Y1: 40}
	item.FontSize = 12
	item.Text = "hello\nworld"
	face := itemFace(item, (item.FontSize*uiScale)+2)
	lineH := item.editLayout().lineHeight
	w, _ := text.Measure("wo", face, 0)
	pad := (item.BorderPad + item.Padding + currentStyle.TextPadding) * uiScale
	mpos := point{X: pad + float32(w), Y: pad + lineH + 1}
	item.clickItem(mpos, true)
	if item.CursorPos != 8 {
		t.Fatalf("cursor pos = %d want 8", item.CursorPos)
	}
}
