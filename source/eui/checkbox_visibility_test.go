package eui

import "testing"

func TestDisabledToggleVisibilityAcrossPalettes(t *testing.T) {
	isolateThemeTest(t)
	entries, err := embeddedThemes.ReadDir("themes/palettes")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		name := entry.Name()[:len(entry.Name())-len(".json")]
		if name == "Example" {
			continue
		}
		t.Run(name, func(t *testing.T) {
			if err := LoadTheme(name); err != nil {
				t.Fatal(err)
			}
			for _, kind := range []struct {
				name  string
				style *itemData
			}{
				{"Checkbox", &currentTheme.Checkbox},
				{"Radio", &currentTheme.Radio},
			} {
				style := disabledStyle(kind.style)
				if ratio := textContrast(style.Color, currentTheme.Window.BGColor); ratio < 3 {
					t.Errorf("%s disabled outline/fill disappears against window: contrast %.2f", kind.name, ratio)
				}
				for _, background := range []Color{style.Color, currentTheme.Window.BGColor} {
					if ratio := textContrast(style.TextColor, background); ratio < 3 {
						t.Errorf("%s disabled checkmark or caption is unreadable: contrast %.2f", kind.name, ratio)
					}
				}
			}
		})
	}
}
