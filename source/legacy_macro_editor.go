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
		kind:           "Macro",
		description:    "Edits apply to every character using this file. Save & Reload also reloads the selected session's enabled macros.",
		checkedMessage: "No macro syntax errors found.",
		savedMessage:   "Saved. Use Save & Reload to apply it to this session's enabled macros.",
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
