package main

import (
	"context"
	"encoding/binary"
	"net"
	"reflect"
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
