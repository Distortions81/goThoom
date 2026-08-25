//go:build integration
// +build integration

package main

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
)

// Metadata is optional for local scripts; the filename supplies name and ID.
func TestScriptMetadataIsOptional(t *testing.T) {
	origDir := dataDirPath
	dataDirPath = t.TempDir()
	t.Cleanup(func() { dataDirPath = origDir })

	plugDir := filepath.Join(dataDirPath, "scripts")
	if err := os.MkdirAll(plugDir, 0o755); err != nil {
		t.Fatalf("mkdir scripts: %v", err)
	}
	src := `package main
func Init() {}
`
	if err := os.WriteFile(filepath.Join(plugDir, "meta.go"), []byte(src), 0o644); err != nil {
		t.Fatalf("write script: %v", err)
	}

	// Scan the isolated directory directly. loadScripts uses the directory next
	// to the test binary, which is intentionally not the data directory.
	scriptMu = sync.RWMutex{}
	scriptInvalid = map[string]bool{}
	scriptDisabled = map[string]bool{}
	scriptEnabledFor = map[string]scriptScope{}
	scanned := scanscripts([]string{plugDir}, nil)
	owner := "meta"
	info, ok := scanned[owner]
	if !ok || info.invalid || info.name != "meta" || info.id != "meta" || info.apiVer != scriptAPICurrentVersion {
		t.Fatalf("simple script metadata = %+v", scanned)
	}
	scriptInvalid[owner] = info.invalid
	scriptDisabled[owner] = info.invalid

	playerName = "Tester"
	setscriptEnabled(owner, true, false)
	if s, ok := scriptEnabledFor[owner]; !ok || s.empty() {
		t.Fatalf("valid simple script was not enabled: %+v", s)
	}
	if scriptDisabled[owner] {
		t.Fatal("valid simple script remained disabled")
	}
}
