package main

import (
	"strings"
	"testing"
)

func TestMacroLintFindsAdvisoryProblems(t *testing.T) {
	for _, tt := range []struct{ body, want string }{
		{"pause", "pause requires one argument"},
		{"call one two", "call requires one argument"},
		{"set @x", "requires a variable and value"},
		{"if @x\nend if", "requires a value, comparison, and value"},
		{"if @x == 1", "no matching end if"},
		{"random\nend if", "end if has no matching if"},
		{"else", "else has no matching if"},
		{"or", "or has no matching random"},
		{"if @x == 1\nelse\nelse\nend if", "branch follows an unconditional else"},
		{"label mark\nlabel mark", "duplicate label"},
		{"label", "label requires one unquoted name"},
	} {
		program := parseLegacyMacroSources([]legacyMacroSource{{Path: "lint.mac", Text: "f1\n{\n" + tt.body + "\n}\n"}})
		warnings := lintLegacyMacroProgram(program)
		var messages []string
		for _, warning := range warnings {
			if warning.Location.Line < 3 || !strings.HasSuffix(warning.Location.Path, "lint.mac") {
				t.Fatal("lint lost source location", warning)
			}
			messages = append(messages, warning.Message)
		}
		if !strings.Contains(strings.Join(messages, "\n"), tt.want) {
			t.Fatalf("%q: warnings %q, want %q", tt.body, messages, tt.want)
		}
	}
}

func TestMacroLintLeavesDynamicValuesAlone(t *testing.T) {
	program := parseLegacyMacroSources([]legacyMacroSource{{Path: "lint.mac", Text: "set name thing\nf1\n{\nset @x @unknown\ncall @dynamic\ngoto @label\npause @frames\nrandom @option\nif @x @comparison @other\n\"else\"\nelse if @a == 1\nmessage \"x\"\nelse\nmessage \"y\"\nend if\nor\n\"random\"\nend random\n}\n"}})
	if warnings := lintLegacyMacroProgram(program); len(warnings) != 0 {
		t.Fatalf("valid dynamic code produced warnings: %v", warnings)
	}
}

func TestMacroEditorCheckShowsLintWithoutBlockingSave(t *testing.T) {
	ed := macroEditorFixture(t)
	editMacroForTest(ed, "f1\n{\npause\n}\n")
	if !ed.check() || !strings.Contains(ed.message, "lint warning") || !strings.Contains(ed.message, "pause requires") {
		t.Fatal("Check did not distinguish lint from syntax errors", ed.message)
	}
	if !ed.save(true) {
		t.Fatal("advisory lint blocked save/reload", ed.message)
	}
}
