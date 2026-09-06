package main

import (
	"testing"
	"time"

	"gothoom/eui"
)

func TestPerformanceTabUsesSettingsRenderCache(t *testing.T) {
	initFont()
	originalWindow := settingsWin
	settingsWin = nil
	t.Cleanup(func() {
		if settingsWin != nil {
			settingsWin.RemoveWindow()
		}
		settingsWin = originalWindow
	})

	makeSettingsWindow()
	if settingsWin.NoCache {
		t.Fatal("quality window redraws its full contents every frame")
	}
	if settingsWin.RefreshInterval() != 100*time.Millisecond {
		t.Fatalf("quality refresh interval = %v, want 100ms", settingsWin.RefreshInterval())
	}
}

func TestPerformanceTabUsesTopicTabs(t *testing.T) {
	initFont()
	originalScale := eui.UIScale()
	originalWidth, originalHeight := eui.ScreenSize()
	originalSettings := gs
	originalWindow := settingsWin
	settingsWin = nil
	gs = gsdef
	t.Cleanup(func() {
		if settingsWin != nil {
			settingsWin.RemoveWindow()
		}
		settingsWin = originalWindow
		gs = originalSettings
		eui.SetUIScale(originalScale)
		eui.SetScreenSize(originalWidth, originalHeight)
	})

	makeSettingsWindow()
	if len(settingsWin.Contents) != 1 {
		t.Fatalf("quality window root count = %d, want 1", len(settingsWin.Contents))
	}
	outer := settingsWin.Contents[0].Tabs[6].Contents[1]
	wantTabs := []string{"Artwork", "Motion", "Lighting & Effects", "Caching", "Power Saving"}
	if len(outer.Tabs) != len(wantTabs) {
		t.Fatalf("options tab count = %d, want %d", len(outer.Tabs), len(wantTabs))
	}
	for i, name := range wantTabs {
		if outer.Tabs[i].Name != name {
			t.Errorf("tab %d = %q, want %q", i, outer.Tabs[i].Name, name)
		}
	}

	locations := map[string]string{}
	var visit func(*eui.ItemData, string)
	visit = func(item *eui.ItemData, topic string) {
		if item.Text != "" {
			locations[item.Text] = topic
		}
		if item.Label != "" {
			locations[item.Label] = topic
		}
		for _, child := range item.Contents {
			visit(child, topic)
		}
	}
	for _, tab := range outer.Tabs {
		visit(tab, tab.Name)
	}
	for control, topic := range map[string]string{
		"Subpixel movement":            "Motion",
		"Character Animation Blending": "Motion",
		"World Animation Blending":     "Motion",
		"Character Shadows":            "Lighting & Effects",
		"Lighting Effects":             "Lighting & Effects",
		"Potato GPU (4096px Limit)":    "Caching",
		"Sprite cache":                 "Caching", "Batch room artwork loading": "Caching",
		"Precache Sounds": "Caching", "Show asset activity dots": "Caching",
		"Power-save FPS": "Power Saving", "VSync - Limit FPS": "Power Saving",
		"Artwork scale override": "Artwork",
	} {
		if got := locations[control]; got != topic {
			t.Errorf("%q is in %q, want %q", control, got, topic)
		}
	}
	selectSettingsTab("Performance")
	settingsWin.MarkOpen()
	for _, scale := range []float32{1, 1.25, 1.3, 2} {
		eui.SetScreenSize(int(1280*scale), int(800*scale))
		eui.SetUIScale(scale)
		wantSize := settingsWin.GetSize()
		for i, tab := range outer.Tabs {
			outer.ActiveTab = i
			settingsWin.Refresh()
			size := settingsWin.GetSize()
			if size != wantSize {
				t.Errorf("%s resized Settings: %v, want %v", tab.Name, size, wantSize)
			}
			if width := tab.GetSize().X / scale; width < settingsPanelWidth-8 {
				t.Errorf("%s uses only %.0f of %.0f logical pixels of content width", tab.Name, width, settingsPanelWidth)
			}
			t.Logf("%s at %.2fx: %.0fx%.0f", tab.Name, scale, size.X, size.Y)
			if horizontal, vertical := settingsWin.RequiresScroll(); horizontal || vertical {
				t.Errorf("%s at %.2fx requires scrollbars: horizontal=%v vertical=%v", tab.Name, scale, horizontal, vertical)
			}
			if size.X > 730*scale || size.Y > 750*scale {
				t.Errorf("%s exceeds compact layout at %.2fx: %v", tab.Name, scale, size)
			}
		}
	}

}

func TestPerformanceTabGroupsAndDisablesShaderEffects(t *testing.T) {
	initFont()
	originalSettings := gs
	originalWindow := settingsWin
	settingsWin = nil
	gs = gsdef
	gs.ShadersEnabled = false
	t.Cleanup(func() {
		if settingsWin != nil {
			settingsWin.RemoveWindow()
		}
		settingsWin = originalWindow
		gs = originalSettings
	})

	makeSettingsWindow()
	if shadersEnabledCB == nil || shadersEnabledCB.Disabled {
		t.Fatal("shader master is missing or disabled")
	}
	for name, control := range map[string]*eui.ItemData{
		"lighting":                  shaderLightingCB,
		"mobile light-cone shadows": mobileLightConeShadowsCB,
		"faster character shadows":  fasterCharacterShadowsCB,
		"magic effects":             replacementEffectsCB,
		"mobile blending":           animCB,
		"world blending":            pictBlendCB,
	} {
		if control == nil || !control.Disabled {
			t.Errorf("%s control was not greyed out by the shader master", name)
		}
	}
	if motionCB == nil || motionCB.Disabled {
		t.Fatal("CPU motion smoothing was incorrectly disabled with shaders")
	}
	if upscaleModeDD == nil || upscaleModeDD.Disabled {
		t.Fatal("CPU artwork upscaling was incorrectly disabled with shaders")
	}

	contains := func(root *eui.ItemData, target *eui.ItemData) bool {
		var visit func(*eui.ItemData) bool
		visit = func(item *eui.ItemData) bool {
			if item == target {
				return true
			}
			for _, child := range item.Contents {
				if visit(child) {
					return true
				}
			}
			return false
		}
		return visit(root)
	}
	findSection := func(title string, target *eui.ItemData) *eui.ItemData {
		var found *eui.ItemData
		var visit func(*eui.ItemData)
		visit = func(item *eui.ItemData) {
			if found != nil {
				return
			}
			for _, child := range item.Contents {
				if child.Text == title && contains(item, target) {
					found = item
					return
				}
			}
			for _, child := range item.Contents {
				visit(child)
			}
			for _, tab := range item.Tabs {
				visit(tab)
			}
		}
		for _, item := range settingsWin.Contents {
			visit(item)
		}
		return found
	}
	shaderSection := findSection("Lighting & Effects", shaderLightingCB)
	if shaderSection == nil {
		t.Fatal("performance options are missing the Lighting & Effects group")
	}
	for _, control := range []*eui.ItemData{shaderLightingCB, mobileLightConeShadowsCB, replacementEffectsCB} {
		if !contains(shaderSection, control) {
			t.Errorf("shader-backed control %q is outside the Lighting & Effects group", control.Text+control.Label)
		}
	}
	if contains(shaderSection, upscaleModeDD) {
		t.Fatal("CPU artwork upscaling is still inside the Lighting & Effects group")
	}
	artworkSection := findSection("Artwork Scaling", upscaleModeDD)
	if artworkSection == nil {
		t.Fatal("quality window is missing artwork upscaling from Artwork Scaling")
	}
	shadowSection := findSection("Shadows", fasterCharacterShadowsCB)
	if shadowSection == nil {
		t.Fatal("quality window is missing Faster Character Shadows from Shadows")
	}
}
