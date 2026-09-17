package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gothoom/eui"

	"golang.org/x/text/encoding/charmap"
)

func macroDocumentFixture(t *testing.T, raw []byte) *sourceDocument {
	t.Helper()
	oldDir := dataDirPath
	dataDirPath = t.TempDir()
	t.Cleanup(func() { dataDirPath = oldDir })
	if err := os.MkdirAll(legacyMacrosDir(), 0755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(legacyMacrosDir(), "example.mac")
	if err := os.WriteFile(path, raw, 0640); err != nil {
		t.Fatal(err)
	}
	doc, err := loadLegacyMacroDocument(path)
	if err != nil {
		t.Fatal(err)
	}
	return doc
}

func TestMacroEditorSearchSelectsMatchesWithoutChangingDraft(t *testing.T) {
	ed := macroEditorFixture(t)
	editMacroForTest(ed, "\"/hello\" \"Hello there\\r\"\n")
	before := ed.input.Text
	if !ed.win.Searchable || ed.win.OnSearch == nil || ed.win.OnSearchNext == nil {
		t.Fatal("macro editor has no window search")
	}
	ed.win.SearchText = "hello"
	ed.win.OnSearch("hello")
	if ed.input.SelectedText() != "hello" {
		t.Fatal("initial search did not select the first match")
	}
	ed.win.OnSearchNext(false)
	if ed.input.SelectedText() != "Hello" {
		t.Fatal("next did not advance to the second match")
	}
	ed.win.OnSearchNext(true)
	if ed.input.SelectedText() != "hello" {
		t.Fatal("previous did not return to the first match")
	}
	ed.win.OnSearch("missing")
	if !strings.Contains(ed.status.Text, "No matches") || ed.input.Text != before {
		t.Fatal("search failed to report no matches or changed the draft")
	}
}

func TestMacroEditorToolbarHistory(t *testing.T) {
	ed := macroEditorFixture(t)
	if ed.root.Contents[0] != ed.toolbar || ed.root.Contents[1] != ed.input {
		t.Fatal("toolbar does not sit directly above the editor")
	}
	if !ed.undoButton.Disabled || !ed.redoButton.Disabled {
		t.Fatal("new editor has enabled history actions")
	}
	before := ed.input.Text
	ed.input.ReplaceText(before + "f2 \"/wave\\r\"\n")
	after := ed.input.Text
	if ed.undoButton.Disabled || !ed.redoButton.Disabled {
		t.Fatal("typing did not enable Undo")
	}
	clickMacroEditorButton(t, ed.win, "Undo")
	if ed.input.Text != before || ed.dirty() || !ed.undoButton.Disabled || ed.redoButton.Disabled || !ed.input.Focused {
		t.Fatal("toolbar Undo did not restore the saved draft and focus")
	}
	clickMacroEditorButton(t, ed.win, "Redo")
	if ed.input.Text != after || !ed.dirty() || ed.undoButton.Disabled || !ed.redoButton.Disabled || !ed.input.Focused {
		t.Fatal("toolbar Redo did not restore the edit and focus")
	}
	clickMacroEditorButton(t, ed.win, "Undo")
	ed.input.ReplaceText(before + "f3 \"/look\\r\"\n")
	if !ed.redoButton.Disabled {
		t.Fatal("new edit retained stale Redo button")
	}
}

func TestMacroDocumentPreservesEncodingAndNewlines(t *testing.T) {
	for _, newline := range []string{"\n", "\r\n", "\r"} {
		for _, encoding := range []string{"utf8", "bom", "macroman"} {
			t.Run(encoding+"/"+strings.ReplaceAll(strings.ReplaceAll(newline, "\r", "CR"), "\n", "LF"), func(t *testing.T) {
				original := "// café" + newline + "f1 \"/look\\r\"" + newline
				raw := []byte(original)
				if encoding == "bom" {
					raw = append([]byte{0xef, 0xbb, 0xbf}, raw...)
				}
				if encoding == "macroman" {
					var err error
					raw, err = charmap.Macintosh.NewEncoder().Bytes(raw)
					if err != nil {
						t.Fatal(err)
					}
				}
				doc := macroDocumentFixture(t, raw)
				if strings.Contains(doc.savedText, "\r") {
					t.Fatal("editor text was not normalized")
				}
				if err := doc.save(doc.savedText); err != nil {
					t.Fatal(err)
				}
				got, _ := os.ReadFile(doc.path)
				if !bytes.Equal(got, raw) {
					t.Fatal("unchanged save rewrote file")
				}
				draft := strings.Replace(doc.savedText, "/look", "/wave", 1)
				if err := doc.save(draft); err != nil {
					t.Fatal(err)
				}
				got, _ = os.ReadFile(doc.path)
				want := bytes.Replace(raw, []byte("/look"), []byte("/wave"), 1)
				if !bytes.Equal(got, want) {
					t.Fatalf("save changed encoding or newline style: %q", got)
				}
				if doc.changed(draft) {
					t.Fatal("saved draft remains dirty")
				}
				info, _ := os.Stat(doc.path)
				if info.Mode().Perm() != 0640 {
					t.Fatal("permissions changed")
				}
				if err := doc.save(draft + "// another save\n"); err != nil {
					t.Fatal("second save", err)
				}
			})
		}
	}
}

func TestMacroDocumentRejectsConflictsAndUnencodableText(t *testing.T) {
	raw, _ := charmap.Macintosh.NewEncoder().Bytes([]byte("// café\nf1 \"/look\\r\"\n"))
	doc := macroDocumentFixture(t, raw)
	if err := doc.save(doc.savedText + "// 👩‍💻\n"); err == nil {
		t.Fatal("unencodable edit accepted")
	}
	got, _ := os.ReadFile(doc.path)
	if !bytes.Equal(got, raw) {
		t.Fatal("encoding failure changed file")
	}
	external := []byte("// external change\n")
	if err := os.WriteFile(doc.path, external, 0640); err != nil {
		t.Fatal(err)
	}
	if err := doc.save(doc.savedText + "// edit\n"); err == nil {
		t.Fatal("overwrote external edit")
	}
	got, _ = os.ReadFile(doc.path)
	if !bytes.Equal(got, external) {
		t.Fatal("external change lost")
	}
}

func TestMacroDocumentChecksDraftAndIncludesWithoutWriting(t *testing.T) {
	doc := macroDocumentFixture(t, []byte("f1 \"/look\\r\"\n"))
	include := filepath.Join(filepath.Dir(doc.path), "shared.mac")
	if err := os.WriteFile(include, []byte("f2 \"/wave\\r\"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	program := doc.check("include \"shared.mac\"\nf3 \"/bow\\r\"\n")
	if err := program.err(); err != nil {
		t.Fatal(err)
	}
	if len(program.Macros) != 2 {
		t.Fatal("draft/include not checked", len(program.Macros))
	}
	program = doc.check("include \"missing.mac\"\n")
	if len(program.Diagnostics) == 0 {
		t.Fatal("missing include not reported")
	}
	got, _ := os.ReadFile(doc.path)
	if !bytes.Equal(got, doc.original) {
		t.Fatal("check wrote draft")
	}
}

func TestMacroDocumentSavesThroughSymlinkAndKeepsMacroIncludeRules(t *testing.T) {
	doc := macroDocumentFixture(t, []byte("f1 \"/look\\r\"\n"))
	alias := filepath.Join(t.TempDir(), "alias.mac")
	if err := os.Symlink(doc.path, alias); err != nil {
		t.Skip(err)
	}
	linked, err := loadLegacyMacroDocument(alias)
	if err != nil {
		t.Fatal(err)
	}
	include := filepath.Join(filepath.Dir(doc.path), "shared.mac")
	if err := os.WriteFile(include, []byte("f2 \"/wave\\r\"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	draft := "include \"shared.mac\"\n"
	program := linked.check(draft)
	if err := program.err(); err != nil {
		t.Fatal("relative include", err)
	}
	if err := linked.save(draft); err != nil {
		t.Fatal(err)
	}
	info, _ := os.Lstat(alias)
	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatal("replaced symlink")
	}
	got, _ := os.ReadFile(doc.path)
	if string(got) != draft {
		t.Fatal("target not updated")
	}
}

func macroEditorFixture(t *testing.T) *sourceEditor {
	t.Helper()
	initFont()
	oldEditors, oldPrompt, oldQuit := sourceEditors, sourceEditorQuitPrompt, sourceEditorQuitRequested
	oldDir, oldSessions := dataDirPath, appSessions
	oldWin, oldList := legacyMacroLibraryWin, legacyMacroLibraryList
	oldW, oldH := eui.ScreenSize()
	oldScale := eui.UIScale()
	sourceEditors = map[string]*sourceEditor{}
	sourceEditorQuitPrompt = nil
	sourceEditorQuitRequested = false
	legacyMacroLibraryWin = nil
	legacyMacroLibraryList = nil
	dataDirPath = t.TempDir()
	appSessions = newSessionManager(mustNewSession(primarySessionID))
	eui.SetScreenSize(1920, 1080)
	eui.SetUIScale(1)
	t.Cleanup(func() {
		if sourceEditorQuitPrompt != nil {
			sourceEditorQuitPrompt.Close()
		}
		for _, ed := range sourceEditors {
			ed.discard = true
			ed.win.Close()
		}
		sourceEditors, sourceEditorQuitPrompt, sourceEditorQuitRequested = oldEditors, oldPrompt, oldQuit
		dataDirPath, appSessions = oldDir, oldSessions
		legacyMacroLibraryWin, legacyMacroLibraryList = oldWin, oldList
		eui.SetScreenSize(oldW, oldH)
		eui.SetUIScale(oldScale)
	})
	if err := os.MkdirAll(legacyMacroLibraryPath(), 0755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(legacyMacroLibraryPath(), "edit.mac")
	if err := os.WriteFile(path, []byte("// Name: Example\nf1 \"/look\\r\"\n"), 0640); err != nil {
		t.Fatal(err)
	}
	ed := openLegacyMacroEditor(legacyMacroLibraryEntry{ID: "edit.mac", Name: "Example", Path: path})
	if ed == nil {
		t.Fatal("editor did not open")
	}
	return ed
}

func editMacroForTest(ed *sourceEditor, value string) {
	ed.input.Text = value
	ed.input.Handler.Emit(eui.UIEvent{Type: eui.EventInputChanged, Item: ed.input, Text: value})
}

func clickMacroEditorButton(t *testing.T, win *eui.WindowData, label string) {
	t.Helper()
	var find func([]*eui.ItemData) bool
	find = func(items []*eui.ItemData) bool {
		for _, item := range items {
			if item.ItemType == eui.ITEM_BUTTON && item.Text == label {
				item.Handler.Emit(eui.UIEvent{Type: eui.EventClick, Item: item})
				return true
			}
			if find(item.Contents) {
				return true
			}
			if len(item.Tabs) > 0 && find(item.Tabs[item.ActiveTab].Contents) {
				return true
			}
		}
		return false
	}
	if win == nil || !find(win.Contents) {
		t.Fatal("missing button", label)
	}
}

func TestMacroEditorKeepsDraftAndConfirmsClose(t *testing.T) {
	ed := macroEditorFixture(t)
	draft := ed.input.Text + "f2 \"/wave\\r\"\n"
	editMacroForTest(ed, draft)
	if !ed.dirty() || ed.saveButton.Disabled || !strings.HasSuffix(ed.win.Title, " *") {
		t.Fatal("draft state not visible")
	}
	same := openLegacyMacroEditor(legacyMacroLibraryEntry{Path: ed.doc.path})
	if same != ed || same.input.Text != draft {
		t.Fatal("opening twice lost draft")
	}
	ed.win.Close()
	if !ed.win.IsOpen() || ed.confirm == nil {
		t.Fatal("unsaved editor closed")
	}
	clickMacroEditorButton(t, ed.confirm, "Keep Editing")
	if !ed.win.IsOpen() || !ed.dirty() {
		t.Fatal("cancel lost draft")
	}
	ed.win.Close()
	clickMacroEditorButton(t, ed.confirm, "Save & Close")
	if ed.win.IsOpen() || len(sourceEditors) != 0 {
		t.Fatal("saved editor remains open")
	}
	got, _ := os.ReadFile(ed.doc.path)
	if string(got) != draft {
		t.Fatal("save and close did not save")
	}
}

func TestMacroEditorDiscardAndFailedSaveKeepDiskSafe(t *testing.T) {
	ed := macroEditorFixture(t)
	original := ed.input.Text
	editMacroForTest(ed, original+"// draft\n")
	external := original + "// external\n"
	if err := os.WriteFile(ed.doc.path, []byte(external), 0640); err != nil {
		t.Fatal(err)
	}
	ed.win.Close()
	clickMacroEditorButton(t, ed.confirm, "Save & Close")
	if !ed.win.IsOpen() || !ed.dirty() || !strings.Contains(ed.message, "Not saved") {
		t.Fatal("failed save lost draft")
	}
	ed.win.Close()
	clickMacroEditorButton(t, ed.confirm, "Discard")
	got, _ := os.ReadFile(ed.doc.path)
	if string(got) != external {
		t.Fatal("discard touched disk")
	}
}

func TestMacroEditorSaveAndReloadChecksBeforeWriting(t *testing.T) {
	ed := macroEditorFixture(t)
	editMacroForTest(ed, "f1 \"unterminated\n")
	if ed.save(true) {
		t.Fatal("invalid draft reloaded")
	}
	got, _ := os.ReadFile(ed.doc.path)
	if !bytes.Equal(got, ed.doc.original) {
		t.Fatal("invalid reload wrote file")
	}
	if !ed.save(false) || ed.dirty() {
		t.Fatal("ordinary Save must allow work in progress")
	}
	if err := os.WriteFile(legacyMacroLibrarySelectionPath(), []byte(`{"global":["edit.mac"]}`), 0640); err != nil {
		t.Fatal(err)
	}
	editMacroForTest(ed, "f3 \"/bow\\r\"\n")
	if !ed.save(true) {
		t.Fatal(ed.message)
	}
	program := selectedAppSession().legacyMacroProgramSnapshot()
	found := false
	for _, macro := range program.Macros {
		if macro.Trigger == "f3" {
			found = true
		}
	}
	if !found {
		t.Fatal("saved macro did not reach selected session")
	}
}

func TestMacroEditorQuitUsesCurrentDraftAndStopsOnSaveFailure(t *testing.T) {
	ed := macroEditorFixture(t)
	editMacroForTest(ed, ed.input.Text+"// first\n")
	quit := false
	if !confirmSourceEditorQuit(func() { quit = true }) {
		t.Fatal("dirty quit was not guarded")
	}
	editMacroForTest(ed, ed.input.Text+"// edited behind popup\n")
	clickMacroEditorButton(t, sourceEditorQuitPrompt, "Save All & Quit")
	got, _ := os.ReadFile(ed.doc.path)
	if !quit || string(got) != ed.input.Text {
		t.Fatal("quit did not save current draft")
	}
	quit = false
	editMacroForTest(ed, ed.input.Text+"// another draft\n")
	if err := os.WriteFile(ed.doc.path, []byte("// external\n"), 0640); err != nil {
		t.Fatal(err)
	}
	confirmSourceEditorQuit(func() { quit = true })
	clickMacroEditorButton(t, sourceEditorQuitPrompt, "Save All & Quit")
	if quit || !ed.dirty() {
		t.Fatal("quit discarded failed save")
	}
}

func TestMacroEditorReservesGameHotkeysWhileTyping(t *testing.T) {
	ed := macroEditorFixture(t)
	if !typingInUI() || !multilineEditorFocused() {
		t.Fatal("macro editor did not claim editing keys")
	}
	eui.ClearFocus(ed.input)
	if multilineEditorFocused() {
		t.Fatal("unfocused editor still claims keys")
	}
}
