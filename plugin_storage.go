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

type pluginStore struct {
	path  string
	data  map[string]any
	dirty bool
	mu    sync.Mutex
}

var (
	pluginStores  = map[string]*pluginStore{}
	pluginStoreMu sync.Mutex
)

func pluginStoragePath(owner string) string {
	pluginMu.RLock()
	name := pluginDisplayNames[owner]
	author := pluginAuthors[owner]
	pluginMu.RUnlock()
	sum := sha256.Sum256([]byte(name + ":" + author))
	file := hex.EncodeToString(sum[:]) + ".json"
	return filepath.Join(dataDirPath, "scripts", "storage", file)
}

func getPluginStore(owner string) *pluginStore {
	pluginStoreMu.Lock()
	ps, ok := pluginStores[owner]
	if ok {
		pluginStoreMu.Unlock()
		return ps
	}
	path := pluginStoragePath(owner)
	data := map[string]any{}
	if b, err := os.ReadFile(path); err == nil {
		if err := json.Unmarshal(b, &data); err != nil {
			log.Printf("load plugin storage %s: %v", path, err)
		}
	}
	ps = &pluginStore{path: path, data: data}
	pluginStores[owner] = ps
	pluginStoreMu.Unlock()
	return ps
}

func pluginStorageGet(owner, key string) any {
	ps := getPluginStore(owner)
	ps.mu.Lock()
	defer ps.mu.Unlock()
	return ps.data[key]
}

func pluginStorageSet(owner, key string, value any) {
	ps := getPluginStore(owner)
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

func pluginStorageDelete(owner, key string) {
	ps := getPluginStore(owner)
	ps.mu.Lock()
	if _, ok := ps.data[key]; ok {
		delete(ps.data, key)
		ps.dirty = true
	}
	ps.mu.Unlock()
}

func savePluginStores() {
	pluginStoreMu.Lock()
	stores := make([]*pluginStore, 0, len(pluginStores))
	for _, ps := range pluginStores {
		stores = append(stores, ps)
	}
	pluginStoreMu.Unlock()
	for _, ps := range stores {
		ps.mu.Lock()
		if !ps.dirty {
			ps.mu.Unlock()
			continue
		}
		data, err := json.MarshalIndent(ps.data, "", "  ")
		if err != nil {
			ps.mu.Unlock()
			log.Printf("save plugin storage %s: %v", ps.path, err)
			continue
		}
		ps.dirty = false
		path := ps.path
		ps.mu.Unlock()
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			log.Printf("save plugin storage %s: %v", path, err)
			continue
		}
		if err := os.WriteFile(path, data, 0o644); err != nil {
			log.Printf("save plugin storage %s: %v", path, err)
		}
	}
}

func init() {
	go func() {
		ticker := time.NewTicker(time.Minute)
		for range ticker.C {
			savePluginStores()
		}
	}()
}
