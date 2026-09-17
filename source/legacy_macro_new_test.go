package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gothoom/eui"
)

func TestNewLegacyMacroFilename(t *testing.T) {
	for input, want := range map[string]string{
		"hello": "hello.mac", " hello.mac ": "hello.mac", "My Macros.txt": "My Macros.txt", "café.MAC": "café.MAC",
	} {
		got, err := newLegacyMacroFilename(input)
		if err != nil || got != want {
			t.Fatalf("filename %q = %q, %v; want %q", input, got, err, want)
		}
	}
	for _, input := range []string{"", " ", ".", "..", ".mac", "../escape.mac", `..\escape.mac`, "/tmp/escape.mac", "bad\nname", "bad\x00name", "bad:name", "trailing.", "wrong.go", "CON.mac", "lpt1", "com¹.mac", strings.Repeat("a", 181)} {
		if _, err := newLegacyMacroFilename(input); err == nil {
			t.Errorf("accepted invalid filename %q", input)
		}
	}
}

func TestCreateLegacyMacroDoesNotOverwriteOrEnable(t *testing.T) {
	oldDir := dataDirPath
	dataDirPath = t.TempDir()
	t.Cleanup(func() { dataDirPath = oldDir })
	entry, err := createLegacyMacro("My macro")
	if err != nil {
		t.Fatal(err)
	}
	if entry.ID != "My macro.mac" || entry.Path != filepath.Join(legacyMacroLibraryPath(), entry.ID) {
		t.Fatalf("unexpected entry: %+v", entry)
	}
	doc, err := loadLegacyMacroDocument(entry.Path)
	if err != nil || doc.savedText != "" {
		t.Fatalf("new document = %+v, %v", doc, err)
	}
	if err := doc.save("f1 \"/look\\r\"\n"); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"My macro.mac", "my MACRO.mac", "KEYS.mac"} {
		if _, err := createLegacyMacro(name); err == nil {
			t.Errorf("accepted existing or bundled name %q", name)
		}
	}
	got, err := os.ReadFile(entry.Path)
	if err != nil || string(got) != "f1 \"/look\\r\"\n" {
		t.Fatalf("existing file changed: %q, %v", got, err)
	}
	if _, err := os.Stat(legacyMacroLibrarySelectionPath()); !os.IsNotExist(err) {
		t.Fatalf("creation wrote macro selections: %v", err)
	}
	entries, err := legacyMacroLibraryEntries()
	if err != nil {
		t.Fatal(err)
	}
	if found := legacyMacroLibraryEntryForTest(entries, entry.ID); found.Path != entry.Path || found.Bundled {
		t.Fatalf("new macro not discoverable as a user macro: %+v", found)
	}
}

func TestNewLegacyMacroWindowCreatesAndOpensEditor(t *testing.T) {
	macroEditorFixture(t)
	oldWin := newLegacyMacroWin
	newLegacyMacroWin = nil
	t.Cleanup(func() {
		if newLegacyMacroWin != nil {
			newLegacyMacroWin.Close()
		}
		newLegacyMacroWin = oldWin
	})
	win := openNewLegacyMacroWindow()
	if openNewLegacyMacroWindow() != win {
		t.Fatal("opened a second creation dialog")
	}
	name := win.Contents[0].Contents[0]
	name.Text = "edit.mac"
	clickMacroEditorButton(t, win, "Create")
	if !win.Open || len(sourceEditors) != 1 || win.Contents[0].Contents[2].Text == "" {
		t.Fatal("duplicate name was not reported in the dialog")
	}
	name.Text = "fresh"
	clickMacroEditorButton(t, win, "Create")
	path := filepath.Join(legacyMacroLibraryPath(), "fresh.mac")
	ed := sourceEditors[path]
	if ed == nil || !ed.input.Focused || ed.input.Text != "" || win.Open || newLegacyMacroWin != nil {
		t.Fatal("new macro did not open in a focused editor and close the dialog")
	}
	ed.input.ReplaceText("f2 \"/wave\\r\"\n")
	clickMacroEditorButton(t, ed.win, "Save")
	if raw, err := os.ReadFile(path); err != nil || string(raw) != ed.input.Text {
		t.Fatalf("new macro could not be saved: %q, %v", raw, err)
	}
	win = openNewLegacyMacroWindow()
	name = win.Contents[0].Contents[0]
	name.Text = "../outside"
	name.Handler.Emit(eui.UIEvent{Type: eui.EventInputChanged, Text: name.Text})
	if !win.DefaultButton.Disabled {
		t.Fatal("invalid filename did not disable Create")
	}
	name.Text = "cancelled.mac"
	clickMacroEditorButton(t, win, "Cancel")
	if _, err := os.Stat(filepath.Join(legacyMacroLibraryPath(), name.Text)); !os.IsNotExist(err) {
		t.Fatalf("cancel created a file: %v", err)
	}
}
