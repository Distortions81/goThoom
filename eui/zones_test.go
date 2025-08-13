package eui

import "testing"

func TestPinToClosestZone(t *testing.T) {
	screenWidth = 100
	screenHeight = 100
	uiScale = 1

	tests := []struct {
		pos point
		h   HZone
		v   VZone
	}{
		{point{0, 0}, HZoneLeft, VZoneTop},
		{point{16, 16}, HZoneLeftCenter, VZoneTopMiddle},
		{point{33, 33}, HZoneCenterLeft, VZoneMiddleTop},
		{point{50, 50}, HZoneCenter, VZoneCenter},
		{point{66, 66}, HZoneCenterRight, VZoneMiddleBottom},
		{point{83, 83}, HZoneRightCenter, VZoneBottomMiddle},
		{point{100, 100}, HZoneRight, VZoneBottom},
	}

	for _, tt := range tests {
		win := &windowData{Position: tt.pos}
		win.PinToClosestZone()
		if win.zone == nil {
			t.Fatalf("zone not set")
		}
		if win.zone.h != tt.h || win.zone.v != tt.v {
			t.Fatalf("pos %+v pinned to (%v,%v); want (%v,%v)", tt.pos, win.zone.h, win.zone.v, tt.h, tt.v)
		}
	}
}

func TestSnapToCorner(t *testing.T) {
	screenWidth = 100
	screenHeight = 100
	uiScale = 1

	winSize := point{X: 10, Y: 10}
	offset := CornerSnapThreshold - 1

	tests := []struct {
		pos point
		h   HZone
		v   VZone
	}{
		{point{offset, offset}, HZoneLeft, VZoneTop},
		{point{float32(screenWidth) - winSize.X - offset, offset}, HZoneRight, VZoneTop},
		{point{offset, float32(screenHeight) - winSize.Y - offset}, HZoneLeft, VZoneBottom},
		{point{float32(screenWidth) - winSize.X - offset, float32(screenHeight) - winSize.Y - offset}, HZoneRight, VZoneBottom},
	}

	for _, tt := range tests {
		win := &windowData{Position: tt.pos, Size: winSize}
		snapToCorner(win)
		if win.zone == nil {
			t.Fatalf("zone not set for pos %+v", tt.pos)
		}
		if win.zone.h != tt.h || win.zone.v != tt.v {
			t.Fatalf("pos %+v snapped to (%v,%v); want (%v,%v)", tt.pos, win.zone.h, win.zone.v, tt.h, tt.v)
		}
	}
}

func TestSnapToWindow(t *testing.T) {
	screenWidth = 200
	screenHeight = 200
	uiScale = 1

	base := &windowData{Position: point{50, 50}, Size: point{30, 30}, Open: true}
	winSize := point{10, 10}
	offset := CornerSnapThreshold - 1

	tests := []struct {
		name   string
		pos    point
		expect point
	}{
		{"left", point{base.Position.X - winSize.X - offset, base.Position.Y}, point{base.Position.X - winSize.X, base.Position.Y}},
		{"right", point{base.Position.X + base.Size.X + offset, base.Position.Y}, point{base.Position.X + base.Size.X, base.Position.Y}},
		{"top", point{base.Position.X, base.Position.Y - winSize.Y - offset}, point{base.Position.X, base.Position.Y - winSize.Y}},
		{"bottom", point{base.Position.X, base.Position.Y + base.Size.Y + offset}, point{base.Position.X, base.Position.Y + base.Size.Y}},
	}

	for _, tt := range tests {
		win := &windowData{Position: tt.pos, Size: winSize, Open: true}
		windows = []*windowData{base, win}
		snapToWindow(win)
		if win.Position != tt.expect {
			t.Fatalf("%s: pos %+v snapped to %+v; want %+v", tt.name, tt.pos, win.Position, tt.expect)
		}
	}
	windows = nil
}

func TestSnapResizeToWindow(t *testing.T) {
	screenWidth = 200
	screenHeight = 200
	uiScale = 1

	base := &windowData{Position: point{100, 50}, Size: point{20, 20}, Open: true}
	win := &windowData{Position: point{50, 50}, Size: point{48, 20}, Open: true}
	windows = []*windowData{base, win}

	snapResize(win, PART_RIGHT)

	expectedWidth := base.Position.X - win.Position.X
	if win.Size.X != expectedWidth {
		t.Fatalf("width snapped to %v; want %v", win.Size.X, expectedWidth)
	}
	windows = nil
}

func TestSnapResizeToScreen(t *testing.T) {
	screenWidth = 200
	screenHeight = 200
	uiScale = 1

	win := &windowData{Position: point{50, 50}, Size: point{20, 141}, Open: true}

	snapResize(win, PART_BOTTOM)

	expectedHeight := float32(screenHeight) - win.Position.Y
	if win.Size.Y != expectedHeight {
		t.Fatalf("height snapped to %v; want %v", win.Size.Y, expectedHeight)
	}
}
