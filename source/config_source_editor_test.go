package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	"gothoom/eui"
)

func TestOnlyLegacyMacrosAcceptMacRoman(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"script.go", "tts_substitute.txt", "palette.json", "style.json", "text.txt", "note.json"} {
		filename := filepath.Join(root, name)
		if err := os.WriteFile(filename, []byte{'c', 'a', 'f', 0x8e}, 0644); err != nil {
			t.Fatal(err)
		}
		if _, err := loadSourceDocument(filename, false); err == nil {
			t.Fatalf("%s accepted MacRoman", name)
		}
		raw := append([]byte{0xef, 0xbb, 0xbf}, []byte("Café 你好 🌟\r\n")...)
		if err := os.WriteFile(filename, raw, 0644); err != nil {
			t.Fatal(err)
		}
		doc, err := loadSourceDocument(filename, false)
		if err != nil || doc.macRoman {
			t.Fatalf("%s did not load Unicode: %v", name, err)
		}
		if err := doc.save(doc.savedText + "雪\n"); err != nil {
			t.Fatal(err)
		}
		got, err := os.ReadFile(filename)
		want := append(append([]byte(nil), raw...), []byte("雪\r\n")...)
		if err != nil || !bytes.Equal(got, want) {
			t.Fatalf("%s lost Unicode, BOM, or line endings: %q, %v", name, got, err)
		}
	}
	filename := filepath.Join(root, "legacy.mac")
	if err := os.WriteFile(filename, []byte{'c', 'a', 'f', 0x8e}, 0644); err != nil {
		t.Fatal(err)
	}
	doc, err := loadLegacyMacroDocument(filename)
	if err != nil || !doc.macRoman || doc.savedText != "café" {
		t.Fatal("legacy macro lost MacRoman support", err)
	}
	if _, err := parseTTSSubstitutions(string([]byte{'a', '=', 0x8e}), true); err == nil {
		t.Fatal("TTS parser accepted MacRoman")
	}
}

func TestTTSSubstitutionEditorChecksSavesAndApplies(t *testing.T) {
	macroEditorFixture(t)
	oldGS, oldActive, oldSubs := gs, storagePathsActivated, ttsSubs
	t.Cleanup(func() { gs, storagePathsActivated, ttsSubs = oldGS, oldActive, oldSubs })
	gs.AssetsPath, storagePathsActivated = t.TempDir(), false
	ttsSubs = map[string]string{"old": "kept"}
	ed := openTTSSubstitutionEditor()
	if ed == nil || ed.options.highlight == nil {
		t.Fatal("TTS editor did not open")
	}
	value := "# Pronunciation\né = ay\nremove=\nsymbol=a=b\n"
	editMacroForTest(ed, value)
	if !ed.check() || substituteTTS("old") != "kept" {
		t.Fatal("Check modified active substitutions")
	}
	if !ed.save(false) || substituteTTS("old") != "kept" {
		t.Fatal("Save applied substitutions")
	}
	clickMacroEditorButton(t, ed.win, "Save & Apply")
	if ed.dirty() || substituteTTS("é remove symbol") != "ay  a=b" {
		t.Fatal("Save & Apply did not load the edited file", ed.message)
	}
	broken := "missing equals\n"
	editMacroForTest(ed, broken)
	if ed.check() || ed.save(true) {
		t.Fatal("invalid substitutions were applied")
	}
	if substituteTTS("é") != "ay" {
		t.Fatal("failed check replaced active substitutions")
	}
	if !ed.save(false) {
		t.Fatal("could not save unfinished substitutions")
	}
	if data, _ := os.ReadFile(ed.doc.path); string(data) != broken {
		t.Fatal("unfinished file changed")
	}
	if openTTSSubstitutionEditor() != ed {
		t.Fatal("opening discarded the draft")
	}
}

func TestConfigEditorJSONFormattingAndHighlights(t *testing.T) {
	value := `{"clé":"é\\\"hi","enabled":true,"nothing":null,"value":-1.2e3}`
	formatted, err := formatJSONSource(value)
	if err != nil || !strings.Contains(formatted, "\n  \"clé\"") {
		t.Fatal("JSON was not formatted", err)
	}
	if again, err := formatJSONSource(formatted); err != nil || again != formatted {
		t.Fatal("JSON format is not stable", err)
	}
	if _, err := formatJSONSource(`{"unfinished":`); err == nil {
		t.Fatal("invalid JSON formatted")
	}
	colors := eui.DefaultSyntaxColors(eui.ColorBlack)
	for _, tc := range []struct {
		source, fragment string
		color            eui.Color
	}{
		{value, "clé", colors.Variables}, {value, "hi", colors.Strings}, {value, "true", colors.Keywords}, {value, "null", colors.Keywords}, {value, "-1.2e3", colors.Numbers},
		{`{"clé":"unfinished`, "unfinished", colors.Strings},
	} {
		spans := highlightJSONSource(tc.source, colors)
		checkGoHighlightSpans(t, tc.source, spans)
		start := utf8.RuneCountInString(tc.source[:strings.Index(tc.source, tc.fragment)])
		for i := start; i < start+utf8.RuneCountInString(tc.fragment); i++ {
			found := false
			for _, span := range spans {
				if i >= span.Start && i < span.End && span.Color == tc.color {
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("%q was not highlighted", tc.fragment)
			}
		}
	}
	checkGoHighlightSpans(t, "# é\né=ay\n", highlightTTSSubstitutions("# é\né=ay\n", colors))
}

func TestThemeSourceEditorsCheckWithoutApplying(t *testing.T) {
	macroEditorFixture(t)
	oldGS, oldDirty := gs, settingsDirty
	oldTheme, oldStyle, accent, saturation := eui.CurrentThemeName(), eui.CurrentStyleName(), eui.AccentColor(), eui.AccentSaturation()
	eui.SetUserDataRoot(t.TempDir())
	t.Cleanup(func() {
		eui.SetUserDataRoot("")
		_ = eui.LoadTheme(oldTheme)
		_ = eui.LoadStyle(oldStyle)
		eui.SetAccentSaturation(saturation)
		eui.SetAccentColor(accent)
		gs, settingsDirty = oldGS, oldDirty
	})
	if err := eui.LoadTheme("AccentDark"); err != nil {
		t.Fatal(err)
	}
	ed := openThemeSourceEditor(false)
	if ed == nil {
		t.Fatal("theme editor did not open")
	}
	before := eui.CurrentSyntaxColors()
	editMacroForTest(ed, `{"Input":{"Color":"#121416ff"},"Syntax":{"Keywords":"#010203ff"}}`)
	if !ed.check() || eui.CurrentSyntaxColors() != before {
		t.Fatal("Check changed the palette")
	}
	if !ed.save(true) || eui.CurrentSyntaxColors().Keywords != eui.NewColor(1, 2, 3, 255) || gs.Theme != "AccentDark" {
		t.Fatal("palette was not applied", ed.message)
	}
	editMacroForTest(ed, `{"Input":{"Color":"not-a-color"}}`)
	before = eui.CurrentSyntaxColors()
	if ed.save(true) || eui.CurrentSyntaxColors() != before {
		t.Fatal("invalid theme affected active palette")
	}
	style := openThemeSourceEditor(true)
	if style == nil {
		t.Fatal("style editor did not open")
	}
	editMacroForTest(style, `{"TextPadding":6}`)
	if !style.check() || !style.save(true) || gs.Style != eui.CurrentStyleName() {
		t.Fatal("style did not apply", style.message)
	}
}
