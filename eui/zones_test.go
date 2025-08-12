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
		{point{20, 20}, HZoneLeftCenter, VZoneTopMiddle},
		{point{40, 40}, HZoneCenterLeft, VZoneMiddleTop},
		{point{60, 60}, HZoneCenterRight, VZoneMiddleBottom},
		{point{80, 80}, HZoneRightCenter, VZoneBottomMiddle},
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
