//go:build test

package eui

import (
	"fmt"
	"testing"
)

// expectedPinPos returns the expected on-screen position for a window with the
// given pin, size, and offset (Position) for the provided screen dimensions.
func expectedPinPos(pin pinType, w, h int, size, offset point) point {
	sw := float32(w)
	sh := float32(h)
	wSize := size.X
	hSize := size.Y
	px := offset.X
	py := offset.Y

	switch pin {
	case PIN_TOP_LEFT:
		return point{X: px, Y: py}
	case PIN_TOP_CENTER:
		return point{X: sw/2 - wSize/2 + px, Y: py}
	case PIN_TOP_RIGHT:
		return point{X: sw - wSize - px, Y: py}
	case PIN_MID_LEFT:
		return point{X: px, Y: sh/2 - hSize/2 + py}
	case PIN_MID_CENTER:
		return point{X: sw/2 - wSize/2 + px, Y: sh/2 - hSize/2 + py}
	case PIN_MID_RIGHT:
		return point{X: sw - wSize - px, Y: sh/2 - hSize/2 + py}
	case PIN_BOTTOM_LEFT:
		return point{X: px, Y: sh - hSize - py}
	case PIN_BOTTOM_CENTER:
		return point{X: sw/2 - wSize/2 + px, Y: sh - hSize - py}
	case PIN_BOTTOM_RIGHT:
		return point{X: sw - wSize - px, Y: sh - hSize - py}
	default:
		return point{X: px, Y: py}
	}
}

// TestPinnedWindowLayout verifies that windows pinned to various anchors stay
// within screen bounds and move appropriately when the screen size changes.
func TestPinnedWindowLayout(t *testing.T) {
	pins := []pinType{
		PIN_TOP_LEFT, PIN_TOP_CENTER, PIN_TOP_RIGHT,
		PIN_MID_LEFT, PIN_MID_CENTER, PIN_MID_RIGHT,
		PIN_BOTTOM_LEFT, PIN_BOTTOM_CENTER, PIN_BOTTOM_RIGHT,
	}

	oldW, oldH := screenWidth, screenHeight
	defer func() { screenWidth, screenHeight = oldW, oldH }()

	for _, pin := range pins {
		t.Run(fmt.Sprintf("pin_%d", pin), func(t *testing.T) {
			windows = nil
			win := &windowData{
				Size:     point{X: 50, Y: 50},
				Position: point{X: 10, Y: 10},
				PinTo:    pin,
				open:     true,
			}
			windows = []*windowData{win}

			SetScreenSize(200, 150)
			exp := expectedPinPos(pin, 200, 150, win.GetSize(), win.GetPos())
			if pos := win.getPosition(); pos != exp {
				t.Fatalf("initial pos got %+v want %+v", pos, exp)
			}

			SetScreenSize(400, 300)
			exp = expectedPinPos(pin, 400, 300, win.GetSize(), win.GetPos())
			pos := win.getPosition()
			if pos != exp {
				t.Fatalf("resized pos got %+v want %+v", pos, exp)
			}

			size := win.GetSize()
			if pos.X < 0 || pos.Y < 0 || pos.X+size.X > float32(screenWidth) || pos.Y+size.Y > float32(screenHeight) {
				t.Fatalf("pin %v out of bounds after resize: pos=%+v size=%+v", pin, pos, size)
			}
		})
	}
}

// TestPinChangeReanchorsWindow ensures that changing a window's PinTo value
// reinterprets its existing Position as an offset from the new anchor.
func TestPinChangeReanchorsWindow(t *testing.T) {
	windows = nil
	oldW, oldH := screenWidth, screenHeight
	defer func() { screenWidth, screenHeight = oldW, oldH; windows = nil }()

	win := &windowData{
		Size:     point{X: 50, Y: 50},
		Position: point{X: 10, Y: 10},
		PinTo:    PIN_TOP_LEFT,
		open:     true,
	}
	windows = []*windowData{win}

	SetScreenSize(200, 150)
	if pos := win.getPosition(); pos != (point{X: 10, Y: 10}) {
		t.Fatalf("unexpected initial pos: %+v", pos)
	}

	win.PinTo = PIN_TOP_RIGHT
	SetScreenSize(200, 150)
	pos := win.getPosition()
	exp := expectedPinPos(PIN_TOP_RIGHT, 200, 150, win.GetSize(), win.GetPos())
	if pos != exp {
		t.Fatalf("reanchored pos got %+v want %+v", pos, exp)
	}
	size := win.GetSize()
	if pos.X < 0 || pos.Y < 0 || pos.X+size.X > float32(screenWidth) || pos.Y+size.Y > float32(screenHeight) {
		t.Fatalf("window out of bounds after reanchor: pos=%+v size=%+v", pos, size)
	}
}
