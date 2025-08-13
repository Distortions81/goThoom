package eui

import "testing"

func TestWindowTilingPreventsOverlap(t *testing.T) {
	screenWidth = 200
	screenHeight = 200
	uiScale = 1
	windows = nil
	SetWindowTiling(true)

	win1 := &windowData{Open: true, Size: point{X: 50, Y: 50}}
	win2 := &windowData{Open: true, Position: point{X: 25, Y: 25}, Size: point{X: 50, Y: 50}}

	win1.AddWindow(false)
	win2.AddWindow(false)

	r1 := win1.getWinRect()
	r2 := win2.getWinRect()
	inter := intersectRect(r1, r2)
	if inter.X1 > inter.X0 && inter.Y1 > inter.Y0 {
		t.Fatalf("windows overlap: r1=%v r2=%v", r1, r2)
	}
}

func TestWindowTilingDisabledAllowsOverlap(t *testing.T) {
	screenWidth = 200
	screenHeight = 200
	uiScale = 1
	windows = nil
	SetWindowTiling(false)

	win1 := &windowData{Open: true, Size: point{X: 50, Y: 50}}
	win2 := &windowData{Open: true, Position: point{X: 25, Y: 25}, Size: point{X: 50, Y: 50}}

	win1.AddWindow(false)
	win2.AddWindow(false)

	r1 := win1.getWinRect()
	r2 := win2.getWinRect()
	inter := intersectRect(r1, r2)
	if inter.X1 <= inter.X0 || inter.Y1 <= inter.Y0 {
		t.Fatalf("expected overlap with tiling disabled")
	}
}

func TestWindowTilingResolvesOverlapAfterDrag(t *testing.T) {
	screenWidth = 200
	screenHeight = 200
	uiScale = 1
	windows = nil
	SetWindowTiling(true)
	SetWindowSnapping(true)

	win1 := &windowData{Open: true, Size: point{X: 50, Y: 50}}
	win2 := &windowData{Open: true, Position: point{X: 100, Y: 100}, Size: point{X: 50, Y: 50}, Movable: true}

	win1.AddWindow(false)
	win2.AddWindow(false)

	dragWindowMove(win2, point{X: -60, Y: -60})

	r1 := win1.getWinRect()
	r2 := win2.getWinRect()
	inter := intersectRect(r1, r2)
	if inter.X1 <= inter.X0 || inter.Y1 <= inter.Y0 {
		t.Fatalf("expected overlap during drag")
	}

	preventOverlap(win2)
	r2 = win2.getWinRect()
	inter = intersectRect(r1, r2)
	if inter.X1 > inter.X0 && inter.Y1 > inter.Y0 {
		t.Fatalf("windows still overlap after preventOverlap")
	}
}
