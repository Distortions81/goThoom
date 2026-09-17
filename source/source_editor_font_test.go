package main

import (
	"path/filepath"
	"testing"

	"gothoom/eui"

	"github.com/hajimehoshi/ebiten/v2/text/v2"
)

func TestEditorFontSizeControlsSharePreferenceAndPreserveDraft(t *testing.T) {
	oldSize, oldDirty := gs.EditorFontSize, settingsDirty
	t.Cleanup(func() { gs.EditorFontSize, settingsDirty = oldSize, oldDirty })
	gs.EditorFontSize = 14
	ed := macroEditorFixture(t)
	filename := filepath.Join(t.TempDir(), "note.txt")
	if err := createEditorFile(filename, []byte("Notes\n")); err != nil {
		t.Fatal(err)
	}
	note := openTextFileEditor(filename, sourceEditorOptions{kind: "Note"})
	if note == nil {
		t.Fatal("note editor did not open")
	}
	before := ed.input.Text
	ed.input.ReplaceText(before + "// draft\n")
	draft := ed.input.Text
	ed.input.CursorPos, ed.input.SelectStart, ed.input.SelectEnd = 3, 1, 3
	settingsDirty = false
	ed.input.OnTextZoom(1)
	if gs.EditorFontSize != 15 || !settingsDirty || ed.input.FontSize != 15 || note.input.FontSize != 15 {
		t.Fatal("font shortcut did not update the shared preference and editors")
	}
	if ed.input.Text != draft || ed.input.CursorPos != 3 || ed.input.SelectStart != 1 || ed.input.SelectEnd != 3 {
		t.Fatal("font shortcut changed the draft or selection")
	}
	if ed.input.Face.(*text.GoTextFace).Size != 17 {
		t.Fatal("font face did not resize")
	}
	slider := newSourceEditorFontSlider()
	slider.Handler.Handle(eui.UIEvent{Type: eui.EventSliderChanged, Value: 23.8})
	if gs.EditorFontSize != 24 || ed.input.FontSize != 24 || slider.Value != 24 {
		t.Fatal("slider did not resize editors in whole steps")
	}
	note.input.OnTextZoom(-1)
	slider.Action()
	if slider.Value != 23 || ed.input.FontSize != 23 {
		t.Fatal("slider did not reflect shortcut zoom")
	}
	clickMacroEditorButton(t, ed.win, "Undo")
	if ed.input.Text != before || ed.dirty() {
		t.Fatal("font changes polluted undo history")
	}
	setSourceEditorFontSize(100)
	if gs.EditorFontSize != 48 || ed.input.FontSize != 48 {
		t.Fatal("upper font-size limit not applied")
	}
	setSourceEditorFontSize(-1)
	if gs.EditorFontSize != 8 || ed.input.FontSize != 8 {
		t.Fatal("lower font-size limit not applied")
	}
}

func TestEditorFontSizeSettingsRoundTrip(t *testing.T) {
	want := gsdef
	want.EditorFontSize = 23
	data, err := marshalSettingsDocument(want)
	if err != nil {
		t.Fatal(err)
	}
	got, err := unmarshalSettingsDocument(data, gsdef)
	if err != nil || got.EditorFontSize != 23 {
		t.Fatalf("font size did not survive reload: %d, %v", got.EditorFontSize, err)
	}
	got, err = unmarshalSettingsDocument([]byte(`{"version":4,"interface":{}}`), gsdef)
	if err != nil || got.EditorFontSize != 11 {
		t.Fatalf("old settings lost default editor size: %d, %v", got.EditorFontSize, err)
	}
	for _, pair := range [][2]int{{-5, 8}, {100, 48}} {
		got.EditorFontSize = pair[0]
		normalizeLoadedNumericSettings(&got)
		if got.EditorFontSize != pair[1] {
			t.Fatal("loaded font size was not bounded")
		}
	}
}
