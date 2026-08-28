package main

import (
	"os"
	"reflect"
	"testing"
)

func TestScriptEnablementUsesDataScriptsEnabledJSON(t *testing.T) {
	originalDataDir := dataDirPath
	scriptMu.Lock()
	originalScopes := scriptEnabledFor
	scriptEnabledFor = map[string]scriptScope{
		"global-script": {All: true},
		"player-script": {Chars: map[string]bool{"Gaia": true, "Kato": true}},
	}
	scriptMu.Unlock()
	t.Cleanup(func() {
		dataDirPath = originalDataDir
		scriptMu.Lock()
		scriptEnabledFor = originalScopes
		scriptMu.Unlock()
	})

	dataDirPath = t.TempDir()
	saveScriptEnablement()
	data, err := os.ReadFile(scriptEnablementPath())
	if err != nil {
		t.Fatal(err)
	}
	want := "{\n  \"global\": [\n    \"global-script\"\n  ],\n  \"players\": {\n    \"Gaia\": [\n      \"player-script\"\n    ],\n    \"Kato\": [\n      \"player-script\"\n    ]\n  }\n}\n"
	if string(data) != want {
		t.Fatalf("enabled.json = %s, want %s", data, want)
	}

	scriptMu.Lock()
	scriptEnabledFor = map[string]scriptScope{}
	scriptMu.Unlock()
	loadScriptEnablement()
	scriptMu.RLock()
	got := scriptEnabledFor
	scriptMu.RUnlock()
	if !reflect.DeepEqual(got, map[string]scriptScope{
		"global-script": {All: true},
		"player-script": {Chars: map[string]bool{"Gaia": true, "Kato": true}},
	}) {
		t.Fatalf("loaded scopes = %#v", got)
	}
}

func TestApplicationShutdownKeepsScriptEnablement(t *testing.T) {
	const owner = "rank-decoder"
	scriptMu.Lock()
	originalScopes := scriptEnabledFor
	originalDisabled := scriptDisabled
	scriptEnabledFor = map[string]scriptScope{owner: {All: true}}
	scriptDisabled = map[string]bool{owner: false}
	scriptMu.Unlock()
	t.Cleanup(func() {
		scriptMu.Lock()
		scriptEnabledFor = originalScopes
		scriptDisabled = originalDisabled
		scriptMu.Unlock()
	})

	deactivateScript(owner, "application shutdown")
	scriptMu.RLock()
	scope, ok := scriptEnabledFor[owner]
	scriptMu.RUnlock()
	if !ok || !scope.All {
		t.Fatalf("shutdown cleared script scope: %#v", scriptEnabledFor)
	}
}
