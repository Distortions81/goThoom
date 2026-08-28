package main

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const bundledScriptDir = "script_library"

type scriptLibraryEntry struct {
	ID          string
	Name        string
	Author      string
	Description string
	Filename    string
}

func scriptLibraryEntries() ([]scriptLibraryEntry, error) {
	files, err := scriptScripts.ReadDir(bundledScriptDir)
	if err != nil {
		return nil, err
	}
	entries := make([]scriptLibraryEntry, 0, len(files))
	for _, file := range files {
		if file.IsDir() || !isUserScriptFile(file.Name()) {
			continue
		}
		source, err := scriptScripts.ReadFile(path.Join(bundledScriptDir, file.Name()))
		if err != nil {
			return nil, fmt.Errorf("read bundled script %s: %w", file.Name(), err)
		}
		base := strings.TrimSuffix(file.Name(), ".go")
		id := scriptMetadataValue(source, "scriptID")
		if id == "" {
			id = normalizeScriptID(base)
		}
		name := scriptMetadataValue(source, "scriptName")
		if name == "" {
			name = base
		}
		entries = append(entries, scriptLibraryEntry{
			ID:          id,
			Name:        name,
			Author:      scriptMetadataValue(source, "scriptAuthor"),
			Description: scriptMetadataValue(source, "scriptDescription"),
			Filename:    file.Name(),
		})
	}
	sort.Slice(entries, func(i, j int) bool {
		return strings.ToLower(entries[i].Name) < strings.ToLower(entries[j].Name)
	})
	return entries, nil
}

func scriptMetadataValue(source []byte, key string) string {
	expression := regexp.MustCompile(`(?m)^\s*(?:var|const)\s+` + regexp.QuoteMeta(key) + `\s*=\s*"([^"]*)"`)
	match := expression.FindSubmatch(source)
	if len(match) < 2 {
		return ""
	}
	return strings.TrimSpace(string(match[1]))
}

func installBundledScript(dir, filename string) (string, error) {
	if filepath.Base(filename) != filename || !isUserScriptFile(filename) {
		return "", fmt.Errorf("invalid bundled script name %q", filename)
	}
	source, err := scriptScripts.ReadFile(path.Join(bundledScriptDir, filename))
	if err != nil {
		return "", fmt.Errorf("read bundled script %q: %w", filename, err)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create scripts folder: %w", err)
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return "", fmt.Errorf("open scripts folder: %w", err)
	}
	defer root.Close()
	destination := filepath.Join(dir, filename)
	file, err := root.OpenFile(filename, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		if os.IsExist(err) {
			return "", fmt.Errorf("%s already exists; it was not changed: %w", filename, os.ErrExist)
		}
		return "", err
	}
	remove := true
	defer func() {
		if remove {
			_ = root.Remove(filename)
		}
	}()
	if _, err := file.Write(source); err != nil {
		_ = file.Close()
		return "", err
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return "", err
	}
	if err := file.Close(); err != nil {
		return "", err
	}
	remove = false
	return destination, nil
}

// populateBundledScriptsIfEmpty installs the embedded examples only when the
// folder has no script packages. Managed editor files do not count as user
// scripts, and no existing script is ever replaced.
func populateBundledScriptsIfEmpty(dir string) error {
	if len(discoverScriptPackages(dir)) != 0 {
		return nil
	}
	entries, err := scriptLibraryEntries()
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if _, err := installBundledScript(dir, entry.Filename); err != nil {
			return err
		}
	}
	return nil
}
