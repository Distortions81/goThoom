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

func TestThemeSwitchUpdatesWindowAndMenuEffects(t *testing.T) {
	isolateThemeTest(t)
	if err := LoadTheme("NeonNight"); err != nil {
		t.Fatal(err)
	}
	win := NewWindow()
	menu, _ := NewDropdown()
	win.AddItem(menu)
	windows = []*windowData{win}
	for _, name := range []string{"SeaGlass", "HighContrast", "Arcade"} {
		if err := LoadTheme(name); err != nil {
			t.Fatal(err)
		}
		if win.ShadowSize != currentTheme.Window.ShadowSize || win.ShadowColor != currentTheme.Window.ShadowColor || win.ShadowFalloff != currentTheme.Window.ShadowFalloff {
			t.Fatalf("%s left stale window effects", name)
		}
		if menu.ShadowSize != currentTheme.Dropdown.ShadowSize || menu.ShadowColor != currentTheme.Dropdown.ShadowColor || menu.ShadowFalloff != currentTheme.Dropdown.ShadowFalloff {
			t.Fatalf("%s left stale menu effects", name)
		}
	}
}
