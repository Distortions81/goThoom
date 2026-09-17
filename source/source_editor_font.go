package main

import (
	"math"

	"gothoom/eui"
)

func newSourceEditorFontSlider() *eui.ItemData {
	slider, events := eui.NewSlider()
	slider.Label = "Editor Text Size"
	slider.MinValue, slider.MaxValue = 8, 48
	slider.IntOnly = true
	slider.Value = float32(sourceEditorFontSize())
	slider.Size = eui.Point{X: 400, Y: settingsControlHeight}
	slider.SetTooltip("Text size shared by macro, Go script, TTS, theme/style, and personal note editors.")
	events.Handle = func(event eui.UIEvent) {
		if event.Type == eui.EventSliderChanged {
			setSourceEditorFontSize(int(math.Round(float64(event.Value))))
			slider.Value = float32(sourceEditorFontSize())
		}
	}
	slider.Action = func() {
		if size := float32(sourceEditorFontSize()); slider.Value != size {
			slider.Value, slider.Dirty = size, true
		}
	}
	return slider
}

func sourceEditorFontSize() int {
	if gs.EditorFontSize == 0 {
		return 11
	}
	return max(8, min(gs.EditorFontSize, 48))
}

func changeSourceEditorFontSize(steps int) {
	setSourceEditorFontSize(sourceEditorFontSize() + steps)
}

func setSourceEditorFontSize(size int) {
	size = max(8, min(size, 48))
	if size == sourceEditorFontSize() {
		return
	}
	gs.EditorFontSize = size
	settingsDirty = true
	for _, ed := range sourceEditors {
		ed.layout()
	}
}
