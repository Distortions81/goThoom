package main

import (
	"fmt"
	"path/filepath"
)

func loadLegacyMacroDocument(path string) (*sourceDocument, error) {
	return loadSourceDocument(path, true)
}

func openLegacyMacroEditor(entry legacyMacroLibraryEntry) *sourceEditor {
	doc, err := loadLegacyMacroDocument(entry.Path)
	if err != nil {
		legacyMacroLibraryReport("open macro editor: " + err.Error())
		return nil
	}
	return openSourceEditor(doc, sourceEditorOptions{
		kind:      "Macro",
		highlight: highlightLegacyMacro,
		format:    formatLegacyMacro,
		lint: func(value string) []string {
			var warnings []string
			for _, warning := range lintLegacyMacroProgram(doc.check(value)) {
				warnings = append(warnings, warning.Error())
			}
			return warnings
		},
		reloadTooltip: "Save the shared macro file and reload the selected session's enabled macros.",
		check: func(value string) error {
			program := doc.check(value)
			if len(program.Diagnostics) == 0 {
				return nil
			}
			for _, diagnostic := range program.Diagnostics {
				legacyMacroLibraryReport(diagnostic.Error())
			}
			first := program.Diagnostics[0]
			return fmt.Errorf("%s:%d:%d: %s. Details are in Console.", filepath.Base(first.Location.Path), first.Location.Line, first.Location.Column, first.Message)
		},
		reload: func() (string, error) {
			err := legacyMacroLibrarySession().loadLegacyMacrosForCharacter(legacyMacroLibraryCurrentCharacter())
			return "Saved; selected session's enabled macros reloaded.", err
		},
		afterSave: refreshLegacyMacroLibraryWindow,
	})
}

// Checking includes the draft root and its files on disk, without running any
// macro or changing the active character's macros.
func (doc *sourceDocument) check(value string) legacyMacroProgram {
	root := legacyMacroSource{Path: doc.path, Text: normalizeSourceEditorText(value)}
	return parseLegacyMacroSourcesWithReader([]legacyMacroSource{root}, func(path string) (legacyMacroSource, bool, error) {
		if filepath.Clean(path) == doc.path {
			return root, true, nil
		}
		return readLegacyMacroSource(path)
	})
}
