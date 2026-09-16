package eui

import "testing"

func TestCheckboxIndicatorUsesTheActiveCheckboxStyle(t *testing.T) {
	isolateThemeTest(t)
	theme := *baseTheme
	theme.Checkbox.Color = NewColor(11, 22, 33, 255)
	theme.Checkbox.OutlineColor = NewColor(44, 55, 66, 255)
	theme.Checkbox.ClickColor = NewColor(77, 88, 99, 255)
	currentTheme = &theme

	if fill, outline := checkboxIndicatorColors(&currentTheme.Checkbox, false); fill != theme.Checkbox.Color || outline != theme.Checkbox.OutlineColor {
		t.Fatalf("unchecked indicator colors = %v, %v; want %v, %v", fill, outline, theme.Checkbox.Color, theme.Checkbox.OutlineColor)
	}
	if fill, outline := checkboxIndicatorColors(&currentTheme.Checkbox, true); fill != theme.Checkbox.ClickColor || outline != theme.Checkbox.Color {
		t.Fatalf("checked indicator colors = %v, %v; want %v, %v", fill, outline, theme.Checkbox.ClickColor, theme.Checkbox.Color)
	}
}
