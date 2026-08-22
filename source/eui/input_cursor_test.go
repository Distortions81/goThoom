//go:build integration
// +build integration

package eui

import (
	"math"
	"testing"

	text "github.com/hajimehoshi/ebiten/v2/text/v2"
)

func TestCursorIndexAtInput(t *testing.T) {
	uiScale = 1
	item, _ := NewInput()
	item.DrawRect = rect{X0: 0, Y0: 0, X1: 200, Y1: 20}
	item.FontSize = 12
	item.Text = "hello"
	face := itemFace(item, (item.FontSize*uiScale)+2)
	w, _ := text.Measure("hel", face, 0)
	mpos := point{X: float32(w), Y: 0}
	item.clickItem(mpos, true)
	if item.CursorPos != 3 {
		t.Fatalf("cursor pos = %d want 3", item.CursorPos)
	}
}

func TestCursorIndexAtText(t *testing.T) {
	uiScale = 1
	item, _ := NewText()
	item.Filled = true
	item.DrawRect = rect{X0: 0, Y0: 0, X1: 200, Y1: 40}
	item.FontSize = 12
	item.Text = "hello\nworld"
	face := itemFace(item, (item.FontSize*uiScale)+2)
	metrics := face.Metrics()
	lineH := float32(math.Ceil(metrics.HAscent + metrics.HDescent + 2))
	w, _ := text.Measure("wo", face, 0)
	mpos := point{X: float32(w), Y: lineH + 1}
	item.clickItem(mpos, true)
	if item.CursorPos != 8 {
		t.Fatalf("cursor pos = %d want 8", item.CursorPos)
	}
}
