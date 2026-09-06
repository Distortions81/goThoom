package main

import (
	"testing"

	"gothoom/eui"
)

func TestClassicTextColorControlsStayInSync(t *testing.T) {
	initFont()
	oldGS, oldDirty := gs, settingsDirty
	oldWin := textColorsWin
	oldClassic, oldOverride := classicMessageColorsCB, themeTextColorOverrideCB
	oldDark, oldLight := textColorSwatchesDark, textColorSwatchesLight
	textColorsWin, classicMessageColorsCB, themeTextColorOverrideCB = nil, nil, nil
	t.Cleanup(func() {
		if textColorsWin != nil {
			textColorsWin.RemoveWindow()
		}
		gs, settingsDirty = oldGS, oldDirty
		textColorsWin = oldWin
		classicMessageColorsCB, themeTextColorOverrideCB = oldClassic, oldOverride
		textColorSwatchesDark, textColorSwatchesLight = oldDark, oldLight
	})
	gs.ClassicMessageColors = false
	gs.OverrideThemeTextColor = true
	gs.MessageTextColors = defaultMessageTextColors()
	gs.MessageTextColorsLight = defaultLightMessageTextColors()
	makeTextColorsWindow()
	controls := textColorsWin.Contents[0].Contents
	classic := controls[0]
	override := controls[len(controls)-2]
	reset := controls[len(controls)-1]
	toggle := func(item *eui.ItemData, checked bool) {
		item.Checked = checked
		item.Handler.Emit(eui.UIEvent{Type: eui.EventCheckboxChanged, Checked: checked})
	}
	toggle(classic, true)
	if !gs.ClassicMessageColors || gs.OverrideThemeTextColor || override.Checked || !override.Disabled {
		t.Fatal("classic mode did not clear and disable the theme override")
	}
	for _, swatches := range []map[string]*eui.ItemData{textColorSwatchesDark, textColorSwatchesLight} {
		for _, swatch := range swatches {
			if !swatch.Disabled {
				t.Fatal("classic mode left a custom swatch enabled")
			}
		}
	}
	toggle(classic, false)
	if override.Disabled || textColorSwatchesDark[messageTextTypeSay].Disabled || !textColorSwatchesDark[messageTextTypeSystem].Disabled {
		t.Fatal("leaving classic mode did not restore control availability")
	}
	toggle(override, true)
	if !gs.OverrideThemeTextColor || textColorSwatchesDark[messageTextTypeSystem].Disabled || textColorSwatchesLight[messageTextTypeSystem].Disabled {
		t.Fatal("theme override did not enable default text swatches")
	}
	toggle(classic, true)
	reset.Handler.Emit(eui.UIEvent{Type: eui.EventClick})
	if gs.ClassicMessageColors || gs.OverrideThemeTextColor || classic.Checked || override.Checked || override.Disabled {
		t.Fatal("reset did not restore both checkboxes")
	}
}
