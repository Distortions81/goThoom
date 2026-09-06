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
	settingsWin.MarkOpen()
	for _, screen := range []struct {
		width, height int
		scale         float32
	}{
		{1920, 951, 1}, {1920, 951, 1.25}, {1920, 951, 1.3}, {3840, 2160, 2},
	} {
		eui.SetScreenSize(screen.width, screen.height)
		eui.SetUIScale(screen.scale)
		wantSize := settingsWin.GetSize()
		for index, tab := range settingsWin.Contents[0].Tabs {
			settingsWin.Contents[0].ActiveTab = index
			settingsWin.Refresh()
			size := settingsWin.GetSize()
			if size != wantSize || settingsWin.AutoSize || settingsWin.Resizable {
				t.Fatalf("%s changed the fixed Settings size: %v, want %v", tab.Name, size, wantSize)
			}
			t.Logf("%s at %.2fx: %.0fx%.0f", tab.Name, screen.scale, size.X, size.Y)
			if settingsWin.NoCache {
				t.Fatal("settings window bypasses its render cache")
			}
			if settingsWin.RefreshInterval() != 100*time.Millisecond {
				t.Fatalf("settings refresh interval = %v, want 100ms", settingsWin.RefreshInterval())
			}
			if horizontal, vertical := settingsWin.RequiresScroll(); horizontal || vertical {
				t.Fatalf("%s requires scrollbars at %.2fx: horizontal=%t vertical=%t", tab.Name, screen.scale, horizontal, vertical)
			}
			if size.X > 730*screen.scale || size.Y > 730*screen.scale {
				t.Fatalf("%s at %.2fx = %.0fx%.0f, want at most 730x730 logical pixels", tab.Name, screen.scale, size.X, size.Y)
			}
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

func TestSettingsControlsAreGroupedByPurpose(t *testing.T) {
	initFont()
	originalWindow, originalPreset := settingsWin, qualityPresetDD
	settingsWin = nil
	t.Cleanup(func() {
		settingsWin.RemoveWindow()
		settingsWin, qualityPresetDD = originalWindow, originalPreset
	})
	makeSettingsWindow()
	tabs := settingsWin.Contents[0].Tabs
	if len(tabs) != 10 || settingsWin.Contents[0].TabColumns != 5 {
		t.Fatal("settings should have ten categories in two rows")
	}
	locations := map[string]string{}
	var visit func([]*eui.ItemData, string)
	visit = func(items []*eui.ItemData, page string) {
		for _, item := range items {
			if item.Text != "" {
				locations[item.Text] = page
			}
			if item.Label != "" {
				locations[item.Label] = page
			}
			visit(item.Contents, page)
		}
	}
	for _, tab := range tabs {
		visit(tab.Contents, tab.Name)
	}
	for control, want := range map[string]string{
		"File Paths": "Files", "Open User Data Folder": "Files", "Open Diagnostics Folder": "Files",
		"Auto-record sessions": "Files", "Download Files": "Files",
		"Always on top": "Display", "Window Shadows": "Display",
		"Timestamp format": "Text", "Show recently on-screen group": "World",
		"Message Bubbles": "Bubbles", "Bubble Lifetime": "Bubbles",
		"TTS Voice": "Audio", "TTS Speed": "Audio", "Notification Settings": "Audio",
		"Keyboard Walk Speed": "Controls", "Middle-click moves windows": "Controls", "Gamepad": "Controls",
		"Server address": "Network", "NLSPT safety (%)": "Network",
		"Setup Wizard": "Tools", "Debug Settings": "Tools", "Reset All Settings": "Tools",
	} {
		if got := locations[control]; got != want {
			t.Errorf("%q lives in %q, want %q", control, got, want)
		}
	}
	for _, moved := range []string{
		"Floating-point sprite coordinates", "Sprite cache", "Power-save FPS", "Batch room artwork loading",
		"Reset Windows", "Audio Mixer", "Keybindings", "Hotkeys", "Enable chat TTS",
		"Audio enhancement for sound effects", "Audio enhancement for music",
		"Network Latency & Server Phase Timing (NLSPT)", "Show Network Timing",
	} {
		if page, exists := locations[moved]; exists {
			t.Errorf("detailed control %q remains in main Settings tab %q", moved, page)
		}
	}
	for _, action := range buildCommandPaletteActions() {
		if action.label == "Settings: Files" {
			action.run()
			if selectedSettingsTab() != "Files" || !settingsWin.IsOpen() {
				t.Fatal("command palette did not open the Files tab")
			}
		}
	}
	if selectedSettingsTab() != "Files" {
		t.Fatal("Files tab is missing from the command palette")
	}
	if _, exists := locations["Advanced Settings"]; exists {
		t.Fatal("obsolete Advanced Settings launcher remains")
	}
}
