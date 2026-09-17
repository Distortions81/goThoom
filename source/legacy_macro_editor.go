package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"golang.org/x/text/encoding/charmap"
)

// The editor keeps normalized text separately from the original file bytes so
// opening and saving an unchanged legacy file never rewrites it.
type legacyMacroDocument struct {
	targetPath    string
	path          string
	original      []byte
	savedText     string
	newline       string
	macRoman, bom bool
}

func normalizeMacroEditorText(value string) string {
	return strings.ReplaceAll(strings.ReplaceAll(value, "\r\n", "\n"), "\r", "\n")
}

func loadLegacyMacroDocument(path string) (*legacyMacroDocument, error) {
	if isWASM {
		return nil, fmt.Errorf("embedded macros are read-only")
	}
	path, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	targetPath, err := filepath.EvalSymlinks(path)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("choose a regular macro file")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	decoded := decodeLegacyMacroSourceText(raw)
	newline := "\n"
	if i := strings.IndexAny(decoded, "\r\n"); i >= 0 && decoded[i] == '\r' {
		newline = "\r"
		if i+1 < len(decoded) && decoded[i+1] == '\n' {
			newline = "\r\n"
		}
	}
	return &legacyMacroDocument{path: path, targetPath: targetPath, original: raw, savedText: normalizeMacroEditorText(decoded),
		newline: newline, macRoman: !utf8.Valid(bytes.TrimPrefix(raw, []byte{0xef, 0xbb, 0xbf})),
		bom: bytes.HasPrefix(raw, []byte{0xef, 0xbb, 0xbf})}, nil
}

func (doc *legacyMacroDocument) changed(value string) bool {
	return normalizeMacroEditorText(value) != doc.savedText
}

func (doc *legacyMacroDocument) encode(value string) ([]byte, error) {
	value = strings.ReplaceAll(normalizeMacroEditorText(value), "\n", doc.newline)
	data := []byte(value)
	if doc.macRoman {
		var err error
		data, err = charmap.Macintosh.NewEncoder().Bytes(data)
		if err != nil {
			return nil, fmt.Errorf("this file uses MacRoman; an added character cannot be saved in that encoding")
		}
	}
	if doc.bom {
		data = append([]byte{0xef, 0xbb, 0xbf}, data...)
	}
	return data, nil
}

func (doc *legacyMacroDocument) save(value string) error {
	if isWASM {
		return fmt.Errorf("embedded macros are read-only")
	}
	targetPath, err := filepath.EvalSymlinks(doc.path)
	if err != nil {
		return fmt.Errorf("read before saving: %w", err)
	}
	if targetPath != doc.targetPath {
		return fmt.Errorf("the macro file now points to a different location; reopen it before saving")
	}
	current, err := os.ReadFile(doc.path)
	if err != nil {
		return fmt.Errorf("read before saving: %w", err)
	}
	if !bytes.Equal(current, doc.original) {
		return fmt.Errorf("the file changed outside this editor; copy your edits before closing and reopening it")
	}
	if !doc.changed(value) {
		return nil
	}
	data, err := doc.encode(value)
	if err != nil {
		return err
	}
	info, err := os.Stat(doc.path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("the macro path is no longer a regular file")
	}
	if err := legacyMacroAtomicWriteFile(doc.targetPath, data, info.Mode().Perm()); err != nil {
		return fmt.Errorf("save macro: %w", err)
	}
	doc.original, doc.savedText = data, normalizeMacroEditorText(value)
	return nil
}

// Checking includes the draft root and its files on disk, without running any
// macro or changing the active character's macros.
func (doc *legacyMacroDocument) check(value string) legacyMacroProgram {
	root := legacyMacroSource{Path: doc.path, Text: normalizeMacroEditorText(value)}
	return parseLegacyMacroSourcesWithReader([]legacyMacroSource{root}, func(path string) (legacyMacroSource, bool, error) {
		if filepath.Clean(path) == doc.path {
			return root, true, nil
		}
		return readLegacyMacroSource(path)
	})
}
