package main

import (
	"strings"
	"testing"
	"unicode/utf8"

	"gothoom/eui"
)

func TestMacroHighlightSyntaxAndOffsets(t *testing.T) {
	for _, background := range []eui.Color{eui.NewColor(20, 22, 24, 255), eui.NewColor(245, 245, 245, 255)} {
		palette := macroHighlightPalette(background)
		for _, tt := range []struct {
			name, source, fragment string
			kind                   macroHighlightKind
		}{
			{"metadata", "// Name: Café\n", "// Name: Café", macroHighlightComment},
			{"keyword", "\tSeT @count -12", "SeT", macroHighlightKeyword},
			{"variable", "\"é\"\t@text.word[1]", "@text.word[1]", macroHighlightVariable},
			{"number", "set @count -12", "-12", macroHighlightNumber},
			{"invalid number", "set @count +-12", "+-12", macroHighlightPlain},
			{"attribute", "$IGNORE_CASE", "$IGNORE_CASE", macroHighlightKeyword},
			{"binding", "command-option-f6 \"/wave\\r\"", "command-option-f6", macroHighlightBinding},
			{"replacement", "'pp' \"/ponder \"", "'pp'", macroHighlightString},
			{"quoted slashes", "message \"https://example.test\" // end", "https://example.test", macroHighlightString},
			{"trailing comment", "message \"é\\\"hi\" // fin", "// fin", macroHighlightComment},
			{"unknown escape", "message \"hello\\n\"", "\\n", macroHighlightString},
			{"block in string", "message \"a/* hidden */b\"", "/* hidden */", macroHighlightComment},
			{"string after block", "message \"a/* hidden */b\"", "b\"", macroHighlightString},
			{"split keyword", "se/*é*/t @count 1", "t", macroHighlightKeyword},
			{"partial string", "set @name \"unfinished\\", "\"unfinished\\", macroHighlightString},
			{"before partial string", "set @name \"unfinished", "@name", macroHighlightVariable},
			{"partial block", "\"é\"\r\n  /* outer /* nested\nrest", "/* outer /* nested\nrest", macroHighlightComment},
			{"after partial quote", "message \"unfinished\npause 5", "pause", macroHighlightKeyword},
			{"unknown identifier", "call MyFunction", "MyFunction", macroHighlightPlain},
		} {
			t.Run(tt.name, func(t *testing.T) {
				spans := highlightLegacyMacro(tt.source, background)
				colors := make([]eui.Color, utf8.RuneCountInString(tt.source))
				end := 0
				for _, span := range spans {
					if span.Start < end || span.End <= span.Start || span.End > len(colors) || span.Color.A != 255 {
						t.Fatalf("invalid span: %+v", span)
					}
					for i := span.Start; i < span.End; i++ {
						colors[i] = span.Color
					}
					end = span.End
				}
				byteStart := strings.Index(tt.source, tt.fragment)
				start := utf8.RuneCountInString(tt.source[:byteStart])
				for i := start; i < start+utf8.RuneCountInString(tt.fragment); i++ {
					if colors[i] != palette[tt.kind] {
						t.Fatalf("%q rune %d color = %v, want %v", tt.fragment, i, colors[i], palette[tt.kind])
					}
				}
			})
		}
	}
}

func TestMacroHighlightBundledCorpus(t *testing.T) {
	for _, entry := range legacyMacroBundledLibrary {
		value, err := legacyMacroLibrarySource(entry)
		if err != nil {
			t.Fatal(err)
		}
		// Also exercise drafts cut off midway through real-world source.
		for _, source := range []string{value, value[:len(value)/2]} {
			end := 0
			for _, span := range highlightLegacyMacro(source, eui.ColorBlack) {
				if span.Start < end || span.End <= span.Start || span.End > utf8.RuneCountInString(source) {
					t.Fatalf("%s: invalid span %+v", entry.Filename, span)
				}
				end = span.End
			}
		}
	}
}

func TestMacroEditorHighlightsAndRefreshesPalette(t *testing.T) {
	ed := macroEditorFixture(t)
	if ed.options.highlight == nil {
		t.Fatal("macro editor has no highlighter")
	}
	before := ed.input.Text
	old := ed.options.highlight("pause 5", eui.ColorBlack)
	ed.input.Color = eui.ColorWhite
	ed.input.Focused = false
	updateSourceEditors()
	next := ed.options.highlight("pause 5", ed.input.Color)
	if ed.highlightBackground != ed.input.Color || old[0].Color == next[0].Color || ed.input.Text != before || ed.dirty() {
		t.Fatal("palette did not refresh independently of the draft")
	}
}
