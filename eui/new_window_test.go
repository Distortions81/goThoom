//go:build test

package eui

import "testing"

// TestNewWindowDefaultsClosed ensures that newly created windows start closed.
func TestNewWindowDefaultsClosed(t *testing.T) {
	prevTheme := currentTheme
	currentTheme = nil
	win := NewWindow()
	if win.open {
		t.Fatalf("expected new window to be closed by default")
	}
	currentTheme = prevTheme
}
