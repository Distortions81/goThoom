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
		w.RemoveWindow()
	}

	// Main portal window covering 100x100 at origin.
	mainWin := eui.NewWindow(&eui.WindowData{})
	mainWin.Size = eui.Point{X: 100, Y: 100}
	mainWin.Open = true
	mainWin.MainPortal = true
	mainWin.AddWindow(false)

	// With only the main portal, the point should not be considered over UI.
	if pointInUI(10, 10) {
		t.Fatalf("pointInUI should ignore MainPortal window")
	}

	// Add a regular window at the same position.
	frontWin := eui.NewWindow(&eui.WindowData{})
	frontWin.Size = eui.Point{X: 20, Y: 20}
	frontWin.Open = true
	frontWin.AddWindow(false)

	if !pointInUI(10, 10) {
		t.Fatalf("pointInUI should detect top window")
	}

	// Cleanup
	frontWin.RemoveWindow()
	mainWin.RemoveWindow()
}
