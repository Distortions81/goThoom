//go:build integration
// +build integration

package main

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
)

// Test that plugins missing required metadata are marked invalid and disabled.
func TestPluginMissingMetaDisabled(t *testing.T) {
	origDir := dataDirPath
	dataDirPath = t.TempDir()
	t.Cleanup(func() { dataDirPath = origDir })

	plugDir := filepath.Join(dataDirPath, "plugins")
	if err := os.MkdirAll(plugDir, 0o755); err != nil {
		t.Fatalf("mkdir plugins: %v", err)
	}
	src := `package main
const PluginName = "MetaTest"
`
	if err := os.WriteFile(filepath.Join(plugDir, "meta.go"), []byte(src), 0o644); err != nil {
		t.Fatalf("write plugin: %v", err)
	}

	// Reset plugin state.
	pluginMu = sync.RWMutex{}
	pluginDisplayNames = map[string]string{}
	pluginAuthors = map[string]string{}
	pluginCategories = map[string]string{}
	pluginSubCategories = map[string]string{}
	pluginInvalid = map[string]bool{}
	pluginDisabled = map[string]bool{}
	pluginEnabledFor = map[string]pluginScope{}
	pluginNames = map[string]bool{}

	loadPlugins()
	owner := "MetaTest_meta"
	if !pluginInvalid[owner] {
		t.Fatalf("plugin not marked invalid: %+v", pluginInvalid)
	}
	if !pluginDisabled[owner] {
		t.Fatalf("plugin not disabled")
	}

	playerName = "Tester"
	setPluginEnabled(owner, true, false)
	if s, ok := pluginEnabledFor[owner]; ok && !s.empty() {
		t.Fatalf("invalid plugin unexpectedly enabled: %+v", s)
	}
	if !pluginDisabled[owner] {
		t.Fatalf("invalid plugin became enabled")
	}
}
