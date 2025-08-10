package main

import (
	"testing"

	"github.com/Distortions81/EUI/eui"
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
