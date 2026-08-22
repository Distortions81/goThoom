package eui

import "testing"

func TestDragClearsZone(t *testing.T) {
	screenWidth = 100
	screenHeight = 100
	uiScale = 1

	win := &windowData{Movable: true}
	win.SetZone(HZoneLeft, VZoneTop)
	oldPos := win.Position
	delta := point{X: 5, Y: 5}

	dragWindowMove(win, delta)

	if win.zone != nil {
		t.Fatalf("zone not cleared")
	}
	expect := pointAdd(oldPos, delta)
	if win.Position != expect {
		t.Fatalf("expected position %+v, got %+v", expect, win.Position)
	}
}

func TestDragUnsnapThreshold(t *testing.T) {
	screenWidth = 100
	screenHeight = 100
	uiScale = 1

	win := &windowData{Movable: true}
	// Snap to the top-left corner
	if !snapToCorner(win) {
		t.Fatalf("expected window to snap")
	}

	// Drag slightly within the unsnap threshold
	dragWindowMove(win, point{X: UnsnapThreshold - 1, Y: 0})
	if win.zone != nil {
		t.Fatalf("zone not cleared")
	}
	if !win.snapAnchorActive {
		t.Fatalf("snap anchor deactivated too soon")
	}
	if !win.snapAnchorActive {
		snapToCorner(win)
	}
	if win.zone != nil {
		t.Fatalf("window resnapped prematurely")
	}

	// Move further to exceed the unsnap threshold
	dragWindowMove(win, point{X: 2, Y: 0})
	if win.snapAnchorActive {
		t.Fatalf("snap anchor should deactivate after threshold")
	}
	if !win.snapAnchorActive {
		snapToCorner(win)
	}
	if win.zone != nil {
		t.Fatalf("window snapped after moving away")
	}
}

func TestSnapToWindowWithSnapAnchor(t *testing.T) {
	screenWidth = 100
	screenHeight = 100
	uiScale = 1
	windowSnapping = true

	base := &windowData{Position: point{20, 0}, Size: point{10, 10}, Open: true}
	win := &windowData{Size: point{10, 10}, Open: true}
	windows = []*windowData{base, win}
	defer func() { windows = nil }()

	if !snapToCorner(win) {
		t.Fatalf("expected window to snap to corner")
	}

	dragWindowMove(win, point{X: 9, Y: 0})
	if !win.snapAnchorActive {
		t.Fatalf("snap anchor deactivated too soon")
	}

	snapped := false
	if !win.snapAnchorActive {
		snapped = snapToCorner(win)
	}
	if !snapped && snapToWindow(win) {
		win.clampToScreen()
	}

	expected := point{X: base.Position.X - win.Size.X, Y: 0}
	if win.Position != expected {
		t.Fatalf("expected snap to window at %+v, got %+v", expected, win.Position)
	}
}
