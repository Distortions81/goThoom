package main

import (
	"context"
	"encoding/binary"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	scriptapi "gt2"
)

func TestSessionIDsMapToFourStableSlots(t *testing.T) {
	for slot := range maxSessions {
		id, ok := sessionIDForSlot(slot)
		if !ok {
			t.Fatalf("slot %d did not produce a session ID", slot)
		}
		gotSlot, ok := id.Slot()
		if !ok || gotSlot != slot {
			t.Fatalf("session ID %d maps to slot %d, %v; want %d, true", id, gotSlot, ok, slot)
		}
	}
	for _, slot := range []int{-1, maxSessions} {
		if id, ok := sessionIDForSlot(slot); ok || id.Valid() {
			t.Fatalf("invalid slot %d produced valid session ID %d", slot, id)
		}
	}
	if _, err := newSession(0); err == nil {
		t.Fatal("session ID zero was accepted")
	}
}

func TestSessionsOwnIndependentLoginRequestsAndStagedPasswords(t *testing.T) {
	first := mustNewSession(1)
	second := mustNewSession(2)
	first.login.setRequest(sessionLoginRequest{
		host:         "first.example:5010",
		character:    "First Hero",
		passwordHash: "first-hash",
	})
	second.login.setRequest(sessionLoginRequest{
		host:      "second.example:5010",
		character: "Second Hero",
		password:  "second-password",
	})

	firstSnapshot := first.login.requestSnapshot()
	secondSnapshot := second.login.requestSnapshot()
	first.login.setDemoCandidate("First Demo")
	stageSessionPasswordUpdate(first, "First Hero", "first-new", true)
	stageSessionPasswordUpdate(second, "Second Hero", "second-new", false)

	if firstSnapshot.host != "first.example:5010" || firstSnapshot.character != "First Hero" || firstSnapshot.passwordHash != "first-hash" {
		t.Fatalf("first login snapshot changed after later edits: %+v", firstSnapshot)
	}
	if secondSnapshot.host != "second.example:5010" || secondSnapshot.character != "Second Hero" || secondSnapshot.password != "second-password" {
		t.Fatalf("second login request = %+v", secondSnapshot)
	}
	if got := second.login.requestSnapshot(); !reflect.DeepEqual(got, secondSnapshot) {
		t.Fatalf("changing first login request changed second: got %+v want %+v", got, secondSnapshot)
	}
	if _, ok := first.login.takeStagedPassword("Second Hero"); ok {
		t.Fatal("first session consumed second session's staged password")
	}
	if update, ok := first.login.takeStagedPassword("First Hero"); !ok || !update.remember {
		t.Fatalf("first staged password = %+v, %v", update, ok)
	}
	if update, ok := second.login.takeStagedPassword("Second Hero"); !ok || update.remember {
		t.Fatalf("second staged password = %+v, %v", update, ok)
	}
}

func TestConcurrentSessionLoginsUseOwningRequests(t *testing.T) {
	firstServer := newFakeServerWithResults(t, 0)
	secondServer := newFakeServerWithResults(t, 0)
	defer firstServer.close()
	defer secondServer.close()

	originalDataDir := dataDirPath
	dataDirPath = t.TempDir()
	t.Cleanup(func() { dataDirPath = originalDataDir })

	first := mustNewSession(1)
	second := mustNewSession(2)
	first.login.setRequest(sessionLoginRequest{
		host:      firstServer.addr(),
		character: "First Hero",
		password:  "first-password",
	})
	second.login.setRequest(sessionLoginRequest{
		host:      secondServer.addr(),
		character: "Second Hero",
		password:  "second-password",
	})

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	errs := make(chan error, 2)
	go func() { errs <- loginSessionWithDemoCandidates(first, ctx, 1, nil) }()
	go func() { errs <- loginSessionWithDemoCandidates(second, ctx, 1, nil) }()
	for range 2 {
		if err := <-errs; err != nil {
			t.Fatalf("concurrent login: %v", err)
		}
	}

	if got, want := firstServer.attemptedLoginNames(), []string{"First Hero"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("first server login names = %v, want %v", got, want)
	}
	if got, want := secondServer.attemptedLoginNames(), []string{"Second Hero"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("second server login names = %v, want %v", got, want)
	}
	if got := first.login.requestSnapshot().character; got != "First Hero" {
		t.Fatalf("first session login character = %q", got)
	}
	if got := second.login.requestSnapshot().character; got != "Second Hero" {
		t.Fatalf("second session login character = %q", got)
	}
}

func TestSessionsOwnIndependentInventory(t *testing.T) {
	first, err := newSession(1)
	if err != nil {
		t.Fatal(err)
	}
	second, err := newSession(2)
	if err != nil {
		t.Fatal(err)
	}

	first.inventory.add(100, -1, "Moonstone", false)
	second.inventory.add(200, -1, "Sunstone", true)
	first.inventory.add(100, -1, "Moonstone", false)

	firstItems := first.inventory.snapshot()
	secondItems := second.inventory.snapshot()
	if len(firstItems) != 1 || firstItems[0].ID != 100 || firstItems[0].Quantity != 2 {
		t.Fatalf("first session inventory = %+v", firstItems)
	}
	if len(secondItems) != 1 || secondItems[0].ID != 200 || !secondItems[0].Equipped || secondItems[0].Quantity != 1 {
		t.Fatalf("second session inventory = %+v", secondItems)
	}
	first.inventory.reset()
	if len(first.inventory.snapshot()) != 0 || len(second.inventory.snapshot()) != 1 {
		t.Fatal("resetting one session changed another session's inventory")
	}
}

func TestSessionsOwnIndependentLegacyMacroRuntimes(t *testing.T) {
	first := mustNewSession(1)
	second := mustNewSession(2)
	first.setCharacterName("First Hero")
	second.setCharacterName("Second Hero")
	first.publishChat("first session text", messageTextTypeSystem)
	second.publishChat("second session text", messageTextTypeSystem)

	program := parseLegacyMacroSources([]legacyMacroSource{{
		Path: "session-test.mac",
		Text: strings.Join([]string{
			"setglobal owner @my.name",
			"setglobal textlog @env.textlog",
			"\"first\" \"/pose first\\r\"",
			"\"second\" \"/pose second\\r\"",
		}, "\n"),
	}})
	if err := program.err(); err != nil {
		t.Fatalf("parse macro program: %v", err)
	}
	firstRuntime := newSessionLegacyMacroRuntime(first, program)
	secondRuntime := newSessionLegacyMacroRuntime(second, program)
	first.automation.legacyRuntime = firstRuntime
	second.automation.legacyRuntime = secondRuntime

	if got := firstRuntime.globalsSnapshot()["owner"]; got != "First Hero" {
		t.Fatalf("first macro owner = %q, want First Hero", got)
	}
	if got := secondRuntime.globalsSnapshot()["owner"]; got != "Second Hero" {
		t.Fatalf("second macro owner = %q, want Second Hero", got)
	}
	if got := firstRuntime.globalsSnapshot()["textlog"]; got != "first session text" {
		t.Fatalf("first macro text log = %q", got)
	}
	if got := secondRuntime.globalsSnapshot()["textlog"]; got != "second session text" {
		t.Fatalf("second macro text log = %q", got)
	}
	if !firstRuntime.triggerExpression("first", 1) {
		t.Fatal("first runtime did not trigger its expression")
	}
	if !secondRuntime.triggerExpression("second", 1) {
		t.Fatal("second runtime did not trigger its expression")
	}
	first.commands.mu.Lock()
	firstCommand := first.commands.pending
	first.commands.mu.Unlock()
	second.commands.mu.Lock()
	secondCommand := second.commands.pending
	second.commands.mu.Unlock()
	if firstCommand != "/pose first" || secondCommand != "/pose second" {
		t.Fatalf("macro commands crossed sessions: first=%q second=%q", firstCommand, secondCommand)
	}

	first.queueLegacyMacroMove(legacyMacroMove{Direction: legacyMacroMoveEast})
	firstInput := first.input.next()
	secondInput := second.input.next()
	if !firstInput.mouseDown || firstInput.mouseX <= 0 || firstInput.mouseY != 0 {
		t.Fatalf("first macro movement = %+v", firstInput)
	}
	if secondInput != (inputState{}) {
		t.Fatalf("first macro movement changed second session input: %+v", secondInput)
	}

	first.resetConnectionModels()
	if first.legacyMacroRuntimeSnapshot() != nil {
		t.Fatal("reset retained the first session macro runtime")
	}
	if second.legacyMacroRuntimeSnapshot() != secondRuntime {
		t.Fatal("resetting first session removed the second macro runtime")
	}
}

func TestSessionsOwnIndependentScriptEventQueues(t *testing.T) {
	first := mustNewSession(1)
	second := mustNewSession(2)
	const owner = "session-queue-isolation"

	firstQueue := startSessionScriptEventQueue(first, owner)
	secondQueue := startSessionScriptEventQueue(second, owner)
	if firstQueue == nil || secondQueue == nil || firstQueue == secondQueue {
		t.Fatal("sessions did not create independent script event queues")
	}
	if currentSessionScriptEventQueue(first, owner) != firstQueue {
		t.Fatal("first session did not retain its queue")
	}
	if currentSessionScriptEventQueue(second, owner) != secondQueue {
		t.Fatal("second session did not retain its queue")
	}

	firstRan := make(chan struct{}, 1)
	secondRan := make(chan struct{}, 1)
	if !queueScriptCallbackOn(firstQueue, owner, "first", func() { firstRan <- struct{}{} }) {
		t.Fatal("first session did not accept its callback")
	}
	if !queueScriptCallbackOn(secondQueue, owner, "second", func() { secondRan <- struct{}{} }) {
		t.Fatal("second session did not accept its callback")
	}
	for name, ran := range map[string]<-chan struct{}{"first": firstRan, "second": secondRan} {
		select {
		case <-ran:
		case <-time.After(time.Second):
			t.Fatalf("%s session callback did not run", name)
		}
	}

	if stopped := stopSessionScriptEventQueue(first, owner); stopped != firstQueue {
		t.Fatalf("first session stopped queue = %p, want %p", stopped, firstQueue)
	}
	if currentSessionScriptEventQueue(first, owner) != nil {
		t.Fatal("stopping first queue retained it")
	}
	if currentSessionScriptEventQueue(second, owner) != secondQueue || !scriptEventQueueIsCurrent(owner, secondQueue) {
		t.Fatal("stopping first queue changed the second session queue")
	}

	second.resetConnectionModels()
	if currentSessionScriptEventQueue(second, owner) != nil {
		t.Fatal("resetting a session retained its script queues")
	}
}

func TestSecondarySessionLoadsAndStopsLegacyMacros(t *testing.T) {
	originalDataDir := dataDirPath
	dataDirPath = t.TempDir()
	t.Cleanup(func() { dataDirPath = originalDataDir })
	if err := os.MkdirAll(legacyMacrosDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	macro := strings.Join([]string{
		"@login",
		"{",
		"\t\"/pose entered\\r\"",
		"}",
	}, "\n")
	if err := os.WriteFile(filepath.Join(legacyMacrosDir(), "Second Hero"), []byte(macro), 0o644); err != nil {
		t.Fatal(err)
	}

	session := mustNewSession(2)
	session.setCharacterName("Second Hero")
	if err := session.loadLegacyMacrosForCharacter("Second Hero"); err != nil {
		t.Fatalf("load secondary macros: %v", err)
	}
	if session.legacyMacroRuntimeSnapshot() == nil {
		t.Fatal("secondary macro runtime was not installed")
	}
	session.advanceLegacyMacros(1)
	session.commands.mu.Lock()
	command := session.commands.pending
	session.commands.mu.Unlock()
	if command != "/pose entered" {
		t.Fatalf("secondary @login command = %q", command)
	}

	session.resetConnectionModels()
	if session.legacyMacroRuntimeSnapshot() != nil {
		t.Fatal("secondary macro runtime survived session reset")
	}
}

func TestSessionScriptDataExportsUseOwningSession(t *testing.T) {
	scriptPermissionMu.Lock()
	originalPermissions := scriptPermissionGrants["shared-script"]
	scriptPermissionGrants["shared-script"] = map[string]bool{"data": true}
	scriptPermissionMu.Unlock()
	t.Cleanup(func() {
		scriptPermissionMu.Lock()
		if originalPermissions == nil {
			delete(scriptPermissionGrants, "shared-script")
		} else {
			scriptPermissionGrants["shared-script"] = originalPermissions
		}
		scriptPermissionMu.Unlock()
	})

	first := mustNewSession(1)
	second := mustNewSession(2)
	first.setCharacterName("First Hero")
	second.setCharacterName("Second Hero")
	first.setScriptLocation("First Shore")
	second.setScriptLocation("Second Shore")
	first.inventory.add(100, -1, "Moonstone", true)
	second.inventory.add(200, -1, "Sunstone", false)
	first.players.observeAppearance("First Friend", 10, nil, false)
	second.players.observeAppearance("Second Friend", 20, nil, false)
	first.setPlayerIndex(3)
	second.setPlayerIndex(7)
	first.draw.mu.Lock()
	first.draw.current.hp, first.draw.current.hpMax = 3, 10
	first.draw.current.liveMobs = []frameMobile{{Index: 3, H: 10}}
	first.draw.current.descriptors = map[uint8]frameDescriptor{3: {Index: 3, Name: "First Hero", Type: kDescPlayer}}
	first.draw.mu.Unlock()
	second.draw.mu.Lock()
	second.draw.current.hp, second.draw.current.hpMax = 8, 12
	second.draw.current.liveMobs = []frameMobile{{Index: 7, H: 20}}
	second.draw.current.descriptors = map[uint8]frameDescriptor{7: {Index: 7, Name: "Second Hero", Type: kDescPlayer}}
	second.draw.mu.Unlock()

	firstExports := exportsForScriptCandidate("shared-script", &scriptCandidate{session: first})["gt2/gt2"]
	secondExports := exportsForScriptCandidate("shared-script", &scriptCandidate{session: second})["gt2/gt2"]
	firstSelf := firstExports["Self"].Interface().(func() scriptapi.Character)()
	secondSelf := secondExports["Self"].Interface().(func() scriptapi.Character)()
	if firstSelf.Name != "First Hero" || firstSelf.Health != 3 || firstSelf.Location != "First Shore" {
		t.Fatalf("first script self = %+v", firstSelf)
	}
	if secondSelf.Name != "Second Hero" || secondSelf.Health != 8 || secondSelf.Location != "Second Shore" {
		t.Fatalf("second script self = %+v", secondSelf)
	}
	firstInventory := firstExports["Inventory"].Interface().(func() []InventoryItem)()
	secondInventory := secondExports["Inventory"].Interface().(func() []InventoryItem)()
	if len(firstInventory) != 1 || firstInventory[0].Name != "Moonstone" || len(secondInventory) != 1 || secondInventory[0].Name != "Sunstone" {
		t.Fatalf("script inventories crossed sessions: first=%+v second=%+v", firstInventory, secondInventory)
	}
	firstPlayers := firstExports["Players"].Interface().(func() []scriptapi.Player)()
	secondPlayers := secondExports["Players"].Interface().(func() []scriptapi.Player)()
	if len(firstPlayers) != 1 || firstPlayers[0].Name != "First Friend" || len(secondPlayers) != 1 || secondPlayers[0].Name != "Second Friend" {
		t.Fatalf("script players crossed sessions: first=%+v second=%+v", firstPlayers, secondPlayers)
	}
	secondWorld := secondExports["CurrentWorld"].Interface().(func() scriptapi.World)()
	if !secondWorld.HasSelf || secondWorld.Self.Name != "Second Hero" || secondWorld.Self.H != 20 {
		t.Fatalf("second script world = %+v", secondWorld)
	}
}

func TestSessionScriptsOwnInterpreterAndChatLifecycle(t *testing.T) {
	const firstOwner = "first-session-runtime"
	const secondOwner = "second-session-runtime"
	scriptPermissionMu.Lock()
	originalFirst := scriptPermissionGrants[firstOwner]
	originalSecond := scriptPermissionGrants[secondOwner]
	originalFirstReview := scriptPermissionReviews[firstOwner]
	originalSecondReview := scriptPermissionReviews[secondOwner]
	scriptPermissionGrants[firstOwner] = map[string]bool{"messages": true, "session": true, "storage": true}
	scriptPermissionGrants[secondOwner] = map[string]bool{"messages": true, "session": true, "storage": true}
	scriptPermissionReviews[firstOwner] = map[string]bool{"messages": true, "session": true, "storage": true}
	scriptPermissionReviews[secondOwner] = map[string]bool{"messages": true, "session": true, "storage": true}
	scriptPermissionMu.Unlock()
	t.Cleanup(func() {
		scriptPermissionMu.Lock()
		if originalFirst == nil {
			delete(scriptPermissionGrants, firstOwner)
		} else {
			scriptPermissionGrants[firstOwner] = originalFirst
		}
		if originalSecond == nil {
			delete(scriptPermissionGrants, secondOwner)
		} else {
			scriptPermissionGrants[secondOwner] = originalSecond
		}
		if originalFirstReview == nil {
			delete(scriptPermissionReviews, firstOwner)
		} else {
			scriptPermissionReviews[firstOwner] = originalFirstReview
		}
		if originalSecondReview == nil {
			delete(scriptPermissionReviews, secondOwner)
		} else {
			scriptPermissionReviews[secondOwner] = originalSecondReview
		}
		scriptPermissionMu.Unlock()
	})

	first := mustNewSession(1)
	second := mustNewSession(2)
	first.setCharacterName("First Hero")
	second.setCharacterName("Second Hero")
	src := []byte(`package main
import "gt2"
func Init() {
	gt2.OnLogin(func(event gt2.LifecycleEvent) { gt2.Store("login", event.Character) })
	gt2.OnLogout(func(event gt2.LifecycleEvent) { gt2.Store("logout", event.Character) })
	gt2.OnStop(func(event gt2.LifecycleEvent) { gt2.Store("stopped", event.Character) })
	gt2.OnChat(gt2.ChatFilter{}, func(event gt2.ChatEvent) { gt2.Store("chat", event.Raw) })
}
`)
	if err := first.startSessionScript(firstOwner, src, restrictedStdlib(), nil); err != nil {
		t.Fatalf("start first session script: %v", err)
	}
	if err := second.startSessionScript(secondOwner, src, restrictedStdlib(), nil); err != nil {
		t.Fatalf("start second session script: %v", err)
	}
	first.automation.scriptMu.RLock()
	firstRuntime := first.automation.scripts[firstOwner]
	first.automation.scriptMu.RUnlock()
	second.automation.scriptMu.RLock()
	secondRuntime := second.automation.scripts[secondOwner]
	second.automation.scriptMu.RUnlock()
	if firstRuntime == nil || secondRuntime == nil || firstRuntime.queue == secondRuntime.queue || firstRuntime.prepared.interpreter == secondRuntime.prepared.interpreter {
		t.Fatal("sessions did not receive independent script interpreters and queues")
	}
	if got := scriptStorageGet(firstOwner, "login"); got != "First Hero" {
		t.Fatalf("first login lifecycle = %#v", got)
	}
	if got := scriptStorageGet(secondOwner, "login"); got != "Second Hero" {
		t.Fatalf("second login lifecycle = %#v", got)
	}

	first.publishChat("first session chat", messageTextTypeSystem)
	deadline := time.Now().Add(time.Second)
	for scriptStorageGet(firstOwner, "chat") != "first session chat" && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if got := scriptStorageGet(firstOwner, "chat"); got != "first session chat" {
		t.Fatalf("first chat callback = %#v", got)
	}
	if got := scriptStorageGet(secondOwner, "chat"); got != nil {
		t.Fatalf("first chat crossed into second session script: %#v", got)
	}

	first.stopSessionScript(firstOwner, "test stop")
	if got := scriptStorageGet(firstOwner, "stopped"); got != "First Hero" {
		t.Fatalf("first stop lifecycle = %#v", got)
	}
	if currentSessionScriptEventQueue(first, firstOwner) != nil {
		t.Fatal("stopping first script retained its queue")
	}
	if currentSessionScriptEventQueue(second, secondOwner) != secondRuntime.queue {
		t.Fatal("stopping first script changed second session queue")
	}
	second.resetConnectionModels()
	if got := scriptStorageGet(secondOwner, "logout"); got != "Second Hero" {
		t.Fatalf("second logout lifecycle = %#v", got)
	}
	if currentSessionScriptEventQueue(second, secondOwner) != nil {
		t.Fatal("resetting second session retained its script queue")
	}
}

func TestSessionsOwnIndependentScriptTimersAndTickWaiters(t *testing.T) {
	const owner = "shared-session-timers"
	grantScriptPermissionsForTest(t, owner)

	first := mustNewSession(1)
	second := mustNewSession(2)
	source := []byte(`package main
import("gt2";"time")
var repeat gt2.Timer
var after gt2.Timer
var waiter gt2.Task
func Init(){
	repeat=gt2.Repeat(time.Hour,func(){})
	after=gt2.After(time.Hour,func(){})
	waiter=gt2.StartTask(func(){gt2.WaitTicks(2)})
}
`)
	if err := first.startSessionScript(owner, source, restrictedStdlib(), nil); err != nil {
		t.Fatalf("start first session script: %v", err)
	}
	if err := second.startSessionScript(owner, source, restrictedStdlib(), nil); err != nil {
		t.Fatalf("start second session script: %v", err)
	}
	t.Cleanup(func() {
		first.stopSessionScript(owner, "test cleanup")
		second.stopSessionScript(owner, "test cleanup")
	})

	waitForWaiter := func(session *Session) {
		t.Helper()
		deadline := time.Now().Add(time.Second)
		for session.automation.scriptTimers.tickWaiterCount(owner) != 1 {
			if time.Now().After(deadline) {
				t.Fatalf("session %d did not register its tick waiter", session.ID())
			}
			time.Sleep(time.Millisecond)
		}
	}
	waitForWaiter(first)
	waitForWaiter(second)
	if len(first.automation.scriptTimers.repeatsSnapshot(owner)) != 1 || len(second.automation.scriptTimers.repeatsSnapshot(owner)) != 1 {
		t.Fatal("session repeat timers were not registered with their owners")
	}

	var firstRepeat, firstAfter, secondRepeat, secondAfter Timer
	var firstWaiter, secondWaiter Task
	readTimers := func(session *Session, repeat, after *Timer, waiter *Task) {
		t.Helper()
		queue := currentSessionScriptEventQueue(session, owner)
		if !queueScriptCallbackWaitOn(queue, owner, "inspect timers", func() {
			value, _ := queue.interpreter.Eval("repeat")
			*repeat = value.Interface().(Timer)
			value, _ = queue.interpreter.Eval("after")
			*after = value.Interface().(Timer)
			value, _ = queue.interpreter.Eval("waiter")
			*waiter = value.Interface().(Task)
		}) {
			t.Fatalf("session %d timer inspection failed", session.ID())
		}
	}
	readTimers(first, &firstRepeat, &firstAfter, &firstWaiter)
	readTimers(second, &secondRepeat, &secondAfter, &secondWaiter)
	if !firstRepeat.Active() || !firstAfter.Active() || !firstWaiter.Active() || !secondRepeat.Active() || !secondAfter.Active() || !secondWaiter.Active() {
		t.Fatal("session timers and waiters were not active before advancement")
	}

	first.advanceScriptTick()
	first.advanceScriptTick()
	deadline := time.Now().Add(time.Second)
	for firstWaiter.Active() && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if firstWaiter.Active() {
		t.Fatal("first session tick waiter did not resume")
	}
	if !secondWaiter.Active() {
		t.Fatal("first session ticks resumed second session waiter")
	}

	first.stopSessionScript(owner, "test stop")
	if firstRepeat.Active() || firstAfter.Active() {
		t.Fatal("stopping first session retained its timers")
	}
	if !secondRepeat.Active() || !secondAfter.Active() || second.automation.scriptTimers.tickWaiterCount(owner) != 1 {
		t.Fatal("stopping first session changed second session timer state")
	}
	second.advanceScriptTick()
	second.advanceScriptTick()
	deadline = time.Now().Add(time.Second)
	for secondWaiter.Active() && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if secondWaiter.Active() {
		t.Fatal("second session tick waiter did not resume on its own ticks")
	}
}

func TestSessionTaskCancellationUsesOwningCommandStream(t *testing.T) {
	const owner = "shared-session-task"
	grantScriptPermissionsForTest(t, owner)
	first := mustNewSession(1)
	second := mustNewSession(2)
	source := []byte(`package main
import "gt2"
var job gt2.Task
var ticket gt2.CommandTicket
func Init(){job=gt2.StartTask(func(){ticket=gt2.QueueCommand("/pose sit");gt2.WaitTicks(100)})}
`)
	if err := first.startSessionScript(owner, source, restrictedStdlib(), nil); err != nil {
		t.Fatalf("start first session task: %v", err)
	}
	if err := second.startSessionScript(owner, source, restrictedStdlib(), nil); err != nil {
		t.Fatalf("start second session task: %v", err)
	}
	t.Cleanup(func() {
		first.stopSessionScript(owner, "test cleanup")
		second.stopSessionScript(owner, "test cleanup")
	})
	deadline := time.Now().Add(time.Second)
	for (first.automation.scriptTimers.tickWaiterCount(owner) != 1 || second.automation.scriptTimers.tickWaiterCount(owner) != 1) && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if first.automation.scriptTimers.tickWaiterCount(owner) != 1 || second.automation.scriptTimers.tickWaiterCount(owner) != 1 {
		t.Fatal("session tasks did not reach their waits")
	}

	var firstJob, secondJob Task
	var firstTicket, secondTicket CommandTicket
	readTask := func(session *Session, job *Task, ticket *CommandTicket) {
		t.Helper()
		queue := currentSessionScriptEventQueue(session, owner)
		if !queueScriptCallbackWaitOn(queue, owner, "inspect task", func() {
			value, _ := queue.interpreter.Eval("job")
			*job = value.Interface().(Task)
			value, _ = queue.interpreter.Eval("ticket")
			*ticket = value.Interface().(CommandTicket)
		}) {
			t.Fatalf("session %d task inspection failed", session.ID())
		}
	}
	readTask(first, &firstJob, &firstTicket)
	readTask(second, &secondJob, &secondTicket)
	if !firstJob.Active() || !secondJob.Active() || firstTicket.Status().State != scriptapi.CommandQueued || secondTicket.Status().State != scriptapi.CommandQueued {
		t.Fatal("session tasks or their commands were not active")
	}

	first.stopSessionScript(owner, "test stop")
	if firstJob.Active() || firstTicket.Status().State != scriptapi.CommandCancelled {
		t.Fatalf("first task cleanup = active %v ticket %+v", firstJob.Active(), firstTicket.Status())
	}
	if !secondJob.Active() || secondTicket.Status().State != scriptapi.CommandQueued {
		t.Fatalf("first task cleanup changed second task = active %v ticket %+v", secondJob.Active(), secondTicket.Status())
	}
	second.commands.mu.Lock()
	secondPending := second.commands.pending
	second.commands.mu.Unlock()
	if secondPending != "/pose sit" {
		t.Fatalf("first task cleanup removed second session command %q", secondPending)
	}
}

func TestSessionsOwnIndependentCommandQueues(t *testing.T) {
	first, err := newSession(1)
	if err != nil {
		t.Fatal(err)
	}
	second, err := newSession(2)
	if err != nil {
		t.Fatal(err)
	}

	first.commands.enqueue("/pose sit")
	second.commands.enqueue("/pose stand")
	first.commands.mu.Lock()
	first.commands.pendingID = 7
	first.commands.pendingSent = true
	first.commands.mu.Unlock()
	second.commands.mu.Lock()
	second.commands.pendingID = 7
	second.commands.pendingSent = true
	second.commands.mu.Unlock()

	if _, ok := first.commands.acknowledgeAt(7); !ok {
		t.Fatal("first session did not acknowledge its command")
	}
	if !first.commands.idle() {
		t.Fatal("first session retained an acknowledged command")
	}
	second.commands.mu.Lock()
	secondPending, secondID, secondSent := second.commands.pending, second.commands.pendingID, second.commands.pendingSent
	second.commands.mu.Unlock()
	if secondPending != "/pose stand" || secondID != 7 || !secondSent {
		t.Fatalf("acknowledging first session changed second queue: %q id=%d sent=%v", secondPending, secondID, secondSent)
	}
}

func TestCommandTicketCancelsOnlyItsOwningSession(t *testing.T) {
	first, err := newSession(1)
	if err != nil {
		t.Fatal(err)
	}
	second, err := newSession(2)
	if err != nil {
		t.Fatal(err)
	}
	queue := &scriptEventQueue{}
	firstTicket := newSessionScriptCommandTicket(first.commands, "first", queue)
	secondTicket := newSessionScriptCommandTicket(second.commands, "second", queue)

	first.commands.mu.Lock()
	first.commands.pending = "/first"
	first.commands.pendingTicket = firstTicket.state
	first.commands.mu.Unlock()
	second.commands.mu.Lock()
	second.commands.pending = "/second"
	second.commands.pendingTicket = secondTicket.state
	second.commands.mu.Unlock()

	if !firstTicket.Cancel() {
		t.Fatal("first session ticket was not cancelled")
	}
	if status := firstTicket.Status(); status.State != scriptapi.CommandCancelled {
		t.Fatalf("first ticket status = %+v", status)
	}
	if status := secondTicket.Status(); status.State != scriptapi.CommandQueued {
		t.Fatalf("second ticket status changed to %+v", status)
	}
	second.commands.mu.Lock()
	secondPending := second.commands.pending
	second.commands.mu.Unlock()
	if secondPending != "/second" {
		t.Fatalf("cancelling first session removed second command %q", secondPending)
	}
}

func TestSessionsOwnIndependentFrameState(t *testing.T) {
	first := mustNewSession(1)
	second := mustNewSession(2)

	first.frames.set(12, 9)
	second.frames.set(40, -1)
	first.frames.updateCounters(10)
	first.frames.updateCounters(12)
	second.frames.updateCounters(40)

	if ack, resend := first.frames.snapshot(); ack != 12 || resend != 9 {
		t.Fatalf("first frame state = %d/%d, want 12/9", ack, resend)
	}
	if ack, resend := second.frames.snapshot(); ack != 40 || resend != -1 {
		t.Fatalf("second frame state = %d/%d, want 40/-1", ack, resend)
	}
	_, _, firstReceived, firstLost := first.frames.packetLoss()
	_, _, secondReceived, secondLost := second.frames.packetLoss()
	if firstReceived != 2 || firstLost != 1 {
		t.Fatalf("first frame counters = received %d lost %d, want 2/1", firstReceived, firstLost)
	}
	if secondReceived != 1 || secondLost != 0 {
		t.Fatalf("second frame counters = received %d lost %d, want 1/0", secondReceived, secondLost)
	}

	first.frames.resetStatistics()
	_, _, firstReceived, firstLost = first.frames.packetLoss()
	_, _, secondReceived, secondLost = second.frames.packetLoss()
	if firstReceived != 0 || firstLost != 0 || secondReceived != 1 || secondLost != 0 {
		t.Fatal("resetting one session changed another session's frame counters")
	}
}

func TestSessionsOwnIndependentNetworkTiming(t *testing.T) {
	first := mustNewSession(1)
	second := mustNewSession(2)
	start := time.Date(2026, time.September, 12, 12, 0, 0, 0, time.UTC)

	first.timing.recordServerFrame(10, start)
	first.timing.recordServerFrame(11, start.Add(180*time.Millisecond))
	second.timing.recordServerFrame(40, start)
	second.timing.recordServerFrame(41, start.Add(260*time.Millisecond))
	first.timing.recordReply(35 * time.Millisecond)
	second.timing.recordReply(90 * time.Millisecond)
	first.timing.pnaLead(180*time.Millisecond, 5*time.Millisecond)
	if use, reason := first.timing.pnaStatus(1, start); use || reason != "recent packet loss" {
		t.Fatalf("first PNA fallback = use %v reason %q", use, reason)
	}

	firstLast, firstInterval, _, firstRate, firstSamples := first.timing.cadenceSnapshot()
	secondLast, secondInterval, _, secondRate, secondSamples := second.timing.cadenceSnapshot()
	firstReply, _ := first.timing.snapshot()
	secondReply, _ := second.timing.snapshot()
	if firstLast != start.Add(180*time.Millisecond) || firstInterval != 180*time.Millisecond || firstRate == secondRate || firstSamples != 1 {
		t.Fatalf("first cadence = last %v interval %v rate %v samples %d", firstLast, firstInterval, firstRate, firstSamples)
	}
	if secondLast != start.Add(260*time.Millisecond) || secondInterval != 260*time.Millisecond || secondSamples != 1 {
		t.Fatalf("second cadence = last %v interval %v rate %v samples %d", secondLast, secondInterval, secondRate, secondSamples)
	}
	if firstReply != 35*time.Millisecond || secondReply != 90*time.Millisecond {
		t.Fatalf("reply timings crossed sessions: first %v second %v", firstReply, secondReply)
	}
	second.timing.controllerMu.Lock()
	secondController := second.timing.controller
	second.timing.controllerMu.Unlock()
	if secondController != (pnaControllerState{}) {
		t.Fatalf("first PNA learning changed second controller: %+v", secondController)
	}
	if use, reason := second.timing.pnaStatus(0, start); !use || reason != "" {
		t.Fatalf("first PNA fallback changed second status: use %v reason %q", use, reason)
	}

	first.timing.resetCadence()
	first.timing.resetReply()
	first.timing.resetController()
	first.timing.resetFallback()
	_, firstInterval, _, _, firstSamples = first.timing.cadenceSnapshot()
	secondLast, secondInterval, _, _, secondSamples = second.timing.cadenceSnapshot()
	secondReply, _ = second.timing.snapshot()
	if firstInterval != framems*time.Millisecond || firstSamples != 0 {
		t.Fatalf("first timing reset = interval %v samples %d", firstInterval, firstSamples)
	}
	if secondLast.IsZero() || secondInterval != 260*time.Millisecond || secondSamples != 1 || secondReply != 90*time.Millisecond {
		t.Fatal("resetting first session changed second network timing")
	}
}

func TestSessionsOwnIndependentDrawModels(t *testing.T) {
	first := mustNewSession(1)
	second := mustNewSession(2)

	first.draw.mu.Lock()
	first.draw.current.hp = 25
	first.draw.current.descriptors[1] = frameDescriptor{Index: 1, Name: "First"}
	first.draw.frame = 12
	first.draw.mu.Unlock()
	first.draw.markChanged()
	second.draw.mu.Lock()
	second.draw.current.hp = 80
	second.draw.current.descriptors[2] = frameDescriptor{Index: 2, Name: "Second"}
	second.draw.frame = 40
	second.draw.mu.Unlock()
	second.draw.markChanged()

	firstState, firstFrame, firstGeneration := first.draw.snapshot()
	secondState, secondFrame, secondGeneration := second.draw.snapshot()
	if firstState.hp != 25 || firstState.descriptors[1].Name != "First" || firstFrame != 12 || firstGeneration != 1 {
		t.Fatalf("first draw snapshot = hp %d descriptors %+v frame %d generation %d", firstState.hp, firstState.descriptors, firstFrame, firstGeneration)
	}
	if secondState.hp != 80 || secondState.descriptors[2].Name != "Second" || secondFrame != 40 || secondGeneration != 1 {
		t.Fatalf("second draw snapshot = hp %d descriptors %+v frame %d generation %d", secondState.hp, secondState.descriptors, secondFrame, secondGeneration)
	}

	first.draw.reset()
	firstState, firstFrame, _ = first.draw.snapshot()
	secondState, secondFrame, _ = second.draw.snapshot()
	if firstState.hp != 0 || len(firstState.descriptors) != 0 || firstFrame != 0 {
		t.Fatalf("first draw reset = hp %d descriptors %+v frame %d", firstState.hp, firstState.descriptors, firstFrame)
	}
	if secondState.hp != 80 || secondState.descriptors[2].Name != "Second" || secondFrame != 40 {
		t.Fatal("resetting first session changed second draw model")
	}
}

func TestHandleSessionDrawStateTargetsOnlyRequestedSession(t *testing.T) {
	first := mustNewSession(1)
	second := mustNewSession(2)
	originalEncrypted := drawStateEncrypted
	originalMovieMode := movieMode
	t.Cleanup(func() {
		drawStateEncrypted = originalEncrypted
		movieMode = originalMovieMode
	})
	drawStateEncrypted = false
	movieMode = false

	first.commands.enqueue("/first")
	second.commands.enqueue("/second")
	first.commands.mu.Lock()
	first.commands.pendingID = 7
	first.commands.pendingSent = true
	first.commands.mu.Unlock()
	second.commands.mu.Lock()
	second.commands.pendingID = 7
	second.commands.pendingSent = true
	second.commands.mu.Unlock()

	packet := minimalDrawStatePacket()
	packet[2] = 7
	if !handleSessionDrawStateAt(second, packet, true, time.Now()) {
		t.Fatal("secondary session rejected valid draw state")
	}

	if ack, resend := first.frames.snapshot(); ack != 0 || resend != 0 {
		t.Fatalf("first frame state changed to %d/%d", ack, resend)
	}
	if ack, resend := second.frames.snapshot(); ack != 1 || resend != 0 {
		t.Fatalf("second frame state = %d/%d, want 1/0", ack, resend)
	}
	firstState, firstFrame, _ := first.draw.snapshot()
	secondState, secondFrame, _ := second.draw.snapshot()
	if firstState.hp != 0 || firstFrame != 0 {
		t.Fatalf("first draw state changed to hp %d frame %d", firstState.hp, firstFrame)
	}
	if secondState.hp != 10 || secondState.hpMax != 10 || secondFrame != 1 {
		t.Fatalf("second draw state = hp %d/%d frame %d", secondState.hp, secondState.hpMax, secondFrame)
	}
	first.commands.mu.Lock()
	firstPending := first.commands.pending
	first.commands.mu.Unlock()
	second.commands.mu.Lock()
	secondPending := second.commands.pending
	second.commands.mu.Unlock()
	if firstPending != "/first" || secondPending != "" {
		t.Fatalf("command acknowledgements crossed sessions: first %q second %q", firstPending, secondPending)
	}
}

func TestCaptureSessionDrawSnapshotUsesRequestedSession(t *testing.T) {
	first := mustNewSession(1)
	second := mustNewSession(2)

	first.draw.current.hp = 25
	first.draw.current.descriptors[1] = frameDescriptor{Index: 1, Name: "First"}
	first.draw.markChanged()
	second.draw.current.hp = 80
	second.draw.current.hpMax = 100
	second.draw.current.logicalFrame = 40
	second.draw.current.descriptors[2] = frameDescriptor{Index: 2, Name: "Second"}
	second.draw.current.nameMobs = []frameMobile{{Index: 2, H: 10, V: 20}}
	second.draw.frame = 40
	second.draw.markChanged()

	var snap drawSnapshot
	if !captureSessionDrawSnapshotIfChanged(second, &snap) {
		t.Fatal("initial secondary snapshot was not captured")
	}
	if snap.hp != 80 || snap.hpMax != 100 || snap.logicalFrame != 40 || snap.descriptors[2].Name != "Second" {
		t.Fatalf("secondary snapshot = hp %d/%d logical frame %d descriptors %+v", snap.hp, snap.hpMax, snap.logicalFrame, snap.descriptors)
	}
	if _, found := snap.descriptors[1]; found {
		t.Fatal("secondary snapshot contains first session descriptor")
	}
	if len(snap.mobiles) != 1 || snap.mobiles[0].Index != 2 {
		t.Fatalf("secondary snapshot mobiles = %+v", snap.mobiles)
	}
	if captureSessionDrawSnapshotIfChanged(second, &snap) {
		t.Fatal("unchanged secondary snapshot was captured again")
	}
	first.draw.markChanged()
	if captureSessionDrawSnapshotIfChanged(second, &snap) {
		t.Fatal("first session generation invalidated secondary snapshot")
	}
}

func TestSecondaryStateDataStaysSessionScoped(t *testing.T) {
	secondary := mustNewSession(2)
	originalSettings := gs
	originalMovieMode := movieMode
	originalBlockSound := blockSound
	originalCombined := combinedSessionEvents
	t.Cleanup(func() {
		gs = originalSettings
		movieMode = originalMovieMode
		blockSound = originalBlockSound
		combinedSessionEvents = originalCombined
	})
	gs.SpeechBubbles = true
	gs.BubbleNormal = true
	gs.BubbleOtherPlayers = true
	movieMode = false
	blockSound = true
	combinedSessionEvents = newSessionEventLog(maxSessionEvents)

	primaryInventoryBefore := primarySession.inventory.snapshot()
	primaryChatBefore := len(chatLog.Delta(0).entries)
	primaryConsoleBefore := len(consoleLog.Delta(0).entries)
	playersMu.RLock()
	primaryPlayersBefore := len(players)
	playersMu.RUnlock()

	if _, _, err := parseSessionDrawState(secondary, sessionStateDataForTest(), false); err != nil {
		t.Fatalf("secondary state-data parse: %v", err)
	}

	items := secondary.inventory.snapshot()
	if len(items) != 1 || items[0].ID != 100 || items[0].Name != "Staff" {
		t.Fatalf("secondary inventory = %+v", items)
	}
	if after := primarySession.inventory.snapshot(); !reflect.DeepEqual(after, primaryInventoryBefore) {
		t.Fatalf("secondary inventory packet changed primary inventory: before %+v after %+v", primaryInventoryBefore, after)
	}
	secondary.draw.mu.Lock()
	bubbles := append([]bubble(nil), secondary.draw.current.bubbles...)
	secondary.draw.mu.Unlock()
	if len(bubbles) != 1 || bubbles[0].Text != "hello" || bubbles[0].OwnerName != "Bob" {
		t.Fatalf("secondary bubbles = %+v", bubbles)
	}
	presence := secondary.players.snapshot()
	if len(presence) != 1 || presence[0].Name != "Bob" {
		t.Fatalf("secondary player presence = %+v", presence)
	}
	events := secondary.events.log.snapshot()
	if len(events) != 2 {
		t.Fatalf("secondary events = %+v, want chat and sound", events)
	}
	if events[0].Source != 2 || events[0].Kind != sessionEventChat || events[0].Text != "Bob says, hello" {
		t.Fatalf("secondary chat event = %+v", events[0])
	}
	if events[1].Source != 2 || events[1].Kind != sessionEventSound || !reflect.DeepEqual(events[1].SoundIDs, []uint16{58}) {
		t.Fatalf("secondary sound event = %+v", events[1])
	}
	combined := combinedSessionEvents.snapshot()
	if !reflect.DeepEqual(combined, events) {
		t.Fatalf("combined events = %+v, want %+v", combined, events)
	}
	if got := len(chatLog.Delta(0).entries); got != primaryChatBefore {
		t.Fatalf("secondary chat entered primary chat log: before %d after %d", primaryChatBefore, got)
	}
	if got := len(consoleLog.Delta(0).entries); got != primaryConsoleBefore {
		t.Fatalf("secondary output entered primary console log: before %d after %d", primaryConsoleBefore, got)
	}
	playersMu.RLock()
	primaryPlayersAfter := len(players)
	playersMu.RUnlock()
	if primaryPlayersAfter != primaryPlayersBefore {
		t.Fatalf("secondary presence changed primary player directory: before %d after %d", primaryPlayersBefore, primaryPlayersAfter)
	}
}

func TestSessionsDeduplicateSoundsIndependently(t *testing.T) {
	first := mustNewSession(1)
	second := mustNewSession(2)

	if got := first.filterSounds([]uint16{58, 58}, true); !reflect.DeepEqual(got, []uint16{58}) {
		t.Fatalf("first sound frame = %v", got)
	}
	if got := first.filterSounds([]uint16{58}, true); len(got) != 0 {
		t.Fatalf("first duplicate sound frame = %v, want none", got)
	}
	if got := second.filterSounds([]uint16{58}, true); !reflect.DeepEqual(got, []uint16{58}) {
		t.Fatalf("first session suppressed second session sound: %v", got)
	}
}

func TestDisconnectClosesOnlyOwningSessionTransport(t *testing.T) {
	first := mustNewSession(2)
	second := mustNewSession(3)
	firstClient, firstServer := net.Pipe()
	secondClient, secondServer := net.Pipe()
	defer firstServer.Close()
	defer secondServer.Close()
	firstCtx, firstCancel := context.WithCancel(context.Background())
	secondCtx, secondCancel := context.WithCancel(context.Background())
	defer firstCancel()
	defer secondCancel()
	if !first.transport.begin(firstCancel) || !second.transport.begin(secondCancel) {
		t.Fatal("failed to begin independent transports")
	}
	firstGeneration, ok := first.transport.attach(firstClient, firstClient)
	if !ok {
		t.Fatal("failed to attach first transport")
	}
	if _, ok := second.transport.attach(secondClient, secondClient); !ok {
		t.Fatal("failed to attach second transport")
	}
	first.inventory.add(100, -1, "First", false)
	second.inventory.add(200, -1, "Second", false)

	handleSessionDisconnect(first)

	select {
	case <-firstCtx.Done():
	default:
		t.Fatal("first transport cancellation did not fire")
	}
	select {
	case <-secondCtx.Done():
		t.Fatal("disconnecting first session canceled second transport")
	default:
	}
	if first.transport.connected() {
		t.Fatal("first transport remained connected")
	}
	if !second.transport.connected() {
		t.Fatal("second transport was disconnected")
	}
	if !first.transport.finish(firstGeneration) {
		t.Fatal("first transport lifecycle did not finish")
	}
	completeSessionDisconnect(first)
	if len(first.inventory.snapshot()) != 0 {
		t.Fatal("first session models were not cleared after its loops joined")
	}
	if items := second.inventory.snapshot(); len(items) != 1 || items[0].ID != 200 {
		t.Fatalf("second session models changed during first disconnect: %+v", items)
	}
	second.transport.disconnect()
}

func TestSendSessionPlayerInputUsesOwningSession(t *testing.T) {
	first := mustNewSession(1)
	second := mustNewSession(2)
	first.frames.set(11, -1)
	second.frames.set(42, 7)
	first.commands.enqueue("/first")
	second.commands.enqueue("/second")

	connection := &bufConn{}
	if err := sendSessionPlayerInput(second, connection, 3, 4, true, false); err != nil {
		t.Fatal(err)
	}
	if command := extractCommandText(t, connection); command != "/second" {
		t.Fatalf("secondary packet command = %q", command)
	}
	packet := connection.Bytes()[2:]
	if ack := int32(binary.BigEndian.Uint32(packet[8:12])); ack != 42 {
		t.Fatalf("secondary packet ack = %d, want 42", ack)
	}
	if resend := int32(binary.BigEndian.Uint32(packet[12:16])); resend != 7 {
		t.Fatalf("secondary packet resend = %d, want 7", resend)
	}
	first.commands.mu.Lock()
	firstPending, firstSent := first.commands.pending, first.commands.pendingSent
	first.commands.mu.Unlock()
	if firstPending != "/first" || firstSent {
		t.Fatalf("secondary send changed first command state: %q sent=%v", firstPending, firstSent)
	}
}

func TestSessionDispatchRoutesDrawPacketToOwningSession(t *testing.T) {
	first := mustNewSession(1)
	second := mustNewSession(2)
	originalMovieMode := movieMode
	originalEncrypted := drawStateEncrypted
	t.Cleanup(func() {
		movieMode = originalMovieMode
		drawStateEncrypted = originalEncrypted
	})
	movieMode = false
	drawStateEncrypted = false

	dispatchSessionIncomingServerMessage(second, incomingServerMessage{
		data:       minimalDrawStatePacket(),
		receivedAt: time.Now(),
	}, false)

	if ack := second.frames.acknowledged(); ack != 1 {
		t.Fatalf("secondary dispatch ack = %d, want 1", ack)
	}
	if ack := first.frames.acknowledged(); ack != 0 {
		t.Fatalf("secondary dispatch changed first ack to %d", ack)
	}
	secondState, _, _ := second.draw.snapshot()
	firstState, _, _ := first.draw.snapshot()
	if secondState.hp != 10 || firstState.hp != 0 {
		t.Fatalf("dispatch draw states = first hp %d second hp %d", firstState.hp, secondState.hp)
	}
}

func TestStaleTransportFinishCannotClearReplacement(t *testing.T) {
	session := mustNewSession(2)
	firstClient, firstServer := net.Pipe()
	defer firstServer.Close()
	if generation, ok := session.transport.attach(firstClient, firstClient); !ok {
		t.Fatal("failed to attach first transport")
	} else {
		session.transport.disconnect()
		secondClient, secondServer := net.Pipe()
		defer secondServer.Close()
		defer session.transport.disconnect()
		if _, ok := session.transport.attach(secondClient, secondClient); !ok {
			t.Fatal("failed to attach replacement transport")
		}
		session.transport.finish(generation)
		if !session.transport.connected() {
			t.Fatal("stale transport completion cleared replacement connection")
		}
	}
}

func sessionStateDataForTest() []byte {
	stateData := []byte{0, 1, 1, byte(kBubbleNormal)}
	stateData = append(stateData, encodeMacRoman("hello")...)
	stateData = append(stateData, 0, 1, 0, 58)
	stateData = append(stateData, byte(kInvCmdAdd), 0, 100)
	stateData = append(stateData, encodeMacRoman("Staff")...)
	stateData = append(stateData, 0, byte(kInvCmdNone))

	descriptor := []byte{1, kDescPlayer, 0, 0}
	descriptor = append(descriptor, encodeMacRoman("Bob")...)
	descriptor = append(descriptor, 0, 0)

	data := make([]byte, 0, 32+len(stateData))
	data = append(data, 0)
	data = append(data, make([]byte, 8)...)
	binary.BigEndian.PutUint32(data[1:5], 1)
	data = append(data, 1)
	data = append(data, descriptor...)
	data = append(data, 10, 10, 10, 10, 10, 10, 0)
	data = append(data, 0, 0)
	data = append(data, byte(len(stateData)>>8), byte(len(stateData)))
	data = append(data, stateData...)
	return data
}
