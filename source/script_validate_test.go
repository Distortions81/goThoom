package main

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestValidateScriptFileRunsInitWithoutActivating(t *testing.T) {
	owner := "validate-test"
	dir := t.TempDir()
	path := filepath.Join(dir, "valid.go")
	source := `package main
import "gt2"
func Init() {
	gt2.Command("validate-only", func(string) {})
	gt2.Store("validate-only", true)
}
`
	if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	scriptMu = sync.RWMutex{}
	scriptDisabled = map[string]bool{owner: false}
	scriptCommandOwners = map[string]string{}
	scriptCommands = map[string]scriptCommandHandler{}
	scriptStoreMu = sync.Mutex{}
	scriptStores = map[string]*scriptStore{}

	if err := validateScriptFile(owner, path); err != nil {
		t.Fatalf("validate: %v", err)
	}
	if _, exists := scriptCommands["validate-only"]; exists {
		t.Fatal("validation activated a command")
	}
	if value := scriptStorageGet(owner, "validate-only"); value != nil {
		t.Fatalf("validation changed storage: %v", value)
	}
	if scriptIsRunning(owner) != true {
		t.Fatal("validation changed the running state")
	}
}

func TestValidateScriptFileReportsPathAndInitError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "broken.go")
	source := `package main
func Init() { panic("bad init") }
`
	if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	err := validateScriptFile("validate-broken", path)
	if err == nil || !strings.Contains(err.Error(), path) || !strings.Contains(err.Error(), "bad init") {
		t.Fatalf("validation error = %v", err)
	}
}

func TestValidateScriptFileExplainsUnsupportedImport(t *testing.T) {
	path := filepath.Join(t.TempDir(), "old.go")
	source := `package main
import "gt"
func Init() {}
`
	if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	err := validateScriptFile("validate-old-import", path)
	if err == nil || !strings.Contains(err.Error(), `unsupported import "gt"`) || !strings.Contains(err.Error(), `use "gt2"`) {
		t.Fatalf("old import validation error = %v", err)
	}
}

func TestValidateScriptFileExplainsAPIVersion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "v1.go")
	source := `package main
const scriptAPIVersion = 1
func Init() {}
`
	if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	err := validateScriptFile("validate-v1", path)
	if err == nil || !strings.Contains(err.Error(), "unsupported script API version 1") || !strings.Contains(err.Error(), "supports version 2") {
		t.Fatalf("API version validation error = %v", err)
	}
}
