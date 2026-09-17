package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gothoom/eui"
)

func TestJSONColorSwatchesRecognizeOnlyWholeValues(t *testing.T) {
	value := `{"clé":"#ABCDEF","alpha":"#11223344","#ffffff":"name","text":"try #000000","escape":"\"#ffffff\"","bad":"#12xy56","short":"#fff"}`
	spans := jsonColorSwatches(value)
	if len(spans) != 2 {
		t.Fatalf("swatches = %+v", spans)
	}
	runes := []rune(value)
	if string(runes[spans[0].Start:spans[0].End]) != "#ABCDEF" || spans[0].Color != eui.NewColor(171, 205, 239, 255) || spans[1].Color != eui.NewColor(17, 34, 51, 68) {
		t.Fatal("Unicode offsets or color channels changed")
	}
	if len(jsonColorSwatches(`{"unfinished":"#123456`)) != 0 {
		t.Fatal("unfinished value became a swatch")
	}
}

func TestEditorColorSwatchEditsAreUndoableAndDraftOnly(t *testing.T) {
	macroEditorFixture(t)
	oldPicker := colorPickerWin
	colorPickerWin = nil
	t.Cleanup(func() {
		if colorPickerWin != nil {
			colorPickerWin.Close()
		}
		colorPickerWin = oldPicker
	})
	filename := filepath.Join(t.TempDir(), "palette.json")
	before := "{\"clé\":\"#ABCDEF\",\"other\":\"#11223344\"}\n"
	if err := createEditorFile(filename, []byte(before)); err != nil {
		t.Fatal(err)
	}
	ed := openTextFileEditor(filename, sourceEditorOptions{kind: "Color Theme", colorSwatches: true, highlight: highlightJSONSource})
	span := jsonColorSwatches(before)[0]
	ed.editColorSwatch(span)
	if colorPickerWin == nil {
		t.Fatal("swatch did not open the picker")
	}
	clickMacroEditorButton(t, colorPickerWin, "Cancel")
	if ed.input.Text != before || ed.dirty() {
		t.Fatal("Cancel modified the draft")
	}
	ed.applyColorSwatch(before, span, span.Color)
	if ed.input.Text != before || ed.input.CanUndo() {
		t.Fatal("unchanged color normalized the literal")
	}
	ed.editColorSwatch(span)
	var changePicker func([]*eui.ItemData) bool
	changePicker = func(items []*eui.ItemData) bool {
		for _, item := range items {
			if item.ItemType == eui.ITEM_COLORWHEEL {
				item.OnColorChange(eui.ColorBlack)
				return true
			}
			if changePicker(item.Contents) {
				return true
			}
		}
		return false
	}
	if !changePicker(colorPickerWin.Contents) {
		t.Fatal("picker has no color wheel")
	}
	clickMacroEditorButton(t, colorPickerWin, "Apply")
	if !ed.dirty() || !strings.Contains(ed.input.Text, `"#000000"`) || !strings.Contains(ed.input.Text, `"#11223344"`) {
		t.Fatal("picker did not edit just the selected literal", ed.input.Text)
	}
	if raw, _ := os.ReadFile(filename); string(raw) != before {
		t.Fatal("picker saved without asking")
	}
	ed.input.Undo()
	if ed.input.Text != before || ed.dirty() {
		t.Fatal("Undo failed to restore exact original source")
	}
	ed.applyColorSwatch(before, span, eui.NewColor(10, 20, 30, 40))
	if !strings.Contains(ed.input.Text, `"#0a141e28"`) {
		t.Fatal("transparent color lost alpha")
	}
	changed := ed.input.Text
	ed.applyColorSwatch(before, span, eui.NewColor(0, 0, 0, 255))
	if ed.input.Text != changed || !strings.Contains(ed.message, "draft changed") {
		t.Fatal("stale picker overwrote a newer draft")
	}
}
