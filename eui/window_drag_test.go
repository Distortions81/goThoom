//go:build test

package eui

import "testing"

// TestWindowDragClickScaled ensures that dragging a window while uiScale is not 1
// still allows clicks within its controls.
func TestWindowDragClickScaled(t *testing.T) {
	prevScale := uiScale
	uiScale = 2
	defer func() { uiScale = prevScale }()

	// Create a window and a button inside it.
	btn := *defaultButton
	btn.Position = point{X: 0, Y: 0}
	btn.Size = point{X: 10, Y: 10}
	btn.Margin = 0

	win := &windowData{
		Position:    point{X: 0, Y: 0},
		Size:        point{X: 50, Y: 50},
		open:        true,
		Movable:     true,
		Margin:      0,
		Padding:     0,
		Border:      0,
		TitleHeight: 0,
	}

	win.Contents = []*itemData{&btn}
	btn.win = win
	windows = []*windowData{win}
	defer func() { windows = nil }()

	// Establish initial draw rect for the button.
	winPos := win.getPosition()
	itemPos := pointAdd(winPos, btn.getPosition(win))
	btn.DrawRect = rect{
		X0: itemPos.X,
		Y0: itemPos.Y,
		X1: itemPos.X + btn.GetSize().X,
		Y1: itemPos.Y + btn.GetSize().Y,
	}

	// Simulate dragging the window.
	oldPos := win.getPosition()
	win.Position = pointAdd(win.Position, point{X: 5, Y: 7})
	delta := pointSub(win.getPosition(), oldPos)
	shiftDrawRects(win, delta)

	// Simulate a click inside the button at its new position.
	clickPos := point{X: btn.DrawRect.X0 + 1, Y: btn.DrawRect.Y0 + 1}
	win.clickWindowItems(clickPos, true)

	if btn.Clicked.IsZero() {
		t.Fatalf("expected button click to be registered after drag")
	}
}
