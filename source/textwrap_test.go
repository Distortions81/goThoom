package main

import (
	"testing"

	text "github.com/hajimehoshi/ebiten/v2/text/v2"
)

func TestWrapTextPreservesSpaces(t *testing.T) {
	face := &text.GoTextFace{Size: 12}
	_, lines := wrapText("foo  bar", face, 1000)
	if len(lines) != 1 {
		t.Fatalf("lines = %d want 1", len(lines))
	}
	if lines[0] != "foo  bar" {
		t.Fatalf("line = %q want %q", lines[0], "foo  bar")
	}
}
