package main

import (
	"testing"
	"time"

	"gothoom/eui"
)

func TestSettingsWindowFitsCurrentScreenLayout(t *testing.T) {
	initFont()
	originalWindow := settingsWin
	originalPreset := qualityPresetDD
	originalWidth, originalHeight := eui.ScreenSize()
	settingsWin = nil
	eui.SetScreenSize(1920, 951)
	t.Cleanup(func() {
		if settingsWin != nil {
			settingsWin.RemoveWindow()
		}
		settingsWin = originalWindow
		qualityPresetDD = originalPreset
		eui.SetScreenSize(originalWidth, originalHeight)
	})

	makeSettingsWindow()
	size := settingsWin.GetSize()
	t.Logf("settings window size: %.0fx%.0f", size.X, size.Y)
	if settingsWin.NoCache {
		t.Fatal("settings window bypasses its render cache")
	}
	if settingsWin.RefreshInterval() != 100*time.Millisecond {
		t.Fatalf("settings refresh interval = %v, want 100ms", settingsWin.RefreshInterval())
	}
	if size.X > 1200 || size.Y > 700 {
		t.Fatalf("settings window size = %.0fx%.0f, want at most 1200x700 for the current screen", size.X, size.Y)
	}
}
