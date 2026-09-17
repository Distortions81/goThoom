package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"gothoom/eui"
)

func createEditorFile(filename string, initial []byte) error {
	if isWASM {
		return fmt.Errorf("file editing is available in the desktop client")
	}
	if err := os.MkdirAll(filepath.Dir(filename), 0755); err != nil {
		return err
	}
	f, err := os.OpenFile(filename, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if os.IsExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	_, err = f.Write(initial)
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		_ = os.Remove(filename)
	}
	return err
}

func openTextFileEditor(filename string, options sourceEditorOptions) *sourceEditor {
	doc, err := loadSourceDocument(filename, false)
	if err != nil {
		consoleMessage("[editor] " + err.Error())
		return nil
	}
	return openSourceEditor(doc, options)
}

func openTTSSubstitutionEditor() *sourceEditor {
	filename := filepath.Join(ttsDataDirPath(), ttsSubstituteFile)
	if err := createEditorFile(filename, defaultTTSSubstitute); err != nil {
		consoleMessage("[editor] " + err.Error())
		return nil
	}
	return openTextFileEditor(filename, sourceEditorOptions{
		kind: "TTS Substitutions", highlight: highlightTTSSubstitutions,
		check:       func(value string) error { _, err := parseTTSSubstitutions(value, true); return err },
		reloadLabel: "Save & Apply", reloadTooltip: "Save substitutions and apply them to future speech.",
		reload: func() (string, error) {
			data, err := os.ReadFile(filename)
			if err != nil {
				return "", err
			}
			subs, err := parseTTSSubstitutions(string(data), true)
			if err != nil {
				return "", err
			}
			ttsSubsMu.Lock()
			ttsSubs = subs
			ttsSubsMu.Unlock()
			return "Saved; substitutions will be used for future speech.", nil
		},
	})
}

func parseTTSSubstitutions(value string, strict bool) (map[string]string, error) {
	if !utf8.ValidString(value) {
		return nil, fmt.Errorf("TTS substitutions must use UTF-8 encoding")
	}
	subs := map[string]string{}
	for index, line := range strings.Split(strings.TrimPrefix(value, "\ufeff"), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		from, to, ok := strings.Cut(line, "=")
		from = strings.TrimSpace(from)
		if !ok || from == "" {
			if strict {
				return nil, fmt.Errorf("line %d: use original=replacement with a nonempty original", index+1)
			}
			continue
		}
		subs[from] = strings.TrimSpace(to)
	}
	return subs, nil
}

func formatJSONSource(value string) (string, error) {
	var out bytes.Buffer
	if err := json.Indent(&out, []byte(value), "", "  "); err != nil {
		return "", err
	}
	return strings.TrimSpace(out.String()) + "\n", nil
}

func openThemeSourceEditor(style bool) *sourceEditor {
	if isWASM {
		return nil
	}
	if restoreThemePreview != nil {
		restoreThemePreview()
	}
	name, kind := eui.CurrentThemeName(), "Color Theme"
	validate := eui.ValidateThemeSource
	if style {
		name, kind, validate = eui.CurrentStyleName(), "Style Theme", eui.ValidateStyleSource
	}
	filename, err := eui.EditableThemeFile(name, style)
	if err != nil {
		consoleMessage("[editor] " + err.Error())
		return nil
	}
	return openTextFileEditor(filename, sourceEditorOptions{
		kind: kind, highlight: highlightJSONSource, format: formatJSONSource, formatPosition: remapGoFormattedPosition, colorSwatches: true,
		check:       func(value string) error { return validate([]byte(value)) },
		reloadLabel: "Save & Apply", reloadTooltip: "Save and select this " + strings.ToLower(kind) + ".",
		reload: func() (string, error) {
			if restoreThemePreview != nil {
				restoreThemePreview()
			}
			if style {
				if err := eui.LoadStyle(name); err != nil {
					return "", err
				}
				gs.Style = name
			} else {
				if err := loadThemeChoice(name); err != nil {
					return "", err
				}
				gs.Theme, gs.Style = name, eui.CurrentStyleName()
			}
			settingsDirty = true
			refreshThemePreview()
			return "Saved; " + name + " is now the selected " + strings.ToLower(kind) + ".", nil
		},
	})
}
