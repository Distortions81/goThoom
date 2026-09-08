package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/text/encoding/charmap"
)

func triggerEditorFixture(t *testing.T, raw []byte, keys bool) *legacyTriggerDocument {
	t.Helper()
	old := dataDirPath
	dataDirPath = t.TempDir()
	t.Cleanup(func() { dataDirPath = old })
	if err := os.MkdirAll(legacyMacroLibraryPath(), 0755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(legacyMacroLibraryPath(), "edit.mac")
	if err := os.WriteFile(path, raw, 0640); err != nil {
		t.Fatal(err)
	}
	doc, err := loadLegacyTriggerDocument(legacyMacroLibraryEntry{ID: "edit.mac", Name: "Edit example", Path: path}, keys)
	if err != nil {
		t.Fatal(err)
	}
	return doc
}

func TestLegacyTriggerEditsPreserveSourceAndCommentContext(t *testing.T) {
	original := "// Name: Example\r\n// Heal nearby\r\n/* setup */ \"/heal\"\r\n{\r\n  // keep this with the body\r\n  \"/use bag\\r\" // trailing comment\r\n}\r\n'pp' \"/ponder \"\r\nf6 \"/look\\r\"\r\n"
	doc := triggerEditorFixture(t, []byte(original), false)
	if len(doc.fields) != 2 {
		t.Fatalf("fields=%d", len(doc.fields))
	}
	if err := doc.save([]string{"/h", "think"}); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(doc.entry.Path)
	if err != nil {
		t.Fatal(err)
	}
	want := strings.Replace(strings.Replace(original, "\"/heal\"", "\"/h\"", 1), "'pp'", "'think'", 1)
	if string(got) != want {
		t.Fatalf("source changed outside trigger tokens:\n%s", got)
	}
	info, _ := os.Stat(doc.entry.Path)
	if info.Mode().Perm() != 0640 {
		t.Fatal("file permissions changed")
	}
	var attached bool
	for _, comment := range doc.program.Comments {
		if comment.Text == "// Heal nearby" {
			attached = comment.AttachedTo.Line == 3
		}
	}
	if !attached {
		t.Fatal("standalone comment did not attach to the following declaration")
	}
}

func TestLegacyTriggerEncodingAndEscapes(t *testing.T) {
	for _, encoding := range []string{"utf8-bom", "macroman"} {
		t.Run(encoding, func(t *testing.T) {
			source := "// café\n\"/old\" \"/look\\r\"\n"
			raw := append([]byte{0xef, 0xbb, 0xbf}, []byte(source)...)
			if encoding == "macroman" {
				var err error
				raw, err = charmap.Macintosh.NewEncoder().Bytes([]byte(source))
				if err != nil {
					t.Fatal(err)
				}
			}
			doc := triggerEditorFixture(t, raw, false)
			if err := doc.save([]string{"/say\"hi\\there"}); err != nil {
				t.Fatal(err)
			}
			current, _ := os.ReadFile(doc.entry.Path)
			prefix := raw[:bytes.IndexByte(raw, '\n')+1]
			if !bytes.HasPrefix(current, prefix) {
				t.Fatal("comment or encoding changed")
			}
			parsed := parseLegacyMacroSources([]legacyMacroSource{{Path: doc.entry.Path, Text: decodeLegacyMacroSourceText(current)}})
			if parsed.Macros[0].Trigger != "/say\"hi\\there" {
				t.Fatalf("escaped trigger=%q", parsed.Macros[0].Trigger)
			}
		})
	}
}

func TestLegacyTriggerRejectsConflictsAndExternalChanges(t *testing.T) {
	raw := []byte("\"/first\" \"one\"\n\"/second\" \"two\"\n")
	doc := triggerEditorFixture(t, raw, false)
	if err := doc.save([]string{"/second", "/second"}); err == nil || !strings.Contains(err.Error(), "conflicts") {
		t.Fatalf("conflict=%v", err)
	}
	got, _ := os.ReadFile(doc.entry.Path)
	if !bytes.Equal(got, raw) {
		t.Fatal("conflict modified source")
	}
	if err := doc.save([]string{"/second", "/first"}); err != nil {
		t.Fatalf("valid trigger swap: %v", err)
	}
	if err := doc.save([]string{"/new", "/second"}); err == nil || !strings.Contains(err.Error(), "outside") {
		t.Fatalf("stale save=%v", err)
	}
}

func TestLegacyTriggerEditsIncludedFiles(t *testing.T) {
	doc := triggerEditorFixture(t, []byte("\"/root\" \"one\"\n"), false)
	include := filepath.Join(legacyMacrosDir(), "shared.mac")
	if err := os.WriteFile(include, []byte("// shared command\n\"/shared\" \"two\"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(doc.entry.Path, []byte("include \"shared.mac\"\n\"/root\" \"one\"\n"), 0640); err != nil {
		t.Fatal(err)
	}
	doc, err := loadLegacyTriggerDocument(doc.entry, false)
	if err != nil {
		t.Fatal(err)
	}
	values := make([]string, len(doc.fields))
	for i, f := range doc.fields {
		values[i] = f.declaration.Trigger + "2"
	}
	if err := doc.save(values); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(include)
	if string(got) != "// shared command\n\"/shared2\" \"two\"\n" {
		t.Fatalf("include=%s", got)
	}
}

func TestLegacyTriggerKeyMouseAndPunctuationFormatting(t *testing.T) {
	doc := triggerEditorFixture(t, []byte("f6 \"/look\\r\"\nshift-click2 \"/info\\r\"\n"), true)
	if err := doc.save([]string{"Control-F8", "Option-Click3"}); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"'", "\\", "Shift-\"", "Numpad-enter", "Control-wheelup", "-"} {
		value := key
		if key == "-" {
			value = "minus"
		}
		if _, err := formatLegacyTrigger(legacyTriggerField{}, value); err != nil {
			t.Errorf("%q: %v", key, err)
		}
	}
	for _, invalid := range []string{"", "notakey", "f6\n\"/injected\"", "/* comment */"} {
		if _, err := formatLegacyTrigger(legacyTriggerField{}, invalid); err == nil {
			t.Errorf("accepted %q", invalid)
		}
	}
}

func TestLegacyTriggerRenameKeepsRuntimeBehavior(t *testing.T) {
	doc := triggerEditorFixture(t, []byte("// say hello\n\"/hello\" \"/say \" @text \"\\r\"\nf6 \"/look\\r\"\n"), false)
	if err := doc.save([]string{"/hi"}); err != nil {
		t.Fatal(err)
	}
	source, _, err := readLegacyMacroSource(doc.entry.Path)
	if err != nil {
		t.Fatal(err)
	}
	var sent []string
	runtime := newLegacyMacroRuntimeWithHooks(parseLegacyMacroSources([]legacyMacroSource{source}), legacyMacroRuntimeHooks{SendText: func(text string) { sent = append(sent, text) }})
	if runtime.triggerExpression("/hello world", 0) {
		t.Fatal("old command still triggers")
	}
	if !runtime.triggerExpression("/hi world", 0) {
		t.Fatal("renamed command did not trigger")
	}
	if len(sent) != 1 || sent[0] != "/say world" {
		t.Fatalf("renamed body output=%q", sent)
	}
	if started, _ := runtime.triggerKey("f6", 0, 0); !started {
		t.Fatal("unrelated key binding changed")
	}
}
