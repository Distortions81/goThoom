package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	scriptapi "gt2"
)

func scriptAPIFixturePath(name string) string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "testdata", "scripts", name)
}

// TestScriptAPISmoke loads a simple script script (via Yaegi) that uses the
// gt2 API and verifies side effects through the existing script machinery.
func TestScriptAPISmoke(t *testing.T) {
	// Isolate script storage and related files
	origDir := dataDirPath
	dataDirPath = t.TempDir()
	t.Cleanup(func() { dataDirPath = origDir })

	// Reset shared script state
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
	scriptHotkeyFnMu = sync.RWMutex{}
	scriptHotkeyFns = map[string]map[string]func(InputEvent) bool{}

	// Reset shortcuts and handlers
	shortcutMu = sync.RWMutex{}
	shortcutMaps = map[string]map[string]string{}
	shortcutRegistrations = map[string]scriptRegistrationHandle{}
	chatHandlersMu = sync.RWMutex{}
	scriptStructuredChatHandlers = nil
	scriptServerMessageHandlers = nil
	scriptLifecycleHandlers = nil
	scriptChangeHandlers = nil

	// Owner metadata required for storage hashing, messages, etc.
	const owner = "apitest_owner"
	scriptDisplayNames[owner] = "APISmoke"
	scriptAuthors[owner] = "Test"

	// Load the script script source and execute
	srcPath := scriptAPIFixturePath("api_smoke.go")
	src, err := os.ReadFile(srcPath)
	if err != nil {
		t.Fatalf("read script: %v", err)
	}
	loadscriptSource(owner, "APISmoke", srcPath, src, restrictedStdlib())

	// Wait for Init() to signal readiness via storage key
	waitFor := func(cond func() bool, d time.Duration) bool {
		deadline := time.Now().Add(d)
		for time.Now().Before(deadline) {
			if cond() {
				return true
			}
			time.Sleep(5 * time.Millisecond)
		}
		return cond()
	}

	if ok := waitFor(func() bool { return scriptStorageGet(owner, "started") == "yes" }, time.Second); !ok {
		t.Fatalf("script did not start")
	}

	// 1) Shortcut added
	shortcutMu.RLock()
	shortcuts := shortcutMaps[owner]
	shortcutMu.RUnlock()
	if shortcuts == nil || shortcuts["yy"] != "/yell " {
		t.Fatalf("shortcut not added: %+v", shortcuts)
	}

	// 2) Command registered and works (writes last_args)
	handler, ok := scriptCommands["apit_cmd"]
	if !ok || handler == nil {
		t.Fatalf("command not registered: %+v", scriptCommands)
	}
	handler("X")
	if ok := waitFor(func() bool { return scriptStorageGet(owner, "last_args") == "X" }, time.Second); !ok {
		t.Fatalf("command did not persist args; got %v", scriptStorageGet(owner, "last_args"))
	}
	if scriptCommands["removed_cmd"] != nil {
		t.Fatal("subscription removed before activation left its command registered")
	}

	// 3) Function hotkey present and triggers
	if fn, ok := scriptGetHotkeyFn(owner, "Ctrl-Alt-T"); !ok || fn == nil {
		t.Fatalf("hotkey function not registered")
	} else {
		if fn(makeScriptInputEvent("Ctrl-Alt-T")) {
			t.Fatal("binding ignored event.Consume")
		}
		if ok := waitFor(func() bool { return scriptStorageGet(owner, "hotkey") == "triggered" }, time.Second); !ok {
			t.Fatalf("hotkey did not run; got %v", scriptStorageGet(owner, "hotkey"))
		}
		if got := scriptStorageGet(owner, "hotkey_combo"); got != "Ctrl-Alt-T" {
			t.Fatalf("binding event combo = %v", got)
		}
	}

	// 4) Chat trigger fires
	dispatchScriptChat("Hero says, ping now")
	if ok := waitFor(func() bool { return scriptStorageGet(owner, "chat") == "ping" }, time.Second); !ok {
		t.Fatalf("chat trigger did not fire; got %v", scriptStorageGet(owner, "chat"))
	}
	dispatchScriptChat("Hero says, structured hello")
	if ok := waitFor(func() bool { return scriptStorageGet(owner, "structured_message") != nil }, time.Second); !ok {
		t.Fatal("structured chat handler did not run")
	}
	if got := scriptStorageGet(owner, "structured_speaker"); got != "Hero" {
		t.Fatalf("structured speaker = %v", got)
	}
	if got := scriptStorageGet(owner, "structured_message"); got != "structured hello" {
		t.Fatalf("structured message = %v", got)
	}
	runServerMessageHandlers(scriptapi.ServerMessage{Message: "structured server hello", Type: messageTextTypeSystem})
	if ok := waitFor(func() bool { return scriptStorageGet(owner, "server_message") != nil }, time.Second); !ok {
		t.Fatal("structured server-message handler did not run")
	}
	if got := scriptStorageGet(owner, "server_type"); got != messageTextTypeSystem {
		t.Fatalf("server message type = %v", got)
	}

	// 5) Structured server-message trigger fires
	runServerMessageHandlers(scriptapi.ServerMessage{Message: "all ready here", Type: messageTextTypeSystem})
	if ok := waitFor(func() bool { return scriptStorageGet(owner, "console") == "ready" }, time.Second); !ok {
		t.Fatalf("console trigger did not fire; got %v", scriptStorageGet(owner, "console"))
	}

	// 6) Input text set by script
	if got := scriptInputText(); got != "test-in" {
		t.Fatalf("input text = %q, want %q", got, "test-in")
	}
}

// TestScriptAPIFull exercises most of the gt2 API via a script script.
func TestScriptAPIFull(t *testing.T) {
	// Isolate data dir
	origDir := dataDirPath
	dataDirPath = t.TempDir()
	t.Cleanup(func() { dataDirPath = origDir })

	// Enable console output from scripts for Print()
	// Preload some environment: last click, player name/players, inventory
	lastClickMu.Lock()
	lastClick = ClickInfo{X: 10, Y: 20, Button: 2, OnMobile: false}
	lastClickMu.Unlock()
	playerName = "Hero"
	playersMu.Lock()
	players = map[string]*Player{
		"Hero":   {Name: "Hero", IsNPC: false},
		"Other":  {Name: "Other", IsNPC: false},
		"Goblin": {Name: "Goblin", IsNPC: true},
	}
	playersMu.Unlock()

	resetInventory()
	addInventoryItem(200, -1, "Shield", true)
	addInventoryItem(201, -1, "Sword", false)

	// Reset script state
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
	scriptHotkeyFnMu = sync.RWMutex{}
	scriptHotkeyFns = map[string]map[string]func(InputEvent) bool{}
	shortcutMu = sync.RWMutex{}
	shortcutMaps = map[string]map[string]string{}
	shortcutRegistrations = map[string]scriptRegistrationHandle{}
	chatHandlersMu = sync.RWMutex{}
	scriptStructuredChatHandlers = nil
	scriptServerMessageHandlers = nil
	scriptLifecycleHandlers = nil
	scriptChangeHandlers = nil
	overlayMu = sync.RWMutex{}
	scriptOverlayOps = map[string][]overlayOp{}
	scriptRepeats = map[string][]*scriptRepeatRegistration{}
	scriptTickWaiters = map[string][]*tickWaiter{}

	// Owner and metadata
	const owner = "apifull_owner"
	scriptDisplayNames[owner] = "APIFull"
	scriptAuthors[owner] = "Test"

	// Load the script
	srcPath := scriptAPIFixturePath("api_full.go")
	src, err := os.ReadFile(srcPath)
	if err != nil {
		t.Fatalf("read script: %v", err)
	}
	consoleLog = messageLog{max: maxMessages}
	loadscriptSource(owner, "APIFull", srcPath, src, restrictedStdlib())

	// Helper wait
	waitFor := func(cond func() bool, d time.Duration) bool {
		deadline := time.Now().Add(d)
		for time.Now().Before(deadline) {
			if cond() {
				return true
			}
			time.Sleep(5 * time.Millisecond)
		}
		return cond()
	}

	if ok := waitFor(func() bool { return scriptStorageGet(owner, "started") == "yes" }, 2*time.Second); !ok {
		t.Fatalf("script did not start")
	}
	if got := scriptStorageGet(owner, "config_default"); got != false {
		t.Fatalf("config default = %v, want false", got)
	}
	if scriptStorageGet(owner, "loaded_string") != "hello" || scriptStorageGet(owner, "loaded_bool") != true ||
		scriptStorageGet(owner, "loaded_integer") != 7 || scriptStorageGet(owner, "loaded_decimal") != 2.5 {
		t.Fatalf("typed storage reads failed: %v/%v/%v/%v", scriptStorageGet(owner, "loaded_string"), scriptStorageGet(owner, "loaded_bool"), scriptStorageGet(owner, "loaded_integer"), scriptStorageGet(owner, "loaded_decimal"))
	}
	if scriptStorageGet(owner, "loaded_json_ok") != true || scriptStorageGet(owner, "loaded_json_name") != "state" || scriptStorageGet(owner, "loaded_json_count") != 3 {
		t.Fatalf("JSON storage read failed: %v/%v/%v", scriptStorageGet(owner, "loaded_json_ok"), scriptStorageGet(owner, "loaded_json_name"), scriptStorageGet(owner, "loaded_json_count"))
	}
	if !scriptSetConfigValue(owner, "enabled", true) {
		t.Fatal("script config was not registered")
	}
	if ok := waitFor(func() bool { return scriptStorageGet(owner, "config_callback") == "yes" }, time.Second); !ok {
		t.Fatalf("script config callback did not run")
	}

	// Console output includes the debug print message
	if msgs := getConsoleMessages(); len(msgs) == 0 || !strings.Contains(strings.Join(msgs, "\n"), "apifull:init") {
		t.Fatalf("console missing print: %v", msgs)
	}

	// Input/shortcuts
	if got := scriptInputText(); got != "in_text" {
		t.Fatalf("input %q want %q", got, "in_text")
	}
	shortcutMu.RLock()
	if m := shortcutMaps[owner]; m == nil || m["yy"] != "/yell " || m["gg"] != "/give " {
		t.Fatalf("shortcuts: %+v", m)
	}
	shortcutMu.RUnlock()

	// Commands
	if _, ok := scriptCommands["apit_cmd"]; !ok {
		t.Fatalf("command not registered")
	}
	scriptCommands["apit_cmd"]("ARG")
	if ok := waitFor(func() bool { return scriptStorageGet(owner, "last_args") == "ARG" }, time.Second); !ok {
		t.Fatalf("cmd handler failed")
	}
	// Send effect
	cmds := getQueuedCommands()
	if len(cmds) == 0 {
		t.Fatalf("no commands queued: %v", cmds)
	}

	// Overlay ops
	overlayMu.RLock()
	ops := append([]overlayOp(nil), scriptOverlayOps[owner]...)
	overlayMu.RUnlock()
	if len(ops) < 3 {
		t.Fatalf("overlay ops: %+v", ops)
	}

	// World size and image size
	if scriptStorageGet(owner, "world_w") != gameAreaSizeX || scriptStorageGet(owner, "world_h") != gameAreaSizeY {
		t.Fatalf("world size wrong: %v,%v", scriptStorageGet(owner, "world_w"), scriptStorageGet(owner, "world_h"))
	}
	// Image size likely 0,0 without resources
	if scriptStorageGet(owner, "img_w") != 0 || scriptStorageGet(owner, "img_h") != 0 {
		t.Fatalf("image size unexpected non-zero: %v,%v", scriptStorageGet(owner, "img_w"), scriptStorageGet(owner, "img_h"))
	}

	// Player/world info
	if scriptStorageGet(owner, "me") != "Hero" {
		t.Fatalf("me wrong: %v", scriptStorageGet(owner, "me"))
	}
	if scriptStorageGet(owner, "cl_version") != clVersion {
		t.Fatalf("CLVersion wrong: %v", scriptStorageGet(owner, "cl_version"))
	}
	if scriptStorageGet(owner, "player_field") != false {
		t.Fatalf("expanded Player field unavailable: %v", scriptStorageGet(owner, "player_field"))
	}
	if v, ok := scriptStorageGet(owner, "players_len").(int); !ok || v != 3 {
		t.Fatalf("players_len wrong: %v", scriptStorageGet(owner, "players_len"))
	}
	if v, ok := scriptStorageGet(owner, "inv_len").(int); !ok || v != 2 {
		t.Fatalf("inv_len wrong: %v", scriptStorageGet(owner, "inv_len"))
	}
	if scriptStorageGet(owner, "has_shield") != true || scriptStorageGet(owner, "is_equipped") != true {
		t.Fatalf("has/is equip wrong: %v/%v", scriptStorageGet(owner, "has_shield"), scriptStorageGet(owner, "is_equipped"))
	}

	// Last-click snapshot
	if scriptStorageGet(owner, "click_x") != int(10) || scriptStorageGet(owner, "click_y") != int(20) || scriptStorageGet(owner, "click_btn") != "RightClick" {
		t.Fatalf("last click mismatch: x=%v y=%v b=%v", scriptStorageGet(owner, "click_x"), scriptStorageGet(owner, "click_y"), scriptStorageGet(owner, "click_btn"))
	}

	// Advance ticks for WaitTicks
	scriptAdvanceTick()
	scriptAdvanceTick()
	if ok := waitFor(func() bool { return scriptStorageGet(owner, "slept") == "yes" }, time.Second); !ok {
		t.Fatalf("sleep ticks not completed")
	}

	// Structured events
	dispatchScriptChat("Goblin says, ping")
	runServerMessageHandlers(scriptapi.ServerMessage{Message: "system ready", Type: messageTextTypeSystem})
	dispatchScriptChange(ChangeEvent{Type: ChangeInventory})
	if ok := waitFor(func() bool {
		return scriptStorageGet(owner, "chat_message") == "ping" &&
			scriptStorageGet(owner, "chat_npc") == true &&
			scriptStorageGet(owner, "server_type") == messageTextTypeSystem &&
			scriptStorageGet(owner, "inventory_changed") == true
	}, time.Second); !ok {
		t.Fatalf("structured events failed: chat=%v npc=%v server=%v inventory=%v",
			scriptStorageGet(owner, "chat_message"), scriptStorageGet(owner, "chat_npc"),
			scriptStorageGet(owner, "server_type"), scriptStorageGet(owner, "inventory_changed"))
	}

	// Bind registration is visible and callable.
	list := scriptHotkeys(owner)
	foundBinding := false
	for _, hk := range list {
		if hk.Combo == "Ctrl-Alt-F" {
			foundBinding = true
		}
	}
	if !foundBinding {
		t.Fatalf("hotkeys list not as expected: %+v", list)
	}
	if binding, ok := scriptGetHotkeyFn(owner, "Ctrl-Alt-F"); !ok || binding(makeScriptInputEvent("Ctrl-Alt-F")) {
		t.Fatal("binding missing or did not consume input")
	}
	if ok := waitFor(func() bool { return scriptStorageGet(owner, "hkf") == "ok" }, time.Second); !ok {
		t.Fatal("binding callback did not run")
	}
}
