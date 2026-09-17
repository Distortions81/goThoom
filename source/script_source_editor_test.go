package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const editorScriptSource = "package main\n\nconst scriptID = \"editor-test\"\nconst scriptName = \"Editor Test\"\n\nvar Version = \"one\"\n\nfunc Init() {}\n"

func scriptSourceEditorFixture(t *testing.T, folder bool) *sourceEditor {
	t.Helper()
	macroEditorFixture(t) // Also isolates the shared windows and quit guard.
	oldSettings, oldPackages := gs, scriptPackages
	oldPaths, oldNames, oldAuthors := scriptPaths, scriptDisplayNames, scriptAuthors
	oldCats, oldSubs, oldDescriptions, oldVersions := scriptCategories, scriptSubCategories, scriptDescriptions, scriptAPIVersions
	oldWin, oldInfo := scriptsWin, scriptInfoWin
	gs.ScriptsPath = t.TempDir()
	scriptsWin, scriptInfoWin = nil, nil
	scriptPaths, scriptDisplayNames, scriptAuthors = map[string]string{}, map[string]string{}, map[string]string{}
	scriptCategories, scriptSubCategories, scriptDescriptions, scriptAPIVersions = map[string]string{}, map[string]string{}, map[string]string{}, map[string]int{}
	t.Cleanup(func() {
		gs, scriptPackages = oldSettings, oldPackages
		scriptPaths, scriptDisplayNames, scriptAuthors = oldPaths, oldNames, oldAuthors
		scriptCategories, scriptSubCategories, scriptDescriptions, scriptAPIVersions = oldCats, oldSubs, oldDescriptions, oldVersions
		scriptsWin, scriptInfoWin = oldWin, oldInfo
	})
	path := filepath.Join(gs.ScriptsPath, "editor.go")
	if folder {
		path = filepath.Join(gs.ScriptsPath, "editor", "main.go")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(editorScriptSource), 0640); err != nil {
		t.Fatal(err)
	}
	scriptPackages = scanscripts(scriptSearchDirs(), nil)
	grantScriptPermissionsForTest(t, "editor-test")
	ed := openScriptSourceEditor("editor-test", path)
	if ed == nil {
		t.Fatal("script editor did not open")
	}
	return ed
}

func TestScriptSourceEditorChecksDraftAndSavesWorkInProgress(t *testing.T) {
	ed := scriptSourceEditorFixture(t, false)
	if !strings.HasPrefix(ed.win.Title, "Edit Script") || !ed.win.Searchable || !ed.input.Multiline || !ed.input.AcceptTab {
		t.Fatal("script editor is missing shared editing controls")
	}
	ed.win.OnSearch("VERSION")
	if ed.input.SelectedText() != "Version" {
		t.Fatal("search did not select script text")
	}
	editMacroForTest(ed, "package main\nfunc Init() {\n")
	if openScriptSourceEditor("editor-test", ed.doc.path) != ed {
		t.Fatal("opening again lost the draft")
	}
	if ed.check() || ed.save(true) {
		t.Fatal("invalid draft passed validation")
	}
	data, _ := os.ReadFile(ed.doc.path)
	if string(data) != editorScriptSource {
		t.Fatal("checking modified the file")
	}
	if !ed.save(false) || ed.dirty() {
		t.Fatal("plain save rejected work in progress")
	}
	editMacroForTest(ed, editorScriptSource)
	if !ed.check() {
		t.Fatal(ed.message)
	}
	editMacroForTest(ed, strings.Replace(editorScriptSource, `Version = "one"`, `Version int = "one"`, 1))
	if ed.check() {
		t.Fatal("type error passed validation")
	}
	editMacroForTest(ed, editorScriptSource)
	if running, _ := scriptManagerSession().scriptRuntimeSnapshot("editor-test"); running {
		t.Fatal("check activated the script")
	}
	if !ed.save(true) {
		t.Fatal(ed.message)
	}
	if running, _ := scriptManagerSession().scriptRuntimeSnapshot("editor-test"); running {
		t.Fatal("saving activated the stopped script")
	}
}

func TestScriptSourceEditorFolderValidationAndReload(t *testing.T) {
	ed := scriptSourceEditorFixture(t, true)
	asset := filepath.Join(filepath.Dir(ed.doc.path), "message.txt")
	if err := os.WriteFile(asset, []byte("asset value"), 0644); err != nil {
		t.Fatal(err)
	}
	scriptPackages = scanscripts(scriptSearchDirs(), nil)
	value := strings.Replace(editorScriptSource, "one", "two", 1)
	editMacroForTest(ed, value)
	if !ed.check() {
		t.Fatal(ed.message)
	}
	session := scriptManagerSession()
	if err := session.startSessionScript("editor-test", []byte(editorScriptSource), restrictedStdlib(), scriptPackages["editor-test"].assets); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { session.stopSessionScript("editor-test", "test cleanup") })
	other := mustNewSession(2)
	if err := other.startSessionScript("editor-test", []byte(editorScriptSource), restrictedStdlib(), nil); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { other.stopSessionScript("editor-test", "test cleanup") })
	if !ed.save(true) {
		t.Fatal(ed.message)
	}
	for _, tc := range []struct {
		session *Session
		want    string
	}{{session, "two"}, {other, "one"}} {
		q := currentSessionScriptEventQueue(tc.session, "editor-test")
		got, err := q.interpreter.Eval("Version")
		if err != nil || got.String() != tc.want {
			t.Fatalf("version = %v, %v; want %q", got, err, tc.want)
		}
	}
	data, err := session.automation.scripts["editor-test"].prepared.candidate.assets.read("message.txt")
	if err != nil || string(data) != "asset value" {
		t.Fatal("reload lost package assets", err)
	}

}

func TestScriptSourceEditorValidationDoesNotCommitEffects(t *testing.T) {
	ed := scriptSourceEditorFixture(t, false)
	editMacroForTest(ed, "package main\nimport \"gt2\"\nfunc Init(){gt2.Store(\"editor-check\", true);gt2.Command(\"editor-check\",func(string){})}\n")
	if !ed.check() {
		t.Fatal(ed.message)
	}
	if scriptStorageGet("editor-test", "editor-check") != nil {
		t.Fatal("validation persisted storage")
	}
	commands, _, _, _, _ := scriptRegistrationSummaryForSession(scriptManagerSession(), "editor-test")
	if len(commands) != 0 {
		t.Fatal("validation registered commands")
	}
	editMacroForTest(ed, "package main\nfunc Init(){panic(\"bad draft init\")}\n")
	if ed.check() || !strings.Contains(ed.message, "bad draft init") {
		t.Fatal("Init error was not reported", ed.message)
	}
}

func TestScriptSourceEditorConflictAndCombinedQuit(t *testing.T) {
	ed := scriptSourceEditorFixture(t, false)
	for _, draft := range sourceEditors {
		editMacroForTest(draft, draft.input.Text+"// unsaved\n")
	}
	quit := false
	if !confirmSourceEditorQuit(func() { quit = true }) {
		t.Fatal("missing combined quit prompt")
	}
	clickMacroEditorButton(t, sourceEditorQuitPrompt, "Save All & Quit")
	if !quit || len(dirtySourceEditors()) != 0 {
		t.Fatal("quit did not save both kinds of draft")
	}
	editMacroForTest(ed, ed.input.Text+"// local\n")
	if err := os.WriteFile(ed.doc.path, []byte("// external\n"), 0640); err != nil {
		t.Fatal(err)
	}
	if ed.save(false) || !ed.dirty() {
		t.Fatal("external change was overwritten")
	}
}

func TestScriptSourceEditorKeepsPermissionReview(t *testing.T) {
	ed := scriptSourceEditorFixture(t, false)
	scriptPermissionMu.Lock()
	scriptPermissionGrants["editor-test"], scriptPermissionReviews["editor-test"] = nil, nil
	scriptPermissionMu.Unlock()
	editMacroForTest(ed, "package main\nimport \"gt2\"\nfunc Init(){gt2.Send(\"/look\")}\n")
	if ed.check() || !strings.Contains(ed.message, "permissions required") {
		t.Fatal(ed.message)
	}
	if !ed.save(false) {
		t.Fatal("permissions blocked a plain source save")
	}
	if scriptHasPermission("editor-test", "send") {
		t.Fatal("editor granted a permission")
	}
}

func TestScriptSourceDocumentEncodingAndZipAvailability(t *testing.T) {
	path := filepath.Join(t.TempDir(), "script.go")
	raw := append([]byte{0xef, 0xbb, 0xbf}, []byte(strings.ReplaceAll(editorScriptSource, "\n", "\r\n"))...)
	if err := os.WriteFile(path, raw, 0640); err != nil {
		t.Fatal(err)
	}
	doc, err := loadSourceDocument(path, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := doc.save(doc.savedText); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	if !bytes.Equal(raw, data) {
		t.Fatal("unchanged script lost its byte encoding")
	}
	if err := doc.save(doc.savedText + "// café\n"); err != nil {
		t.Fatal(err)
	}
	data, _ = os.ReadFile(path)
	if !bytes.HasPrefix(data, []byte{0xef, 0xbb, 0xbf}) || !bytes.HasSuffix(data, []byte("// café\r\n")) {
		t.Fatal("script encoding or newlines changed")
	}
	if err := os.WriteFile(path, []byte{0x8e}, 0640); err != nil {
		t.Fatal(err)
	}
	if _, err := loadSourceDocument(path, false); err == nil {
		t.Fatal("non-UTF-8 script accepted")
	}
	if reason := scriptEditorUnavailable("unknown-editor-zip", "example.zip"); !strings.Contains(reason, "Unpack") {
		t.Fatal(reason)
	}
}
