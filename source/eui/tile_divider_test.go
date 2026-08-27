package eui

import (
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
)

func TestTileDividerUsesWideGrabArea(t *testing.T) {
	d := TileDivider{
		Orientation: TileDividerVertical,
		Position:    100,
		Start:       20,
		End:         200,
		Thickness:   4,
		HitSize:     16,
	}
	if !tileDividerHit(d, point{X: 107, Y: 100}) {
		t.Fatal("divider did not include its enlarged grab area")
	}
	if tileDividerHit(d, point{X: 109, Y: 100}) {
		t.Fatal("divider grab area extended beyond its configured size")
	}
}

func TestDockedWindowHasNoStandaloneCornerDrag(t *testing.T) {
	originalScale := uiScale
	uiScale = 1
	t.Cleanup(func() { uiScale = originalScale })

	win := NewWindow()
	win.Position = point{X: 20, Y: 20}
	win.Size = point{X: 200, Y: 100}
	win.Movable = true
	win.Resizable = true
	win.Docked = true
	if got := win.getWindowPart(point{X: 20, Y: 20}, true); got != PART_NONE {
		t.Fatalf("docked corner hit = %v, want none", got)
	}
}

func TestDockedWindowDoesNotDrawStandaloneOutline(t *testing.T) {
	win := NewWindow()
	win.Outlined = true
	win.Border = 2
	if !win.drawsStandaloneOutline() {
		t.Fatal("ordinary outlined window did not request its outline")
	}
	win.Docked = true
	if win.drawsStandaloneOutline() {
		t.Fatal("docked pane requested an outer window outline")
	}
}

func TestFixedNonMovableWindowCornerDoesNotBecomeMoveHandle(t *testing.T) {
	originalScale := uiScale
	uiScale = 1
	t.Cleanup(func() { uiScale = originalScale })

	win := NewWindow()
	win.Position = point{X: 20, Y: 20}
	win.Size = point{X: 200, Y: 100}
	win.Movable = false
	win.Resizable = false
	if got := win.getWindowPart(point{X: 20, Y: 20}, true); got != PART_NONE {
		t.Fatalf("fixed corner hit = %v, want none", got)
	}
}

func TestStandaloneWindowCoversWorkspaceDivider(t *testing.T) {
	originalWindows := windows
	originalScale := uiScale
	uiScale = 1
	t.Cleanup(func() {
		windows = originalWindows
		uiScale = originalScale
	})

	utility := NewWindow()
	utility.Open = true
	utility.Position = point{X: 80, Y: 40}
	utility.Size = point{X: 80, Y: 120}
	windows = []*windowData{utility}

	if !tileDividerCoveredByStandaloneWindow(point{X: 100, Y: 100}) {
		t.Fatal("standalone window did not cover workspace divider input")
	}
	utility.Docked = true
	if tileDividerCoveredByStandaloneWindow(point{X: 100, Y: 100}) {
		t.Fatal("docked pane incorrectly covered workspace divider input")
	}
}

func TestStandaloneWindowsReceiveInputAboveDockedPanes(t *testing.T) {
	originalWindows := windows
	originalOrder := inputWindowOrder
	t.Cleanup(func() {
		windows = originalWindows
		inputWindowOrder = originalOrder
	})

	utilityBack := NewWindow()
	utilityFront := NewWindow()
	dockedBack := NewWindow()
	dockedFront := NewWindow()
	dockedBack.Docked = true
	dockedFront.Docked = true
	windows = []*windowData{utilityBack, dockedBack, utilityFront, dockedFront}

	got := windowsFrontToBack()
	want := []*windowData{utilityFront, utilityBack, dockedFront, dockedBack}
	if len(got) != len(want) {
		t.Fatalf("input order has %d windows, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("input order item %d did not preserve layered z-order", i)
		}
	}
}

func TestPointerCursorClaimedTracksEUIResizeCursor(t *testing.T) {
	original := cursorShape
	t.Cleanup(func() { cursorShape = original })

	cursorShape = ebiten.CursorShapeEWResize
	if !PointerCursorClaimed() {
		t.Fatal("resize cursor was not claimed by EUI")
	}
	cursorShape = ebiten.CursorShapeDefault
	if PointerCursorClaimed() {
		t.Fatal("default cursor remained claimed by EUI")
	}
}
