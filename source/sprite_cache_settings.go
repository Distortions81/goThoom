package main

import (
	"fmt"
	"strings"

	text "github.com/hajimehoshi/ebiten/v2/text/v2"
	"gothoom/eui"
)

var spriteCachePresets = []struct {
	name, explanation string
	baseMiB           int
}{
	{"Minimal", "Leaves the most room for other apps. Artwork may reload more often.", 128},
	{"Compact", "Keeps a smaller cache while reducing artwork reloads.", 256},
	{"Balanced", "Recommended. Keeps recently used artwork ready to reduce loading pauses.", 512},
	{"Generous", "Keeps more artwork ready when revisiting areas.", 1024},
	{"Maximum", "Keeps the largest reserve for long sessions and frequent area changes.", 2048},
}

func spriteCacheExplanation(baseMiB int) string {
	explanation := "Uses your custom reserve from the configuration."
	for _, preset := range spriteCachePresets {
		if preset.baseMiB == baseMiB {
			explanation = preset.explanation
			break
		}
	}
	format := func(mib int) string {
		if mib%1024 == 0 {
			return fmt.Sprintf("%d GiB", mib/1024)
		}
		return fmt.Sprintf("%d MiB", mib)
	}
	return fmt.Sprintf("%s\n\nScales with sprite resolution:\n2×: %s; 3×: %s; 4×: %s.\nActual memory use can be higher. Restart to rebuild the reserve.",
		explanation, format(scaledSpriteCacheMiB(baseMiB, 2)), format(scaledSpriteCacheMiB(baseMiB, 3)), format(scaledSpriteCacheMiB(baseMiB, 4)))
}

func newSpriteCacheControls(width float32) (*eui.ItemData, *eui.ItemData) {
	selector, events := eui.NewDropdown()
	selector.Label = "Sprite cache"
	selector.Size = eui.Point{X: width, Y: 24}
	selector.SetTooltip("Choose how much artwork to keep ready. The reserve automatically scales with sprite resolution.")
	current := spriteCacheMiB(gs.SpriteCacheMiB)
	selector.Selected = -1
	var values []int
	for index, preset := range spriteCachePresets {
		values = append(values, preset.baseMiB)
		selector.Options = append(selector.Options, preset.name)
		if preset.baseMiB == current {
			selector.Selected = index
		}
	}
	if selector.Selected < 0 {
		selector.Selected = len(values)
		selector.Options = append(selector.Options, "Custom")
		values = append(values, current)
	}
	explanation, _ := eui.NewText()
	explanation.FontSize = 10
	refreshExplanation := func(value int) {
		scale := max(float32(0.1), eui.UIScale())
		fontSize := explanation.FontSize*scale + 2
		face := &text.GoTextFace{Source: eui.FontSource(), Size: float64(fontSize)}
		_, lines := eui.WrapText(spriteCacheExplanation(value), face, float64((width-4)*scale))
		explanation.Text = strings.Join(lines, "\n")
		explanation.Size = eui.Point{X: width, Y: (fontSize*1.2*float32(len(lines)) + 4) / scale}
		explanation.Dirty = true
		if explanation.ParentWindow != nil {
			explanation.ParentWindow.Refresh()
		}
	}
	refreshExplanation(current)
	events.Handle = func(ev eui.UIEvent) {
		if ev.Type != eui.EventDropdownSelected || ev.Index < 0 || ev.Index >= len(values) {
			return
		}
		gs.SpriteCacheMiB = values[ev.Index]
		settingsDirty = true
		refreshExplanation(gs.SpriteCacheMiB)
	}
	return selector, explanation
}
