package main

import (
	"bytes"
	"os"
	"path"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestGoScriptFormatBundledCorpus(t *testing.T) {
	entries, err := scriptLibraryEntries()
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		t.Run(entry.Filename, func(t *testing.T) {
			data, err := scriptScripts.ReadFile(path.Join(bundledScriptDir, entry.Filename))
			if err != nil {
				t.Fatal(err)
			}
			got, err := formatGoScript(string(data))
			if err != nil {
				t.Fatal(err)
			}
			again, err := formatGoScript(got)
			if err != nil || again != got {
				t.Fatalf("formatting is not idempotent: %v", err)
			}
		})
	}
}

func TestGoScriptEditorFormatsOnOpen(t *testing.T) {
	for _, tc := range []struct{ name, before, want string }{
		{"unformatted", "package main\nfunc Init( ){\nprintln(\"café\")   \n}\n", "package main\n\nfunc Init() {\n\tprintln(\"café\")\n}\n"},
		{"formatted", editorScriptSource, editorScriptSource},
		{"unfinished", "package main\nfunc Init(){", "package main\nfunc Init(){"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ed := scriptSourceEditorFixture(t, false)
			filename := ed.doc.path
			ed.win.Close()
			if err := os.WriteFile(filename, []byte(tc.before), 0640); err != nil {
				t.Fatal(err)
			}
			ed = openScriptSourceEditor("editor-test", filename)
			if ed == nil || ed.input.Text != tc.want {
				t.Fatalf("script did not open with expected text: %q", tc.want)
			}
			changed := tc.before != tc.want
			if ed.dirty() != changed || ed.input.CanUndo() != changed {
				t.Fatal("opening did not preserve the saved baseline and undo history")
			}
			if raw, err := os.ReadFile(filename); err != nil || string(raw) != tc.before {
				t.Fatalf("opening changed the file: %q, %v", raw, err)
			}
			if changed {
				clickMacroEditorButton(t, ed.win, "Undo")
				if ed.input.Text != tc.before || ed.dirty() {
					t.Fatal("Undo did not restore the original source")
				}
			}
			draft := "package main\nfunc Init( ){}"
			editMacroForTest(ed, draft)
			if reopened := openScriptSourceEditor("editor-test", filename); reopened != ed || reopened.input.Text != draft {
				t.Fatal("reopening changed the existing draft")
			}
		})
	}
}

func TestGoScriptEditorFormatAndSave(t *testing.T) {
	ed := scriptSourceEditorFixture(t, false)
	before := "package main\nfunc Init( ){\nprintln(\"café\")   \n}\n"
	want := "package main\n\nfunc Init() {\n\tprintln(\"café\")\n}\n"
	editMacroForTest(ed, before)
	start := utf8.RuneCountInString(before[:strings.Index(before, "café")])
	ed.input.CursorPos, ed.input.SelectStart, ed.input.SelectEnd = start+4, start, start+4
	clickMacroEditorButton(t, ed.win, "Format")
	if ed.input.Text != want || ed.input.SelectedText() != "café" || !ed.dirty() || !ed.input.Focused {
		t.Fatal("Format lost selection, focus, or draft state", ed.input.Text)
	}
	if raw, _ := os.ReadFile(ed.doc.path); string(raw) != editorScriptSource {
		t.Fatal("Format wrote the file without Save")
	}
	clickMacroEditorButton(t, ed.win, "Undo")
	if ed.input.Text != before || ed.input.SelectedText() != "café" {
		t.Fatal("Undo did not restore the original source and selection")
	}
	clickMacroEditorButton(t, ed.win, "Redo")
	if ed.input.Text != want || ed.input.SelectedText() != "café" {
		t.Fatal("Redo did not restore the formatted source and selection")
	}
	editMacroForTest(ed, before)
	if !ed.save(false) {
		t.Fatal(ed.message)
	}
	if raw, _ := os.ReadFile(ed.doc.path); string(raw) != want || ed.dirty() {
		t.Fatalf("Save did not format: %q", raw)
	}
	for _, broken := range []string{"package main\nfunc Init(){", "package main\nvar s = \"unfinished", "package main\n/* unfinished"} {
		editMacroForTest(ed, broken)
		if ed.format() || ed.input.Text != broken || !strings.HasPrefix(ed.message, "Not formatted:") {
			t.Fatal("Format changed an unfinished draft", ed.message)
		}
		if !ed.save(false) || !strings.Contains(ed.message, "without formatting") {
			t.Fatal("unfinished draft could not be saved", ed.message)
		}
		if raw, _ := os.ReadFile(ed.doc.path); string(raw) != broken || ed.input.Text != broken {
			t.Fatal("saving unfinished draft changed its text")
		}
	}
}

func TestGoScriptEditorFormattedSaveKeepsEncoding(t *testing.T) {
	ed := scriptSourceEditorFixture(t, false)
	filename := ed.doc.path
	ed.win.Close()
	before := "package main\r\nfunc Init( ){\r\nprintln(\"café\")\r\n}\r\n"
	raw := append([]byte{0xef, 0xbb, 0xbf}, []byte(before)...)
	if err := os.WriteFile(filename, raw, 0640); err != nil {
		t.Fatal(err)
	}
	ed = openScriptSourceEditor("editor-test", filename)
	if ed == nil || !ed.save(false) {
		t.Fatal("could not save formatted script")
	}
	want := append([]byte{0xef, 0xbb, 0xbf}, []byte("package main\r\n\r\nfunc Init() {\r\n\tprintln(\"café\")\r\n}\r\n")...)
	if data, err := os.ReadFile(filename); err != nil || !bytes.Equal(data, want) {
		t.Fatalf("formatted save lost encoding or newlines: %q, %v", data, err)
	}
}

func TestGoScriptFormatRemapsSelection(t *testing.T) {
	for _, tc := range []struct{ source, selected string }{
		{"package main\nimport (\n\"strings\"\n\"fmt\"\n)\nfunc Init(){}", "strings"},
		{"package main\nimport (\n\"strings\"\n\"fmt\"\n)\nfunc Init(){}", "fmt"},
		{"package main\nfunc Init(){a:=1; println(a)}", "println"},
		{"package main\nfunc Init(){a:=1; println(a)}", "1"},
		{"package main\nfunc Init(){value:=1; println(value)}", "value"},
		{"package main\nfunc Init(){\nif true{\nprintln(`café\nhello`)\n}\n}", "café\nhello"},
		{"package main\nfunc Init(){\nprintln(\"one\"); println(\"two\")\n}", "two"},
	} {
		after, err := formatGoScript(tc.source)
		if err != nil {
			t.Fatal(err)
		}
		start := utf8.RuneCountInString(tc.source[:strings.Index(tc.source, tc.selected)])
		end := start + utf8.RuneCountInString(tc.selected)
		a, b := remapGoFormattedPosition(tc.source, after, start, false), remapGoFormattedPosition(tc.source, after, end, true)
		if a > b || string([]rune(after)[a:b]) != tc.selected {
			t.Fatalf("selection %q was not preserved after formatting: %q (%d:%d)", tc.selected, after, a, b)
		}
	}
}

func TestGoScriptEditorFormatSelectionEdges(t *testing.T) {
	for _, direction := range []string{"forward", "backward", "caret"} {
		t.Run(direction, func(t *testing.T) {
			ed := scriptSourceEditorFixture(t, false)
			before := "package main\nfunc Init(){n:=42;println(n)}"
			editMacroForTest(ed, before)
			start := strings.Index(before, "42")
			end := start + 2
			if direction == "backward" {
				start, end = end, start
			} else if direction == "caret" {
				start = end
			}
			ed.input.CursorPos, ed.input.SelectStart, ed.input.SelectEnd = end, start, end
			clickMacroEditorButton(t, ed.win, "Format")
			want := "42"
			if direction == "caret" {
				want = ""
			}
			if ed.input.SelectedText() != want || ed.input.CursorPos != ed.input.SelectEnd {
				t.Fatal("formatting lost selection direction or selected inserted whitespace")
			}
			if direction == "backward" && ed.input.SelectStart <= ed.input.SelectEnd {
				t.Fatal("formatting reversed the selection")
			}
		})
	}
}
