package main

import (
	"sync"
	"testing"

	scriptapi "gt"
)

func registerScriptConfigTestOption(t *testing.T, owner, key, label, help, scope, typ string, defaultValue, callback, validate any, choices []string, min, max, step float64) any {
	t.Helper()
	entry, ok := makeTypedScriptConfigEntry(owner, key, label, help, scope, typ, defaultValue, callback, validate, choices, min, max, step)
	if !ok {
		t.Fatalf("failed to create %s option %q", typ, key)
	}
	scriptRegisterConfig(owner, entry)
	return entry.Value
}

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
	value := registerScriptConfigTestOption(t, "plug", "enabled", "Enabled", "Turns the feature on.", scriptapi.ScopeGlobal, "bool", true, func(v bool) {
		callbackCalls++
		callbackValue = v
	}, func(v bool) bool { return true }, nil, 0, 0, 0)
	if value != true {
		t.Fatalf("default value = %v, want true", value)
	}
	if !scriptSetConfigValue("plug", "enabled", false) {
		t.Fatal("failed to update boolean config")
	}
	if callbackCalls != 1 || callbackValue {
		t.Fatalf("callback calls/value = %d/%v", callbackCalls, callbackValue)
	}

	if value := registerScriptConfigTestOption(t, "plug", "count", "Count", "", scriptapi.ScopeGlobal, "int", 7, nil, nil, nil, 0, 20, 1); value != 7 {
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
	value = registerScriptConfigTestOption(t, "plug", "enabled", "Enabled", "", scriptapi.ScopeGlobal, "bool", true, nil, nil, nil, 0, 0, 0)
	if value != false {
		t.Fatalf("persisted value = %v, want false", value)
	}
	entry := scriptConfigEntries["plug"][0]
	if entry.Default != true || entry.Value != false {
		t.Fatalf("default/current values = %v/%v", entry.Default, entry.Value)
	}

	if _, ok := makeTypedScriptConfigEntry("plug", "bad key", "Bad", "", scriptapi.ScopeGlobal, "bool", true, nil, nil, nil, 0, 0, 0); ok {
		t.Fatal("invalid setting key was accepted")
	}
	if len(scriptConfigEntries["plug"]) != 1 {
		t.Fatalf("unsupported config type was registered: %+v", scriptConfigEntries["plug"])
	}
}

func TestScriptConfigValidationAndCharacterScope(t *testing.T) {
	originalName := playerName
	playerName = "Alpha"
	t.Cleanup(func() { playerName = originalName })
	origDataDir := dataDirPath
	dataDirPath = t.TempDir()
	t.Cleanup(func() { dataDirPath = origDataDir })
	scriptDisplayNames = map[string]string{"plug": "Plug"}
	scriptAuthors = map[string]string{"plug": "Test"}
	scriptConfigEntries = map[string][]scriptConfigEntry{}
	scriptStores = map[string]*scriptStore{}
	startScriptEventQueue("plug")

	registerScriptConfigTestOption(t, "plug", "volume", "Volume", "", scriptapi.ScopeCharacter, "int", 5, nil, func(v int) bool { return v%2 == 1 }, nil, 1, 9, 2)
	if scriptSetConfigValue("plug", "volume", 10) || scriptSetConfigValue("plug", "volume", 4) {
		t.Fatal("range or custom validation accepted an invalid value")
	}
	if !scriptSetConfigValue("plug", "volume", 7) {
		t.Fatal("valid character-scoped value was rejected")
	}
	if got := scriptStorageGet("plug", "__config__:character:alpha:volume"); got != 7 {
		t.Fatalf("character value stored as %v", got)
	}
	playerName = "Beta"
	entry, ok := makeTypedScriptConfigEntry("plug", "volume", "Volume", "", scriptapi.ScopeCharacter, "int", 5, nil, nil, nil, 1, 9, 1)
	if !ok || entry.Value != 5 {
		t.Fatalf("different character inherited value: %+v", entry)
	}
}
