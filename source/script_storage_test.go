//go:build integration
// +build integration

package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestScriptStorageSetGetDelete(t *testing.T) {
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
	sum := sha256.Sum256([]byte(owner))
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
	m = map[string]any{}
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, ok := m["foo"]; ok {
		t.Fatalf("value not deleted: %v", m)
	}
}

func TestScriptStorageRejectsUnsupportedValuesImmediately(t *testing.T) {
	withoutConsoleTimestamps(t)
	origDir := dataDirPath
	dataDirPath = t.TempDir()
	t.Cleanup(func() { dataDirPath = origDir })

	const owner = "storage_test"
	scriptDisplayNames = map[string]string{owner: "Storage Test"}
	scriptAuthors = map[string]string{owner: "Test"}
	scriptStores = map[string]*scriptStore{}
	scriptStoreMu = sync.Mutex{}
	consoleLog = messageLog{max: maxMessages}

	if !scriptStorageSet(owner, "value", "kept") {
		t.Fatal("supported string value was rejected")
	}
	savescriptStores()
	store := getscriptStore(owner)
	if store.dirty {
		t.Fatal("saved store remained dirty")
	}

	if scriptStorageSet(owner, "value", func() {}) {
		t.Fatal("function value was accepted")
	}
	if got := scriptStorageGet(owner, "value"); got != "kept" {
		t.Fatalf("rejected value replaced existing data: %v", got)
	}
	if store.dirty {
		t.Fatal("rejected value marked the store dirty")
	}
	drainScriptDispatcher()
	msgs := getConsoleMessages()
	if len(msgs) == 0 || !strings.Contains(msgs[len(msgs)-1], `cannot store "value"`) ||
		!strings.Contains(msgs[len(msgs)-1], "unsupported type: func()") {
		t.Fatalf("unsupported value error was not reported immediately: %v", msgs)
	}

	consoleLog = messageLog{max: maxMessages}
	candidate := &scriptCandidate{}
	candidate.setStorage(owner, "candidate", make(chan int))
	if got := candidate.getStorage(owner, "candidate"); got != nil {
		t.Fatalf("candidate staged unsupported value: %v", got)
	}
	drainScriptDispatcher()
	if msgs := getConsoleMessages(); len(msgs) == 0 || !strings.Contains(msgs[len(msgs)-1], `cannot store "candidate"`) {
		t.Fatalf("candidate did not report unsupported value during Init: %v", msgs)
	}
}
