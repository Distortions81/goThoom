package eui

import "testing"

func TestSetWindowShadows(t *testing.T) {
	original := windowShadows
	t.Cleanup(func() { SetWindowShadows(original) })

	win := NewWindow()
	win.Dirty = false
	windows = append(windows, win)
	t.Cleanup(func() { windows = windows[:len(windows)-1] })

	SetWindowShadows(false)
	if windowShadows {
		t.Fatal("window shadows remained enabled")
	}
	if !win.Dirty {
		t.Fatal("changing window shadows did not invalidate window caches")
	}
}
