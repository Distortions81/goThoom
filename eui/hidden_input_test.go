//go:build test

package eui

import (
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
)

func TestHiddenInputCached(t *testing.T) {
	DebugMode = true
	defer func() { DebugMode = false }()

	input := *defaultInput
	input.Hide = true
	input.Text = "secret"
	input.Theme = baseTheme

	win := *defaultTheme
	win.Theme = baseTheme
	win.Size = point{X: 100, Y: 100}
	win.Contents = []*itemData{&input}

	windows = nil
	win.Open()
	screen := ebiten.NewImage(200, 200)

	win.Dirty = true
	Draw(screen)
	rc := input.RenderCount

	Draw(screen)
	if input.RenderCount != rc {
		t.Fatalf("expected hidden input to remain cached")
	}
}
