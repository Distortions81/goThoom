package main

import (
	"crypto/sha256"
	"encoding/json"
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

// syncBundledScripts restores missing included scripts and updates only copies
// that still match the last installed content. Unknown local files are preserved.
func syncBundledScripts(dir string) error {
	entries, err := scriptLibraryEntries()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	manifest, err := readBundledScriptManifest(dir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		desired, err := scriptScripts.ReadFile(path.Join(bundledScriptDir, entry.Filename))
		if err != nil {
			return err
		}
		destination := filepath.Join(dir, entry.Filename)
		info, err := os.Lstat(destination)
		if os.IsNotExist(err) {
			if _, err := installBundledScript(dir, entry.Filename); err != nil {
				return err
			}
		} else if err != nil {
			return err
		} else {
			// A directory or symlink at this name belongs to the user.
			if !info.Mode().IsRegular() {
				continue
			}
			current, err := os.ReadFile(destination)
			if err != nil {
				return err
			}
			currentHash := bundledScriptHash(current)
			if currentHash != bundledScriptHash(desired) {
				if manifest[entry.Filename] != currentHash && !previousBundledScriptHashes[entry.Filename][currentHash] {
					continue
				}
				if err := legacyMacroAtomicWriteFile(destination, desired, info.Mode().Perm()); err != nil {
					return err
				}
			}
		}
		manifest[entry.Filename] = bundledScriptHash(desired)
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	return legacyMacroAtomicWriteFile(filepath.Join(dir, bundledScriptManifestName), append(data, '\n'), 0o644)
}

const bundledScriptManifestName = ".gothoom-bundled-scripts.json"

func bundledScriptHash(data []byte) string { return fmt.Sprintf("%x", sha256.Sum256(data)) }

func readBundledScriptManifest(dir string) (map[string]string, error) {
	data, err := os.ReadFile(filepath.Join(dir, bundledScriptManifestName))
	if os.IsNotExist(err) {
		return map[string]string{}, nil
	}
	if err != nil {
		return nil, err
	}
	var manifest map[string]string
	if json.Unmarshal(data, &manifest) != nil || manifest == nil {
		// Without a baseline, only exact current bundled copies may be adopted.
		manifest = map[string]string{}
	}
	return manifest, nil
}

func includedScriptLabel(scriptPath string) string {
	if filepath.Clean(filepath.Dir(scriptPath)) != filepath.Clean(userScriptsDir()) {
		return ""
	}
	filename := filepath.Base(scriptPath)
	desired, err := scriptScripts.ReadFile(path.Join(bundledScriptDir, filename))
	if err != nil || !isUserScriptFile(filename) {
		return ""
	}
	current, err := os.ReadFile(scriptPath)
	if err != nil {
		return ""
	}
	if bundledScriptHash(current) == bundledScriptHash(desired) {
		return "Included with goThoom"
	}
	manifest, err := readBundledScriptManifest(userScriptsDir())
	if err == nil && manifest[filename] != "" {
		return "Included with goThoom (modified)"
	}
	return ""
}
