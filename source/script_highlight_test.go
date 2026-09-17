package main

import (
	"path"
	"strings"
	"testing"
	"unicode/utf8"

	"gothoom/eui"
)

func TestGoScriptHighlightSyntaxAndOffsets(t *testing.T) {
	for _, background := range []eui.Color{eui.ColorBlack, eui.ColorWhite} {
		palette := sourceHighlightPalette(background)
		for _, tt := range []struct {
			name, source, fragment string
			kind                   sourceHighlightKind
		}{
			{"keyword", "package main", "package", sourceHighlightKeyword},
			{"identifier", "var café = 42", "café", sourceHighlightPlain},
			{"decimal", "var café = 42", "42", sourceHighlightNumber},
			{"hex", "n := 0xFF_FF", "0xFF_FF", sourceHighlightNumber},
			{"binary", "n := 0b1010", "0b1010", sourceHighlightNumber},
			{"float", "n := 1.25e-3", "1.25e-3", sourceHighlightNumber},
			{"imaginary", "n := 2.5i", "2.5i", sourceHighlightNumber},
			{"rune", "r := 'é'", "'é'", sourceHighlightString},
			{"escaped quote", "s := \"é\\\"//hi\"", "\"é\\\"//hi\"", sourceHighlightString},
			{"comment in string", "s := \"/* hi */\"", "/* hi */", sourceHighlightString},
			{"raw string", "s := `é\r\n// hi\r\n`\nvar n = 2", "`é\r\n// hi\r\n`", sourceHighlightString},
			{"after raw string", "s := `é\r\n// hi\r\n`\nvar n = 2", "var", sourceHighlightKeyword},
			{"line comment", "\t// é\r\nvar n = 2", "// é\r", sourceHighlightComment},
			{"block comment", "/* é\r\n \"hi\" */var n = 2", "/* é\r\n \"hi\" */", sourceHighlightComment},
			{"after block comment", "/* é\r\n \"hi\" */var n = 2", "var", sourceHighlightKeyword},
			{"semicolon in comment", "n := 1 /* é\n hi */\nvar x = 2", "var", sourceHighlightKeyword},
			{"line directive", "//line elsewhere.go:900\nvar n = 2", "var", sourceHighlightKeyword},
			{"partial quote", "s := \"unfinished\\", "\"unfinished\\", sourceHighlightString},
			{"after partial quote", "s := \"unfinished\nvar n = 2", "var", sourceHighlightKeyword},
			{"partial rune", "r := 'é", "'é", sourceHighlightString},
			{"partial raw string", "s := `é\r\nrest", "`é\r\nrest", sourceHighlightString},
			{"partial comment", "/* é\r\nrest", "/* é\r\nrest", sourceHighlightComment},
			{"after illegal token", "#\nvar n = 2", "var", sourceHighlightKeyword},
			{"BOM", "\ufeffpackage main", "package", sourceHighlightKeyword},
		} {
			t.Run(tt.name, func(t *testing.T) {
				spans := highlightGoScript(tt.source, background)
				checkGoHighlightSpans(t, tt.source, spans)
				colors := make([]eui.Color, utf8.RuneCountInString(tt.source))
				for _, span := range spans {
					for i := span.Start; i < span.End; i++ {
						colors[i] = span.Color
					}
				}
				start := utf8.RuneCountInString(tt.source[:strings.Index(tt.source, tt.fragment)])
				for i := start; i < start+utf8.RuneCountInString(tt.fragment); i++ {
					if colors[i] != palette[tt.kind] {
						t.Fatalf("%q rune %d color = %v, want %v", tt.fragment, i, colors[i], palette[tt.kind])
					}
				}
			})
		}
	}
}

func checkGoHighlightSpans(t *testing.T, source string, spans []eui.TextColorSpan) {
	t.Helper()
	end, length := 0, utf8.RuneCountInString(source)
	for _, span := range spans {
		if span.Start < end || span.End <= span.Start || span.End > length || span.Color.A != 255 {
			t.Fatalf("invalid span: %+v (source length %d)", span, length)
		}
		end = span.End
	}
}

func TestGoScriptHighlightBundledCorpus(t *testing.T) {
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
			for _, source := range []string{string(data), strings.ReplaceAll(string(data), "\n", "\r\n")} {
				for _, end := range []int{0, 1, len(source) / 2, len(source)} {
					checkGoHighlightSpans(t, source[:end], highlightGoScript(source[:end], eui.ColorBlack))
				}
			}
		})
	}
}

func TestGoScriptEditorHighlightsAndRefreshesPalette(t *testing.T) {
	ed := scriptSourceEditorFixture(t, false)
	if ed.options.highlight == nil {
		t.Fatal("script editor has no highlighter")
	}
	before := ed.input.Text
	old := ed.options.highlight(before, eui.ColorBlack)
	ed.input.Color = eui.ColorWhite
	ed.input.Focused = false
	updateSourceEditors()
	next := ed.options.highlight(before, ed.input.Color)
	if len(old) == 0 || len(next) == 0 || ed.highlightBackground != ed.input.Color || old[0].Color == next[0].Color || ed.input.Text != before || ed.dirty() {
		t.Fatal("palette did not refresh independently of the draft")
	}
}
