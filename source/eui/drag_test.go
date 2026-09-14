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
	oldWidth, oldHeight, oldScale := screenWidth, screenHeight, uiScale
	oldSnapping := windowSnapping
	t.Cleanup(func() {
		screenWidth, screenHeight, uiScale = oldWidth, oldHeight, oldScale
		windowSnapping = oldSnapping
	})

	screenWidth = 100
	screenHeight = 100
	uiScale = 1
	windowSnapping = true

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
	if want := (point{}); win.Position != want {
		t.Fatalf("window slipped off its magnetic anchor: position = %+v, want %+v", win.Position, want)
	}
	if !win.snapAnchorX || !win.snapAnchorY {
		t.Fatalf("magnetic axes released too soon: X=%t Y=%t", win.snapAnchorX, win.snapAnchorY)
	}

	// Move further horizontally. Only the X axis should release, and it should
	// jump directly to the cursor-relative position instead of slipping there.
	dragWindowMove(win, point{X: 2, Y: 0})
	if win.snapAnchorX || !win.snapAnchorY {
		t.Fatalf("magnetic axes released incorrectly: X=%t Y=%t", win.snapAnchorX, win.snapAnchorY)
	}
	if want := (point{X: UnsnapThreshold + 1}); win.Position != want {
		t.Fatalf("released position = %+v, want cursor-relative %+v", win.Position, want)
	}
	if !win.snapAnchorActive {
		t.Fatal("Y magnetic anchor should remain active")
	}
}

func TestSnapToWindowWithSnapAnchor(t *testing.T) {
	oldWidth, oldHeight, oldScale := screenWidth, screenHeight, uiScale
	oldWindows, oldSnapping := windows, windowSnapping
	t.Cleanup(func() {
		screenWidth, screenHeight, uiScale = oldWidth, oldHeight, oldScale
		windows, windowSnapping = oldWindows, oldSnapping
	})

	screenWidth = 200
	screenHeight = 200
	uiScale = 1
	windowSnapping = true

	base := &windowData{Position: point{100, 20}, Size: point{10, 100}, Open: true}
	win := &windowData{Position: point{81, 60}, Size: point{10, 20}, Open: true, Movable: true}
	windows = []*windowData{base, win}

	if !snapMovedWindow(win) {
		t.Fatalf("expected window to snap to nearby window")
	}
	expected := point{X: 90, Y: 60}
	if win.Position != expected {
		t.Fatalf("expected initial snap at %+v, got %+v", expected, win.Position)
	}

	dragWindowMove(win, point{X: -5, Y: 5})
	if !win.snapAnchorActive {
		t.Fatalf("snap anchor deactivated too soon")
	}
	if snapMovedWindow(win) {
		t.Fatal("window resnapped before leaving its anchor threshold")
	}
	expected = point{X: 90, Y: 65}
	if win.Position != expected {
		t.Fatalf("window slipped off its magnetic anchor: position = %+v, want %+v", win.Position, expected)
	}
	if want := (point{X: 76, Y: 65}); win.snapDragPosition != want {
		t.Fatalf("cursor-relative position = %+v, want %+v", win.snapDragPosition, want)
	}

	dragWindowMove(win, point{X: -18, Y: 0})
	if win.snapAnchorActive {
		t.Fatal("snap anchor remained active beyond the unsnap threshold")
	}
	if want := (point{X: 58, Y: 65}); win.Position != want {
		t.Fatalf("released position = %+v, want cursor-relative %+v", win.Position, want)
	}
	if snapMovedWindow(win) {
		t.Fatal("window resnapped after moving beyond the snap threshold")
	}
}

func TestMoveSnappingUsesWindowScale(t *testing.T) {
	oldWidth, oldHeight, oldScale := screenWidth, screenHeight, uiScale
	oldWindows, oldSnapping := windows, windowSnapping
	t.Cleanup(func() {
		screenWidth, screenHeight, uiScale = oldWidth, oldHeight, oldScale
		windows, windowSnapping = oldWindows, oldSnapping
	})

	screenWidth, screenHeight, uiScale = 800, 600, 2
	windowSnapping = true
	t.Run("scaled screen corner", func(t *testing.T) {
		win := &windowData{Position: point{X: 291, Y: 191}, Size: point{X: 100, Y: 100}}
		if !snapToCorner(win) {
			t.Fatal("scaled window did not snap to the bottom-right corner")
		}
		if want := (point{X: 300, Y: 200}); win.Position != want {
			t.Fatalf("position = %+v, want %+v", win.Position, want)
		}
	})

	t.Run("mixed-scale window edges", func(t *testing.T) {
		base := &windowData{
			Position: point{X: 410, Y: 100},
			Size:     point{X: 100, Y: 160},
			NoScale:  true,
			Open:     true,
		}
		win := &windowData{
			Position: point{X: 100, Y: 50},
			Size:     point{X: 100, Y: 80},
			Open:     true,
		}
		windows = []*windowData{base, win}
		if !snapMovedWindow(win) {
			t.Fatal("scaled window did not snap to unscaled window")
		}
		if want := (point{X: 105, Y: 50}); win.Position != want {
			t.Fatalf("position = %+v, want %+v", win.Position, want)
		}
		if win.GetPos().X+win.GetSize().X != base.GetPos().X {
			t.Fatal("mixed-scale window edges are not physically aligned")
		}
	})
}

func TestMoveSnapsToScreenEdgeAwayFromCorner(t *testing.T) {
	oldWidth, oldHeight, oldScale := screenWidth, screenHeight, uiScale
	oldWindows, oldSnapping := windows, windowSnapping
	t.Cleanup(func() {
		screenWidth, screenHeight, uiScale = oldWidth, oldHeight, oldScale
		windows, windowSnapping = oldWindows, oldSnapping
	})

	screenWidth, screenHeight, uiScale = 200, 200, 1
	windowSnapping = true
	win := &windowData{Position: point{X: 9, Y: 75}, Size: point{X: 50, Y: 50}, Open: true}
	windows = []*windowData{win}
	if !snapMovedWindow(win) {
		t.Fatal("window did not snap to the left screen edge")
	}
	if want := (point{X: 0, Y: 75}); win.Position != want {
		t.Fatalf("position = %+v, want %+v", win.Position, want)
	}
	if win.zone != nil {
		t.Fatal("single-edge snap unexpectedly assigned a corner zone")
	}
	if !win.snapAnchorActive {
		t.Fatal("screen-edge snap did not establish an unsnap anchor")
	}
}

func TestMoveSnappingChoosesClosestWindowEdge(t *testing.T) {
	oldWidth, oldHeight, oldScale := screenWidth, screenHeight, uiScale
	oldWindows, oldSnapping := windows, windowSnapping
	t.Cleanup(func() {
		screenWidth, screenHeight, uiScale = oldWidth, oldHeight, oldScale
		windows, windowSnapping = oldWindows, oldSnapping
	})

	screenWidth, screenHeight, uiScale = 500, 300, 1
	windowSnapping = true
	near := &windowData{Position: point{X: 154, Y: 50}, Size: point{X: 40, Y: 50}, Open: true}
	far := &windowData{Position: point{X: 158, Y: 50}, Size: point{X: 40, Y: 50}, Open: true}
	win := &windowData{Position: point{X: 100, Y: 50}, Size: point{X: 50, Y: 50}, Open: true}
	windows = []*windowData{near, far, win}

	if !snapMovedWindow(win) {
		t.Fatal("window did not snap")
	}
	if want := (point{X: 104, Y: 50}); win.Position != want {
		t.Fatalf("position = %+v, want closest edge at %+v", win.Position, want)
	}
}

func TestMoveSnappingAlignsAdjacentWindowEdges(t *testing.T) {
	oldWidth, oldHeight, oldScale := screenWidth, screenHeight, uiScale
	oldWindows, oldSnapping := windows, windowSnapping
	t.Cleanup(func() {
		screenWidth, screenHeight, uiScale = oldWidth, oldHeight, oldScale
		windows, windowSnapping = oldWindows, oldSnapping
	})

	screenWidth, screenHeight, uiScale = 500, 300, 1
	windowSnapping = true
	base := &windowData{Position: point{X: 220, Y: 100}, Size: point{X: 100, Y: 100}, Open: true}
	win := &windowData{Position: point{X: 153, Y: 115}, Size: point{X: 50, Y: 50}, Open: true}
	windows = []*windowData{base, win}

	if !snapMovedWindow(win) {
		t.Fatal("window did not snap at normal drag distance")
	}
	if want := (point{X: 170, Y: 100}); win.Position != want {
		t.Fatalf("position = %+v, want adjacent aligned edges at %+v", win.Position, want)
	}
}
