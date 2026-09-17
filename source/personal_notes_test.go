package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gothoom/eui"
)

func personalNotesFixture(t *testing.T) {
	t.Helper()
	macroEditorFixture(t)
	oldPanel, oldDetails, oldCharacters, oldGS := personalNotes, personalNoteDetailsWindows, characters, gs
	personalNotes, personalNoteDetailsWindows = nil, map[string]*eui.WindowData{}
	characters = []Character{{Name: "Gaia"}, {Name: "Alt"}}
	gs.LastCharacter = "Gaia"
	t.Cleanup(func() {
		for _, win := range personalNoteDetailsWindows {
			win.Close()
		}
		if personalNotes != nil {
			personalNotes.win.Close()
		}
		personalNotes, personalNoteDetailsWindows, characters, gs = oldPanel, oldDetails, oldCharacters, oldGS
	})
}

func TestPersonalNotesScopeTagsAndPersistence(t *testing.T) {
	personalNotesFixture(t)
	global, err := createPersonalNote(" Hunt checklist ", "#Hunt, supplies, hunt, , supplies", "")
	if err != nil {
		t.Fatal(err)
	}
	player, err := createPersonalNote("Training café", "practice", " Gaia ")
	if err != nil {
		t.Fatal(err)
	}
	if global.Subject != "Hunt checklist" || strings.Join(global.Tags, ",") != "Hunt,supplies" {
		t.Fatalf("tags not normalized: %+v", global)
	}
	for _, tc := range []struct {
		note                 *personalNote
		query, player, scope string
		want                 bool
	}{
		{global, "#hunt supplies", "Alt", "Global + Player", true},
		{player, "café", "gaia", "Global + Player", true},
		{player, "", "Alt", "Global + Player", false},
		{player, "", "", "Player only", false},
		{global, "", "Gaia", "Player only", false},
		{player, "practice", "Alt", "All notes", true},
		{player, "", "Gaia", "Global only", false},
		{global, "missing", "Gaia", "Global only", false},
	} {
		if got := personalNoteMatches(tc.note, tc.query, tc.player, tc.scope); got != tc.want {
			t.Fatalf("scope/query mismatch: %+v", tc)
		}
	}
	ed := openPersonalNote(player)
	if ed == nil || !strings.Contains(ed.win.Title, player.Subject) {
		t.Fatal("note editor did not open")
	}
	for _, label := range []string{"Check", "Format", "Save & Reload"} {
		if noteWindowHasText(ed.win, label) {
			t.Fatalf("notes have an unnecessary %s action", label)
		}
	}
	body := "  preserve spacing  \n\nCafé notes\n"
	editMacroForTest(ed, body)
	if !ed.save(false) {
		t.Fatal(ed.message)
	}
	if raw, _ := os.ReadFile(ed.doc.path); string(raw) != body {
		t.Fatal("note formatting changed")
	}
	editMacroForTest(ed, body+"unsaved\n")
	if err := savePersonalNoteDetails(player, "Training plan", "practice, later", ""); err != nil {
		t.Fatal(err)
	}
	if !ed.dirty() || ed.input.Text != body+"unsaved\n" || !strings.Contains(ed.win.Title, "Training plan") {
		t.Fatal("editing details changed the body draft or lost its title")
	}
	if !ed.save(false) {
		t.Fatal("details save conflicted with body save", ed.message)
	}
	reloaded, err := loadPersonalNote(player.id)
	if err != nil || reloaded.Subject != "Training plan" || reloaded.Player != "" || len(reloaded.Tags) != 2 {
		t.Fatal("metadata did not persist", err)
	}
	notes, err := listPersonalNotes()
	if err != nil || len(notes) != 2 {
		t.Fatal("could not list notes", err)
	}
	duplicate, err := createPersonalNote(global.Subject, "", "")
	if err != nil || duplicate.id == global.id {
		t.Fatal("duplicate subjects overwrote a note", err)
	}
}

func TestPersonalNotesConflictsQuitAndTrash(t *testing.T) {
	personalNotesFixture(t)
	note, err := createPersonalNote("Keep me", "", "")
	if err != nil {
		t.Fatal(err)
	}
	ed := openPersonalNote(note)
	editMacroForTest(ed, "unsaved text")
	if !confirmSourceEditorQuit(func() {}) || sourceEditorQuitPrompt == nil {
		t.Fatal("quit did not protect note draft")
	}
	clickMacroEditorButton(t, sourceEditorQuitPrompt, "Keep Editing")
	if err := trashPersonalNote(note); err == nil {
		t.Fatal("trashed an unsaved draft")
	}
	if !ed.save(false) {
		t.Fatal(ed.message)
	}
	if err := trashPersonalNote(note); err != nil {
		t.Fatal(err)
	}
	if _, ok := sourceEditors[ed.doc.path]; ok {
		t.Fatal("trashed note editor remains open")
	}
	if data, err := os.ReadFile(filepath.Join(personalNotesDir(), "Trash", note.id, "text.txt")); err != nil || string(data) != "unsaved text" {
		t.Fatal("Trash lost note contents", err)
	}
	if notes, err := listPersonalNotes(); err != nil || len(notes) != 0 {
		t.Fatal("trashed note is still listed", err)
	}
	note, err = createPersonalNote("External conflict", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(note.details.path, []byte(`{"subject":"external"}`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := savePersonalNoteDetails(note, "overwrite", "", ""); err == nil {
		t.Fatal("overwrote external metadata change")
	}
	for _, id := range []string{"../elsewhere", "note-../../elsewhere", "other"} {
		if _, err := personalNotePath(id, "text.txt"); err == nil {
			t.Fatal("accepted unsafe ID", id)
		}
	}
}

func TestPersonalNotesLibraryAndDetailsUI(t *testing.T) {
	personalNotesFixture(t)
	global, err := createPersonalNote("Global checklist", "travel", "")
	if err != nil {
		t.Fatal(err)
	}
	_, err = createPersonalNote("Gaia training", "practice", "Gaia")
	if err != nil {
		t.Fatal(err)
	}
	_, err = createPersonalNote("Alt training", "practice", "Alt")
	if err != nil {
		t.Fatal(err)
	}
	showPersonalNotes()
	panel := personalNotes
	if !noteWindowHasText(panel.win, "Global checklist") || !noteWindowHasText(panel.win, "Gaia training") || noteWindowHasText(panel.win, "Alt training") {
		t.Fatal("default player filter is wrong")
	}
	panel.win.OnSearch("#travel")
	if !noteWindowHasText(panel.win, "Global checklist") || noteWindowHasText(panel.win, "Gaia training") {
		t.Fatal("tag search did not filter")
	}
	panel.win.OnSearch("")
	panel.scope.Selected = 3
	panel.refreshList()
	if !noteWindowHasText(panel.win, "Alt training") {
		t.Fatal("all notes did not include other players")
	}
	form := openPersonalNoteDetails(global, "Gaia")
	if openPersonalNoteDetails(global, "Gaia") != form {
		t.Fatal("duplicate details dialogs")
	}
	fields := form.Contents[0].Contents
	fields[0].Text = "Renamed note"
	fields[1].Text = "new, #tag"
	fields[2].Handler.Emit(eui.UIEvent{Type: eui.EventCheckboxChanged, Checked: false})
	fields[3].Text = "Alt"
	clickMacroEditorButton(t, form, "Save Details")
	got, err := loadPersonalNote(global.id)
	if err != nil || got.Subject != "Renamed note" || got.Player != "Alt" || strings.Join(got.Tags, ",") != "new,tag" {
		t.Fatal("Details did not save", err)
	}
	form = openPersonalNoteDetails(nil, "Gaia")
	form.Contents[0].Contents[0].Text = "New from UI"
	clickMacroEditorButton(t, form, "Create & Open")
	found := false
	for _, ed := range sourceEditors {
		if ed.options.displayName == "New from UI" {
			found = true
		}
	}
	if !found {
		t.Fatal("new note did not open in the editor")
	}
}

func noteWindowHasText(win *eui.WindowData, text string) bool {
	var walk func([]*eui.ItemData) bool
	walk = func(items []*eui.ItemData) bool {
		for _, item := range items {
			if item.Text == text || walk(item.Contents) {
				return true
			}
		}
		return false
	}
	return walk(win.Contents)
}
