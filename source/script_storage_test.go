//go:build integration
// +build integration

package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestscriptStorageSetGetDelete(t *testing.T) {
	origDir := dataDirPath
	dataDirPath = t.TempDir()
	t.Cleanup(func() { dataDirPath = origDir })

	owner := "plug_file"
	scriptDisplayNames = map[string]string{owner: "Plug"}
	scriptAuthors = map[string]string{owner: "Auth"}
	scriptStores = map[string]*scriptStore{}
	scriptStoreMu = sync.Mutex{}

	if v := scriptStorageGet(owner, "foo"); v != nil {
		t.Fatalf("expected nil, got %v", v)
	}

	scriptStorageSet(owner, "foo", "bar")
	if v := scriptStorageGet(owner, "foo"); v != "bar" {
		t.Fatalf("got %v, want bar", v)
	}

	store := getscriptStore(owner)
	if !store.dirty {
		t.Fatalf("store not marked dirty")
	}

	savescriptStores()

	if store.dirty {
		t.Fatalf("store still dirty after save")
	}

	path := scriptStoragePath(owner)
	sum := sha256.Sum256([]byte("Plug:Auth"))
	wantFile := hex.EncodeToString(sum[:]) + ".json"
	if filepath.Base(path) != wantFile {
		t.Fatalf("path %s does not match hash %s", filepath.Base(path), wantFile)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read storage: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if m["foo"] != "bar" {
		t.Fatalf("file contents %v", m)
	}

	scriptStorageDelete(owner, "foo")
	savescriptStores()
	data, err = os.ReadFile(path)
	if err != nil {
		t.Fatalf("read storage: %v", err)
	}
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, ok := m["foo"]; ok {
		t.Fatalf("value not deleted: %v", m)
	}
}
