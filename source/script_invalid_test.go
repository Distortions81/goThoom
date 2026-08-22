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
func TestscriptMissingMetaDisabled(t *testing.T) {
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

	// Reset script state.
	scriptMu = sync.RWMutex{}
	scriptDisplayNames = map[string]string{}
	scriptAuthors = map[string]string{}
	scriptCategories = map[string]string{}
	scriptSubCategories = map[string]string{}
	scriptInvalid = map[string]bool{}
	scriptDisabled = map[string]bool{}
	scriptEnabledFor = map[string]scriptScope{}
	scriptNames = map[string]bool{}

	loadscripts()
	owner := "MetaTest_meta"
	if !scriptInvalid[owner] {
		t.Fatalf("script not marked invalid: %+v", scriptInvalid)
	}
	if !scriptDisabled[owner] {
		t.Fatalf("script not disabled")
	}

	playerName = "Tester"
	setscriptEnabled(owner, true, false)
	if s, ok := scriptEnabledFor[owner]; ok && !s.empty() {
		t.Fatalf("invalid script unexpectedly enabled: %+v", s)
	}
	if !scriptDisabled[owner] {
		t.Fatalf("invalid script became enabled")
	}
}
