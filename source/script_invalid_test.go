//go:build integration
// +build integration

package main

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
)

// Test that scripts missing required metadata are marked invalid and disabled.
func TestScriptMissingMetaDisabled(t *testing.T) {
	origDir := dataDirPath
	dataDirPath = t.TempDir()
	t.Cleanup(func() { dataDirPath = origDir })

	plugDir := filepath.Join(dataDirPath, "scripts")
	if err := os.MkdirAll(plugDir, 0o755); err != nil {
		t.Fatalf("mkdir scripts: %v", err)
	}
	src := `package main
const scriptName = "MetaTest"
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
	owner := "MetaTest_meta"
	info, ok := scanned[owner]
	if !ok || !info.invalid {
		t.Fatalf("script not marked invalid: %+v", scanned)
	}
	scriptInvalid[owner] = info.invalid
	scriptDisabled[owner] = info.invalid

	playerName = "Tester"
	setscriptEnabled(owner, true, false)
	if s, ok := scriptEnabledFor[owner]; ok && !s.empty() {
		t.Fatalf("invalid script unexpectedly enabled: %+v", s)
	}
	if !scriptDisabled[owner] {
		t.Fatalf("invalid script became enabled")
	}
}
