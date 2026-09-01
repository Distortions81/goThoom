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

func TestMarkOpenCallsOnOpenOnlyForClosedWindow(t *testing.T) {
	windows = nil
	win := NewWindow()
	calls := 0
	win.OnOpen = func() { calls++ }

	win.MarkOpen()
	win.MarkOpen()
	if calls != 1 {
		t.Fatalf("OnOpen calls while already open = %d, want 1", calls)
	}

	win.Close()
	win.MarkOpen()
	if calls != 2 {
		t.Fatalf("OnOpen calls after reopening = %d, want 2", calls)
	}
}
