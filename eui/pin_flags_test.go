//go:build test

package eui

import "testing"

func TestAddWindowPinnedDisablesFlags(t *testing.T) {
	win := &windowData{
		Size:      point{X: 10, Y: 10},
		PinTo:     PIN_TOP_LEFT,
		Movable:   true,
		Resizable: true,
		open:      true,
	}
	win.AddWindow(false)
	defer func() { windows = nil }()
	if win.Movable || win.Resizable {
		t.Fatalf("expected pinned window to be immovable and non-resizable")
	}
}

func TestAddWindowUnpinnedHonorsFlags(t *testing.T) {
	win := &windowData{
		Size:      point{X: 10, Y: 10},
		PinTo:     PIN_NONE,
		Movable:   true,
		Resizable: true,
		open:      true,
	}
	win.AddWindow(false)
	defer func() { windows = nil }()
	if !win.Movable || !win.Resizable {
		t.Fatalf("expected unpinned window to honor Movable/Resizable")
	}
}
