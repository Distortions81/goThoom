//go:build test

package eui

import "testing"

func TestRemoveWindowUpdatesActiveWindow(t *testing.T) {
	win0 := &windowData{Title: "win0", open: false}
	win1 := &windowData{Title: "win1", open: true}
	win2 := &windowData{Title: "win2", open: true}

	windows = []*windowData{win0, win1, win2}
	activeWindow = win2

	win2.RemoveWindow()
	if activeWindow != win1 {
		t.Fatalf("expected active window to be win1, got %v", activeWindow)
	}

	win1.RemoveWindow()
	if activeWindow != nil {
		t.Fatalf("expected active window to be nil, got %v", activeWindow)
	}

	win0.RemoveWindow()
}
