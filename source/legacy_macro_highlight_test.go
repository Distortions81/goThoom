package main

import (
	"strings"
	"testing"
	"unicode/utf8"

	"gothoom/eui"
)

func TestMacroHighlightSyntaxAndOffsets(t *testing.T) {
	for _, background := range []eui.Color{eui.NewColor(20, 22, 24, 255), eui.NewColor(245, 245, 245, 255)} {
		colors := eui.DefaultSyntaxColors(background)
		palette := sourceHighlightPalette(colors)
		for _, tt := range []struct {
			name, source, fragment string
			kind                   sourceHighlightKind
		}{
			{"metadata", "// Name: Café\n", "// Name: Café", sourceHighlightComment},
			{"keyword", "\tSeT @count -12", "SeT", sourceHighlightKeyword},
			{"variable", "\"é\"\t@text.word[1]", "@text.word[1]", sourceHighlightVariable},
			{"number", "set @count -12", "-12", sourceHighlightNumber},
			{"invalid number", "set @count +-12", "+-12", sourceHighlightPlain},
			{"attribute", "$IGNORE_CASE", "$IGNORE_CASE", sourceHighlightKeyword},
			{"binding", "command-option-f6 \"/wave\\r\"", "command-option-f6", sourceHighlightBinding},
			{"replacement", "'pp' \"/ponder \"", "'pp'", sourceHighlightString},
			{"quoted slashes", "message \"https://example.test\" // end", "https://example.test", sourceHighlightString},
			{"trailing comment", "message \"é\\\"hi\" // fin", "// fin", sourceHighlightComment},
			{"unknown escape", "message \"hello\\n\"", "\\n", sourceHighlightString},
			{"block in string", "message \"a/* hidden */b\"", "/* hidden */", sourceHighlightComment},
			{"string after block", "message \"a/* hidden */b\"", "b\"", sourceHighlightString},
			{"split keyword", "se/*é*/t @count 1", "t", sourceHighlightKeyword},
			{"partial string", "set @name \"unfinished\\", "\"unfinished\\", sourceHighlightString},
			{"before partial string", "set @name \"unfinished", "@name", sourceHighlightVariable},
			{"partial block", "\"é\"\r\n  /* outer /* nested\nrest", "/* outer /* nested\nrest", sourceHighlightComment},
			{"after partial quote", "message \"unfinished\npause 5", "pause", sourceHighlightKeyword},
			{"unknown identifier", "call MyFunction", "MyFunction", sourceHighlightPlain},
		} {
			t.Run(tt.name, func(t *testing.T) {
				spans := highlightLegacyMacro(tt.source, colors)
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
			for _, span := range highlightLegacyMacro(source, eui.DefaultSyntaxColors(eui.ColorBlack)) {
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
	oldSettings := gs
	t.Cleanup(func() { gs = oldSettings })
	oldColors := eui.DefaultSyntaxColors(eui.ColorBlack)
	old := ed.options.highlight("pause 5", oldColors)
	colors := eui.DefaultSyntaxColors(eui.ColorWhite)
	gs.EditorUseCustomColors, gs.EditorSyntaxColors = true, &colors
	ed.input.Focused = false
	updateSourceEditors()
	next := ed.options.highlight("pause 5", sourceEditorColors())
	if ed.highlightColors != colors || old[0].Color == next[0].Color || ed.input.Text != before || ed.dirty() {
		t.Fatal("palette did not refresh independently of the draft")
	}
}
