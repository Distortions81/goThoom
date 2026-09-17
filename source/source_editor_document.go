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
// opening and saving an unchanged source file never rewrites it.
type sourceDocument struct {
	targetPath    string
	path          string
	original      []byte
	savedText     string
	newline       string
	macRoman, bom bool
}

func normalizeSourceEditorText(value string) string {
	return strings.ReplaceAll(strings.ReplaceAll(value, "\r\n", "\n"), "\r", "\n")
}

func loadSourceDocument(path string, allowMacRoman bool) (*sourceDocument, error) {
	if isWASM {
		return nil, fmt.Errorf("source files are read-only in the browser")
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
		return nil, fmt.Errorf("choose a regular source file")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	text := bytes.TrimPrefix(raw, []byte{0xef, 0xbb, 0xbf})
	macRoman := allowMacRoman && !utf8.Valid(text)
	if !macRoman && !utf8.Valid(text) {
		return nil, fmt.Errorf("text files must use UTF-8 encoding")
	}
	decoded := string(text)
	if macRoman {
		decoded = decodeMacRoman(text)
	}
	newline := "\n"
	if i := strings.IndexAny(decoded, "\r\n"); i >= 0 && decoded[i] == '\r' {
		newline = "\r"
		if i+1 < len(decoded) && decoded[i+1] == '\n' {
			newline = "\r\n"
		}
	}
	return &sourceDocument{path: path, targetPath: targetPath, original: raw, savedText: normalizeSourceEditorText(decoded),
		newline: newline, macRoman: macRoman,
		bom: bytes.HasPrefix(raw, []byte{0xef, 0xbb, 0xbf})}, nil
}

func (doc *sourceDocument) changed(value string) bool {
	return normalizeSourceEditorText(value) != doc.savedText
}

func (doc *sourceDocument) encode(value string) ([]byte, error) {
	if !utf8.ValidString(value) {
		return nil, fmt.Errorf("text must be valid Unicode")
	}
	value = strings.ReplaceAll(normalizeSourceEditorText(value), "\n", doc.newline)
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

func (doc *sourceDocument) save(value string) error {
	if isWASM {
		return fmt.Errorf("source files are read-only in the browser")
	}
	targetPath, err := filepath.EvalSymlinks(doc.path)
	if err != nil {
		return fmt.Errorf("read before saving: %w", err)
	}
	if targetPath != doc.targetPath {
		return fmt.Errorf("the source file now points to a different location; reopen it before saving")
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
		return fmt.Errorf("the source path is no longer a regular file")
	}
	if err := legacyMacroAtomicWriteFile(doc.targetPath, data, info.Mode().Perm()); err != nil {
		return fmt.Errorf("save file: %w", err)
	}
	doc.original, doc.savedText = data, normalizeSourceEditorText(value)
	return nil
}
