package main

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"log"
	"os"
	"path/filepath"
)

// sanitizeAreaJSONs walks the provided script directories looking for
// area definition JSON files and attempts to repair common formatting
// mistakes that break the JSON decoder. Currently the only fixer
// removes stray '.' characters that sometimes get appended after a
// value.
func sanitizeAreaJSONs(dirs []string) {
	for _, dir := range dirs {
		if dir == "" {
			continue
		}
		areaDir := filepath.Join(dir, "areas")
		entries, err := os.ReadDir(areaDir)
		if err != nil {
			if !os.IsNotExist(err) {
				log.Printf("scan areas %s: %v", areaDir, err)
			}
			continue
		}
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			if filepath.Ext(entry.Name()) != ".json" {
				continue
			}
			path := filepath.Join(areaDir, entry.Name())
			if err := sanitizeAreaJSONFile(path, entry); err != nil {
				log.Printf("sanitize area %s: %v", entry.Name(), err)
			}
		}
	}
}

func sanitizeAreaJSONFile(path string, info fs.DirEntry) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if json.Valid(data) {
		return nil
	}
	cleaned, changed := fixAreaJSON(data)
	if !changed || !json.Valid(cleaned) {
		return nil
	}
	// Preserve original file mode when possible.
	var perm fs.FileMode = 0o644
	if info != nil {
		if fi, err := info.Info(); err == nil {
			perm = fi.Mode().Perm()
		}
	} else if fi, err := os.Stat(path); err == nil {
		perm = fi.Mode().Perm()
	}
	if err := os.WriteFile(path, cleaned, perm); err != nil {
		return err
	}
	log.Printf("sanitize area %s", filepath.Base(path))
	return nil
}

func fixAreaJSON(data []byte) ([]byte, bool) {
	var out bytes.Buffer
	out.Grow(len(data))
	inString := false
	escape := false
	changed := false
	for i := 0; i < len(data); i++ {
		c := data[i]
		if inString {
			out.WriteByte(c)
			if escape {
				escape = false
				continue
			}
			if c == '\\' {
				escape = true
				continue
			}
			if c == '"' {
				inString = false
			}
			continue
		}
		switch c {
		case '"':
			inString = true
			out.WriteByte(c)
			continue
		case '.':
			// Look ahead to see if this dot sits immediately before a
			// delimiter such as ',', '}', or ']'. If so, drop it; these
			// dots commonly appear after sentences that were copied into
			// JSON without removing the trailing period.
			j := i + 1
			for j < len(data) {
				if data[j] == ' ' || data[j] == '\t' || data[j] == '\n' || data[j] == '\r' {
					j++
					continue
				}
				break
			}
			if j < len(data) {
				next := data[j]
				if next == ',' || next == '}' || next == ']' {
					changed = true
					continue
				}
			}
		}
		out.WriteByte(c)
	}
	if !changed {
		return data, false
	}
	return out.Bytes(), true
}
