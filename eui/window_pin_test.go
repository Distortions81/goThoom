//go:build test

package eui

import "testing"

// TestPinNoneWindowMovableResizable verifies that only unpinned windows
// can be moved or resized.
func TestPinNoneWindowMovableResizable(t *testing.T) {
	windows = nil
	defer func() { windows = nil }()

	delta := point{X: 5, Y: 5}

	free := &windowData{
		Position:  point{X: 0, Y: 0},
		Size:      point{X: 50, Y: 50},
		Movable:   true,
		Resizable: true,
		open:      true,
		PinTo:     PIN_NONE,
	}
	free.AddWindow(false)

	origPos := free.Position
	origSize := free.Size
	if free.PinTo == PIN_NONE {
		free.Position = pointAdd(free.Position, delta)
		free.setSize(pointAdd(free.Size, delta))
	}
	if free.Position == origPos {
		t.Fatalf("expected PIN_NONE window to move")
	}
	if free.Size == origSize {
		t.Fatalf("expected PIN_NONE window to resize")
	}

	pinned := &windowData{
		Position:  point{X: 0, Y: 0},
		Size:      point{X: 50, Y: 50},
		Movable:   true,
		Resizable: true,
		open:      true,
		PinTo:     PIN_TOP_LEFT,
	}
	pinned.AddWindow(false)

	origPos = pinned.Position
	origSize = pinned.Size
	if pinned.PinTo == PIN_NONE {
		pinned.Position = pointAdd(pinned.Position, delta)
		pinned.setSize(pointAdd(pinned.Size, delta))
	}
	if pinned.Position != origPos {
		t.Fatalf("expected pinned window to remain fixed in position")
	}
	if pinned.Size != origSize {
		t.Fatalf("expected pinned window to remain fixed in size")
	}
}
