//go:build test

package eui

import (
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
)

func TestWindowRefreshRerenders(t *testing.T) {
	DebugMode = true
	defer func() { DebugMode = false }()

	textItem := *defaultText
	textItem.Text = "before"
	textItem.Theme = baseTheme

	win := *defaultTheme
	win.Theme = baseTheme
	win.Size = point{X: 100, Y: 100}
	win.Contents = []*itemData{&textItem}

	windows = nil
	win.Open()
	screen := ebiten.NewImage(200, 200)

	win.Dirty = true
	Draw(screen)
	rc0 := textItem.RenderCount

	textItem.Text = "after"
	win.Refresh()
	Draw(screen)

	if textItem.RenderCount <= rc0 {
		t.Fatalf("expected render count to increase after Refresh")
	}
}

func TestWindowRefreshTitleUpdates(t *testing.T) {
	DebugMode = true
	defer func() { DebugMode = false }()

	win := *defaultTheme
	win.Theme = baseTheme
	win.Size = point{X: 100, Y: 100}
	windows = nil
	win.Open()
	win.SetTitle("short")
	screen := ebiten.NewImage(200, 200)

	win.Dirty = true
	Draw(screen)
	w0 := win.titleTextW

	win.SetTitle("a much longer title")
	Draw(screen)

	if win.titleTextW <= w0 {
		t.Fatalf("expected title width to increase after title update")
	}
}
