package eui

import (
	"strings"
	"testing"
)

func TestTextWindowInputActionReservesSpaceAndStaysOutsideScrollingInput(t *testing.T) {
	if err := Init(); err != nil {
		t.Fatal(err)
	}
	win, list, input := NewTextWindow("Chat", HZoneLeft, VZoneTop, true)
	defer win.RemoveWindow()
	win.Size = Point{X: 260, Y: 180}
	action, _ := NewButton()
	action.Size = Point{X: 28, Y: 28}
	var cache TextWindowWrapCache
	options := TextWindowOptions{FontSize: 12, InputText: strings.Repeat("a long draft ", 40), InputEditable: true, InputAction: action}
	UpdateTextWindow(win, list, input, nil, options, &cache)
	row := input.Parent
	if row == list.Parent || row.Scrollable || action.Parent != row || !input.Scrollable {
		t.Fatal("action shares the scrolling input viewport")
	}
	if input.Size.X+action.Size.X > row.Size.X+0.01 || input.Contents[0].Size.X+input.Contents[0].Position.X > input.Size.X+0.01 {
		t.Fatal("input did not reserve the action width")
	}
	withAction := input.Size.X
	options.InputAction = nil
	UpdateTextWindow(win, list, input, nil, options, &cache)
	if len(row.Contents) != 1 || input.Size.X <= withAction {
		t.Fatal("hiding action did not reclaim its space")
	}
	options.InputText = "short"
	options.InputAction = action
	UpdateTextWindow(win, list, input, nil, options, &cache)
	if input.Scrollable || row.Size.Y < action.Size.Y || input.Size.Y != row.Size.Y {
		t.Fatal("single-line input does not fit the action")
	}
}
