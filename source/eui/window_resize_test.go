package eui

import (
	"fmt"
	"testing"
)

func setupWindowResizeTest(t *testing.T, scale float32, noScale bool) {
	t.Helper()
	oldWidth, oldHeight, oldScale := screenWidth, screenHeight, uiScale
	oldWindows, oldSnapping := windows, windowSnapping
	t.Cleanup(func() {
		screenWidth, screenHeight, uiScale = oldWidth, oldHeight, oldScale
		windows, windowSnapping = oldWindows, oldSnapping
	})
	uiScale = scale
	windowScale := scale
	if noScale {
		windowScale = 1
	}
	screenWidth, screenHeight = int(600*windowScale), int(400*windowScale)
	windows = nil
}

func TestWindowResizeStopsAtScreenEdges(t *testing.T) {
	for _, scale := range []float32{1, 1.5, 2} {
		for _, noScale := range []bool{false, true} {
			for _, snapping := range []bool{false, true} {
				for _, tt := range []struct {
					name             string
					part             dragType
					delta, pos, size point
				}{
					{"left", PART_LEFT, point{-1000, 0}, point{0, 80}, point{300, 160}},
					{"right", PART_RIGHT, point{1000, 0}, point{100, 80}, point{500, 160}},
					{"top", PART_TOP, point{0, -1000}, point{100, 0}, point{200, 240}},
					{"bottom", PART_BOTTOM, point{0, 1000}, point{100, 80}, point{200, 320}},
					{"top left", PART_TOP_LEFT, point{-1000, -1000}, point{0, 0}, point{300, 240}},
					{"top right", PART_TOP_RIGHT, point{1000, -1000}, point{100, 0}, point{500, 240}},
					{"bottom left", PART_BOTTOM_LEFT, point{-1000, 1000}, point{0, 80}, point{300, 320}},
					{"bottom right", PART_BOTTOM_RIGHT, point{1000, 1000}, point{100, 80}, point{500, 320}},
				} {
					t.Run(fmt.Sprintf("%s/scale=%g/noScale=%t/snapping=%t", tt.name, scale, noScale, snapping), func(t *testing.T) {
						setupWindowResizeTest(t, scale, noScale)
						windowSnapping = snapping
						win := NewWindow()
						win.NoScale = noScale
						win.Position, win.Size = point{100, 80}, point{200, 160}
						windows = []*windowData{win}
						for i := 0; i < 3; i++ {
							dragWindowResize(win, tt.part, tt.delta)
							snapResize(win, tt.part)
							if win.Position != tt.pos || win.Size != tt.size {
								t.Fatalf("drag %d: position=%v size=%v, want %v %v", i, win.Position, win.Size, tt.pos, tt.size)
							}
						}
						// Reversing direction should shrink immediately, without accumulated overshoot.
						before := win.Size
						dragWindowResize(win, tt.part, point{X: -tt.delta.X / 50, Y: -tt.delta.Y / 50})
						if win.Size == before {
							t.Fatal("window did not shrink after reversing the drag")
						}
					})
				}
			}
		}
	}
}

func TestWindowResizeMinimumKeepsOppositeEdgesFixed(t *testing.T) {
	setupWindowResizeTest(t, 1, false)
	win := NewWindow()
	win.Position, win.Size = point{100, 80}, point{200, 160}
	windows = []*windowData{win}
	dragWindowResize(win, PART_TOP_LEFT, point{1000, 20})
	if win.Position != (point{236, 100}) || win.Size != (point{64, 140}) {
		t.Fatalf("minimum width moved the fixed edge or blocked vertical resizing: %v %v", win.Position, win.Size)
	}
}

func TestWindowResizeReleasesZone(t *testing.T) {
	setupWindowResizeTest(t, 1, false)
	win := NewWindow()
	win.Size = point{200, 160}
	win.SetZone(HZoneCenter, VZoneCenter)
	windows = []*windowData{win}
	pos := win.Position
	dragWindowResize(win, PART_RIGHT, point{30, 0})
	if win.zone != nil || win.Position != pos || win.Size != (point{230, 160}) {
		t.Fatalf("zone moved the opposite edge while resizing: %v %v", win.Position, win.Size)
	}
}

func TestWindowResizeSnapKeepsOppositeEdgeFixed(t *testing.T) {
	for _, scale := range []float32{1, 2} {
		for _, part := range []dragType{PART_LEFT, PART_TOP} {
			t.Run(fmt.Sprintf("scale=%g/edge=%d", scale, part), func(t *testing.T) {
				setupWindowResizeTest(t, scale, false)
				windowSnapping = true
				win := NewWindow()
				win.Position, win.Size = point{5, 5}, point{200, 160}
				windows = []*windowData{win}
				fixed := pointAdd(win.Position, win.Size)
				if !snapResize(win, part) {
					t.Fatal("edge did not snap to screen")
				}
				if pointAdd(win.Position, win.Size) != fixed {
					t.Fatal("snapping moved the opposite edge")
				}
			})
		}
	}
}

func TestWindowResizeSnapsToOtherWindowAtUIScale(t *testing.T) {
	setupWindowResizeTest(t, 2, false)
	windowSnapping = true
	win := NewWindow()
	win.Position, win.Size = point{100, 80}, point{200, 160}
	other := NewWindow()
	other.NoScale, other.Open = true, true
	other.Position, other.Size = point{610, 160}, point{100, 200}
	windows = []*windowData{win, other}
	if !snapResize(win, PART_RIGHT) {
		t.Fatal("resize did not snap to the neighboring window")
	}
	if win.Position != (point{100, 80}) || win.Size != (point{205, 160}) {
		t.Fatalf("scaled snap moved the fixed edge or used the wrong target: %v %v", win.Position, win.Size)
	}
}
