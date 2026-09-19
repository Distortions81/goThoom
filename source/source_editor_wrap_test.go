package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gothoom/eui"
)

func TestEditorWordWrapPreferencePreservesDraftAndSavedFile(t *testing.T) {
	oldWrap, oldDirty := gs.EditorWordWrap, settingsDirty
	t.Cleanup(func() { gs.EditorWordWrap, settingsDirty = oldWrap, oldDirty })
	gs.EditorWordWrap = true
	ed := macroEditorFixture(t)
	path := filepath.Join(t.TempDir(), "song.tune")
	original := "@120 " + strings.Repeat("c4 d4 ", 80) + "\n"
	if err := os.WriteFile(path, []byte(original), 0600); err != nil {
		t.Fatal(err)
	}
	song := openTextFileEditor(path, sourceEditorOptions{kind: "Tune"})
	if !ed.input.WordWrap || !song.input.WordWrap || !song.wordWrap.Checked {
		t.Fatal("wrapping did not start enabled")
	}
	song.input.ReplaceText(original + "p4")
	song.input.CursorPos, song.input.SelectStart, song.input.SelectEnd = 11, 5, 11
	for _, enabled := range []bool{false, true} {
		song.wordWrap.Handler.Emit(eui.UIEvent{Type: eui.EventCheckboxChanged, Checked: enabled})
		if !settingsDirty || gs.EditorWordWrap != enabled || ed.input.WordWrap != enabled || song.input.WordWrap != enabled || ed.wordWrap.Checked != enabled {
			t.Fatal("wrap did not update editors and preference")
		}
		if song.input.Text != original+"p4" || song.input.CursorPos != 11 || song.input.SelectStart != 5 || song.input.SelectEnd != 11 {
			t.Fatal("toggle modified draft or selection")
		}
	}
	song.input.Undo()
	if song.input.Text != original || song.dirty() {
		t.Fatal("wrap changed undo history")
	}
	if !song.save(false) {
		t.Fatal("save failed")
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != original {
		t.Fatal("wrapping added physical line breaks")
	}
}

func TestEditorWordWrapSettingsRoundTrip(t *testing.T) {
	for _, enabled := range []bool{true, false} {
		want := gsdef
		want.EditorWordWrap = enabled
		data, err := marshalSettingsDocument(want)
		if err != nil {
			t.Fatal(err)
		}
		got, err := unmarshalSettingsDocument(data, gsdef)
		if err != nil || got.EditorWordWrap != enabled {
			t.Fatalf("wrap preference lost: %v", err)
		}
	}
	got, err := unmarshalSettingsDocument([]byte(`{"version":4,"interface":{}}`), gsdef)
	if err != nil || !got.EditorWordWrap {
		t.Fatal("older settings did not default to word wrap")
	}
}
