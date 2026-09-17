package main

import (
	"testing"

	"gothoom/eui"
)

func editorSettingsFixture(t *testing.T) *sourceEditor {
	t.Helper()
	ed := scriptSourceEditorFixture(t, false)
	oldGS, oldDirty := gs, settingsDirty
	oldPanel, oldPicker := sourceEditorSettings, colorPickerWin
	oldTheme, oldStyle := eui.CurrentThemeName(), eui.CurrentStyleName()
	sourceEditorSettings, colorPickerWin = nil, nil
	gs.EditorUseCustomColors, gs.EditorSyntaxColors = false, nil
	t.Cleanup(func() {
		if sourceEditorSettings != nil {
			sourceEditorSettings.win.Close()
		}
		if colorPickerWin != nil {
			colorPickerWin.Close()
		}
		gs, settingsDirty = oldGS, oldDirty
		sourceEditorSettings, colorPickerWin = oldPanel, oldPicker
		_ = eui.LoadTheme(oldTheme)
		_ = eui.LoadStyle(oldStyle)
	})
	if err := eui.LoadTheme("AccentDark"); err != nil {
		t.Fatal(err)
	}
	clickMacroEditorButton(t, ed.win, "Settings")
	if sourceEditorSettings == nil {
		t.Fatal("Settings did not open editor preferences")
	}
	return ed
}

func TestEditorSyntaxColorSettings(t *testing.T) {
	ed := editorSettingsFixture(t)
	panel := sourceEditorSettings
	before := ed.input.Text
	toggle := func(checked bool) {
		panel.custom.Handler.Emit(eui.UIEvent{Type: eui.EventCheckboxChanged, Checked: checked})
	}
	if panel.custom.Checked || !panel.reset.Disabled || gs.EditorSyntaxColors != nil {
		t.Fatal("editor should initially follow the theme")
	}
	for _, swatch := range panel.swatches {
		if !swatch.Disabled {
			t.Fatal("theme colors must be read-only")
		}
	}
	toggle(true)
	if gs.EditorSyntaxColors == nil || *gs.EditorSyntaxColors != eui.CurrentSyntaxColors() || !settingsDirty || panel.reset.Disabled {
		t.Fatal("custom palette did not start with the theme colors")
	}
	// Exercise the real picker: Cancel leaves the palette alone; Apply updates it.
	swatch := panel.swatches[0]
	swatch.Handler.Emit(eui.UIEvent{Type: eui.EventClick})
	if colorPickerWin == nil {
		t.Fatal("swatch did not open the color picker")
	}
	clickMacroEditorButton(t, colorPickerWin, "Cancel")
	if *gs.EditorSyntaxColors != eui.CurrentSyntaxColors() {
		t.Fatal("Cancel modified the custom palette")
	}
	swatch.Handler.Emit(eui.UIEvent{Type: eui.EventClick})
	var setBrightness func([]*eui.ItemData) bool
	setBrightness = func(items []*eui.ItemData) bool {
		for _, item := range items {
			if item.ItemType == eui.ITEM_COLORWHEEL {
				item.OnColorChange(eui.ColorWhite)
				return true
			}
			if setBrightness(item.Contents) {
				return true
			}
		}
		return false
	}
	if !setBrightness(colorPickerWin.Contents) {
		t.Fatal("picker has no color wheel")
	}
	clickMacroEditorButton(t, colorPickerWin, "Apply")
	custom := *gs.EditorSyntaxColors
	if custom.Comments == eui.CurrentSyntaxColors().Comments || swatch.WheelColor != custom.Comments {
		t.Fatal("Apply did not update the custom swatch")
	}
	for _, editor := range sourceEditors {
		if editor.highlightColors != custom {
			t.Fatal("color change did not reach every open editor")
		}
	}
	toggle(false)
	if sourceEditorColors() != eui.CurrentSyntaxColors() || *gs.EditorSyntaxColors != custom || !swatch.Disabled {
		t.Fatal("theme mode lost the custom palette")
	}
	if err := eui.LoadTheme("AccentLight"); err != nil {
		t.Fatal(err)
	}
	updateSourceEditors()
	if panel.colors != eui.CurrentSyntaxColors() || ed.highlightColors != panel.colors {
		t.Fatal("theme change did not refresh settings and editor")
	}
	toggle(true)
	if sourceEditorColors() != custom || swatch.Disabled {
		t.Fatal("custom palette was not restored")
	}
	clickMacroEditorButton(t, panel.win, "Copy Theme Colors")
	if sourceEditorColors() != eui.CurrentSyntaxColors() || !gs.EditorUseCustomColors {
		t.Fatal("copy theme did not replace the custom palette")
	}
	if ed.input.Text != before || ed.dirty() || ed.input.CanUndo() {
		t.Fatal("color settings changed the draft or undo history")
	}
	openSourceEditorSettings()
	if sourceEditorSettings != panel {
		t.Fatal("opening settings created a duplicate window")
	}
}

func TestEditorSyntaxColorsRefreshWithoutBackgroundChange(t *testing.T) {
	ed := editorSettingsFixture(t)
	background := ed.input.Color
	// Simulate a live palette reload that changes only a syntax color.
	colors := ed.input.Theme.Syntax
	colors.Keywords = eui.NewColor(221, 177, 133, 255)
	original := ed.input.Theme.Syntax
	ed.input.Theme.Syntax = colors
	t.Cleanup(func() { ed.input.Theme.Syntax = original })
	updateSourceEditors()
	if ed.input.Color != background || ed.highlightColors != colors || sourceEditorSettings.colors != colors {
		t.Fatal("syntax-only theme change did not refresh open editors")
	}
}

func TestEditorSyntaxColorsSettingsRoundTrip(t *testing.T) {
	want := gsdef
	colors := eui.DefaultSyntaxColors(eui.ColorBlack)
	colors.Numbers = eui.NewColor(12, 34, 56, 78)
	want.EditorUseCustomColors, want.EditorSyntaxColors = true, &colors
	data, err := marshalSettingsDocument(want)
	if err != nil {
		t.Fatal(err)
	}
	got, err := unmarshalSettingsDocument(data, gsdef)
	if err != nil || !got.EditorUseCustomColors || got.EditorSyntaxColors == nil || *got.EditorSyntaxColors != colors {
		t.Fatalf("editor colors did not survive settings reload: %v", err)
	}
	legacy, err := unmarshalSettingsDocument([]byte(`{"version":4,"interface":{}}`), gsdef)
	if err != nil || legacy.EditorUseCustomColors || legacy.EditorSyntaxColors != nil {
		t.Fatalf("old settings should follow the theme: %v", err)
	}
}
