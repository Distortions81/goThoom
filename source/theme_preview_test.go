package main

import (
	"testing"

	"gothoom/eui"
)

func TestThemeHoverPreviewRestoresAndCommits(t *testing.T) {
	oldTheme, oldStyle := eui.CurrentThemeName(), eui.CurrentStyleName()
	oldAccent, oldSaturation := eui.AccentColor(), eui.AccentSaturation()
	t.Cleanup(func() {
		_ = eui.LoadTheme(oldTheme)
		_ = eui.LoadStyle(oldStyle)
		eui.SetAccentSaturation(oldSaturation)
		eui.SetAccentColor(oldAccent)
	})
	for _, palette := range []bool{true, false} {
		if err := eui.LoadTheme("AccentDark"); err != nil {
			t.Fatal(err)
		}
		if err := eui.LoadStyle("Outline"); err != nil {
			t.Fatal(err)
		}
		eui.SetAccentSaturation(0.25)
		eui.SetAccentColor(eui.NewColor(100, 170, 200, 255))
		accent := eui.AccentColor()
		dropdown, handler := eui.NewDropdown()
		dropdown.Options = []string{"Outline", "Rounded", "missing-preview-style"}
		if palette {
			dropdown.Options = []string{"AccentDark", "NeonNight", "missing-preview-palette"}
		}
		dropdown.Selected = 0
		committed := ""
		handler.Handle = func(ev eui.UIEvent) {
			committed = dropdown.Options[ev.Index]
			if palette {
				_ = eui.LoadTheme(committed)
			} else {
				_ = eui.LoadStyle(committed)
			}
		}
		bindThemePreview(dropdown, palette, nil)
		hover := func(index int) { dropdown.HoverIndex = index; dropdown.OnHover(index) }
		assertRestored := func() {
			t.Helper()
			if eui.CurrentThemeName() != "AccentDark" || eui.CurrentStyleName() != "Outline" || eui.AccentColor() != accent || eui.AccentSaturation() != 0.25 {
				t.Fatal("preview did not restore palette, custom style and accent")
			}
		}
		hover(1)
		if committed != "" || eui.CurrentStyleName() != "Rounded" {
			t.Fatal("hover failed or committed prematurely")
		}
		if palette && eui.CurrentThemeName() != "NeonNight" {
			t.Fatal("palette was not previewed")
		}
		hover(-1)
		assertRestored()
		hover(1)
		hover(0)
		assertRestored()
		hover(1)
		hover(2)
		assertRestored()
		hover(1)
		dropdown.Selected = 1
		handler.Emit(eui.UIEvent{Type: eui.EventDropdownSelected, Index: 1})
		hover(-1)
		if committed != dropdown.Options[1] || eui.CurrentStyleName() != "Rounded" {
			t.Fatal("selection was reverted on dismissal")
		}
		if palette && eui.CurrentThemeName() != "NeonNight" {
			t.Fatal("selected palette was reverted")
		}
	}
}
