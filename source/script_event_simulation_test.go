package main

import (
	"runtime"
	"testing"

	scriptapi "gt2"
)

// scriptEventSimulator drives script callbacks through the same registrations
// and per-script queue used by the client, but without polling or wall-clock
// sleeps. Keep proof-script tests on this path so their results are repeatable.
type scriptEventSimulator struct {
	owner string
}

func (sim scriptEventSimulator) barrier(t *testing.T) {
	t.Helper()
	queue := currentScriptEventQueue(sim.owner)
	if queue == nil || !queueScriptCallbackWaitOn(queue, sim.owner, "Test barrier", func() {}) {
		t.Fatalf("script event queue for %q is not running", sim.owner)
	}
	// Client-facing actions such as Send and Print cross the main-thread
	// dispatcher after the script callback finishes.
	drainScriptDispatcher()
}

func (sim scriptEventSimulator) command(t *testing.T, name, args string) {
	t.Helper()
	sim.startCommand(t, name, args)
	sim.barrier(t)
}

func (sim scriptEventSimulator) startCommand(t *testing.T, name, args string) {
	t.Helper()
	key := normalizeScriptCommand(name)
	scriptMu.RLock()
	handler := scriptCommands[key]
	scriptMu.RUnlock()
	if handler == nil {
		t.Fatalf("script command /%s is not registered", key)
	}
	handler(args)
}

func (sim scriptEventSimulator) commandAndAdvanceTicks(t *testing.T, name, args string, ticks int) {
	t.Helper()
	key := normalizeScriptCommand(name)
	scriptMu.RLock()
	handler := scriptCommands[key]
	scriptMu.RUnlock()
	if handler == nil {
		t.Fatalf("script command /%s is not registered", key)
	}
	handler(args)
	for tick := 0; tick < ticks; tick++ {
		waiting := false
		for attempt := 0; attempt < 100_000; attempt++ {
			scriptMu.RLock()
			waiting = len(scriptTickWaiters[sim.owner]) > 0
			scriptMu.RUnlock()
			if waiting {
				break
			}
			runtime.Gosched()
		}
		if !waiting {
			t.Fatalf("script %q did not wait for tick %d", sim.owner, tick+1)
		}
		scriptAdvanceTick()
	}
	sim.barrier(t)
}

func (sim scriptEventSimulator) input(t *testing.T, event InputEvent) bool {
	t.Helper()
	handler, ok := scriptGetHotkeyFn(sim.owner, event.Chord)
	if !ok || handler == nil {
		t.Fatalf("script input %q is not registered", event.Chord)
	}
	return handler(event)
}

func (sim scriptEventSimulator) chat(t *testing.T, message string) {
	t.Helper()
	dispatchScriptChat(message)
	sim.barrier(t)
}

func (sim scriptEventSimulator) serverMessage(t *testing.T, event scriptapi.ServerMessage) {
	t.Helper()
	runServerMessageHandlers(event)
	sim.barrier(t)
}

func (sim scriptEventSimulator) inventory(t *testing.T, inventory []InventoryItem) {
	t.Helper()
	dispatchScriptChange(ChangeEvent{Type: ChangeInventory, Inventory: inventory})
	sim.barrier(t)
}

func (sim scriptEventSimulator) login(t *testing.T, character string) {
	t.Helper()
	scriptSessionLogin(character)
	sim.barrier(t)
}

func (sim scriptEventSimulator) timers(t *testing.T) {
	t.Helper()
	scriptMu.RLock()
	repeats := append([]*scriptRepeatRegistration(nil), scriptRepeats[sim.owner]...)
	scriptMu.RUnlock()
	if len(repeats) == 0 {
		t.Fatalf("script %q has no repeating timers", sim.owner)
	}
	for _, repeat := range repeats {
		if repeat != nil {
			queueScriptCallbackOn(repeat.eventQueue, sim.owner, repeat.event, repeat.callback)
		}
	}
	sim.barrier(t)
}

func (sim scriptEventSimulator) inventoryUpdate(t *testing.T, previous *scriptStateWaiter, update func()) *scriptStateWaiter {
	t.Helper()
	var waiter *scriptStateWaiter
	for attempt := 0; attempt < 100_000; attempt++ {
		scriptMu.RLock()
		for _, candidate := range scriptStateWaiters[sim.owner] {
			if candidate != nil && candidate != previous {
				waiter = candidate
				break
			}
		}
		scriptMu.RUnlock()
		if waiter != nil {
			break
		}
		runtime.Gosched()
	}
	if waiter == nil {
		t.Fatalf("script %q did not wait for an inventory update", sim.owner)
	}
	if update != nil {
		update()
	}
	notifyScriptStateWaiters()
	return waiter
}

func TestScriptEventSimulator(t *testing.T) {
	const owner = "event_simulator_test"
	resetScriptCallbackTestState(t, owner)

	source := []byte(`package main

import (
	"time"
	"gt2"
)

func Init() {
	gt2.Command("simulate", func(args string) { gt2.Store("command", args) })
	gt2.Bind("Shift-LeftClick", func(event gt2.InputEvent) {
		gt2.Store("click_x", event.ScreenX)
		event.Consume()
	})
	gt2.OnChat(gt2.ChatFilter{Contains: "hello"}, func(event gt2.ChatEvent) {
		gt2.Store("chat", event.Speaker)
	})
	gt2.OnServerMessage(gt2.ServerMessageFilter{Type: "system"}, func(event gt2.ServerMessage) {
		gt2.Store("server", event.Message)
	})
	gt2.OnChange(gt2.ChangeInventory, func(event gt2.ChangeEvent) {
		gt2.Store("inventory_count", len(event.Inventory))
	})
	gt2.OnLogin(func(event gt2.LifecycleEvent) { gt2.Store("login", event.Character) })
	gt2.Repeat(time.Hour, func() {
		gt2.Store("timer_count", gt2.LoadInteger("timer_count", 0)+1)
	})
}
`)
	prepared, err := prepareScriptSource(owner, source, restrictedStdlib())
	if err != nil {
		t.Fatalf("prepare script: %v", err)
	}
	if err := scriptCandidateConflict(owner, prepared.candidate); err != nil {
		disposePreparedScript(prepared)
		t.Fatalf("script conflicts: %v", err)
	}
	activatePreparedScript(owner, prepared)
	t.Cleanup(func() {
		if scriptIsRunning(owner) {
			disablescript(owner, "test cleanup")
		}
	})

	sim := scriptEventSimulator{owner: owner}
	sim.command(t, "/simulate", "one two")
	click := makeScriptInputEvent("Shift-LeftClick")
	click.ScreenX = 123
	if sim.input(t, click) {
		t.Fatal("consumed click was passed through")
	}
	sim.chat(t, "Guide says, hello there")
	sim.serverMessage(t, scriptapi.ServerMessage{Type: "system", Message: "Server ready"})
	sim.inventory(t, []InventoryItem{{Name: "Dagger"}, {Name: "Shield"}})
	sim.login(t, "Hero")
	sim.timers(t)

	want := map[string]any{
		"command":         "one two",
		"click_x":         123,
		"chat":            "Guide",
		"server":          "Server ready",
		"inventory_count": 2,
		"login":           "Hero",
		"timer_count":     1,
	}
	for key, expected := range want {
		if got := scriptStorageGet(owner, key); got != expected {
			t.Errorf("stored %s = %#v, want %#v", key, got, expected)
		}
	}
}
