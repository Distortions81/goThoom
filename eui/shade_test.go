package eui

import "testing"

func TestShadeToggle(t *testing.T) {
	uiScale = 1
	win := &windowData{
		Position:    point{X: 0, Y: 0},
		Size:        point{X: 100, Y: 60},
		TitleHeight: 10,
		Open:        true,
		Closable:    true,
	}
	sr := win.shadeRect()
	mpos := point{X: (sr.X0 + sr.X1) / 2, Y: (sr.Y0 + sr.Y1) / 2}
	local := point{X: mpos.X / uiScale, Y: mpos.Y / uiScale}
	part := win.getWindowPart(local, true)
	if part != PART_SHADE {
		t.Fatalf("expected PART_SHADE, got %v", part)
	}
	win.ToggleShade()
	if !win.Shaded {
		t.Fatalf("expected window to be shaded")
	}
	win.ToggleShade()
	if win.Shaded {
		t.Fatalf("expected window to be unshaded")
	}
}
