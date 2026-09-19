package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

//go:embed data/Tunes/*.tune
var bundledBardTunes embed.FS

// Install each included tune once. Existing files, edits, and later deletions
// belong to the player; a new client version never replaces them.
func installBundledBardTunes() error {
	if isWASM {
		return nil
	}
	dir := bardTunesDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	manifestPath := filepath.Join(dir, ".included-tunes.json")
	installed := make(map[string]bool)
	if data, err := os.ReadFile(manifestPath); err == nil {
		if err := json.Unmarshal(data, &installed); err != nil {
			return fmt.Errorf("read included tune list: %w", err)
		}
		if installed == nil {
			installed = make(map[string]bool)
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	entries, err := bundledBardTunes.ReadDir("data/Tunes")
	if err != nil {
		return err
	}
	existing, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	changed := false
	for _, entry := range entries {
		if installed[entry.Name()] {
			continue
		}
		// The embedded legacy name is the stable install-history key. Keep
		// it so an upgrade never reinstalls a deleted or renamed copy.
		nativeName := bardNativeFilename(entry.Name())
		found := false
		for _, file := range existing {
			found = found || strings.EqualFold(file.Name(), entry.Name()) || strings.EqualFold(file.Name(), nativeName)
		}
		if !found {
			data, err := bundledBardTunes.ReadFile("data/Tunes/" + entry.Name())
			if err != nil {
				return err
			}
			if _, err := createBardTune(nativeName, string(data)); err != nil {
				return err
			}
		}
		installed[entry.Name()], changed = true, true
	}
	if !changed {
		return nil
	}
	data, err := json.MarshalIndent(installed, "", "  ")
	if err != nil {
		return err
	}
	return legacyMacroAtomicWriteFile(manifestPath, append(data, '\n'), 0644)
}
