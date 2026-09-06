package main

import "gothoom/eui"

// bindThemePreview previews without changing persisted settings. Restore before
// selection so the normal handler remains the only path that commits a choice.
func bindThemePreview(dropdown *eui.ItemData, palette bool, changed func()) {
	var active bool
	var theme, style string
	var accent eui.Color
	var saturation float64
	restore := func() {
		if !active {
			return
		}
		active = false
		if palette {
			_ = eui.LoadTheme(theme)
			eui.SetAccentSaturation(saturation)
			eui.SetAccentColor(accent)
		}
		_ = eui.LoadStyle(style)
		if changed != nil {
			changed()
		}
	}
	dropdown.OnHover = func(index int) {
		if dropdown.HoverIndex < 0 || index == dropdown.Selected || index < 0 || index >= len(dropdown.Options) {
			restore()
			return
		}
		if !active {
			theme, style = eui.CurrentThemeName(), eui.CurrentStyleName()
			accent, saturation = eui.AccentColor(), eui.AccentSaturation()
			active = true
		}
		var err error
		if palette {
			// A custom palette without a recommendation keeps the chosen style.
			_ = eui.LoadStyle(style)
			err = eui.LoadTheme(dropdown.Options[index])
		} else {
			err = eui.LoadStyle(dropdown.Options[index])
		}
		if err != nil {
			restore()
			return
		}
		if changed != nil {
			changed()
		}
	}
	commit := dropdown.Handler.Handle
	dropdown.Handler.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventDropdownSelected {
			restore()
		}
		if commit != nil {
			commit(ev)
		}
	}
}

func refreshThemePreview() {
	updateInventoryWindow()
	updatePlayersWindow()
	refreshMessageTextWindows()
	updateDimmedScreenBG()
}
