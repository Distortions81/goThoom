package eui

import "testing"

func TestBringForwardReordersBackgroundWindowsWithinTheirLayer(t *testing.T) {
	oldWindows := windows
	oldActive := activeWindow
	windows = nil
	activeWindow = nil
	t.Cleanup(func() {
		windows = oldWindows
		activeWindow = oldActive
	})

	first := NewWindow()
	first.AlwaysDrawFirst = true
	second := NewWindow()
	second.AlwaysDrawFirst = true
	utility := NewWindow()
	first.AddWindow(false)
	second.AddWindow(false)
	utility.AddWindow(false)

	first.BringForward()
	if len(windows) != 3 || windows[0] != second || windows[1] != first || windows[2] != utility {
		t.Fatalf("window order = %p %p %p", windows[0], windows[1], windows[2])
	}
	if activeWindow != first {
		t.Fatal("brought-forward background window did not become active")
	}
}
