package eui

import "testing"

func TestNoSurfaceButtonSurvivesStyleChanges(t *testing.T) {
	if err := Init(); err != nil {
		t.Fatal(err)
	}
	isolateThemeTest(t)
	button, _ := NewButton()
	button.NoSurface = true
	if err := LoadStyle("Outline"); err != nil {
		t.Fatal(err)
	}
	if !button.NoSurface {
		t.Fatal("style change restored the button surface")
	}
	if !button.Outlined || button.Border <= 0 {
		t.Fatal("test style does not exercise outlined buttons")
	}
	if itemDrawsOutline(button, button.themeStyle()) {
		t.Fatal("NoSurface button retained its outline")
	}
	button.NoSurface = false
	if !itemDrawsOutline(button, button.themeStyle()) {
		t.Fatal("ordinary outlined button lost its outline")
	}
}
