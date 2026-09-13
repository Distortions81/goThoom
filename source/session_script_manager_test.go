package main

import (
	"sync"
	"testing"

	scriptapi "gt2"
)

func TestScriptsManagerUsesSelectedSessionRuntime(t *testing.T) {
	originalSessions := appSessions
	manager := newSessionManager(primarySession)
	slots := manager.materializeAllSessions()
	appSessions = manager
	t.Cleanup(func() { appSessions = originalSessions })
	selected := slots[1]
	selected.setCharacterName("Second")
	selected.automation.scriptMu.Lock()
	selected.automation.scripts["example"] = &sessionScriptInstance{owner: "example"}
	selected.automation.scriptConfigs["example"] = []scriptConfigEntry{{Key: "enabled", Scope: scriptapi.ScopeGlobal}}
	selected.automation.localCommands["hello"] = sessionScriptCommand{owner: "example", original: "hello"}
	selected.automation.scriptMu.Unlock()
	if !manager.selectSession(selected.ID()) {
		t.Fatal("could not select second session")
	}
	if got := scriptManagerSession(); got != selected {
		t.Fatalf("Scripts manager session = %p, want %p", got, selected)
	}
	if got := scriptRuntimeStatusForSession(selected, "example", scriptScope{All: true}, false, "", false); got != "Running" {
		t.Fatalf("selected runtime status = %q, want Running", got)
	}
	commands, bindings, events, timers, settings := scriptRegistrationSummaryForSession(selected, "example")
	if len(commands) != 1 || commands[0] != "/hello" || len(settings) != 1 || len(bindings) != 0 || len(events) != 0 || timers != 0 {
		t.Fatalf("selected registration summary = commands %v bindings %v events %v timers %d settings %v", commands, bindings, events, timers, settings)
	}
}

func TestGlobalScriptConfigChangeReachesEverySessionRuntime(t *testing.T) {
	originalSessions := appSessions
	originalEntries := scriptConfigEntries
	originalStores := scriptStores
	originalConfigMu := scriptConfigMu
	originalStoreMu := scriptStoreMu
	manager := newSessionManager(primarySession)
	slots := manager.materializeAllSessions()
	appSessions = manager
	scriptConfigMu = sync.RWMutex{}
	scriptStoreMu = sync.Mutex{}
	scriptConfigEntries = map[string][]scriptConfigEntry{}
	scriptStores = map[string]*scriptStore{}
	t.Cleanup(func() {
		appSessions = originalSessions
		scriptConfigMu = originalConfigMu
		scriptStoreMu = originalStoreMu
		scriptConfigEntries = originalEntries
		scriptStores = originalStores
	})
	const owner = "shared-config"
	primaryEntry := scriptConfigEntry{Key: "enabled", Type: "bool", Scope: scriptapi.ScopeGlobal, Default: true, Value: true}
	scriptConfigEntries[owner] = []scriptConfigEntry{primaryEntry}
	for _, session := range slots[1:3] {
		session.automation.scriptConfigs[owner] = []scriptConfigEntry{primaryEntry}
	}
	if !scriptSetConfigValueForSession(slots[1], owner, "enabled", false) {
		t.Fatal("selected session could not change global script config")
	}
	if got := scriptConfigEntries[owner][0].Value; got != false {
		t.Fatalf("primary runtime value = %v, want false", got)
	}
	for _, session := range slots[1:3] {
		if got := session.scriptConfigEntriesSnapshot(owner)[0].Value; got != false {
			t.Fatalf("session %d runtime value = %v, want false", session.ID(), got)
		}
	}
}
