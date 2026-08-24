package main

import (
	"sync"
	"testing"
)

func TestScriptConfigDefaultsCallbacksAndPersistence(t *testing.T) {
	origDataDir := dataDirPath
	dataDirPath = t.TempDir()
	t.Cleanup(func() { dataDirPath = origDataDir })

	scriptMu = sync.RWMutex{}
	scriptDisplayNames = map[string]string{"plug": "Plug"}
	scriptAuthors = map[string]string{"plug": "Test"}
	scriptConfigMu = sync.RWMutex{}
	scriptConfigEntries = map[string][]scriptConfigEntry{}
	scriptStoreMu = sync.Mutex{}
	scriptStores = map[string]*scriptStore{}
	scriptEventMu = sync.Mutex{}
	scriptEventQueues = map[string]*scriptEventQueue{}
	startScriptEventQueue("plug")
	scriptsList = nil

	callbackCalls := 0
	callbackValue := true
	value := scriptAddConfig("plug", "Enabled", "check-box", true, func(v bool) {
		callbackCalls++
		callbackValue = v
	})
	if value != true {
		t.Fatalf("default value = %v, want true", value)
	}
	if !scriptSetConfigValue("plug", "enabled", false) {
		t.Fatal("failed to update boolean config")
	}
	if callbackCalls != 1 || callbackValue {
		t.Fatalf("callback calls/value = %d/%v", callbackCalls, callbackValue)
	}

	if value := scriptAddConfig("plug", "Count", "int-slider", 7); value != 7 {
		t.Fatalf("integer default = %v, want 7", value)
	}
	if !scriptSetConfigValue("plug", "count", float32(12.8)) {
		t.Fatal("failed to update integer config")
	}
	if got := scriptConfigEntries["plug"][1].Value; got != 12 {
		t.Fatalf("integer value = %v, want 12", got)
	}

	scriptConfigEntries = map[string][]scriptConfigEntry{}
	scriptStores = map[string]*scriptStore{}
	value = scriptAddConfig("plug", "Enabled", "bool", true)
	if value != false {
		t.Fatalf("persisted value = %v, want false", value)
	}
	entry := scriptConfigEntries["plug"][0]
	if entry.Default != true || entry.Value != false {
		t.Fatalf("default/current values = %v/%v", entry.Default, entry.Value)
	}

	if value := scriptAddConfig("plug", "Bad", "unknown", true); value != nil {
		t.Fatalf("unsupported config type returned %v", value)
	}
	if len(scriptConfigEntries["plug"]) != 1 {
		t.Fatalf("unsupported config type was registered: %+v", scriptConfigEntries["plug"])
	}
}
