package main

import (
	"encoding/json"
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

func TestSyncBundledScriptsPreservesEditsAndUpdatesPristineCopies(t *testing.T) {
	entries, err := scriptLibraryEntries()
	if err != nil {
		t.Fatal(err)
	}
	for _, scenario := range []string{"pristine", "edited", "unknown", "damaged", "null", "missing", "historical"} {
		t.Run(scenario, func(t *testing.T) {
			dir := t.TempDir()
			if err := syncBundledScripts(dir); err != nil {
				t.Fatal(err)
			}
			entry := entries[0]
			filename := filepath.Join(dir, entry.Filename)
			desired, err := os.ReadFile(filename)
			if err != nil {
				t.Fatal(err)
			}
			old := []byte("package main\n// previous included version\n")
			manifest, err := readBundledScriptManifest(dir)
			if err != nil {
				t.Fatal(err)
			}
			manifest[entry.Filename] = bundledScriptHash(old)
			current := old
			if scenario == "edited" {
				current = append(append([]byte{}, old...), []byte("// personal edit\n")...)
			}
			if err := os.WriteFile(filename, current, 0o644); err != nil {
				t.Fatal(err)
			}
			data, err := json.Marshal(manifest)
			if err != nil {
				t.Fatal(err)
			}
			switch scenario {
			case "historical":
				oldHashes := previousBundledScriptHashes[entry.Filename]
				previousBundledScriptHashes[entry.Filename] = map[string]bool{bundledScriptHash(old): true}
				t.Cleanup(func() { previousBundledScriptHashes[entry.Filename] = oldHashes })
				data = []byte("{}")
			case "unknown":
				data = []byte("{}")
			case "damaged":
				data = []byte("{")
			case "null":
				data = []byte("null")
			case "missing":
				if err := os.Remove(filename); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.WriteFile(filepath.Join(dir, bundledScriptManifestName), data, 0o644); err != nil {
				t.Fatal(err)
			}
			for range 2 {
				if err := syncBundledScripts(dir); err != nil {
					t.Fatal(err)
				}
				got, err := os.ReadFile(filename)
				if err != nil {
					t.Fatal(err)
				}
				want := current
				if scenario == "pristine" || scenario == "missing" || scenario == "historical" {
					want = desired
				}
				if string(got) != string(want) {
					t.Fatalf("unexpected content: %q", got)
				}
			}
		})
	}
}

func TestSyncBundledScriptsAdoptsExistingExactCopy(t *testing.T) {
	dir := t.TempDir()
	entries, err := scriptLibraryEntries()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := installBundledScript(dir, entries[0].Filename); err != nil {
		t.Fatal(err)
	}
	if err := syncBundledScripts(dir); err != nil {
		t.Fatal(err)
	}
	manifest, err := readBundledScriptManifest(dir)
	if err != nil {
		t.Fatal(err)
	}
	if manifest[entries[0].Filename] == "" {
		t.Fatal("existing included copy was not tracked")
	}
}

func TestSyncBundledScriptsPreservesSymlink(t *testing.T) {
	entries, err := scriptLibraryEntries()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	target := filepath.Join(t.TempDir(), "personal.go")
	const custom = "// personal script\n"
	if err := os.WriteFile(target, []byte(custom), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, entries[0].Filename)
	if err := os.Symlink(target, link); err != nil {
		t.Skip(err)
	}
	if err := syncBundledScripts(dir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Readlink(link); err != nil {
		t.Fatal("symlink was replaced:", err)
	}
	got, err := os.ReadFile(target)
	if err != nil || string(got) != custom {
		t.Fatalf("symlink target changed: %q, %v", got, err)
	}
}

func TestIncludedScriptLabels(t *testing.T) {
	original := dataDirPath
	dataDirPath = t.TempDir()
	t.Cleanup(func() { dataDirPath = original })
	dir := userScriptsDir()
	if err := syncBundledScripts(dir); err != nil {
		t.Fatal(err)
	}
	entries, err := scriptLibraryEntries()
	if err != nil {
		t.Fatal(err)
	}
	filename := filepath.Join(dir, entries[0].Filename)
	if got := includedScriptLabel(filename); got != "Included with goThoom" {
		t.Fatal(got)
	}
	if err := os.WriteFile(filename, []byte("// local edit"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := includedScriptLabel(filename); got != "Included with goThoom (modified)" {
		t.Fatal(got)
	}
	if got := includedScriptLabel(filepath.Join(t.TempDir(), entries[0].Filename)); got != "" {
		t.Fatal(got)
	}
}
