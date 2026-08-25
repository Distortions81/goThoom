package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestBundledScriptLibraryListsEveryExample(t *testing.T) {
	entries, err := scriptLibraryEntries()
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) < 10 {
		t.Fatalf("example count = %d", len(entries))
	}
	seenIDs := map[string]bool{}
	seenFiles := map[string]bool{}
	for _, entry := range entries {
		if entry.ID == "" || entry.Name == "" || entry.Filename == "" {
			t.Fatalf("incomplete library entry: %+v", entry)
		}
		if seenIDs[entry.ID] {
			t.Fatalf("duplicate example ID %q", entry.ID)
		}
		if seenFiles[entry.Filename] {
			t.Fatalf("duplicate example filename %q", entry.Filename)
		}
		seenIDs[entry.ID] = true
		seenFiles[entry.Filename] = true
	}
}

func TestInstallBundledScriptNeverOverwritesLocalFile(t *testing.T) {
	entries, err := scriptLibraryEntries()
	if err != nil || len(entries) == 0 {
		t.Fatalf("read library: %v", err)
	}
	dir := t.TempDir()
	entry := entries[0]
	path, err := installBundledScript(dir, entry.Filename)
	if err != nil {
		t.Fatalf("install: %v", err)
	}
	if filepath.Base(path) != entry.Filename {
		t.Fatalf("installed path = %q", path)
	}
	const localEdit = "// my local edit\n"
	if err := os.WriteFile(path, []byte(localEdit), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := installBundledScript(dir, entry.Filename); !errors.Is(err, os.ErrExist) {
		t.Fatalf("second install error = %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != localEdit {
		t.Fatalf("local edit was overwritten: %q", data)
	}
	if _, err := installBundledScript(dir, "../escape.go"); err == nil {
		t.Fatal("path traversal was accepted")
	}
}
