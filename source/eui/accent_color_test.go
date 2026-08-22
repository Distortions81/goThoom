package eui

import "testing"

// Test that setting the accent color updates the named "accent" color reference.
func TestSetAccentColorUpdatesNamedColor(t *testing.T) {
	namedColors = map[string]Color{}
	SetAccentColor(NewColor(255, 0, 0, 255))
	var c Color
	if err := c.UnmarshalJSON([]byte("\"accent\"")); err != nil {
		t.Fatalf("failed to unmarshal accent: %v", err)
	}
	if c != AccentColor() {
		t.Fatalf("named accent %v does not match accent color %v", c, AccentColor())
	}
}

// Test that setting the accent color marks all windows dirty so they redraw.
func TestSetAccentColorMarksWindowsDirty(t *testing.T) {
	oldWindows := windows
	oldNamed := namedColors
	defer func() {
		windows = oldWindows
		namedColors = oldNamed
	}()

	win1 := &windowData{Open: true}
	win2 := &windowData{Open: true}
	windows = []*windowData{win1, win2}
	namedColors = map[string]Color{}

	SetAccentColor(NewColor(0, 0, 255, 255))

	for i, w := range windows {
		if !w.Dirty {
			t.Fatalf("window %d not marked dirty", i)
		}
	}
}
