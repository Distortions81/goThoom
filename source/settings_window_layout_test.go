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
		if size.X > 1200*scale || size.Y > 730*scale {
			t.Fatalf("settings window size at %.2fx = %.0fx%.0f, want at most %.0fx%.0f", scale, size.X, size.Y, 1200*scale, 730*scale)
		}
	}
}

func TestCombineMessagesControlLivesInTiledLayoutWindow(t *testing.T) {
	initFont()
	originalSettingsWin := settingsWin
	originalTileLayoutWin := tileLayoutWin
	originalCombineControl := settingsCombineMessagesCB
	settingsWin = nil
	tileLayoutWin = nil
	settingsCombineMessagesCB = nil
	t.Cleanup(func() {
		if settingsWin != nil {
			settingsWin.RemoveWindow()
		}
		if tileLayoutWin != nil {
			tileLayoutWin.RemoveWindow()
		}
		settingsWin = originalSettingsWin
		tileLayoutWin = originalTileLayoutWin
		settingsCombineMessagesCB = originalCombineControl
	})

	makeSettingsWindow()
	makeTileLayoutWindow()

	containsText := func(root *eui.WindowData, want string) bool {
		var visit func(items []*eui.ItemData) bool
		visit = func(items []*eui.ItemData) bool {
			for _, item := range items {
				if item.Text == want || visit(item.Contents) {
					return true
				}
				for _, tab := range item.Tabs {
					if visit(tab.Contents) {
						return true
					}
				}
			}
			return false
		}
		return visit(root.Contents)
	}

	if containsText(settingsWin, "Combine chat + console") {
		t.Fatal("combine chat control remains in the general Settings window")
	}
	if !containsText(tileLayoutWin, "Combine chat + console") {
		t.Fatal("combine chat control is missing from the tiled layout window")
	}
}

func TestAlternateGameSideDisabledForCenteredTiledLayout(t *testing.T) {
	initFont()
	originalSettings := gs
	originalTileLayoutWin := tileLayoutWin
	originalCombineControl := settingsCombineMessagesCB
	originalDirty := settingsDirty
	gs = gsdef
	gs.TiledWindows = true
	gs.TiledLayout = TiledLayoutCenter
	tileLayoutWin = nil
	settingsCombineMessagesCB = nil
	t.Cleanup(func() {
		if tileLayoutWin != nil {
			tileLayoutWin.RemoveWindow()
		}
		gs = originalSettings
		tileLayoutWin = originalTileLayoutWin
		settingsCombineMessagesCB = originalCombineControl
		settingsDirty = originalDirty
	})

	makeTileLayoutWindow()
	var layout, gameSide *eui.ItemData
	var visit func([]*eui.ItemData)
	visit = func(items []*eui.ItemData) {
		for _, item := range items {
			switch item.Label {
			case "Layout":
				layout = item
			case "Alternate game side":
				gameSide = item
			}
			visit(item.Contents)
		}
	}
	visit(tileLayoutWin.Contents)
	if layout == nil || gameSide == nil {
		t.Fatal("tiled layout window is missing arrangement controls")
	}
	if !gameSide.Disabled {
		t.Fatal("alternate game side is enabled for the centered layout")
	}

	layout.Handler.Emit(eui.UIEvent{Item: layout, Type: eui.EventDropdownSelected, Index: int(TiledLayoutSide)})
	if gameSide.Disabled {
		t.Fatal("alternate game side remains disabled for the side layout")
	}
	layout.Handler.Emit(eui.UIEvent{Item: layout, Type: eui.EventDropdownSelected, Index: int(TiledLayoutCenter)})
	if !gameSide.Disabled {
		t.Fatal("alternate game side was not disabled after selecting the centered layout")
	}
}

func TestClassicBubbleLifetimeDisablesModernSliders(t *testing.T) {
	initFont()
	originalSettings := gs
	originalWindow := settingsWin
	originalDirty := settingsDirty
	gs = gsdef
	gs.BubbleLifetimeMode = BubbleLifetimeClassic
	settingsWin = nil
	t.Cleanup(func() {
		if settingsWin != nil {
			settingsWin.RemoveWindow()
		}
		gs = originalSettings
		settingsWin = originalWindow
		settingsDirty = originalDirty
	})

	makeSettingsWindow()
	items := make(map[string]*eui.ItemData)
	var visit func([]*eui.ItemData)
	visit = func(children []*eui.ItemData) {
		for _, item := range children {
			if item.Label != "" {
				items[item.Label] = item
			}
			visit(item.Contents)
			for _, tab := range item.Tabs {
				visit(tab.Contents)
			}
		}
	}
	visit(settingsWin.Contents)

	lifetime := items["Bubble Lifetime"]
	base := items["Modern Base Life (s)"]
	perWord := items["Modern Life per Word (s)"]
	if lifetime == nil || base == nil || perWord == nil {
		t.Fatal("settings window is missing bubble lifetime controls")
	}
	if !base.Disabled || !perWord.Disabled {
		t.Fatal("modern lifetime sliders are enabled in classic mode")
	}

	lifetime.Handler.Emit(eui.UIEvent{Item: lifetime, Type: eui.EventDropdownSelected, Index: 0})
	if base.Disabled || perWord.Disabled {
		t.Fatal("modern lifetime sliders remain disabled after selecting modern mode")
	}
	lifetime.Handler.Emit(eui.UIEvent{Item: lifetime, Type: eui.EventDropdownSelected, Index: 1})
	if !base.Disabled || !perWord.Disabled {
		t.Fatal("modern lifetime sliders remain enabled after selecting classic mode")
	}
}
