package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"time"
)

type scriptStore struct {
	path  string
	data  map[string]any
	dirty bool
	mu    sync.Mutex
}

var (
	scriptStores  = map[string]*scriptStore{}
	scriptStoreMu sync.Mutex
)

func scriptStoragePath(owner string) string {
	scriptMu.RLock()
	name := scriptDisplayNames[owner]
	author := scriptAuthors[owner]
	scriptMu.RUnlock()
	sum := sha256.Sum256([]byte(name + ":" + author))
	file := hex.EncodeToString(sum[:]) + ".json"
	return filepath.Join(dataDirPath, "scripts", "storage", file)
}

func getscriptStore(owner string) *scriptStore {
	scriptStoreMu.Lock()
	ps, ok := scriptStores[owner]
	if ok {
		scriptStoreMu.Unlock()
		return ps
	}
	path := scriptStoragePath(owner)
	data := map[string]any{}
	if b, err := os.ReadFile(path); err == nil {
		if err := json.Unmarshal(b, &data); err != nil {
			log.Printf("load script storage %s: %v", path, err)
		}
	}
	ps = &scriptStore{path: path, data: data}
	scriptStores[owner] = ps
	scriptStoreMu.Unlock()
	return ps
}

func scriptStorageGet(owner, key string) any {
	ps := getscriptStore(owner)
	ps.mu.Lock()
	defer ps.mu.Unlock()
	return ps.data[key]
}

func scriptStorageSet(owner, key string, value any) {
	ps := getscriptStore(owner)
	ps.mu.Lock()
	if old, ok := ps.data[key]; !ok || !reflect.DeepEqual(old, value) {
		if ps.data == nil {
			ps.data = make(map[string]any)
		}
		ps.data[key] = value
		ps.dirty = true
	}
	ps.mu.Unlock()
}

func scriptStorageDelete(owner, key string) {
	ps := getscriptStore(owner)
	ps.mu.Lock()
	if _, ok := ps.data[key]; ok {
		delete(ps.data, key)
		ps.dirty = true
	}
	ps.mu.Unlock()
}

func savescriptStores() {
	if isWASM {
		// Skip persistence in WASM.
		return
	}
	scriptStoreMu.Lock()
	stores := make([]*scriptStore, 0, len(scriptStores))
	for _, ps := range scriptStores {
		stores = append(stores, ps)
	}
	scriptStoreMu.Unlock()
	for _, ps := range stores {
		ps.mu.Lock()
		if !ps.dirty {
			ps.mu.Unlock()
			continue
		}
		data, err := json.MarshalIndent(ps.data, "", "  ")
		if err != nil {
			ps.mu.Unlock()
			log.Printf("save script storage %s: %v", ps.path, err)
			continue
		}
		ps.dirty = false
		path := ps.path
		ps.mu.Unlock()
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			log.Printf("save script storage %s: %v", path, err)
			continue
		}
		if err := os.WriteFile(path, data, 0o644); err != nil {
			log.Printf("save script storage %s: %v", path, err)
		}
	}
}

func init() {
	go func() {
		ticker := time.NewTicker(time.Minute)
		for range ticker.C {
			savescriptStores()
		}
	}()
}
