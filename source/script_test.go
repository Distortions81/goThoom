package main

import (
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestScriptPrintAlwaysWritesToConsole(t *testing.T) {
	withoutConsoleTimestamps(t)
	consoleLog = messageLog{max: maxMessages}

	scriptConsole("visible script message")

	msgs := getConsoleMessages()
	if len(msgs) != 1 || msgs[0] != "visible script message" {
		t.Fatalf("script print output = %v", msgs)
	}
}

func TestScriptRegistrationUsesDebugTraceNotConsole(t *testing.T) {
	withoutConsoleTimestamps(t)
	originalDebug := gs.scriptEventDebug
	gs.scriptEventDebug = true
	t.Cleanup(func() { gs.scriptEventDebug = originalDebug })
	consoleLog = messageLog{max: maxMessages}
	scriptDebugMu.Lock()
	scriptDebugLines = nil
	scriptDebugMu.Unlock()

	const owner = "registration-trace-test"
	const command = "registration_trace_test"
	scriptMu.Lock()
	scriptDisabled[owner] = false
	delete(scriptCommands, command)
	delete(scriptCommandOwners, command)
	scriptMu.Unlock()
	t.Cleanup(func() {
		scriptMu.Lock()
		delete(scriptDisabled, owner)
		delete(scriptCommands, command)
		delete(scriptCommandOwners, command)
		scriptMu.Unlock()
	})

	scriptRegisterCommand(owner, command, func(string) {})
	if messages := getConsoleMessages(); len(messages) != 0 {
		t.Fatalf("registration leaked into console: %v", messages)
	}
	scriptDebugMu.Lock()
	lines := append([]string(nil), scriptDebugLines...)
	scriptDebugMu.Unlock()
	if len(lines) != 1 || !strings.Contains(lines[0], "Registered command: /"+command) {
		t.Fatalf("registration debug trace = %v", lines)
	}
}

// Test that script equip command skips already equipped items.
func TestScriptEquipAlreadyEquipped(t *testing.T) {
	withoutConsoleTimestamps(t)
	resetInventory()
	addInventoryItem(200, -1, "Shield", true)
	consoleLog = messageLog{max: maxMessages}
	pendingCommand = ""
	scriptEquipByName("tester", "Shield")
	msgs := getConsoleMessages()
	if len(msgs) == 0 || msgs[len(msgs)-1] != "Shield already equipped, skipping" {
		t.Fatalf("unexpected console messages %v", msgs)
	}
	if pendingCommand != "" {
		t.Fatalf("pending command queued: %q", pendingCommand)
	}
}

func TestScriptEquipWaitsForServerInventoryUpdate(t *testing.T) {
	const owner = "server_equipment_test"
	resetScriptCallbackTestState(t, owner)
	resetCommandStateForTest(t, 1)
	resetInventory()
	t.Cleanup(resetInventory)
	addInventoryItem(200, -1, "Shield", false)

	scriptEquipByName(owner, "Shield")
	items := getInventory()
	if len(items) != 1 || items[0].Equipped {
		t.Fatalf("script equip changed local state before server update: %+v", items)
	}
	if got, want := getQueuedCommands(), []string{"/equip 200"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("script equip commands=%v, want %v", got, want)
	}
}

// getQueuedCommands returns the pending command followed by any queued commands.
func getQueuedCommands() []string {
	cmds := append([]string{}, commandQueue...)
	if pendingCommand != "" {
		cmds = append([]string{pendingCommand}, cmds...)
	}
	return cmds
}

func TestScriptSendUsesFIFOAndThrottle(t *testing.T) {
	origSpamKill := gs.ScriptSpamKill
	gs.ScriptSpamKill = true
	t.Cleanup(func() { gs.ScriptSpamKill = origSpamKill })

	const owner = "command_send_test"
	scriptMu = sync.RWMutex{}
	scriptDisabled = map[string]bool{owner: false}
	scriptSendHistory = map[string][]time.Time{}
	clearCommands()
	t.Cleanup(clearCommands)

	scriptCommand(owner, " /one ")
	scriptCommand(owner, "/two")
	scriptCommand(owner, "/three")

	if got, want := getQueuedCommands(), []string{"/one", "/two", "/three"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("command order = %v, want %v", got, want)
	}
	if got := len(scriptSendHistory[owner]); got != 3 {
		t.Fatalf("throttle history count = %d, want 3", got)
	}
	scriptCommand(owner, "   ")
	if got := len(scriptSendHistory[owner]); got != 3 {
		t.Fatalf("empty command consumed throttle budget: %d", got)
	}
}

func TestScriptCommandRejectionsAreVisible(t *testing.T) {
	withoutConsoleTimestamps(t)
	const owner = "command_rejection_test"
	resetScriptCallbackTestState(t, owner)
	resetInventory()

	scriptCommand(owner, "   ")
	scriptEquipByName(owner, "missing item")
	now := time.Now()
	scriptSendHistory[owner] = make([]time.Time, 30)
	for i := range scriptSendHistory[owner] {
		scriptSendHistory[owner][i] = now
	}
	scriptCommand(owner, "/too-many")

	messages := strings.Join(getConsoleMessages(), "\n")
	for _, want := range []string{
		"command rejected: empty command",
		"equip target not found: missing item",
		"command rejected: rate limit exceeded",
	} {
		if !strings.Contains(messages, want) {
			t.Fatalf("missing visible error %q in %s", want, messages)
		}
	}
}

func TestScriptWithEquipmentRestoresAfterPanic(t *testing.T) {
	const owner = "equipment_task_test"
	resetScriptCallbackTestState(t, owner)
	resetInventory()
	clearCommands()
	t.Cleanup(func() {
		resetInventory()
		clearCommands()
	})
	addInventoryItem(10, -1, "Training Sword", true)
	addInventoryItem(20, -1, "Lucky Die", false)

	func() {
		defer func() { _ = recover() }()
		scriptWithEquipment(owner, "Lucky Die", func() {
			scriptCommand(owner, "/roll")
			panic("task failed")
		})
	}()

	items := getInventory()
	for _, item := range items {
		if item.ID == 20 && item.Equipped {
			t.Fatal("temporary equipment remained equipped after panic")
		}
		if item.ID == 10 && !item.Equipped {
			t.Fatal("previous equipment was not restored")
		}
	}
	if got, want := getQueuedCommands(), []string{"/equip 20", "/roll", "/unequip 20"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("equipment task commands = %v, want %v", got, want)
	}
}

func TestScriptBindRegistersFunctionHotkey(t *testing.T) {
	origHotkeys := hotkeys
	origFns := scriptHotkeyFns
	origCommands := scriptCommands
	origOwners := scriptCommandOwners
	origDisabled := scriptDisabled
	origEnabled := scriptHotkeyEnabled
	origDataDir := dataDirPath
	defer func() {
		hotkeys = origHotkeys
		scriptHotkeyFns = origFns
		scriptCommands = origCommands
		scriptCommandOwners = origOwners
		scriptDisabled = origDisabled
		scriptHotkeyEnabled = origEnabled
		dataDirPath = origDataDir
	}()

	hotkeys = nil
	scriptHotkeyFns = map[string]map[string]func(InputEvent) bool{}
	scriptCommands = map[string]scriptCommandHandler{}
	scriptCommandOwners = map[string]string{}
	scriptDisabled = map[string]bool{}
	scriptHotkeyEnabled = map[string]map[string]bool{}
	dataDirPath = t.TempDir()

	ran := false
	scriptAddHotkeyFn("test", "F4", func(InputEvent) { ran = true })

	if _, ok := scriptCommands["__hk_f4"]; ok {
		t.Fatal("Bind registered a server-command handler")
	}
	fn, ok := scriptGetHotkeyFn("test", "F4")
	if !ok || fn == nil {
		t.Fatal("Bind did not register a function hotkey")
	}
	fn(InputEvent{Chord: "F4", Key: "F4"})
	if !ran {
		t.Fatal("Bind hotkey did not invoke its handler")
	}
}

// Test registering and running a mixed-case command and ensuring disabled scripts
// cannot run commands.
func TestScriptRegisterAndDisableCommand(t *testing.T) {
	withoutConsoleTimestamps(t)
	// Reset shared state.
	scriptMu = sync.RWMutex{}
	scriptCommands = map[string]scriptCommandHandler{}
	scriptCommandOwners = map[string]string{}
	scriptDisabled = map[string]bool{}
	scriptInvalid = map[string]bool{}
	scriptEnabledFor = map[string]scriptScope{}
	scriptSendHistory = map[string][]time.Time{}
	consoleLog = messageLog{max: maxMessages}
	commandQueue = nil
	pendingCommand = ""

	owner := "tester"
	scriptRegisterCommand(owner, "MiXeD", func(args string) {
		consoleMessage("handled " + args)
	})

	if _, ok := scriptCommands["mixed"]; !ok {
		t.Fatalf("command not registered: %v", scriptCommands)
	}

	handler := scriptCommands["mixed"]
	handler("input")

	msgs := getConsoleMessages()
	if len(msgs) == 0 || msgs[len(msgs)-1] != "handled input" {
		t.Fatalf("unexpected console messages %v", msgs)
	}

	// Disable script and ensure Send does nothing.
	scriptDisabled[owner] = true
	consoleLog = messageLog{max: maxMessages}
	commandQueue = nil
	pendingCommand = ""

	scriptCommand(owner, "/wave")

	if msgs := getConsoleMessages(); len(msgs) != 0 {
		t.Fatalf("console output when script disabled: %v", msgs)
	}
	if cmds := getQueuedCommands(); len(cmds) != 0 {
		t.Fatalf("commands queued when script disabled: %v", cmds)
	}
}

// Test that when a script registers a command but is later disabled, user-entered
// commands with that name still fall through to the server.
func TestDisabledscriptCommandFallsThrough(t *testing.T) {
	// Reset shared state.
	scriptCommands = map[string]scriptCommandHandler{}
	scriptCommandOwners = map[string]string{}
	scriptDisabled = map[string]bool{}
	pendingCommand = ""

	owner := "tester"
	scriptRegisterCommand(owner, "sleep", func(args string) {})
	scriptDisabled[owner] = true

	txt := "/sleep"
	if strings.HasPrefix(txt, "/") {
		parts := strings.SplitN(strings.TrimPrefix(txt, "/"), " ", 2)
		name := strings.ToLower(parts[0])
		args := ""
		if len(parts) > 1 {
			args = parts[1]
		}
		if handler, ok := scriptCommands[name]; ok && handler != nil {
			owner := scriptCommandOwners[name]
			if !scriptDisabled[owner] {
				consoleMessage("> " + txt)
				go handler(args)
			} else {
				pendingCommand = txt
			}
		} else {
			pendingCommand = txt
		}
	}

	if pendingCommand != "/sleep" {
		t.Fatalf("pending command %q, want %q", pendingCommand, "/sleep")
	}
}

// Test that registering a command twice logs a conflict and keeps the original handler.
func TestScriptRegisterCommandConflict(t *testing.T) {
	withoutConsoleTimestamps(t)
	// Reset shared state.
	scriptMu = sync.RWMutex{}
	scriptCommands = map[string]scriptCommandHandler{}
	scriptCommandOwners = map[string]string{}
	consoleLog = messageLog{max: maxMessages}

	owner1 := "one"
	owner2 := "two"

	ran := false
	scriptRegisterCommand(owner1, "cmd", func(args string) { ran = true })

	// Clear console messages before second registration attempt.
	consoleLog = messageLog{max: maxMessages}

	scriptRegisterCommand(owner2, "cmd", func(args string) {})

	msgs := getConsoleMessages()
	want := "[script] command conflict: /cmd already registered"
	if len(msgs) == 0 || msgs[len(msgs)-1] != want {
		t.Fatalf("unexpected console messages %v", msgs)
	}

	// Ensure original handler remains registered.
	if h, ok := scriptCommands["cmd"]; ok {
		h("")
	}
	if !ran {
		t.Fatalf("original handler overwritten")
	}
}
