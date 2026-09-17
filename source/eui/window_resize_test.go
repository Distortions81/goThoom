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

func TestWindowResizeGrabAreaOwnsPointer(t *testing.T) {
	for _, scale := range []float32{1, 1.5, 2} {
		for _, noScale := range []bool{false, true} {
			t.Run(fmt.Sprintf("scale=%g/noScale=%t", scale, noScale), func(t *testing.T) {
				setupWindowResizeTest(t, scale, noScale)
				front, back := NewWindow(), NewWindow()
				front.Open, front.Resizable, front.NoScroll, front.NoScale = true, true, true, noScale
				front.Position, front.Size = point{100, 80}, point{200, 160}
				back.Open, back.NoScale = true, noScale
				back.Position, back.Size = point{}, point{600, 400}
				ordered := []*windowData{front, back}
				// Only the front window may cover the workspace divider below.
				windows = []*windowData{front}
				for _, tt := range []struct {
					name string
					pos  point
					part dragType
				}{
					{"left", point{98, 160}, PART_LEFT},
					{"right", point{302, 160}, PART_RIGHT},
					{"top", point{200, 78}, PART_TOP},
					{"bottom", point{200, 242}, PART_BOTTOM},
					{"top left", point{88, 68}, PART_TOP_LEFT},
					{"top right", point{312, 68}, PART_TOP_RIGHT},
					{"bottom left", point{88, 252}, PART_BOTTOM_LEFT},
					{"bottom right", point{312, 252}, PART_BOTTOM_RIGHT},
				} {
					t.Run(tt.name, func(t *testing.T) {
						p := point{X: tt.pos.X * front.scale(), Y: tt.pos.Y * front.scale()}
						if front.getWinRect().containsPoint(p) {
							t.Fatal("fixture must exercise the grab area outside the window")
						}
						if got := front.getWindowPart(tt.pos, false); got != tt.part {
							t.Fatalf("hover part = %v, want %v", got, tt.part)
						}
						if windowAtPointer(ordered, p) != front {
							t.Fatal("resize cursor area sent the press to the window underneath")
						}
						if !tileDividerCoveredByStandaloneWindow(p) {
							t.Fatal("resize cursor area did not block the workspace divider")
						}
						if windowAtPointer([]*windowData{back, front}, p) != back {
							t.Fatal("covered resize handle intercepted the top window")
						}
					})
				}
				outside := point{X: 330 * front.scale(), Y: 270 * front.scale()}
				if windowAtPointer(ordered, outside) != back {
					t.Fatal("pointer outside the resize cursor area was intercepted")
				}
				corner := point{X: 312 * front.scale(), Y: 252 * front.scale()}
				front.Resizable = false
				if windowAtPointer(ordered, corner) != back {
					t.Fatal("non-resizable window claimed an outside resize area")
				}
				front.Resizable, front.Docked = true, true
				if windowAtPointer(ordered, corner) != back {
					t.Fatal("docked window claimed an outside resize area")
				}
				front.Docked, front.Open = false, false
				if windowAtPointer(ordered, corner) != back {
					t.Fatal("closed window claimed an outside resize area")
				}
			})
		}
	}
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

func TestWindowResizeDetachesWithoutFastMouseMovement(t *testing.T) {
	setupWindowResizeTest(t, 1, false)
	windowSnapping = true
	win := NewWindow()
	win.Position, win.Size = point{100, 80}, point{191, 160}
	other := NewWindow()
	other.Position, other.Size, other.Open = point{300, 80}, point{100, 160}, true
	windows = []*windowData{win, other}

	if !snapResize(win, PART_RIGHT) || win.Size.X != 200 {
		t.Fatalf("right edge did not snap: position=%v size=%v", win.Position, win.Size)
	}
	dragWindowResize(win, PART_RIGHT, point{-5, 0})
	if snapResize(win, PART_RIGHT) {
		t.Fatal("slow resize immediately snapped back to the same edge")
	}
	if win.Size.X != 195 || !win.resizeSnapAnchorX {
		t.Fatalf("slow resize did not begin detaching: size=%v anchored=%t", win.Size, win.resizeSnapAnchorX)
	}

	dragWindowResize(win, PART_RIGHT, point{-8, 0})
	if win.resizeSnapAnchorX {
		t.Fatal("resize anchor remained active beyond the release threshold")
	}
	if snapResize(win, PART_RIGHT) {
		t.Fatal("resize resnapped after leaving the capture radius")
	}
}
