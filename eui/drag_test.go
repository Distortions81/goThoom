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
