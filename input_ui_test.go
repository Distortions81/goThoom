package main

import (
	"testing"

	"go_client/eui"
)

// TestPointInUISkipsMainPortal ensures that the base game window does not block
// interaction with other UI windows.
func TestPointInUISkipsMainPortal(t *testing.T) {
	// Clean up any existing windows to avoid interference between tests.
	for _, w := range eui.Windows() {
		w.Close()
	}

	// Main portal window covering 100x100 at origin.
	mainWin := eui.NewWindow()
	mainWin.Size = eui.Point{X: 100, Y: 100}
	mainWin.MainPortal = true
	mainWin.AddWindow(false)
	mainWin.Open()

	// With only the main portal, the point should not be considered over UI.
	if pointInUI(10, 10) {
		t.Fatalf("pointInUI should ignore MainPortal window")
	}

	// Add a regular window at the same position.
	frontWin := eui.NewWindow()
	frontWin.Size = eui.Point{X: 20, Y: 20}
	frontWin.AddWindow(false)
	frontWin.Open()

	if !pointInUI(10, 10) {
		t.Fatalf("pointInUI should detect top window")
	}

	// Cleanup
	frontWin.Close()
	mainWin.Close()
}

// TestPointInUICoversTitleBar ensures that the window's title area counts as UI.
func TestPointInUICoversTitleBar(t *testing.T) {
	// Clean up any existing windows to avoid interference between tests.
	for _, w := range eui.Windows() {
		w.Close()
	}

	win := eui.NewWindow()
	win.Size = eui.Point{X: 50, Y: 50}
	win.AddWindow(false)
	win.Open()

	pos := win.GetPos()
	size := win.GetSize()
	s := eui.UIScale()
	frame := (win.Margin + win.Border + win.BorderPad + win.Padding) * s
	title := win.GetTitleSize()
	x := int(pos.X + frame + 1)
	y := int(pos.Y + size.Y + frame*2 + title/2)
	if !pointInUI(x, y) {
		t.Fatalf("pointInUI should include title bar region")
	}

	win.Close()
}
