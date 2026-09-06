package main

import (
	"gothoom/eui"
	"testing"
)

func TestColorPickerChangesAreLocalUntilApply(t *testing.T) {
	initFont()
	oldPicker := colorPickerWin
	colorPickerWin = nil
	t.Cleanup(func() {
		if colorPickerWin != nil {
			colorPickerWin.Close()
		}
		colorPickerWin = oldPicker
	})
	accent := eui.AccentColor()
	initial := eui.NewColor(22, 77, 153, 255)
	var applied []eui.Color
	openPicker := func() { openColorPicker("Test color", initial, func(c eui.Color) { applied = append(applied, c) }) }
	var visit func([]*eui.ItemData, string) *eui.ItemData
	visit = func(items []*eui.ItemData, key string) *eui.ItemData {
		for _, item := range items {
			if item.Text == key || item.Label == key || (key == "wheel" && item.ItemType == eui.ITEM_COLORWHEEL) || (key == "value" && item.ItemType == eui.ITEM_INPUT) {
				return item
			}
			if got := visit(item.Contents, key); got != nil {
				return got
			}
		}
		return nil
	}
	button := func(name string) *eui.ItemData {
		it := visit(colorPickerWin.Contents, name)
		if it == nil {
			t.Fatalf("missing %s", name)
		}
		return it
	}
	openPicker()
	button("wheel").OnColorChange(eui.NewColor(230, 50, 80, 255))
	if len(applied) != 0 || eui.AccentColor() != accent {
		t.Fatal("editing picker changed application colors")
	}
	button("Cancel").Handler.Emit(eui.UIEvent{Type: eui.EventClick})
	if len(applied) != 0 || colorPickerWin != nil {
		t.Fatal("cancel committed or left picker open")
	}
	openPicker()
	button("Apply").Handler.Emit(eui.UIEvent{Type: eui.EventClick})
	if len(applied) != 1 || applied[0] != initial {
		t.Fatal("applying unchanged picker altered exact channels")
	}
	openPicker()
	value := button("value")
	value.Handler.Emit(eui.UIEvent{Type: eui.EventInputChanged, Text: "361"})
	if !button("Apply").Disabled {
		t.Fatal("out-of-range hue allowed apply")
	}
	value.Handler.Emit(eui.UIEvent{Type: eui.EventInputChanged, Text: "180"})
	button("Apply").Handler.Emit(eui.UIEvent{Type: eui.EventClick})
	_, s, v, a := initial.HSVA()
	if len(applied) != 2 || applied[1] != eui.NewColorHSV(180, s, v, a) {
		t.Fatal("HSV input did not apply the chosen color")
	}
}
