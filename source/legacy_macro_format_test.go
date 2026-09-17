package main

import (
	"os"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestMacroFormatIndentationAndPreservation(t *testing.T) {
	before := "  // Name: Café  \n  f1\n {\n set @n 2   \n if @n > 0\n message \"hello  \\" + "r\"  \n else\n random no-repeat\n \"one\"\n or\n \"two\"\n end random\n end if\n /* Keep this\n      ASCII art */\n  message \"a/* inside quotes */b\"  \n }\n"
	want := "// Name: Café  \nf1\n{\n\tset @n 2\n\tif @n > 0\n\t\tmessage \"hello  \\" + "r\"\n\telse\n\t\trandom no-repeat\n\t\t\t\"one\"\n\t\tor\n\t\t\t\"two\"\n\t\tend random\n\tend if\n /* Keep this\n      ASCII art */\n  message \"a/* inside quotes */b\"  \n}\n"
	got, err := formatLegacyMacro(before)
	if err != nil || got != want {
		t.Fatalf("format error=%v\ngot:  %q\nwant: %q", err, got, want)
	}
	again, err := formatLegacyMacro(got)
	if err != nil || again != got {
		t.Fatalf("format is not idempotent: %v, %q", err, again)
	}
}

func TestMacroFormatRefusesIncompleteSyntax(t *testing.T) {
	for _, value := range []string{"f1 \"unfinished", "f1\n{\n\"hello\"\n", "/* unfinished", "f1\x00 \"x\""} {
		if _, err := formatLegacyMacro(value); err == nil {
			t.Fatalf("formatted invalid draft %q", value)
		}
	}
}

func TestMacroFormatBundledCorpus(t *testing.T) {
	for _, entry := range legacyMacroBundledLibrary {
		value, err := legacyMacroLibrarySource(entry)
		if err != nil {
			t.Fatal(err)
		}
		value = normalizeSourceEditorText(value)
		got, err := formatLegacyMacro(value)
		// This published fixture intentionally contains an extra closing brace.
		if entry.Filename == "clump-omega-zu.mac" {
			if err == nil || !strings.Contains(err.Error(), "unexpected closing brace") {
				t.Fatalf("known-invalid corpus entry should remain unformatted: %v", err)
			}
			continue
		}
		if err != nil {
			t.Fatalf("%s: %v", entry.Filename, err)
		}
		again, err := formatLegacyMacro(got)
		if err != nil || again != got {
			t.Fatalf("%s: formatting is not idempotent: %v", entry.Filename, err)
		}
		beforeLines, beforeComments, _ := legacyMacroFormatLines(value)
		afterLines, afterComments, _ := legacyMacroFormatLines(got)
		if len(beforeComments) != len(afterComments) {
			t.Fatalf("%s: block comments changed", entry.Filename)
		}
		for i, comment := range beforeComments {
			if comment.Text != afterComments[i].Text {
				t.Fatalf("%s: comment contents changed", entry.Filename)
			}
		}
		// Ignore indentation/location metadata, compare all source tokens.
		tokens := func(lines []legacyMacroLine) string {
			var out strings.Builder
			for _, line := range lines {
				for _, token := range line.Tokens {
					out.WriteByte(token.Quote)
					out.WriteString(token.Text)
					out.WriteByte(0)
				}
			}
			return out.String()
		}
		if tokens(beforeLines) != tokens(afterLines) {
			t.Fatalf("%s: token contents changed", entry.Filename)
		}
	}
}

func TestMacroEditorFormatsOnOpen(t *testing.T) {
	for _, tc := range []struct {
		name, before, want string
	}{
		{"unformatted", "  f1\n {\n message \"café\"  \n }\n", "f1\n{\n\tmessage \"café\"\n}\n"},
		{"formatted", "f1 \"/look\\r\"\n", "f1 \"/look\\r\"\n"},
		{"unfinished", "f1\n{\n\"unfinished", "f1\n{\n\"unfinished"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ed := macroEditorFixture(t)
			path := ed.doc.path
			ed.win.Close()
			if err := os.WriteFile(path, []byte(tc.before), 0640); err != nil {
				t.Fatal(err)
			}
			entry := legacyMacroLibraryEntry{Path: path}
			ed = openLegacyMacroEditor(entry)
			if ed == nil || ed.input.Text != tc.want {
				t.Fatalf("macro did not open with expected text: %q", tc.want)
			}
			changed := tc.before != tc.want
			if ed.dirty() != changed || ed.input.CanUndo() != changed {
				t.Fatal("opening did not preserve the saved baseline and undo history")
			}
			if raw, err := os.ReadFile(path); err != nil || string(raw) != tc.before {
				t.Fatalf("opening changed the file: %q, %v", raw, err)
			}
			if changed {
				clickMacroEditorButton(t, ed.win, "Undo")
				if ed.input.Text != tc.before || ed.dirty() {
					t.Fatal("Undo did not restore the file's original text")
				}
			}
			draft := "  f2 \"/wave\\r\"  \n"
			editMacroForTest(ed, draft)
			if reopened := openLegacyMacroEditor(entry); reopened != ed || reopened.input.Text != draft {
				t.Fatal("reopening changed the existing draft")
			}
		})
	}
}

func TestMacroEditorFormatAndSave(t *testing.T) {
	ed := macroEditorFixture(t)
	before := "  f1\n {\n message \"café\"  \n }\n"
	editMacroForTest(ed, before)
	start := utf8.RuneCountInString(before[:strings.Index(before, "café")])
	ed.input.CursorPos, ed.input.SelectStart, ed.input.SelectEnd = start+4, start, start+4
	clickMacroEditorButton(t, ed.win, "Format")
	if ed.input.SelectedText() != "café" || !ed.dirty() || !ed.input.Focused {
		t.Fatal("Format lost selection, focus, or draft state")
	}
	if raw, _ := os.ReadFile(ed.doc.path); string(raw) == ed.input.Text {
		t.Fatal("Format wrote the file without Save")
	}
	formatted := ed.input.Text
	clickMacroEditorButton(t, ed.win, "Undo")
	if ed.input.Text != before {
		t.Fatal("toolbar Undo did not undo formatting")
	}
	clickMacroEditorButton(t, ed.win, "Redo")
	if ed.input.Text != formatted {
		t.Fatal("toolbar Redo did not redo formatting")
	}
	editMacroForTest(ed, before)
	if !ed.save(false) {
		t.Fatal(ed.message)
	}
	if raw, _ := os.ReadFile(ed.doc.path); string(raw) != "f1\n{\n\tmessage \"café\"\n}\n" || ed.dirty() {
		t.Fatalf("Save did not format: %q", raw)
	}
	broken := "f1\n{\n\"unfinished"
	editMacroForTest(ed, broken)
	if !ed.save(false) || !strings.Contains(ed.message, "without formatting") {
		t.Fatal("unfinished draft could not be saved", ed.message)
	}
	if raw, _ := os.ReadFile(ed.doc.path); string(raw) != broken || ed.input.Text != broken {
		t.Fatal("saving unfinished draft changed its text")
	}
}
