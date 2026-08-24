package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	scriptapi "gt2"
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
	scriptRepeats = map[string][]*scriptRepeatRegistration{}
	scriptTickWaiters = map[string][]*tickWaiter{}
	scriptStateWaiters = map[string][]*scriptStateWaiter{}
	scriptStopping = map[string]bool{}
	scriptDispatchMu = sync.Mutex{}
	scriptDispatchQueue = nil
	scriptEventMu = sync.Mutex{}
	scriptEventQueues = map[string]*scriptEventQueue{}

	shortcutMu = sync.RWMutex{}
	shortcutMaps = map[string]map[string]string{}
	shortcutRegistrations = map[string]scriptRegistrationHandle{}
	chatHandlersMu = sync.RWMutex{}
	scriptStructuredChatHandlers = nil
	scriptServerMessageHandlers = nil
	scriptLifecycleHandlers = nil
	scriptChangeHandlers = nil
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
import (
	"time"
	"gt2"
)
const scriptName = "Refresh"
const scriptAuthor = "Test"
const scriptCategory = "Tests"
const scriptAPIVersion = 2
func Init() {
	gt2.Command("refresh_cmd", func(args string) { gt2.Store("command_version", "` + version + `") })
	gt2.Bind("Ctrl-R", func(event gt2.InputEvent) { gt2.Store("input_version", "` + version + `") })
	gt2.OnChat(gt2.ChatFilter{Contains: "reload"}, func(event gt2.ChatEvent) { gt2.Store("chat_version", "` + version + `") })
	gt2.OnServerMessage(gt2.ServerMessageFilter{Type: "system"}, func(event gt2.ServerMessage) { gt2.Store("server_version", "` + version + `") })
	gt2.OnChange(gt2.ChangeInventory, func(event gt2.ChangeEvent) { gt2.Store("inventory_version", "` + version + `") })
	gt2.OnLogin(func(event gt2.LifecycleEvent) { gt2.Store("login_version", "` + version + `") })
	gt2.Repeat(time.Hour, func() { gt2.Store("timer_version", "` + version + `") })
	gt2.Store("loaded_version", "` + version + `")
}
func Terminate() {
	gt2.Store("terminated_version", "` + version + `")
	gt2.Store("termination_count", gt2.LoadInteger("termination_count", 0)+1)
}
`
		if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
			t.Fatalf("write script: %v", err)
		}
	}
	const owner = "refresh"
	assertSingleRegistrationSet := func() {
		t.Helper()
		scriptMu.RLock()
		commands, repeats := len(scriptCommands), len(scriptRepeats[owner])
		scriptMu.RUnlock()
		hotkeysMu.RLock()
		bindingCount := len(hotkeys)
		hotkeysMu.RUnlock()
		chatHandlersMu.RLock()
		chatCount, serverCount := len(scriptStructuredChatHandlers), len(scriptServerMessageHandlers)
		lifecycleCount, changeCount := len(scriptLifecycleHandlers), len(scriptChangeHandlers)
		chatHandlersMu.RUnlock()
		if commands != 1 || bindingCount != 1 || chatCount != 1 || serverCount != 1 || lifecycleCount != 1 || changeCount != 1 || repeats != 1 {
			t.Fatalf("registration counts = command:%d input:%d chat:%d server:%d lifecycle:%d change:%d timer:%d",
				commands, bindingCount, chatCount, serverCount, lifecycleCount, changeCount, repeats)
		}
	}
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
	if scriptStorageGet(owner, "loaded_version") != "one" {
		t.Fatalf("initial script did not load")
	}
	assertSingleRegistrationSet()
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
	if scriptStorageGet(owner, "loaded_version") != "two" {
		t.Fatalf("enabled script was not reloaded")
	}
	assertSingleRegistrationSet()
	if got := scriptStorageGet(owner, "terminated_version"); got != "one" {
		t.Fatalf("old script was not terminated exactly once: %v", got)
	}
	if got := scriptStorageGet(owner, "termination_count"); got != 1 {
		t.Fatalf("old script termination count = %v, want 1", got)
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
	sim := scriptEventSimulator{owner: owner}
	sim.command(t, "refresh_cmd", "")
	if got := scriptStorageGet(owner, "command_version"); got != "two" {
		t.Fatalf("old command callback remained active: %v", got)
	}
	if !sim.input(t, makeScriptInputEvent("Ctrl-R")) {
		t.Fatal("reload test input unexpectedly consumed")
	}
	sim.chat(t, "Guide says, reload now")
	sim.serverMessage(t, scriptServerMessage("system", "ready"))
	sim.inventory(t, []InventoryItem{{Name: "Dagger"}})
	sim.login(t, "Hero")
	sim.timers(t)
	for _, key := range []string{"input_version", "chat_version", "server_version", "inventory_version", "login_version", "timer_version"} {
		if got := scriptStorageGet(owner, key); got != "two" {
			t.Fatalf("%s = %v, want replacement version two", key, got)
		}
	}

	brokenSource := `package main
import "gt2"
const scriptName = "Refresh"
const scriptAuthor = "Test"
const scriptCategory = "Tests"
const scriptAPIVersion = 2
func Init() { gt2.Store("loaded_version", "compile-broken")
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
	assertSingleRegistrationSet()
	handler = scriptCommands["refresh_cmd"]
	handler("")
	if got := scriptStorageGet(owner, "command_version"); got != "two" {
		t.Fatalf("compile failure replaced the working callback: %v", got)
	}

	panicSource := `package main
import "gt2"
const scriptName = "Refresh"
const scriptAuthor = "Test"
const scriptCategory = "Tests"
const scriptAPIVersion = 2
func Init() {
	gt2.Store("loaded_version", "panic-broken")
	gt2.Command("refresh_cmd", func(args string) { gt2.Store("command_version", "panic-broken") })
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
	assertSingleRegistrationSet()
	handler = scriptCommands["refresh_cmd"]
	handler("")
	if got := scriptStorageGet(owner, "command_version"); got != "two" {
		t.Fatalf("Init panic replaced the working callback: %v", got)
	}

	timeoutSource := `package main
const scriptName = "Refresh"
const scriptAuthor = "Test"
const scriptCategory = "Tests"
const scriptAPIVersion = 2
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
	assertSingleRegistrationSet()
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
	if scriptStorageGet(owner, "loaded_version") != "three" {
		t.Fatal("valid script did not replace failed candidates")
	}
	assertSingleRegistrationSet()
	if got := scriptStorageGet(owner, "terminated_version"); got != "two" {
		t.Fatalf("working script termination count/order is wrong: %v", got)
	}
	if got := scriptStorageGet(owner, "termination_count"); got != 2 {
		t.Fatalf("replacement termination count = %v, want 2", got)
	}
	sim.command(t, "refresh_cmd", "")
	if got := scriptStorageGet(owner, "command_version"); got != "three" {
		t.Fatalf("final replacement callback is not active: %v", got)
	}
	shutdownScripts()
	persisted = readStoredValues()
	if persisted["command_version"] != "three" || persisted["terminated_version"] != "three" || persisted["termination_count"] != float64(3) {
		t.Fatalf("shutdown storage was not flushed: %v", persisted)
	}
}

func scriptServerMessage(messageType, message string) scriptapi.ServerMessage {
	return scriptapi.ServerMessage{Type: messageType, Message: message}
}
