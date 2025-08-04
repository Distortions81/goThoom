package main

import (
	text "github.com/hajimehoshi/ebiten/v2/text/v2"
	"testing"
)

func TestWrapText(t *testing.T) {
	initFont()
	w1, _ := text.Measure("hello", nameFace, 0)
	w2, _ := text.Measure("hello world", nameFace, 0)
	maxWidth := (w1 + w2) / 2
	lines := wrapText("hello world", nameFace, maxWidth)
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}
	if lines[0] != "hello" || lines[1] != "world" {
		t.Fatalf("got lines %#v", lines)
	}
}
