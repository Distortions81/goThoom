package main

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestTypedScriptStorageReaders(t *testing.T) {
	if got := scriptStoredString("value", "fallback"); got != "value" {
		t.Fatalf("string = %q", got)
	}
	if got := scriptStoredBool(true, false); !got {
		t.Fatal("bool was not loaded")
	}
	if got := scriptStoredInteger(float64(42), 0); got != 42 {
		t.Fatalf("integer = %d", got)
	}
	if got := scriptStoredInteger(4.5, 9); got != 9 {
		t.Fatalf("fractional integer did not use fallback: %d", got)
	}
	if got := scriptStoredDecimal(3, 0); got != 3 {
		t.Fatalf("decimal = %v", got)
	}
	stringsValue := scriptStoredStrings([]any{"a", "b"}, nil)
	if len(stringsValue) != 2 || stringsValue[0] != "a" || stringsValue[1] != "b" {
		t.Fatalf("strings = %v", stringsValue)
	}
	stringsValue[0] = "changed"
	original := []string{"kept"}
	loaded := scriptStoredStrings(original, nil)
	loaded[0] = "changed"
	if original[0] != "kept" {
		t.Fatal("loaded string slice shares caller-owned storage")
	}
}

func TestScriptStoredJSON(t *testing.T) {
	type state struct {
		Name  string
		Count int
	}
	var loaded state
	if !scriptStoredJSON(map[string]any{"Name": "example", "Count": float64(3)}, &loaded) {
		t.Fatal("JSON-safe struct was not decoded")
	}
	if loaded.Name != "example" || loaded.Count != 3 {
		t.Fatalf("decoded JSON = %+v", loaded)
	}
	if scriptStoredJSON(map[string]any{}, loaded) {
		t.Fatal("non-pointer JSON target was accepted")
	}
}

func TestMigrateStorageRunsOnceAndStagesVersion(t *testing.T) {
	originalDir := dataDirPath
	dataDirPath = t.TempDir()
	t.Cleanup(func() { dataDirPath = originalDir })
	scriptStores = map[string]*scriptStore{}
	scriptStoreMu = sync.Mutex{}

	const owner = "migration-test"
	setScriptStorageValue(owner, scriptStorageVersionKey, 1)
	candidate := &scriptCandidate{}
	exports := exportsForScriptCandidate(owner, candidate)["gt2/gt2"]
	migrate := exports["MigrateStorage"].Interface().(func(int, func(int)))
	store := exports["Store"].Interface().(func(string, any))
	calls := 0
	migrate(2, func(from int) {
		calls++
		if from != 1 {
			t.Fatalf("migration started at version %d", from)
		}
		store("migrated", true)
	})
	if got := scriptStorageVersion(owner); got != 1 {
		t.Fatalf("staged migration changed live version to %d", got)
	}
	candidate.activate(nil)
	if got := scriptStorageVersion(owner); got != 2 {
		t.Fatalf("activated version = %d", got)
	}
	if got := scriptStorageGet(owner, "migrated"); got != true {
		t.Fatalf("migrated value = %v", got)
	}
	migrate(2, func(int) { calls++ })
	if calls != 1 {
		t.Fatalf("migration ran %d times", calls)
	}
}

func TestWriteFileAtomicReplacesFileAndCleansTemporary(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "storage.json")
	if err := os.WriteFile(path, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := writeFileAtomic(path, []byte("new"), 0o600); err != nil {
		t.Fatalf("atomic write: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "new" {
		t.Fatalf("stored data = %q", data)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("stored mode = %o", info.Mode().Perm())
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "storage.json" {
		t.Fatalf("temporary files remain: %v", entries)
	}
}
