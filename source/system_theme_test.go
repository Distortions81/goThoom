package main

import (
	"testing"
	"time"

	"gothoom/eui"

	"github.com/hajimehoshi/ebiten/v2"
)

func isolateSystemTheme(t *testing.T) {
	t.Helper()
	oldSettings, oldDirty := gs, settingsDirty
	oldQuery, oldCheck, oldPalette := querySystemColorMode, systemThemeLastCheck, systemThemeLastPalette
	oldTheme, oldStyle := eui.CurrentThemeName(), eui.CurrentStyleName()
	oldAccent, oldSaturation := eui.AccentColor(), eui.AccentSaturation()
	t.Cleanup(func() {
		if restoreThemePreview != nil {
			restoreThemePreview()
		}
		gs, settingsDirty = oldSettings, oldDirty
		querySystemColorMode, systemThemeLastCheck, systemThemeLastPalette = oldQuery, oldCheck, oldPalette
		_ = eui.LoadTheme(oldTheme)
		_ = eui.LoadStyle(oldStyle)
		eui.SetAccentSaturation(oldSaturation)
		eui.SetAccentColor(oldAccent)
	})
}

func TestSystemThemePolling(t *testing.T) {
	isolateSystemTheme(t)
	mode, queries := ebiten.ColorModeLight, 0
	querySystemColorMode = func() ebiten.ColorMode { queries++; return mode }
	gs.Theme, settingsDirty = followSystemTheme, false
	if err := loadThemeChoice(gs.Theme); err != nil {
		t.Fatal(err)
	}
	if eui.CurrentThemeName() != "AccentLight" || !eui.IsLightTheme() || queries != 1 {
		t.Fatal("startup did not query and apply the system's light theme")
	}
	start := systemThemeLastCheck
	if err := eui.LoadStyle("Outline"); err != nil {
		t.Fatal(err)
	}
	eui.SetAccentSaturation(0.25)
	eui.SetAccentColor(eui.NewColor(100, 170, 200, 255))
	accent := eui.AccentColor()
	if updateSystemTheme(start.Add(9*time.Second)) || queries != 1 {
		t.Fatal("queried before ten seconds elapsed")
	}
	if updateSystemTheme(start.Add(10*time.Second)) || queries != 2 {
		t.Fatal("unchanged appearance should be queried without applying a theme")
	}
	mode = ebiten.ColorModeDark
	if updateSystemTheme(start.Add(19*time.Second)) || queries != 2 {
		t.Fatal("queried too soon after the previous check")
	}
	if !updateSystemTheme(start.Add(20*time.Second)) || eui.CurrentThemeName() != "AccentDark" || eui.IsLightTheme() {
		t.Fatal("did not apply changed dark appearance at the next check")
	}
	if eui.CurrentStyleName() != "Outline" || eui.AccentColor() != accent || eui.AccentSaturation() != 0.25 {
		t.Fatal("system switch replaced chosen style or accent")
	}
	mode = ebiten.ColorModeLight
	if !updateSystemTheme(start.Add(30*time.Second)) || eui.CurrentThemeName() != "AccentLight" {
		t.Fatal("did not switch back to light")
	}
	if gs.Theme != followSystemTheme || settingsDirty {
		t.Fatal("automatic switch changed persisted preferences")
	}
	gs.Theme = "NeonNight"
	if err := loadThemeChoice(gs.Theme); err != nil {
		t.Fatal(err)
	}
	before := queries
	if updateSystemTheme(start.Add(time.Minute)) || queries != before || eui.CurrentThemeName() != "NeonNight" {
		t.Fatal("manual palette should disable system polling")
	}
}

func TestSystemThemePreviewDefersChange(t *testing.T) {
	isolateSystemTheme(t)
	for _, palette := range []bool{true, false} {
		mode := ebiten.ColorModeDark
		querySystemColorMode = func() ebiten.ColorMode { return mode }
		gs.Theme = followSystemTheme
		if err := loadThemeChoice(gs.Theme); err != nil {
			t.Fatal(err)
		}
		due := systemThemeLastCheck.Add(10 * time.Second)
		dropdown, _ := eui.NewDropdown()
		dropdown.Options = []string{followSystemTheme, "NeonNight"}
		if !palette {
			dropdown.Options = []string{eui.CurrentStyleName(), "Outline"}
		}
		dropdown.Selected = 0
		bindThemePreview(dropdown, palette, nil)
		dropdown.HoverIndex = 1
		dropdown.OnHover(1)
		mode = ebiten.ColorModeLight
		if updateSystemTheme(due) {
			t.Fatal("system change overwrote an active preview")
		}
		dropdown.HoverIndex = -1
		dropdown.OnHover(-1)
		if !updateSystemTheme(due) || eui.CurrentThemeName() != "AccentLight" {
			t.Fatal("deferred system change was lost on preview dismissal")
		}
	}
}

func TestFollowSystemChoicePreviewAndCommit(t *testing.T) {
	isolateSystemTheme(t)
	querySystemColorMode = func() ebiten.ColorMode { return ebiten.ColorModeLight }
	gs.Theme = "NeonNight"
	if err := loadThemeChoice(gs.Theme); err != nil {
		t.Fatal(err)
	}
	dropdown, handler := eui.NewDropdown()
	dropdown.Options = []string{"NeonNight", followSystemTheme}
	dropdown.Selected = 0
	handler.Handle = func(ev eui.UIEvent) {
		gs.Theme = dropdown.Options[ev.Index]
		if err := loadThemeChoice(gs.Theme); err != nil {
			t.Fatal(err)
		}
	}
	bindThemePreview(dropdown, true, nil)
	dropdown.HoverIndex = 1
	dropdown.OnHover(1)
	if eui.CurrentThemeName() != "AccentLight" || gs.Theme != "NeonNight" {
		t.Fatal("follow system preview failed or committed prematurely")
	}
	dropdown.HoverIndex = -1
	dropdown.OnHover(-1)
	if eui.CurrentThemeName() != "NeonNight" {
		t.Fatal("preview did not restore manual palette")
	}
	dropdown.HoverIndex = 1
	dropdown.OnHover(1)
	dropdown.Selected = 1
	handler.Emit(eui.UIEvent{Type: eui.EventDropdownSelected, Index: 1})
	if gs.Theme != followSystemTheme || eui.CurrentThemeName() != "AccentLight" || restoreThemePreview != nil {
		t.Fatal("follow system selection was not committed")
	}
}

func TestSystemThemeDefaultsAndPersistence(t *testing.T) {
	isolateSystemTheme(t)
	querySystemColorMode = func() ebiten.ColorMode { return ebiten.ColorModeUnknown }
	if gsdef.Theme != followSystemTheme || themeChoices()[0] != followSystemTheme {
		t.Fatal("follow system must be the default and first choice")
	}
	if resolveThemeChoice("") != "AccentDark" || resolveThemeChoice(followSystemTheme) != "AccentDark" {
		t.Fatal("unknown system appearance must fall back to AccentDark")
	}
	missing, err := unmarshalSettingsDocument([]byte(`{"version":4}`), gsdef)
	if err != nil || missing.Theme != followSystemTheme {
		t.Fatalf("missing preference did not default to follow system: %q, %v", missing.Theme, err)
	}
	for _, name := range []string{followSystemTheme, "", "AccentDark", "NeonNight"} {
		want := cloneSettings(gsdef)
		want.Theme = name
		data, err := marshalSettingsDocument(want)
		if err != nil {
			t.Fatal(err)
		}
		got, err := unmarshalSettingsDocument(data, gsdef)
		if err != nil || got.Theme != name {
			t.Fatalf("saved theme %q changed to %q: %v", name, got.Theme, err)
		}
		profile, err := captureCharacterProfile("Theme Test", want)
		if err != nil {
			t.Fatal(err)
		}
		got, err = applyCharacterProfile(gsdef, profile)
		if err != nil || got.Theme != name {
			t.Fatalf("profile theme %q changed to %q: %v", name, got.Theme, err)
		}
	}
}
