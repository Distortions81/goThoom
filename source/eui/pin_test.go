package eui

import "testing"

func TestDragbarClickDoesNotTriggerPinWithScale(t *testing.T) {
	uiScale = 2
	win := &windowData{
		Position:    point{X: 0, Y: 0},
		Size:        point{X: 100, Y: 60},
		TitleHeight: 10,
		Movable:     true,
		Open:        true,
		Closable:    true,
	}
	dr := win.dragbarRect()
	mposScreen := point{X: (dr.X0 + dr.X1) / 2, Y: (dr.Y0 + dr.Y1) / 2}
	local := point{X: mposScreen.X / uiScale, Y: mposScreen.Y / uiScale}
	part := win.getWindowPart(local, true)
	if part != PART_BAR {
		t.Fatalf("expected PART_BAR, got %v", part)
	}
}
