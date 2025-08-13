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
