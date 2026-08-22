package eui

import "testing"

func TestCloseUpdatesActiveWindow(t *testing.T) {
	windows = nil
	w1 := NewWindow()
	w2 := NewWindow()
	w1.MarkOpen()
	w2.MarkOpen()
	activeWindow = w2
	w2.Close()
	if activeWindow != w1 {
		t.Fatalf("expected activeWindow to fall back to w1, got %v", activeWindow)
	}
}
