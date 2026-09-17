package eui

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

func validateThemeObject(data []byte) error {
	if !utf8.Valid(data) {
		return fmt.Errorf("themes and styles must use UTF-8 encoding")
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(data, &object); err != nil {
		return err
	}
	if object == nil {
		return fmt.Errorf("expected a JSON object")
	}
	return nil
}

func themeSourceText(data []byte) []byte {
	return bytes.TrimPrefix(data, []byte{0xef, 0xbb, 0xbf})
}

// ValidateThemeSource checks a palette without changing the active theme.
func ValidateThemeSource(data []byte) error {
	_, _, _, err := decodeThemeSource(data)
	return err
}

// ValidateStyleSource checks style values without changing the active style.
func ValidateStyleSource(data []byte) error {
	data = themeSourceText(data)
	if err := validateThemeObject(data); err != nil {
		return err
	}
	next := baseStyle
	return json.Unmarshal(data, &next)
}

// EditableThemeFile returns a user palette/style file. For a bundled choice,
// it creates an editable override from the original JSON, preserving references
// to named colors. Existing user files are never overwritten.
func EditableThemeFile(name string, style bool) (string, error) {
	if name == "" || strings.TrimSpace(name) != name || name == "." || name == ".." || strings.ContainsAny(name, `/\`) {
		return "", fmt.Errorf("invalid theme name")
	}
	group := "palettes"
	if style {
		group = "styles"
	}
	filename := filepath.Join(themeDirectory, group, name+".json")
	if info, err := os.Stat(filename); err == nil {
		if !info.Mode().IsRegular() {
			return "", fmt.Errorf("theme path is not a regular file")
		}
		return filename, nil
	} else if !os.IsNotExist(err) {
		return "", err
	}
	var data []byte
	var err error
	if style {
		data, err = embeddedStyles.ReadFile(path.Join("themes", group, name+".json"))
	} else {
		data, err = embeddedThemes.ReadFile(path.Join("themes", group, name+".json"))
	}
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(filename), 0755); err != nil {
		return "", err
	}
	f, err := os.OpenFile(filename, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if os.IsExist(err) {
		return filename, nil
	}
	if err != nil {
		return "", err
	}
	_, err = bytes.NewReader(data).WriteTo(f)
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		_ = os.Remove(filename)
		return "", err
	}
	return filename, nil
}
