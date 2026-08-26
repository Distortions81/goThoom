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
	originalScale := eui.UIScale()
	settingsWin = nil
	t.Cleanup(func() {
		if settingsWin != nil {
			settingsWin.RemoveWindow()
		}
		settingsWin = originalWindow
		qualityPresetDD = originalPreset
		eui.SetUIScale(originalScale)
		eui.SetScreenSize(originalWidth, originalHeight)
	})

	eui.SetScreenSize(1920, 951)
	eui.SetUIScale(1)
	makeSettingsWindow()
	for _, scale := range []float32{1, 1.25, 1.3} {
		eui.SetUIScale(scale)

		size := settingsWin.GetSize()
		t.Logf("settings window size at %.2fx: %.0fx%.0f", scale, size.X, size.Y)
		if settingsWin.NoCache {
			t.Fatal("settings window bypasses its render cache")
		}
		if settingsWin.RefreshInterval() != 100*time.Millisecond {
			t.Fatalf("settings refresh interval = %v, want 100ms", settingsWin.RefreshInterval())
		}
		if horizontal, vertical := settingsWin.RequiresScroll(); horizontal || vertical {
			t.Fatalf("settings window requires scrollbars at %.2fx: horizontal=%t vertical=%t", scale, horizontal, vertical)
		}
		if size.X > 1200*scale || size.Y > 720*scale {
			t.Fatalf("settings window size at %.2fx = %.0fx%.0f, want at most %.0fx%.0f", scale, size.X, size.Y, 1200*scale, 720*scale)
		}
	}
}
