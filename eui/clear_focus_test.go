//go:build integration
// +build integration

package eui

import "testing"

// TestClearFocus verifies ClearFocus removes focus only when the provided
// item currently holds focus.
func TestClearFocus(t *testing.T) {
	uiScale = 1
	item, _ := NewInput()
	item.DrawRect = rect{X0: 0, Y0: 0, X1: 10, Y1: 10}
	item.clickItem(point{X: 1, Y: 1}, true)
	if focusedItem != item {
		t.Fatalf("expected item to be focused")
	}

	// Clearing focus with a different item should do nothing.
	other, _ := NewInput()
	ClearFocus(other)
	if focusedItem != item {
		t.Fatalf("focus should remain on original item")
	}

	// Clearing focus with the focused item should remove focus.
	ClearFocus(item)
	if focusedItem != nil {
		t.Fatalf("focus was not cleared")
	}
}
