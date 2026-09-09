package main

import (
	"log"
	"time"

	"gothoom/eui"

	"github.com/hajimehoshi/ebiten/v2"
)

const followSystemTheme = "Follow system"
const systemThemeCheckInterval = 10 * time.Second

var querySystemColorMode = ebiten.SystemColorMode
var systemThemeLastCheck time.Time
var systemThemeLastPalette string

// Empty is the legacy spelling of the default theme preference.
func themeChoiceName(name string) string {
	if name == "" {
		return followSystemTheme
	}
	return name
}

func themeChoices() []string {
	names, _ := eui.ListThemes()
	return append([]string{followSystemTheme}, names...)
}

func currentThemeChoice() string {
	if themeChoiceName(gs.Theme) == followSystemTheme {
		return followSystemTheme
	}
	// EUI resolves older palette names to their current equivalents.
	return eui.CurrentThemeName()
}

func resolveThemeChoice(name string) string {
	if themeChoiceName(name) != followSystemTheme {
		return name
	}
	if querySystemColorMode() == ebiten.ColorModeLight {
		return "AccentLight"
	}
	return "AccentDark"
}

func loadThemeChoice(name string) error {
	if restoreThemePreview != nil {
		restoreThemePreview()
	}
	palette := resolveThemeChoice(name)
	if err := eui.LoadTheme(palette); err != nil {
		return err
	}
	systemThemeLastPalette = palette
	systemThemeLastCheck = time.Now()
	return nil
}

// Check on the game thread. Hover previews defer the check until dismissal so
// a system change cannot overwrite the preview or be lost when it is restored.
func updateSystemTheme(now time.Time) bool {
	if themeChoiceName(gs.Theme) != followSystemTheme || restoreThemePreview != nil || now.Sub(systemThemeLastCheck) < systemThemeCheckInterval {
		return false
	}
	systemThemeLastCheck = now
	palette := resolveThemeChoice(gs.Theme)
	if palette == systemThemeLastPalette {
		return false
	}
	style := eui.CurrentStyleName()
	accent, saturation := eui.AccentColor(), eui.AccentSaturation()
	if err := eui.LoadTheme(palette); err != nil {
		log.Printf("load system theme %q: %v", palette, err)
		return false
	}
	// Automatic light/dark changes keep the user's control style and accent.
	_ = eui.LoadStyle(style)
	eui.SetAccentSaturation(saturation)
	eui.SetAccentColor(accent)
	systemThemeLastPalette = palette
	return true
}
