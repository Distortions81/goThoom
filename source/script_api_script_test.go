//go:build integration
// +build integration

package main

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestScriptAPISmoke loads a simple script script (via Yaegi) that uses the
// gt API and verifies side effects through the existing script machinery.
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
	scriptHotkeyFns = map[string]map[string]func(HotkeyEvent){}

	// Reset shortcuts and triggers/handlers
	shortcutMu = sync.RWMutex{}
	shortcutMaps = map[string]map[string]string{}
	triggerHandlersMu = sync.RWMutex{}
	scriptTriggers = map[string][]triggerHandler{}
	scriptConsoleTriggers = map[string][]triggerHandler{}
	chatHandlersMu = sync.RWMutex{}
	scriptChatHandlers = nil
	inputHandlersMu = sync.RWMutex{}
	scriptInputHandlers = nil

	// Owner metadata required for storage hashing, messages, etc.
	const owner = "apitest_owner"
	scriptDisplayNames[owner] = "APISmoke"
	scriptAuthors[owner] = "Test"

	// Load the script script source and execute
	srcPath := filepath.Join("script_tests", "api_smoke.go")
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

	// 3) Function hotkey present and triggers
	if fn, ok := scriptGetHotkeyFn(owner, "Ctrl-Alt-T"); !ok || fn == nil {
		t.Fatalf("hotkey function not registered")
	} else {
		fn(HotkeyEvent{Combo: "Ctrl-Alt-T", Parts: []string{"Ctrl", "Alt", "T"}, Trigger: "T"})
		if ok := waitFor(func() bool { return scriptStorageGet(owner, "hotkey") == "triggered" }, time.Second); !ok {
			t.Fatalf("hotkey did not run; got %v", scriptStorageGet(owner, "hotkey"))
		}
	}

	// 4) Chat trigger fires
	runChatTriggers("ping now")
	if ok := waitFor(func() bool { return scriptStorageGet(owner, "chat") == "ping" }, time.Second); !ok {
		t.Fatalf("chat trigger did not fire; got %v", scriptStorageGet(owner, "chat"))
	}

	// 5) Console trigger fires
	runConsoleTriggers("all ready here")
	if ok := waitFor(func() bool { return scriptStorageGet(owner, "console") == "ready" }, time.Second); !ok {
		t.Fatalf("console trigger did not fire; got %v", scriptStorageGet(owner, "console"))
	}

	// 6) Input text set by script
	if got := scriptInputText(); got != "test-in" {
		t.Fatalf("input text = %q, want %q", got, "test-in")
	}
}

// TestScriptAPIFull exercises most of the gt API via a script script.
func TestScriptAPIFull(t *testing.T) {
	// Isolate data dir
	origDir := dataDirPath
	dataDirPath = t.TempDir()
	t.Cleanup(func() { dataDirPath = origDir })

	// Enable console output from scripts for Print()
	gs.scriptOutputDebug = true

	// Preload some environment: world size, last click, player name/players, inventory
	gameAreaSizeX, gameAreaSizeY = 320, 200
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
	scriptHotkeyFns = map[string]map[string]func(HotkeyEvent){}
	shortcutMu = sync.RWMutex{}
	shortcutMaps = map[string]map[string]string{}
	triggerHandlersMu = sync.RWMutex{}
	scriptTriggers = map[string][]triggerHandler{}
	scriptConsoleTriggers = map[string][]triggerHandler{}
	chatHandlersMu = sync.RWMutex{}
	scriptChatHandlers = nil
	inputHandlersMu = sync.RWMutex{}
	scriptInputHandlers = nil
	overlayMu = sync.RWMutex{}
	scriptOverlayOps = map[string][]overlayOp{}
	scriptTimers = map[string][]*time.Timer{}
	scriptTickerStops = map[string][]chan struct{}{}
	scriptTickWaiters = map[string][]*tickWaiter{}

	// Owner and metadata
	const owner = "apifull_owner"
	scriptDisplayNames[owner] = "APIFull"
	scriptAuthors[owner] = "Test"

	// Load the script
	srcPath := filepath.Join("script_tests", "api_full.go")
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

	// Console output includes the debug print message
	if msgs := getConsoleMessages(); len(msgs) == 0 || !strings.Contains(strings.Join(msgs, "\n"), "apifull:init") {
		t.Fatalf("console missing print: %v", msgs)
	}

	// Input/shortcuts
	if got := scriptInputText(); got != "in_text" {
		t.Fatalf("input %q want %q", got, "in_text")
	}
	out := runInputHandlers("foo test")
	if out != "bar test" {
		t.Fatalf("input handler: %q", out)
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
	// Run/Cmd/RunCommand/EnqueueCommand effects
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
	if scriptStorageGet(owner, "world_w") != 320 || scriptStorageGet(owner, "world_h") != 200 {
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
	if v, ok := scriptStorageGet(owner, "players_len").(int); !ok || v != 3 {
		t.Fatalf("players_len wrong: %v", scriptStorageGet(owner, "players_len"))
	}
	if v, ok := scriptStorageGet(owner, "inv_len").(int); !ok || v != 2 {
		t.Fatalf("inv_len wrong: %v", scriptStorageGet(owner, "inv_len"))
	}
	if scriptStorageGet(owner, "has_shield") != true || scriptStorageGet(owner, "is_equipped") != true {
		t.Fatalf("has/is equip wrong: %v/%v", scriptStorageGet(owner, "has_shield"), scriptStorageGet(owner, "is_equipped"))
	}

	// Input state from last click and key/mouse
	if scriptStorageGet(owner, "key_a") != false || scriptStorageGet(owner, "mouse_right") != false {
		t.Fatalf("key/mouse unexpected true")
	}
	if scriptStorageGet(owner, "wheel_dx") != float64(0) || scriptStorageGet(owner, "wheel_dy") != float64(0) {
		t.Fatalf("wheel non-zero: %v,%v", scriptStorageGet(owner, "wheel_dx"), scriptStorageGet(owner, "wheel_dy"))
	}
	if scriptStorageGet(owner, "click_x") != int(10) || scriptStorageGet(owner, "click_y") != int(20) || scriptStorageGet(owner, "click_btn") != 2 {
		t.Fatalf("last click mismatch: x=%v y=%v b=%v", scriptStorageGet(owner, "click_x"), scriptStorageGet(owner, "click_y"), scriptStorageGet(owner, "click_btn"))
	}

	// String helpers
	mustTrue := []string{"eq_ic", "starts", "ends", "incl"}
	for _, k := range mustTrue {
		if scriptStorageGet(owner, k) != true {
			t.Fatalf("%s not true", k)
		}
	}
	if scriptStorageGet(owner, "lower") != "hello" || scriptStorageGet(owner, "upper") != "HELLO" || scriptStorageGet(owner, "trim") != "hi" || scriptStorageGet(owner, "trim_s") != "hi" || scriptStorageGet(owner, "trim_e") != "hi" {
		t.Fatalf("string helpers wrong")
	}
	if v, ok := scriptStorageGet(owner, "words").([]string); ok {
		if len(v) != 3 {
			t.Fatalf("words len: %v", v)
		}
	}
	// Yaegi may produce []any for slices; tolerate either
	if v, ok := scriptStorageGet(owner, "split").([]string); ok {
		if len(v) != 3 {
			t.Fatalf("split len: %v", v)
		}
	} else if v2, ok2 := scriptStorageGet(owner, "split").([]any); ok2 {
		if len(v2) != 3 {
			t.Fatalf("split len: %v", v2)
		}
	}
	if scriptStorageGet(owner, "join") != "a,b,c" || scriptStorageGet(owner, "repl") != "haper" {
		t.Fatalf("join/repl wrong: %v/%v", scriptStorageGet(owner, "join"), scriptStorageGet(owner, "repl"))
	}

	// Timers
	if ok := waitFor(func() bool { return scriptStorageGet(owner, "after") == "yes" }, time.Second); !ok {
		t.Fatalf("after not fired")
	}
	if ok := waitFor(func() bool { return scriptStorageGet(owner, "afterdur") == "yes" }, time.Second); !ok {
		t.Fatalf("afterdur not fired")
	}
	// Advance ticks for SleepTicks goroutine
	scriptAdvanceTick()
	scriptAdvanceTick()
	if ok := waitFor(func() bool { return scriptStorageGet(owner, "slept") == "yes" }, time.Second); !ok {
		t.Fatalf("sleep ticks not completed")
	}
	if ok := waitFor(func() bool { v := scriptStorageGet(owner, "every"); return v == "2" || v == "3" }, time.Second); !ok {
		t.Fatalf("every not ticking: %v", scriptStorageGet(owner, "every"))
	}
	if ok := waitFor(func() bool { v := scriptStorageGet(owner, "everydur"); return v == "2" || v == "3" }, time.Second); !ok {
		t.Fatalf("everydur not ticking: %v", scriptStorageGet(owner, "everydur"))
	}

	// Triggers
	runChatTriggers("Hero says, ping")   // Self and Player (self)
	runChatTriggers("Other says, ping")  // Other and Player (other)
	runChatTriggers("Goblin says, ping") // NPC
	runChatTriggers("Unknown ping")      // Creature
	runConsoleTriggers("system ready")
	runConsoleTriggers("legacy mode")
	runChatTriggers("bb now")
	runChatTriggers("unit test")

	if ok := waitFor(func() bool {
		return scriptStorageGet(owner, "chat_any") == "1" &&
			scriptStorageGet(owner, "chat_player") == "1" &&
			scriptStorageGet(owner, "chat_npc") == "1" &&
			scriptStorageGet(owner, "chat_creature") == "1" &&
			scriptStorageGet(owner, "chat_self") == "1" &&
			scriptStorageGet(owner, "chat_other") == "1" &&
			scriptStorageGet(owner, "chat_from") == "1" &&
			scriptStorageGet(owner, "chat_pfrom") == "1" &&
			scriptStorageGet(owner, "chat_ofrom") == "1" &&
			scriptStorageGet(owner, "cons_new") == "1" &&
			scriptStorageGet(owner, "cons_old") == "1" &&
			scriptStorageGet(owner, "legacy_trig") == "1" &&
			scriptStorageGet(owner, "sing_trig") == "1" &&
			scriptStorageGet(owner, "allchat") != ""
	}, time.Second); !ok {
		t.Fatalf("triggers not all fired")
	}

	// Player handler
	notifyPlayerHandlers(Player{Name: "Tester"})
	if ok := waitFor(func() bool { return scriptStorageGet(owner, "player_seen") == "Tester" }, time.Second); !ok {
		t.Fatalf("player handler not fired")
	}

	// Hotkeys: the Key-based hotkey should exist; the added/removed Ctrl-U should not
	list := scriptHotkeys(owner)
	foundKey := false
	foundCtrlU := false
	for _, hk := range list {
		if hk.Combo == "Ctrl-Alt-F" {
			foundKey = true
		}
		if hk.Combo == "Ctrl-U" {
			foundCtrlU = true
		}
	}
	if !foundKey || foundCtrlU {
		t.Fatalf("hotkeys list not as expected: %+v", list)
	}
}
