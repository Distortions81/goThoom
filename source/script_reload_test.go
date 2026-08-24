//go:build integration

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestRescanReloadsEnabledScript(t *testing.T) {
	origDataDir := dataDirPath
	origPlayerName := playerName
	origSettings := gs
	dataDirPath = t.TempDir()
	playerName = "Tester"
	gs.Enabledscripts = nil
	t.Cleanup(func() {
		dataDirPath = origDataDir
		playerName = origPlayerName
		gs = origSettings
	})

	scriptMu = sync.RWMutex{}
	scriptNames = map[string]bool{}
	scriptDisplayNames = map[string]string{}
	scriptAuthors = map[string]string{}
	scriptCategories = map[string]string{}
	scriptSubCategories = map[string]string{}
	scriptInvalid = map[string]bool{}
	scriptDisabled = map[string]bool{}
	scriptEnabledFor = map[string]scriptScope{}
	scriptPaths = map[string]string{}
	scriptTerminators = map[string]func(){}
	scriptCommands = map[string]scriptCommandHandler{}
	scriptCommandOwners = map[string]string{}
	scriptSendHistory = map[string][]time.Time{}
	scriptActiveSourceHashes = map[string][32]byte{}
	scriptTimers = map[string][]*time.Timer{}
	scriptTickerStops = map[string][]chan struct{}{}
	scriptTickWaiters = map[string][]*tickWaiter{}

	shortcutMu = sync.RWMutex{}
	shortcutMaps = map[string]map[string]string{}
	inputHandlersMu = sync.RWMutex{}
	scriptInputHandlers = nil
	triggerHandlersMu = sync.RWMutex{}
	scriptTriggers = map[string][]triggerHandler{}
	scriptConsoleTriggers = map[string][]triggerHandler{}
	chatHandlersMu = sync.RWMutex{}
	scriptChatHandlers = nil
	scriptStructuredChatHandlers = nil
	scriptServerMessageHandlers = nil
	scriptLifecycleHandlers = nil
	scriptChangeHandlers = nil
	playerHandlersMu = sync.RWMutex{}
	scriptPlayerHandlers = nil
	scriptHotkeyFnMu = sync.RWMutex{}
	scriptHotkeyFns = map[string]map[string]func(InputEvent) bool{}
	hotkeysMu = sync.RWMutex{}
	hotkeys = nil
	scriptHotkeyMu = sync.RWMutex{}
	scriptHotkeyEnabled = map[string]map[string]bool{}
	overlayMu = sync.RWMutex{}
	scriptOverlayOps = map[string][]overlayOp{}
	scriptStoreMu = sync.Mutex{}
	scriptStores = map[string]*scriptStore{}

	dir := t.TempDir()
	path := filepath.Join(dir, "refresh.go")
	writeVersion := func(version string) {
		t.Helper()
		src := `package main
import "gt"
const scriptName = "Refresh"
const scriptAuthor = "Test"
const scriptCategory = "Tests"
const scriptAPIVersion = 1
func Init() {
	gt.RegisterCommand("refresh_cmd", func(args string) { gt.Store("command_version", "` + version + `") })
	gt.AddHotkey("Ctrl-R", "/wave")
	gt.Store("loaded_version", "` + version + `")
}
func Terminate() {
	gt.Store("terminated_version", "` + version + `")
}
`
		if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
			t.Fatalf("write script: %v", err)
		}
	}
	waitFor := func(cond func() bool) bool {
		deadline := time.Now().Add(time.Second)
		for time.Now().Before(deadline) {
			if cond() {
				return true
			}
			time.Sleep(5 * time.Millisecond)
		}
		return cond()
	}

	const owner = "refresh"
	readStoredValues := func() map[string]any {
		t.Helper()
		data, err := os.ReadFile(scriptStoragePath(owner))
		if err != nil {
			t.Fatalf("read persisted script storage: %v", err)
		}
		values := map[string]any{}
		if err := json.Unmarshal(data, &values); err != nil {
			t.Fatalf("decode persisted script storage: %v", err)
		}
		return values
	}
	scriptEnabledFor[owner] = scriptScope{All: true}
	writeVersion("one")
	rescanScripts([]string{dir})
	if !waitFor(func() bool { return scriptStorageGet(owner, "loaded_version") == "one" }) {
		t.Fatalf("initial script did not load")
	}
	rescanScripts([]string{dir})
	if got := scriptStorageGet(owner, "terminated_version"); got != nil {
		t.Fatalf("unchanged script was restarted: %v", got)
	}
	if got := readStoredValues()["loaded_version"]; got != "one" {
		t.Fatalf("initialization storage was not flushed: %v", got)
	}
	hotkeysMu.Lock()
	if len(hotkeys) != 1 {
		hotkeysMu.Unlock()
		t.Fatalf("initial script hotkey missing: %+v", hotkeys)
	}
	hotkeys[0].Disabled = true
	hotkeysMu.Unlock()
	scriptHotkeyMu.Lock()
	scriptHotkeyEnabled[owner]["Ctrl-R"] = false
	scriptHotkeyMu.Unlock()
	saveHotkeys()

	writeVersion("two")
	rescanScripts([]string{dir})
	if !waitFor(func() bool { return scriptStorageGet(owner, "loaded_version") == "two" }) {
		t.Fatalf("enabled script was not reloaded")
	}
	if got := scriptStorageGet(owner, "terminated_version"); got != "one" {
		t.Fatalf("old script was not terminated exactly once: %v", got)
	}
	persisted := readStoredValues()
	if persisted["loaded_version"] != "two" || persisted["terminated_version"] != "one" {
		t.Fatalf("reload storage was not flushed: %v", persisted)
	}
	hotkeysMu.RLock()
	if len(hotkeys) != 1 || !hotkeys[0].Disabled {
		hotkeysMu.RUnlock()
		t.Fatalf("reload did not preserve disabled hotkey state: %+v", hotkeys)
	}
	hotkeysMu.RUnlock()

	scriptMu.RLock()
	handler := scriptCommands["refresh_cmd"]
	scriptMu.RUnlock()
	if handler == nil {
		t.Fatal("replacement command was not registered")
	}
	handler("")
	if !waitFor(func() bool { return scriptStorageGet(owner, "command_version") == "two" }) {
		t.Fatalf("old command callback remained active: %v", scriptStorageGet(owner, "command_version"))
	}

	brokenSource := `package main
import "gt"
const scriptName = "Refresh"
const scriptAuthor = "Test"
const scriptCategory = "Tests"
const scriptAPIVersion = 1
func Init() { gt.Store("loaded_version", "compile-broken")
`
	if err := os.WriteFile(path, []byte(brokenSource), 0o644); err != nil {
		t.Fatalf("write malformed script: %v", err)
	}
	rescanScripts([]string{dir})
	if got := scriptStorageGet(owner, "loaded_version"); got != "two" {
		t.Fatalf("compile failure changed running script storage: %v", got)
	}
	if !scriptIsRunning(owner) {
		t.Fatal("compile failure stopped the last working script")
	}
	handler = scriptCommands["refresh_cmd"]
	handler("")
	if got := scriptStorageGet(owner, "command_version"); got != "two" {
		t.Fatalf("compile failure replaced the working callback: %v", got)
	}

	panicSource := `package main
import "gt"
const scriptName = "Refresh"
const scriptAuthor = "Test"
const scriptCategory = "Tests"
const scriptAPIVersion = 1
func Init() {
	gt.Store("loaded_version", "panic-broken")
	gt.RegisterCommand("refresh_cmd", func(args string) { gt.Store("command_version", "panic-broken") })
	panic("boom")
}
`
	if err := os.WriteFile(path, []byte(panicSource), 0o644); err != nil {
		t.Fatalf("write panicking script: %v", err)
	}
	rescanScripts([]string{dir})
	if got := scriptStorageGet(owner, "loaded_version"); got != "two" {
		t.Fatalf("Init panic committed staged storage: %v", got)
	}
	if got := scriptStorageGet(owner, "terminated_version"); got != "one" {
		t.Fatalf("failed candidate terminated the working script: %v", got)
	}
	if !scriptIsRunning(owner) {
		t.Fatal("Init panic stopped the last working script")
	}
	handler = scriptCommands["refresh_cmd"]
	handler("")
	if got := scriptStorageGet(owner, "command_version"); got != "two" {
		t.Fatalf("Init panic replaced the working callback: %v", got)
	}

	timeoutSource := `package main
const scriptName = "Refresh"
const scriptAuthor = "Test"
const scriptCategory = "Tests"
const scriptAPIVersion = 1
func Init() { for {} }
`
	if err := os.WriteFile(path, []byte(timeoutSource), 0o644); err != nil {
		t.Fatalf("write timing-out script: %v", err)
	}
	origCallbackLimit := scriptCallbackTimeLimit
	scriptCallbackTimeLimit = 20 * time.Millisecond
	rescanScripts([]string{dir})
	scriptCallbackTimeLimit = origCallbackLimit
	if got := scriptStorageGet(owner, "loaded_version"); got != "two" {
		t.Fatalf("Init timeout changed running script storage: %v", got)
	}
	if !scriptIsRunning(owner) {
		t.Fatal("Init timeout stopped the last working script")
	}
	if got := scriptStorageGet(owner, "terminated_version"); got != "one" {
		t.Fatalf("Init timeout terminated the working script: %v", got)
	}
	handler = scriptCommands["refresh_cmd"]
	handler("")
	if got := scriptStorageGet(owner, "command_version"); got != "two" {
		t.Fatalf("Init timeout replaced the working callback: %v", got)
	}

	writeVersion("three")
	rescanScripts([]string{dir})
	if !waitFor(func() bool { return scriptStorageGet(owner, "loaded_version") == "three" }) {
		t.Fatal("valid script did not replace failed candidates")
	}
	if got := scriptStorageGet(owner, "terminated_version"); got != "two" {
		t.Fatalf("working script termination count/order is wrong: %v", got)
	}
	handler = scriptCommands["refresh_cmd"]
	handler("")
	if !waitFor(func() bool { return scriptStorageGet(owner, "command_version") == "three" }) {
		got := scriptStorageGet(owner, "command_version")
		t.Fatalf("final replacement callback is not active: %v", got)
	}
	shutdownScripts()
	persisted = readStoredValues()
	if persisted["command_version"] != "three" || persisted["terminated_version"] != "three" {
		t.Fatalf("shutdown storage was not flushed: %v", persisted)
	}
}
