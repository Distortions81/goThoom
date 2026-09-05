package main

import (
	"strings"
	"testing"

	text "github.com/hajimehoshi/ebiten/v2/text/v2"
	"gothoom/eui"
)

func TestSpriteCacheReserveScalesWithTextureArea(t *testing.T) {
	for _, tc := range []struct{ base, factor, want int }{
		{512, 1, 128}, {512, 2, 512}, {512, 3, 1152}, {512, 4, 2048},
		{0, 3, 1152}, {256, 3, 576}, {1024, 3, 2304}, {2048, 4, 8192},
		{8192, 4, 8192}, // Existing custom values respect the maximum effective reserve.
	} {
		if got := scaledSpriteCacheMiB(tc.base, tc.factor); got != tc.want {
			t.Errorf("base=%d factor=%d: got %d MiB, want %d", tc.base, tc.factor, got, tc.want)
		}
	}
}

func TestSpriteCachePresetSelectionUpdatesExplanation(t *testing.T) {
	original, originalDirty, originalScale := gs, settingsDirty, eui.UIScale()
	t.Cleanup(func() { gs, settingsDirty = original, originalDirty; eui.SetUIScale(originalScale) })
	gs = gsdef
	initFont()
	for _, scale := range []float32{1, 2} {
		eui.SetUIScale(scale)
		gs.SpriteCacheMiB = 512
		selector, explanation := newSpriteCacheControls(250)
		if selector.Options[selector.Selected] != "Balanced" {
			t.Fatal("default does not select Balanced")
		}
		if !strings.Contains(strings.Join(strings.Fields(explanation.Text), " "), "3×: 1152 MiB") {
			t.Fatal("explanation omits the 3x reserve")
		}
		for index, preset := range spriteCachePresets {
			selector.Handler.Emit(eui.UIEvent{Type: eui.EventDropdownSelected, Index: index})
			if gs.SpriteCacheMiB != preset.baseMiB || !settingsDirty {
				t.Fatalf("%s did not update the saved preference", preset.name)
			}
			want := strings.Join(strings.Fields(spriteCacheExplanation(preset.baseMiB)), " ")
			if got := strings.Join(strings.Fields(explanation.Text), " "); got != want {
				t.Fatalf("%s left a stale explanation", preset.name)
			}
			fontSize := explanation.FontSize*scale + 2
			face := &text.GoTextFace{Source: eui.FontSource(), Size: float64(fontSize)}
			lines := strings.Split(explanation.Text, "\n")
			for _, line := range lines {
				width, _ := text.Measure(line, face, 0)
				if width > float64(explanation.Size.X*scale) {
					t.Fatalf("explanation clips horizontally at UI scale %v: %s", scale, line)
				}
			}
			if fontSize*1.2*float32(len(lines)) > explanation.Size.Y*scale {
				t.Fatal("explanation clips vertically")
			}
		}
	}
	gs.SpriteCacheMiB = 768
	selector, _ := newSpriteCacheControls(250)
	if selector.Options[selector.Selected] != "Custom" || gs.SpriteCacheMiB != 768 {
		t.Fatal("opening the selector overwrote a custom configured reserve")
	}
}
