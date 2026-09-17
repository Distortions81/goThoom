package eui

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestThemeSourceValidationDoesNotApply(t *testing.T) {
	isolateThemeTest(t)
	if err := LoadTheme("AccentDark"); err != nil {
		t.Fatal(err)
	}
	previous, style, colors, name := currentTheme, currentStyle, namedColors, CurrentThemeName()
	for index, source := range []string{`{"Colors":{"accent":"#ff0000ff"},"Input":{"Color":"accent"}}`, `{"Input":{"Color":"missing"}}`, `{"Colors":{"a":"b","b":"a"}}`, `{"Slider":{"SliderFilled":"missing"}}`, `null`, `[]`, `{`} {
		err := ValidateThemeSource([]byte(source))
		if index == 0 {
			if err != nil {
				t.Fatal(err)
			}
		} else if err == nil {
			t.Fatalf("accepted invalid palette %s", source)
		}
		if currentTheme != previous || currentStyle != style || CurrentThemeName() != name || !reflect.DeepEqual(namedColors, colors) {
			t.Fatal("Check changed the active theme")
		}
	}
	if err := ValidateStyleSource([]byte(`{"TextPadding":12}`)); err != nil {
		t.Fatal(err)
	}
	for _, source := range []string{`{"TextPadding":"bad"}`, `null`, `[]`} {
		if err := ValidateStyleSource([]byte(source)); err == nil {
			t.Fatalf("accepted invalid style %s", source)
		}
	}
	if currentStyle != style {
		t.Fatal("Check changed the active style")
	}
}

func TestEditableThemeCopiesPreserveUserFiles(t *testing.T) {
	isolateThemeTest(t)
	for _, style := range []bool{false, true} {
		name, folder := "AccentDark", "palettes"
		if style {
			name, folder = "Default", "styles"
		}
		filename, err := EditableThemeFile(name, style)
		if err != nil {
			t.Fatal(err)
		}
		if filename != filepath.Join(themeDirectory, folder, name+".json") {
			t.Fatal(filename)
		}
		data, err := os.ReadFile(filename)
		if err != nil {
			t.Fatal(err)
		}
		if style {
			err = ValidateStyleSource(data)
		} else {
			err = ValidateThemeSource(data)
		}
		if err != nil {
			t.Fatal(err)
		}
		if !style && !bytes.Contains(data, []byte(`"accent"`)) {
			t.Fatal("copy lost named references")
		}
		custom := []byte(`{"Comment":"keep user edits"}`)
		if err := os.WriteFile(filename, custom, 0644); err != nil {
			t.Fatal(err)
		}
		if _, err := EditableThemeFile(name, style); err != nil {
			t.Fatal(err)
		}
		data, _ = os.ReadFile(filename)
		if !bytes.Equal(data, custom) {
			t.Fatal("existing user theme was replaced")
		}
	}
	for _, name := range []string{"", "..", "../Default", `foo\bar`} {
		if _, err := EditableThemeFile(name, false); err == nil {
			t.Fatalf("accepted %q", name)
		}
	}
}

func TestThemeAndStyleSourcesUseUnicode(t *testing.T) {
	isolateThemeTest(t)
	for _, style := range []bool{false, true} {
		name := "AccentDark"
		if style {
			name = "Default"
		}
		filename, err := EditableThemeFile(name, style)
		if err != nil {
			t.Fatal(err)
		}
		unicode := append([]byte{0xef, 0xbb, 0xbf}, []byte(`{"Comment":"Café 你好 🌟"}`)...)
		if err := os.WriteFile(filename, unicode, 0644); err != nil {
			t.Fatal(err)
		}
		load, check := LoadTheme, ValidateThemeSource
		if style {
			load, check = LoadStyle, ValidateStyleSource
		}
		if err := check(unicode); err != nil {
			t.Fatal(err)
		}
		if err := load(name); err != nil {
			t.Fatal(err)
		}
		invalid := []byte{'{', '"', 'C', 'o', 'm', 'm', 'e', 'n', 't', '"', ':', '"', 0x8e, '"', '}'}
		if err := check(invalid); err == nil {
			t.Fatal("Check accepted invalid UTF-8")
		}
		if err := os.WriteFile(filename, invalid, 0644); err != nil {
			t.Fatal(err)
		}
		if err := load(name); err == nil {
			t.Fatal("loading accepted invalid UTF-8")
		}
	}
}
