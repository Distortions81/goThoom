package main

import (
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"
)

func resetScriptCallbackTestState(t *testing.T, owner string) {
	t.Helper()
	origDataDir := dataDirPath
	dataDirPath = t.TempDir()
	t.Cleanup(func() { dataDirPath = origDataDir })

	scriptMu = sync.RWMutex{}
	scriptDisplayNames = map[string]string{owner: "Callback Test"}
	scriptAuthors = map[string]string{owner: "Test"}
	scriptDisabled = map[string]bool{owner: false}
	scriptEnabledFor = map[string]scriptScope{owner: {All: true}}
	scriptTerminators = map[string]func(){}
	scriptCommands = map[string]scriptCommandHandler{}
	scriptCommandOwners = map[string]string{}
	scriptSendHistory = map[string][]time.Time{}
	scriptTimers = map[string][]*time.Timer{}
	scriptTickerStops = map[string][]chan struct{}{}
	scriptTickWaiters = map[string][]*tickWaiter{}
	scriptStateWaiters = map[string][]*scriptStateWaiter{}
	scriptStopping = map[string]bool{}

	hotkeysMu = sync.RWMutex{}
	hotkeys = nil
	scriptHotkeyMu = sync.RWMutex{}
	scriptHotkeyEnabled = map[string]map[string]bool{}
	scriptHotkeyFnMu = sync.RWMutex{}
	scriptHotkeyFns = map[string]map[string]func(InputEvent) bool{}
	shortcutMu = sync.RWMutex{}
	shortcutMaps = map[string]map[string]string{}
	shortcutRegistrations = map[string]scriptRegistrationHandle{}
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
	overlayMu = sync.RWMutex{}
	scriptOverlayOps = map[string][]overlayOp{}
	scriptConfigMu = sync.RWMutex{}
	scriptConfigEntries = map[string][]scriptConfigEntry{}
	scriptStoreMu = sync.Mutex{}
	scriptStores = map[string]*scriptStore{}
	scriptDispatchMu = sync.Mutex{}
	scriptDispatchQueue = nil
	scriptEventMu = sync.Mutex{}
	scriptEventQueues = map[string]*scriptEventQueue{}
	startScriptEventQueue(owner)
	scriptSessionMu = sync.Mutex{}
	scriptSessionCharacter = ""
	scriptSessionActive = false
	scriptChangeMu = sync.Mutex{}
	scriptChanges = scriptChangeSnapshot{}
	consoleLog = messageLog{max: maxMessages}
}

func TestScriptChangeEvents(t *testing.T) {
	const owner = "change_events_test"
	resetScriptCallbackTestState(t, owner)

	originalSelectedPlayer, originalSelectedID, originalSelectedIdx := selectedPlayerName, selectedInvID, selectedInvIdx
	originalGeneration := worldStateGeneration.Load()
	scriptLocationMu.RLock()
	originalLocation := scriptLocation
	scriptLocationMu.RUnlock()
	stateMu.Lock()
	originalVitals := [6]int{state.hp, state.hpMax, state.sp, state.spMax, state.balance, state.balanceMax}
	stateMu.Unlock()
	t.Cleanup(func() {
		resetInventory()
		selectedPlayerName, selectedInvID, selectedInvIdx = originalSelectedPlayer, originalSelectedID, originalSelectedIdx
		worldStateGeneration.Store(originalGeneration)
		setScriptLocation(originalLocation)
		stateMu.Lock()
		state.hp, state.hpMax, state.sp, state.spMax, state.balance, state.balanceMax =
			originalVitals[0], originalVitals[1], originalVitals[2], originalVitals[3], originalVitals[4], originalVitals[5]
		stateMu.Unlock()
	})

	resetInventory()
	selectedPlayerName, selectedInvID, selectedInvIdx = "", 0, -1
	setScriptLocation("")
	pollScriptChangeEvents()

	var mu sync.Mutex
	var events []ChangeEvent
	scriptRegisterChange(owner, "", func(event ChangeEvent) {
		mu.Lock()
		events = append(events, event)
		mu.Unlock()
	})
	addInventoryItem(42, 0, "Test Blade", true)
	selectedPlayerName, selectedInvID, selectedInvIdx = "Other", 42, 0
	stateMu.Lock()
	state.hp, state.hpMax, state.sp, state.spMax, state.balance, state.balanceMax = 7, 10, 8, 11, 9, 12
	stateMu.Unlock()
	markWorldStateChanged()
	setScriptLocation("Town Square")
	pollScriptChangeEvents()

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		mu.Lock()
		count := len(events)
		mu.Unlock()
		if count == 7 {
			break
		}
		time.Sleep(time.Millisecond)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(events) != 7 {
		t.Fatalf("change events = %+v", events)
	}
	want := []string{ChangeInventory, ChangeEquipment, ChangeSelectedPlayer, ChangeSelectedItem, ChangeVitals, ChangeWorld, ChangeLocation}
	for i, kind := range want {
		if events[i].Type != kind {
			t.Fatalf("change event %d = %q, want %q", i, events[i].Type, kind)
		}
	}
	if events[3].SelectedItem.Base != "Test Blade" || events[4].Health != 7 || events[6].Location != "Town Square" {
		t.Fatalf("structured change payloads = %+v", events)
	}
}

func TestScriptLifecycleEvents(t *testing.T) {
	const owner = "lifecycle_events_test"
	resetScriptCallbackTestState(t, owner)

	var mu sync.Mutex
	var events []LifecycleEvent
	record := func(event LifecycleEvent) {
		mu.Lock()
		events = append(events, event)
		mu.Unlock()
	}
	for _, kind := range []string{lifecycleLogin, lifecycleLogout, lifecycleCharacterChange, lifecycleStop} {
		scriptRegisterLifecycle(owner, kind, record)
	}

	scriptSessionLogin("Hero")
	scriptSessionLogout("Hero")
	scriptSessionLogin("Other")
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		mu.Lock()
		count := len(events)
		mu.Unlock()
		if count == 4 {
			break
		}
		time.Sleep(time.Millisecond)
	}
	mu.Lock()
	count := len(events)
	gotBeforeStop := append([]LifecycleEvent(nil), events...)
	mu.Unlock()
	if count != 4 {
		t.Fatalf("lifecycle events before stop = %+v", gotBeforeStop)
	}
	disablescript(owner, "test stop")

	mu.Lock()
	defer mu.Unlock()
	if len(events) != 5 {
		t.Fatalf("lifecycle events = %+v", events)
	}
	wantTypes := []string{lifecycleLogin, lifecycleLogout, lifecycleCharacterChange, lifecycleLogin, lifecycleStop}
	for i, want := range wantTypes {
		if events[i].Type != want {
			t.Fatalf("event %d type = %q, want %q", i, events[i].Type, want)
		}
	}
	if events[2].PreviousCharacter != "Hero" || events[2].Character != "Other" {
		t.Fatalf("character-change event = %+v", events[2])
	}
	if events[4].Reason != "test stop" {
		t.Fatalf("stop event = %+v", events[4])
	}
	if len(scriptLifecycleHandlers) != 0 {
		t.Fatalf("lifecycle registrations survived stop: %+v", scriptLifecycleHandlers)
	}
}

func TestScriptSubscriptionRemovesExactlyOnce(t *testing.T) {
	const owner = "subscription_test"
	resetScriptCallbackTestState(t, owner)

	subscription := newScriptSubscription()
	subscription.attach(scriptRegisterCommand(owner, "temporary", func(string) {}))
	if !subscription.Active() || scriptCommands["temporary"] == nil {
		t.Fatal("attached subscription is not active")
	}
	subscription.Remove()
	subscription.Remove()
	if subscription.Active() || scriptCommands["temporary"] != nil {
		t.Fatal("removed subscription left its registration active")
	}

	removedBeforeAttach := newScriptSubscription()
	removedBeforeAttach.Remove()
	removedBeforeAttach.attach(scriptRegisterCommand(owner, "staged", func(string) {}))
	if scriptCommands["staged"] != nil {
		t.Fatal("subscription removed before attachment left its registration active")
	}
}

func TestScriptCallbackPanicDisablesAndCleansOwner(t *testing.T) {
	withoutConsoleTimestamps(t)
	const owner = "callback_test"
	resetScriptCallbackTestState(t, owner)

	terminated := false
	scriptTerminators[owner] = func() {
		terminated = true
		scriptStorageSet(owner, "terminated", true)
	}
	scriptRegisterCommand(owner, "owned", func(string) {})
	scriptAddHotkeyFn(owner, "Ctrl-P", func(InputEvent) {})
	scriptAddShortcut(owner, "p", "/ponder")
	scriptRegisterInputHandler(owner, func(text string) string { return text })
	scriptRegisterChat(owner, "", []string{"owned"}, ChatAny, func(string) {})
	scriptRegisterConsole(owner, []string{"owned"}, func(string) {})
	scriptRegisterChatHandler(owner, func(string) {})
	scriptRegisterPlayerHandler(owner, func(Player) {})
	timer := time.AfterFunc(time.Hour, func() {})
	scriptTimers[owner] = []*time.Timer{timer}
	stop := make(chan struct{})
	scriptTickerStops[owner] = []chan struct{}{stop}
	scriptOverlayOps[owner] = []overlayOp{{kind: 0}}
	scriptRegisterConfig(owner, scriptConfigEntry{Key: "owned", Type: "bool", Value: true})

	if runScriptCallback(owner, "Command", func() { panic("boom") }) {
		t.Fatal("panicking callback reported success")
	}
	drainScriptDispatcher()
	if !scriptIsDisabled(owner) || !terminated {
		t.Fatalf("panic did not stop script: disabled=%v terminated=%v", scriptIsDisabled(owner), terminated)
	}
	if scriptStorageGet(owner, "terminated") != true {
		t.Fatal("Terminate storage was not preserved")
	}
	if _, ok := scriptCommands["owned"]; ok {
		t.Fatal("owned command was not removed")
	}
	if len(scriptHotkeys(owner)) != 0 || len(scriptHotkeyFns[owner]) != 0 {
		t.Fatal("owned hotkey was not removed")
	}
	if len(shortcutMaps[owner]) != 0 || len(scriptInputHandlers) != 0 {
		t.Fatal("owned input bindings were not removed")
	}
	if len(scriptTriggers) != 0 || len(scriptConsoleTriggers) != 0 || len(scriptChatHandlers) != 0 || len(scriptPlayerHandlers) != 0 {
		t.Fatal("owned event handlers were not removed")
	}
	if _, ok := scriptTimers[owner]; ok {
		t.Fatal("owned timers were not removed")
	}
	if _, ok := scriptTickerStops[owner]; ok {
		t.Fatal("owned repeating timers were not removed")
	}
	select {
	case <-stop:
	default:
		t.Fatal("owned repeating timer was not cancelled")
	}
	if _, ok := scriptOverlayOps[owner]; ok || len(scriptConfigEntries[owner]) != 0 {
		t.Fatal("owned overlay or config was not removed")
	}
	messages := strings.Join(getConsoleMessages(), "\n")
	if !strings.Contains(messages, "Command callback panic: boom") {
		t.Fatalf("panic was not reported with callback context: %s", messages)
	}
}

func TestInputAndConfigCallbackPanicsUseOwnedGuard(t *testing.T) {
	withoutConsoleTimestamps(t)
	const owner = "callback_test"

	t.Run("input", func(t *testing.T) {
		resetScriptCallbackTestState(t, owner)
		scriptInputHandlers = []inputHandler{{owner: owner, fn: func(string) string { panic("input boom") }}}
		if got := runInputHandlers("original"); got != "original" {
			t.Fatalf("input changed after callback panic: %q", got)
		}
		drainScriptDispatcher()
		if !scriptIsDisabled(owner) {
			t.Fatal("input callback panic did not disable owner")
		}
	})

	t.Run("config", func(t *testing.T) {
		resetScriptCallbackTestState(t, owner)
		scriptConfigEntries[owner] = []scriptConfigEntry{{
			Key:      "enabled",
			Type:     "bool",
			Value:    false,
			Callback: func(bool) { panic("config boom") },
		}}
		if !scriptSetConfigValue(owner, "enabled", true) {
			t.Fatal("config value was not accepted")
		}
		drainScriptDispatcher()
		if !scriptIsDisabled(owner) {
			t.Fatal("config callback panic did not disable owner")
		}
	})
}

func TestScriptDispatcherPreservesOrderAndOwnership(t *testing.T) {
	const owner = "callback_test"
	resetScriptCallbackTestState(t, owner)
	var got []int
	dispatchScript(owner, func() { got = append(got, 1) })
	dispatchScript(owner, func() { got = append(got, 2) })
	if len(got) != 0 {
		t.Fatalf("script work ran outside dispatcher: %v", got)
	}
	drainScriptDispatcher()
	if len(got) != 2 || got[0] != 1 || got[1] != 2 {
		t.Fatalf("dispatcher order = %v, want [1 2]", got)
	}

	dispatchScript(owner, func() { got = append(got, 3) })
	disablescript(owner, "test")
	drainScriptDispatcher()
	if len(got) != 2 {
		t.Fatalf("disabled owner ran queued work: %v", got)
	}
}

func TestActiveScriptAPIMutationsUseDispatcher(t *testing.T) {
	withoutConsoleTimestamps(t)
	const owner = "callback_test"
	resetScriptCallbackTestState(t, owner)
	candidate := &scriptCandidate{active: true, eventQueue: currentScriptEventQueue(owner)}
	exports := exportsForScriptCandidate(owner, candidate)["gt/gt"]
	printFn := exports["Print"].Interface().(func(string))
	printFn("queued output")
	if messages := getConsoleMessages(); len(messages) != 0 {
		t.Fatalf("script API mutated console outside dispatcher: %v", messages)
	}
	drainScriptDispatcher()
	if messages := getConsoleMessages(); len(messages) != 1 || messages[0] != "queued output" {
		t.Fatalf("dispatched script output = %v", messages)
	}
}

func TestScriptEventQueueSerializesAndCancelsCallbacks(t *testing.T) {
	const owner = "callback_test"
	resetScriptCallbackTestState(t, owner)
	firstStarted := make(chan struct{})
	releaseFirst := make(chan struct{})
	secondRan := make(chan struct{}, 1)

	if !queueScriptCallback(owner, "First", func() {
		close(firstStarted)
		<-releaseFirst
	}) {
		t.Fatal("first callback was not queued")
	}
	<-firstStarted
	if !queueScriptCallback(owner, "Second", func() { secondRan <- struct{}{} }) {
		t.Fatal("second callback was not queued")
	}
	select {
	case <-secondRan:
		t.Fatal("second callback ran concurrently with first")
	case <-time.After(20 * time.Millisecond):
	}
	close(releaseFirst)
	select {
	case <-secondRan:
	case <-time.After(time.Second):
		t.Fatal("second callback did not run after first completed")
	}

	blockerStarted := make(chan struct{})
	releaseBlocker := make(chan struct{})
	if !queueScriptCallback(owner, "Blocker", func() {
		close(blockerStarted)
		<-releaseBlocker
	}) {
		t.Fatal("blocker callback was not queued")
	}
	<-blockerStarted
	waitResult := make(chan bool, 1)
	go func() {
		waitResult <- queueScriptCallbackWait(owner, "Cancelled", func() {
			t.Error("cancelled callback ran")
		})
	}()
	queue := currentScriptEventQueue(owner)
	deadline := time.Now().Add(time.Second)
	for {
		queue.mu.Lock()
		pending := len(queue.events)
		queue.mu.Unlock()
		if pending > 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("synchronous callback was not queued")
		}
		time.Sleep(time.Millisecond)
	}
	stopScriptEventQueue(owner)
	select {
	case ok := <-waitResult:
		if ok {
			t.Fatal("cancelled synchronous callback reported success")
		}
	case <-time.After(time.Second):
		t.Fatal("cancelled synchronous callback was not released")
	}
	if queueScriptCallback(owner, "Late", func() {}) {
		t.Fatal("stopped queue accepted another callback")
	}
	newQueue := startScriptEventQueue(owner)
	if queueScriptCallbackOn(queue, owner, "OldGeneration", func() {
		t.Error("old queue callback ran in the new generation")
	}) {
		t.Fatal("old queue generation accepted a callback after restart")
	}
	newQueue.stop()
	close(releaseBlocker)
}

func TestDisableCancelsScriptWaitsTimersAndRepeatingWork(t *testing.T) {
	const owner = "callback_test"
	resetScriptCallbackTestState(t, owner)
	eventQueue := currentScriptEventQueue(owner)
	candidate := &scriptCandidate{active: true, eventQueue: eventQueue}
	exports := exportsForScriptCandidate(owner, candidate)["gt/gt"]
	after := exports["After"].Interface().(func(int, func()))
	every := exports["Every"].Interface().(func(int, func()))
	fired := make(chan string, 4)
	after(80, func() { fired <- "after" })
	every(80, func() { fired <- "every" })
	drainScriptDispatcher()

	waitReturned := make(chan struct{})
	if !queueScriptCallback(owner, "Sleep", func() {
		scriptSleepTicks(owner, eventQueue, 100)
		close(waitReturned)
	}) {
		t.Fatal("sleep callback was not queued")
	}
	deadline := time.Now().Add(time.Second)
	for {
		scriptMu.RLock()
		waiting := len(scriptTickWaiters[owner]) > 0
		scriptMu.RUnlock()
		if waiting {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("SleepTicks did not register its wait")
		}
		time.Sleep(time.Millisecond)
	}
	if !queueScriptCallback(owner, "AfterSleep", func() { fired <- "queued" }) {
		t.Fatal("callback after wait was not queued")
	}

	disablescript(owner, "test")
	select {
	case <-waitReturned:
	case <-time.After(time.Second):
		t.Fatal("disable did not release SleepTicks")
	}
	if currentScriptEventQueue(owner) != nil {
		t.Fatal("disabled script retained its event queue")
	}
	select {
	case event := <-fired:
		t.Fatalf("disabled script work still ran: %s", event)
	case <-time.After(120 * time.Millisecond):
	}
}

func setScriptExecutionLimitsForTest(t *testing.T, callback, budget, window time.Duration) {
	t.Helper()
	origCallback := scriptCallbackTimeLimit
	origBudget := scriptExecutionBudget
	origWindow := scriptExecutionWindow
	scriptCallbackTimeLimit = callback
	scriptExecutionBudget = budget
	scriptExecutionWindow = window
	t.Cleanup(func() {
		scriptCallbackTimeLimit = origCallback
		scriptExecutionBudget = origBudget
		scriptExecutionWindow = origWindow
	})
}

func waitForScriptDisabled(t *testing.T, owner string) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for !scriptIsDisabled(owner) {
		if time.Now().After(deadline) {
			t.Fatal("script was not disabled after exceeding its execution limit")
		}
		time.Sleep(time.Millisecond)
	}
}

func TestScriptCallbackAndPerScriptExecutionLimits(t *testing.T) {
	withoutConsoleTimestamps(t)
	const owner = "callback_test"

	t.Run("callback", func(t *testing.T) {
		resetScriptCallbackTestState(t, owner)
		setScriptExecutionLimitsForTest(t, 20*time.Millisecond, time.Second, time.Second)
		release := make(chan struct{})
		if !queueScriptCallback(owner, "Blocked", func() { <-release }) {
			t.Fatal("blocked callback was not queued")
		}
		waitForScriptDisabled(t, owner)
		drainScriptDispatcher()
		close(release)
		if messages := strings.Join(getConsoleMessages(), "\n"); !strings.Contains(messages, "Blocked callback exceeded the callback time limit") {
			t.Fatalf("callback limit was not reported: %s", messages)
		}
	})

	t.Run("script budget", func(t *testing.T) {
		resetScriptCallbackTestState(t, owner)
		setScriptExecutionLimitsForTest(t, 200*time.Millisecond, 45*time.Millisecond, time.Second)
		firstDone := make(chan struct{})
		if !queueScriptCallback(owner, "First", func() {
			time.Sleep(30 * time.Millisecond)
			close(firstDone)
		}) {
			t.Fatal("first callback was not queued")
		}
		<-firstDone
		if !queueScriptCallback(owner, "Second", func() { time.Sleep(100 * time.Millisecond) }) {
			t.Fatal("second callback was not queued")
		}
		waitForScriptDisabled(t, owner)
		drainScriptDispatcher()
		if messages := strings.Join(getConsoleMessages(), "\n"); !strings.Contains(messages, "Second callback exceeded the script execution budget") {
			t.Fatalf("script budget was not reported: %s", messages)
		}
	})
}

func TestSleepTicksPausesScriptExecutionLimit(t *testing.T) {
	const owner = "callback_test"
	resetScriptCallbackTestState(t, owner)
	setScriptExecutionLimitsForTest(t, 15*time.Millisecond, time.Second, time.Second)
	eventQueue := currentScriptEventQueue(owner)
	done := make(chan struct{})
	if !queueScriptCallback(owner, "Wait", func() {
		scriptSleepTicks(owner, eventQueue, 1)
		close(done)
	}) {
		t.Fatal("wait callback was not queued")
	}
	deadline := time.Now().Add(time.Second)
	for {
		scriptMu.RLock()
		waiting := len(scriptTickWaiters[owner]) > 0
		scriptMu.RUnlock()
		if waiting {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("SleepTicks did not start")
		}
		time.Sleep(time.Millisecond)
	}
	time.Sleep(40 * time.Millisecond)
	if scriptIsDisabled(owner) {
		t.Fatal("owned wait consumed callback execution time")
	}
	scriptAdvanceTick()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("SleepTicks did not resume")
	}
}

func TestScriptWaitIsCancelledOnDisable(t *testing.T) {
	const owner = "wait_cancel_test"
	resetScriptCallbackTestState(t, owner)
	eventQueue := currentScriptEventQueue(owner)
	returned := make(chan struct{})
	go func() {
		scriptWait(owner, eventQueue, time.Hour)
		close(returned)
	}()
	time.Sleep(time.Millisecond)
	disablescript(owner, "test")
	select {
	case <-returned:
	case <-time.After(time.Second):
		t.Fatal("Wait was not cancelled when the script stopped")
	}
}

func TestScriptRepeatTimerStopsIndependently(t *testing.T) {
	const owner = "repeat_timer_test"
	resetScriptCallbackTestState(t, owner)
	var mu sync.Mutex
	count := 0
	timer := newScriptTimer()
	timer.attach(startScriptRepeat(owner, currentScriptEventQueue(owner), time.Millisecond, "Repeat", func() {
		mu.Lock()
		count++
		mu.Unlock()
	}))
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		mu.Lock()
		current := count
		mu.Unlock()
		if current >= 2 {
			break
		}
		time.Sleep(time.Millisecond)
	}
	if !timer.Active() {
		t.Fatal("Repeat timer was not active")
	}
	timer.Stop()
	timer.Stop()
	time.Sleep(10 * time.Millisecond)
	mu.Lock()
	stoppedAt := count
	mu.Unlock()
	time.Sleep(10 * time.Millisecond)
	mu.Lock()
	defer mu.Unlock()
	if timer.Active() || count != stoppedAt {
		t.Fatalf("Repeat continued after Stop: before=%d after=%d", stoppedAt, count)
	}
}

func TestScriptWaitForInventoryChange(t *testing.T) {
	const owner = "inventory_wait_test"
	resetScriptCallbackTestState(t, owner)
	resetInventory()
	t.Cleanup(resetInventory)
	eventQueue := currentScriptEventQueue(owner)
	result := make(chan bool, 1)
	go func() {
		result <- waitForScriptInventory(owner, eventQueue, "Signal Item", true, false, time.Second)
	}()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		scriptMu.RLock()
		registered := len(scriptStateWaiters[owner]) == 1
		scriptMu.RUnlock()
		if registered {
			break
		}
		time.Sleep(time.Millisecond)
	}
	addInventoryItem(77, -1, "Signal Item", false)
	pollScriptChangeEvents()
	select {
	case matched := <-result:
		if !matched {
			t.Fatal("inventory wait returned false after matching change")
		}
	case <-time.After(time.Second):
		t.Fatal("inventory wait did not wake after matching change")
	}
	if waitForScriptInventory(owner, eventQueue, "Missing", true, false, 2*time.Millisecond) {
		t.Fatal("inventory wait unexpectedly matched missing item")
	}
}

func TestScriptInitExecutionLimitInterruptsInterpreter(t *testing.T) {
	setScriptExecutionLimitsForTest(t, 20*time.Millisecond, time.Second, time.Second)
	source := []byte(`package main
func Init() {
	for {}
}
`)
	started := time.Now()
	_, err := prepareScriptSource("init_limit", source, restrictedStdlib())
	if err == nil || !strings.Contains(err.Error(), "Init exceeded the callback time limit") {
		t.Fatalf("Init limit error = %v", err)
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("Init cancellation took too long: %v", elapsed)
	}
}

func TestInterpretedCallbackPanicReportsScriptEventAndSource(t *testing.T) {
	withoutConsoleTimestamps(t)
	const owner = "Panic_Source_panic_source"
	resetScriptCallbackTestState(t, owner)
	stopScriptEventQueue(owner)
	scriptMu.Lock()
	scriptDisplayNames[owner] = "Panic Source"
	scriptDisabled[owner] = true
	scriptMu.Unlock()
	consoleLog = messageLog{max: maxMessages}
	source := []byte(`package main
import "gt"
func Init() {
	gt.RegisterCommand("explode", func(string) {
		panic("kaboom")
	})
}
`)
	if !loadscriptSource(owner, "Panic Source", "panic_source.go", source, restrictedStdlib()) {
		t.Fatal("panic source script did not load")
	}
	handler := scriptCommands["explode"]
	if handler == nil {
		t.Fatal("panic source command was not registered")
	}
	handler("")
	waitForScriptDisabled(t, owner)
	drainScriptDispatcher()

	messages := strings.Join(getConsoleMessages(), "\n")
	if !strings.Contains(messages, "[script:Panic Source] Command callback panic") || !strings.Contains(messages, "kaboom") {
		t.Fatalf("panic report is missing script/event/value: %s", messages)
	}
	if !regexp.MustCompile(`Command callback panic at (?:[^ ]+:)?[0-9]+:[0-9]+:`).MatchString(messages) {
		t.Fatalf("panic report is missing source location: %s", messages)
	}
}

func TestScriptLifecycleStagesCommitAndDisposeExplicitly(t *testing.T) {
	const owner = "lifecycle_test"
	resetScriptCallbackTestState(t, owner)
	stopScriptEventQueue(owner)
	scriptMu.Lock()
	scriptDisabled[owner] = true
	scriptMu.Unlock()
	source := []byte(`package main
import "gt"
func Init() {
	gt.RegisterCommand("lifecycle", func(string) {})
	gt.Save("initialized", "yes")
}
func Terminate() {
	gt.Save("terminated", "yes")
}
`)

	prepared, err := compileScriptSource(owner, source, restrictedStdlib())
	if err != nil {
		t.Fatalf("compile candidate: %v", err)
	}
	if scriptStorageGet(owner, "initialized") != nil || scriptCommands["lifecycle"] != nil {
		t.Fatal("compile stage produced runtime effects")
	}
	if err := initializePreparedScript(prepared); err != nil {
		t.Fatalf("initialize candidate: %v", err)
	}
	if got := prepared.candidate.getStorage(owner, "initialized"); got != "yes" {
		t.Fatalf("initialize stage did not stage storage: %v", got)
	}
	if scriptStorageGet(owner, "initialized") != nil || scriptCommands["lifecycle"] != nil {
		t.Fatal("initialize stage committed effects before activation")
	}

	activatePreparedScript(owner, prepared)
	if scriptStorageGet(owner, "initialized") != "yes" || scriptCommands["lifecycle"] == nil {
		t.Fatal("activation did not commit staged effects")
	}
	deactivated := deactivateScript(owner, "test")
	if !scriptIsDisabled(owner) || currentScriptEventQueue(owner) != nil {
		t.Fatal("deactivation did not revoke execution")
	}
	if scriptCommands["lifecycle"] == nil {
		t.Fatal("deactivation disposed registrations prematurely")
	}
	terminateDeactivatedScript(owner, deactivated)
	if scriptStorageGet(owner, "terminated") != "yes" {
		t.Fatal("Terminate did not run between deactivation and disposal")
	}
	disposeScriptResources(owner, "test", deactivated.eventQueue)
	if scriptCommands["lifecycle"] != nil {
		t.Fatal("disposal did not remove registrations")
	}
}

func TestTerminatePanicStillCleansOwner(t *testing.T) {
	withoutConsoleTimestamps(t)
	const owner = "callback_test"
	resetScriptCallbackTestState(t, owner)
	scriptRegisterCommand(owner, "owned", func(string) {})
	scriptTerminators[owner] = func() { panic("terminate boom") }

	disablescript(owner, "test")

	if !scriptIsDisabled(owner) {
		t.Fatal("script was not disabled")
	}
	if _, ok := scriptCommands["owned"]; ok {
		t.Fatal("Terminate panic prevented ownership cleanup")
	}
	if messages := strings.Join(getConsoleMessages(), "\n"); !strings.Contains(messages, "Terminate panic: terminate boom") {
		t.Fatalf("Terminate panic was not reported: %s", messages)
	}
}
