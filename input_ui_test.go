package main

import (
	"testing"

	"github.com/Distortions81/EUI/eui"
)

// screenToNormal converts screen pixel coordinates to normalized EUI coordinates.
func screenToNormal(x, y int) eui.Point {
	w, h := eui.ScreenSize()
	return eui.Point{X: float32(x) / float32(w), Y: float32(y) / float32(h)}
}

// normalToScreen converts normalized EUI coordinates back to screen pixels.
func normalToScreen(p eui.Point) (int, int) {
	w, h := eui.ScreenSize()
	return int(p.X * float32(w)), int(p.Y * float32(h))
}

// TestPointInUISkipsMainPortal ensures that the base game window does not block
// interaction with other UI windows.
func TestPointInUISkipsMainPortal(t *testing.T) {
	// Clean up any existing windows to avoid interference between tests.
	for _, w := range eui.Windows() {
		w.Close()
	}

	// Main portal window covering 100x100 at origin.
	mainWin := eui.NewWindow()
	mainWin.Size = screenToNormal(100, 100)
	mainWin.MainPortal = true
	mainWin.AddWindow(false)
	mainWin.Open()

	// With only the main portal, the point should not be considered over UI.
	pt := screenToNormal(10, 10)
	px, py := normalToScreen(pt)
	if pointInUI(px, py) {
		t.Fatalf("pointInUI should ignore MainPortal window")
	}

	// Add a regular window at the same position.
	frontWin := eui.NewWindow()
	frontWin.Size = screenToNormal(20, 20)
	frontWin.AddWindow(false)
	frontWin.Open()

	if !pointInUI(px, py) {
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
	win.Size = screenToNormal(50, 50)
	win.AddWindow(false)
	win.Open()

	pos := win.GetPos()
	size := win.GetSize()
	s := eui.UIScale()
	frame := (win.Margin + win.Border + win.BorderPad + win.Padding) * s
	title := win.GetTitleSize()
	w, h := eui.ScreenSize()
	offset := screenToNormal(1, 0).X
	xNorm := (pos.X+frame)/float32(w) + offset
	yNorm := (pos.Y + size.Y + frame*2 + title/2) / float32(h)
	x, y := normalToScreen(eui.Point{X: xNorm, Y: yNorm})
	if !pointInUI(x, y) {
		t.Fatalf("pointInUI should include title bar region")
	}

	win.Close()
}
