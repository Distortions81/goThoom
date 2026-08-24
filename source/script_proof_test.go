package main

import (
	"path/filepath"
	"strings"
	"testing"

	scriptapi "gt2"
)

func activateBundledProofScript(t *testing.T, owner, filename string) scriptEventSimulator {
	t.Helper()
	resetScriptCallbackTestState(t, owner)
	source, err := scriptScripts.ReadFile(filepath.ToSlash(filepath.Join("scripts", filename)))
	if err != nil {
		t.Fatal(err)
	}
	prepared, err := prepareScriptSource(owner, source, restrictedStdlib())
	if err != nil {
		t.Fatalf("prepare %s: %v", filename, err)
	}
	if err := scriptCandidateConflict(owner, prepared.candidate); err != nil {
		disposePreparedScript(prepared)
		t.Fatalf("%s conflicts: %v", filename, err)
	}
	activatePreparedScript(owner, prepared)
	t.Cleanup(func() {
		if scriptIsRunning(owner) {
			disablescript(owner, "test cleanup")
		}
	})
	return scriptEventSimulator{owner: owner}
}

func TestDiceRollProof(t *testing.T) {
	const owner = "dice_roll_proof"
	originalSpamKill := gs.ScriptSpamKill
	gs.ScriptSpamKill = false
	t.Cleanup(func() { gs.ScriptSpamKill = originalSpamKill })
	resetInventory()
	clearCommands()
	t.Cleanup(func() {
		resetInventory()
		clearCommands()
	})
	addInventoryItem(77, -1, "Lucky Dice", false)

	sim := activateBundledProofScript(t, owner, "dice_roll.go")
	sim.commandAndAdvanceTicks(t, "roll", "2d6", 1)

	commands := getQueuedCommands()
	if len(commands) != 3 || commands[0] != "/equip 77" || !strings.HasPrefix(commands[1], "/me rolls 2d6: ") || commands[2] != "/unequip 77" {
		t.Fatalf("dice roll command sequence = %v", commands)
	}
	items := getInventory()
	if len(items) != 1 || items[0].Equipped {
		t.Fatalf("dice equipment was not restored: %+v", items)
	}
}

func TestBardInstrumentProof(t *testing.T) {
	const owner = "bard_instrument_proof"
	originalSpamKill := gs.ScriptSpamKill
	gs.ScriptSpamKill = false
	t.Cleanup(func() { gs.ScriptSpamKill = originalSpamKill })
	resetInventory()
	clearCommands()
	t.Cleanup(func() {
		resetInventory()
		clearCommands()
	})
	addInventoryItem(10, -1, "instrument case", false)

	sim := activateBundledProofScript(t, owner, "bard.go")
	sim.startCommand(t, "playsong", "pine_flute cfed")
	firstWait := sim.inventoryUpdate(t, nil, func() {
		addInventoryItem(20, -1, "pine_flute", false)
	})
	sim.inventoryUpdate(t, firstWait, func() {
		removeInventoryItem(20, -1)
	})
	sim.barrier(t)

	want := []string{
		"/equip 10",
		"/useitem instrument case /remove pine_flute",
		"/equip 20",
		"/useitem pine_flute cfed",
		"/unequip 20",
		"/useitem instrument case /add pine_flute",
		"/unequip 10",
	}
	commands := getQueuedCommands()
	if len(commands) != len(want) {
		t.Fatalf("bard command sequence = %v, want %v", commands, want)
	}
	for index := range want {
		if commands[index] != want[index] {
			t.Fatalf("bard command %d = %q, want %q (all: %v)", index, commands[index], want[index], commands)
		}
	}
	if items := getInventory(); len(items) != 1 || items[0].Name != "instrument case" || items[0].Equipped {
		t.Fatalf("bard inventory was not restored: %+v", items)
	}
}

func TestShiftClickPullPushProof(t *testing.T) {
	const owner = "shift_click_proof"
	originalSpamKill := gs.ScriptSpamKill
	gs.ScriptSpamKill = false
	t.Cleanup(func() { gs.ScriptSpamKill = originalSpamKill })
	clearCommands()
	t.Cleanup(clearCommands)

	sim := activateBundledProofScript(t, owner, "shift_click_pull_push.go")
	pull := makeScriptInputEvent("Shift-LeftClick")
	pull.PlayerName = "Sir Example the Brave"
	pull.SimpleName = "Sir Example"
	if sim.input(t, pull) {
		t.Fatal("player pull click was passed through")
	}
	push := makeScriptInputEvent("Shift-RightClick")
	push.PlayerName = "Friend"
	push.SimpleName = "Friend"
	if sim.input(t, push) {
		t.Fatal("player push click was passed through")
	}
	empty := makeScriptInputEvent("Shift-LeftClick")
	if !sim.input(t, empty) {
		t.Fatal("non-player click was consumed")
	}

	want := []string{"/pull Sir Example", "/push Friend"}
	commands := getQueuedCommands()
	if len(commands) != len(want) || commands[0] != want[0] || commands[1] != want[1] {
		t.Fatalf("shift-click commands = %v, want %v", commands, want)
	}
}

func TestQuickReplyProof(t *testing.T) {
	const owner = "quick_reply_proof"
	originalSpamKill := gs.ScriptSpamKill
	gs.ScriptSpamKill = false
	t.Cleanup(func() { gs.ScriptSpamKill = originalSpamKill })
	clearCommands()
	t.Cleanup(clearCommands)

	sim := activateBundledProofScript(t, owner, "quick_reply.go")
	sim.chat(t, "High Priestess Aria thinks to you, Are you coming?")
	sim.command(t, "r", "On my way")

	want := "/thinkto High Priestess Aria On my way"
	commands := getQueuedCommands()
	if len(commands) != 1 || commands[0] != want {
		t.Fatalf("quick reply commands = %v, want %q", commands, want)
	}
}

func TestRangeryProof(t *testing.T) {
	const owner = "rangery_proof"
	originalSpamKill := gs.ScriptSpamKill
	gs.ScriptSpamKill = false
	t.Cleanup(func() { gs.ScriptSpamKill = originalSpamKill })
	resetInventory()
	clearCommands()
	stateMu.Lock()
	originalSpirit, originalSpiritMax := state.sp, state.spMax
	state.sp, state.spMax = 0, 20
	stateMu.Unlock()
	t.Cleanup(func() {
		resetInventory()
		clearCommands()
		stateMu.Lock()
		state.sp, state.spMax = originalSpirit, originalSpiritMax
		stateMu.Unlock()
	})
	addInventoryItem(30, -1, "Heartwood Charm", false)
	addInventoryItem(31, -1, "Shieldstone", false)

	sim := activateBundledProofScript(t, owner, "rangery.go")
	if !sim.input(t, makeScriptInputEvent("WheelUp")) {
		t.Fatal("no-spirit wheel input should pass through")
	}
	if commands := getQueuedCommands(); len(commands) != 0 {
		t.Fatalf("no-spirit wheel input sent commands: %v", commands)
	}

	stateMu.Lock()
	state.sp = 10
	stateMu.Unlock()
	if sim.input(t, makeScriptInputEvent("WheelUp")) {
		t.Fatal("handled Heartwood wheel input was passed through")
	}
	if sim.input(t, makeScriptInputEvent("F3")) {
		t.Fatal("handled Shieldstone key was passed through")
	}

	want := []string{"/equip 30", "/useitem Heartwood Charm", "/equip 31", "/useitem Shieldstone"}
	commands := getQueuedCommands()
	if len(commands) != len(want) {
		t.Fatalf("Rangery commands = %v, want %v", commands, want)
	}
	for index := range want {
		if commands[index] != want[index] {
			t.Fatalf("Rangery command %d = %q, want %q (all: %v)", index, commands[index], want[index], commands)
		}
	}
}

func TestLastHitCounterProof(t *testing.T) {
	const owner = "last_hit_counter_proof"
	originalPlayerName := playerName
	playerName = "Hero"
	t.Cleanup(func() { playerName = originalPlayerName })

	sim := activateBundledProofScript(t, owner, "last_hit_counter.go")
	chatHandlersMu.RLock()
	serverHandlers := len(scriptServerMessageHandlers)
	chatHandlersMu.RUnlock()
	scriptConfigMu.RLock()
	configEntries := append([]scriptConfigEntry(nil), scriptConfigEntries[owner]...)
	scriptConfigMu.RUnlock()
	if serverHandlers != 1 || len(configEntries) != 2 {
		t.Fatalf("counter registrations = server:%d config:%d", serverHandlers, len(configEntries))
	}
	for _, entry := range configEntries {
		if entry.Scope != scriptapi.ScopeCharacter {
			t.Fatalf("configuration %q scope = %q, want character", entry.Key, entry.Scope)
		}
	}

	sim.serverMessage(t, scriptServerMessage("youkilled", "You killed a vermine."))
	sim.serverMessage(t, scriptServerMessage("youkilled", "You killed a rat."))
	if got := scriptStorageGet(owner, "last-hits:Hero"); got != 2 {
		t.Fatalf("Hero last-hit count = %v, want 2", got)
	}

	playerName = "Other"
	sim.serverMessage(t, scriptServerMessage("youkilled", "You killed a vermine."))
	if got := scriptStorageGet(owner, "last-hits:Other"); got != 1 {
		t.Fatalf("Other last-hit count = %v, want 1", got)
	}
	if got := scriptStorageGet(owner, "last-hits:Hero"); got != 2 {
		t.Fatalf("Hero count changed with character: %v", got)
	}
}

func TestServerScannerProof(t *testing.T) {
	const owner = "server_scanner_proof"
	sim := activateBundledProofScript(t, owner, "server_scanner.go")

	chatHandlersMu.RLock()
	handlerCount := len(scriptServerMessageHandlers)
	chatHandlersMu.RUnlock()
	if handlerCount != 3 {
		t.Fatalf("server scanner handler count = %d, want 3", handlerCount)
	}
	sim.serverMessage(t, scriptServerMessage("logon", "Aria entered the lands."))
	sim.serverMessage(t, scriptServerMessage("location", "Town Square"))
	sim.serverMessage(t, scriptServerMessage("logoff", "Borin left the lands."))
	sim.serverMessage(t, scriptServerMessage("news", "This must not match."))

	want := map[string]any{
		"last-logon":    "Aria entered the lands.",
		"last-location": "Town Square",
		"last-logoff":   "Borin left the lands.",
	}
	for key, expected := range want {
		if got := scriptStorageGet(owner, key); got != expected {
			t.Fatalf("scanner %s = %v, want %v", key, got, expected)
		}
	}
	if got := scriptStorageGet(owner, "news"); got != nil {
		t.Fatalf("unsubscribed server event was stored: %v", got)
	}
}

func TestLongRunningTimerReloadProof(t *testing.T) {
	const owner = "daily_reminder_proof"
	sim := activateBundledProofScript(t, owner, "daily_reminder.go")
	sim.timers(t)
	if got := scriptStorageGet(owner, "reminder-count"); got != 1 {
		t.Fatalf("initial reminder count = %v, want 1", got)
	}

	scriptMu.RLock()
	oldRepeats := append([]*scriptRepeatRegistration(nil), scriptRepeats[owner]...)
	scriptMu.RUnlock()
	if len(oldRepeats) != 1 {
		t.Fatalf("initial timer registrations = %d, want 1", len(oldRepeats))
	}
	source, err := scriptScripts.ReadFile("scripts/daily_reminder.go")
	if err != nil {
		t.Fatal(err)
	}
	if !loadscriptSource(owner, "Daily Reminder", "daily_reminder.go", source, restrictedStdlib()) {
		t.Fatal("daily reminder reload failed")
	}
	select {
	case <-oldRepeats[0].stop:
	default:
		t.Fatal("reload did not cancel the old long-running timer")
	}
	scriptMu.RLock()
	newRepeats := append([]*scriptRepeatRegistration(nil), scriptRepeats[owner]...)
	scriptMu.RUnlock()
	if len(newRepeats) != 1 || newRepeats[0] == oldRepeats[0] {
		t.Fatalf("timer registrations after reload = %+v", newRepeats)
	}

	sim.timers(t)
	if got := scriptStorageGet(owner, "reminder-count"); got != 2 {
		t.Fatalf("replacement reminder count = %v, want 2", got)
	}
	disablescript(owner, "test timer cleanup")
	select {
	case <-newRepeats[0].stop:
	default:
		t.Fatal("disable did not cancel the replacement timer")
	}
	scriptMu.RLock()
	remaining := len(scriptRepeats[owner])
	scriptMu.RUnlock()
	if remaining != 0 {
		t.Fatalf("timer registrations survived disable: %d", remaining)
	}
}
