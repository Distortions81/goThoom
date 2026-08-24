package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
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

const scriptStorageVersionKey = "__gt:storage-version"

var (
	scriptStores  = map[string]*scriptStore{}
	scriptStoreMu sync.Mutex
)

func scriptStoragePath(owner string) string {
	sum := sha256.Sum256([]byte(owner))
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

func scriptStorageSet(owner, key string, value any) bool {
	if err := validateScriptStorageValue(value); err != nil {
		reportScriptStorageError(owner, key, err)
		return false
	}
	setScriptStorageValue(owner, key, value)
	return true
}

func validateScriptStorageValue(value any) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("JSON encoding panic: %v", recovered)
		}
	}()
	_, err = json.Marshal(value)
	return err
}

func reportScriptStorageError(owner, key string, err error) {
	msg := fmt.Sprintf("[script:%s] cannot store %q: %v", owner, key, err)
	log.Print(msg)
	dispatchScriptControl(func() { consoleMessage(msg) })
}

func setScriptStorageValue(owner, key string, value any) {
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

func scriptStorageVersion(owner string) int {
	return scriptStoredInteger(scriptStorageGet(owner, scriptStorageVersionKey), 0)
}

func scriptStoredString(value any, fallback string) string {
	if stored, ok := value.(string); ok {
		return stored
	}
	return fallback
}

func scriptStoredBool(value any, fallback bool) bool {
	if stored, ok := value.(bool); ok {
		return stored
	}
	return fallback
}

func scriptStoredInteger(value any, fallback int) int {
	switch stored := value.(type) {
	case int:
		return stored
	case int8:
		return int(stored)
	case int16:
		return int(stored)
	case int32:
		return int(stored)
	case int64:
		if stored >= int64(scriptMinInt()) && stored <= int64(scriptMaxInt()) {
			return int(stored)
		}
	case float64:
		if stored >= float64(scriptMinInt()) && stored <= float64(scriptMaxInt()) && stored == float64(int(stored)) {
			return int(stored)
		}
	}
	return fallback
}

func scriptStoredDecimal(value any, fallback float64) float64 {
	switch stored := value.(type) {
	case float64:
		return stored
	case float32:
		return float64(stored)
	case int:
		return float64(stored)
	case int64:
		return float64(stored)
	}
	return fallback
}

func scriptStoredStrings(value any, fallback []string) []string {
	switch stored := value.(type) {
	case []string:
		return append([]string(nil), stored...)
	case []any:
		result := make([]string, len(stored))
		for index, item := range stored {
			text, ok := item.(string)
			if !ok {
				return append([]string(nil), fallback...)
			}
			result[index] = text
		}
		return result
	default:
		return append([]string(nil), fallback...)
	}
}

func scriptStoredJSON(value, target any) bool {
	if value == nil || target == nil {
		return false
	}
	targetValue := reflect.ValueOf(target)
	if targetValue.Kind() != reflect.Pointer || targetValue.IsNil() {
		return false
	}
	data, err := json.Marshal(value)
	if err != nil {
		return false
	}
	return json.Unmarshal(data, target) == nil
}

func scriptMaxInt() int { return int(^uint(0) >> 1) }
func scriptMinInt() int { return -scriptMaxInt() - 1 }

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
		savescriptStore(ps)
	}
}

func flushscriptStore(owner string) {
	if isWASM {
		return
	}
	scriptStoreMu.Lock()
	store := scriptStores[owner]
	scriptStoreMu.Unlock()
	if store != nil {
		savescriptStore(store)
	}
}

func savescriptStore(store *scriptStore) {
	store.mu.Lock()
	defer store.mu.Unlock()
	if !store.dirty {
		return
	}
	data, err := json.MarshalIndent(store.data, "", "  ")
	if err != nil {
		log.Printf("save script storage %s: %v", store.path, err)
		return
	}
	path := store.path
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		log.Printf("save script storage %s: %v", path, err)
		return
	}
	if err := writeFileAtomic(path, data, 0o644); err != nil {
		log.Printf("save script storage %s: %v", path, err)
		return
	}
	store.dirty = false
}

func writeFileAtomic(path string, data []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	temporary, err := os.CreateTemp(dir, ".storage-*.tmp")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	removeTemporary := true
	defer func() {
		if removeTemporary {
			_ = os.Remove(temporaryPath)
		}
	}()
	if err := temporary.Chmod(mode); err != nil {
		_ = temporary.Close()
		return err
	}
	if _, err := temporary.Write(data); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return err
	}
	removeTemporary = false

	directory, err := os.Open(dir)
	if err != nil {
		return err
	}
	err = directory.Sync()
	closeErr := directory.Close()
	if err != nil {
		return err
	}
	return closeErr
}

func init() {
	go func() {
		ticker := time.NewTicker(time.Minute)
		for range ticker.C {
			savescriptStores()
		}
	}()
}
