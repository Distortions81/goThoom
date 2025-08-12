//go:build test

package eui

import (
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
)

func TestCloseMarksWindowNotOpen(t *testing.T) {
	win1 := &windowData{Title: "win1", open: true, Size: point{X: 100, Y: 100}}
	win2 := &windowData{Title: "win2", open: true, Size: point{X: 100, Y: 100}}

	windows = []*windowData{win1, win2}
	activeWindow = win2

	win2.Close()

	if win2.open {
		t.Fatalf("expected win2 to be closed")
	}
	if len(windows) != 2 {
		t.Fatalf("expected windows slice to remain, got %d", len(windows))
	}
	if activeWindow != win1 {
		t.Fatalf("expected active window to be win1, got %v", activeWindow)
	}
}

func TestRefreshClosedWindowRerendersOnOpen(t *testing.T) {
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

	win.Close()
	textItem.Text = "after"
	win.Refresh()
	Draw(screen)
	if textItem.RenderCount != rc0 {
		t.Fatalf("expected render count unchanged while closed")
	}

	win.Open()
	Draw(screen)
	if textItem.RenderCount <= rc0 {
		t.Fatalf("expected render count to increase after reopening")
	}
}
