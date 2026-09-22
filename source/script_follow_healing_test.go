package main

import (
	"reflect"
	"testing"
	"time"
)

func TestFollowHealing(t *testing.T) {
	initFont()
	isolateScriptWorld(t)
	resetInventory()
	clearCommands()
	t.Cleanup(func() { resetInventory(); clearCommands() })
	const owner = "follow_healing"
	sim := activateBundledProofScript(t, owner, "follow_player.go")
	sim.login(t, "Hero")
	configure := func(key string, value any) {
		t.Helper()
		if !scriptSetConfigValue(owner, key, value) {
			t.Fatalf("missing setting %s", key)
		}
		sim.barrier(t)
	}
	wantCommands := func(want ...string) {
		t.Helper()
		got := getQueuedCommands()
		if len(got) != len(want) || (len(got) > 0 && !reflect.DeepEqual(got, want)) {
			t.Fatalf("commands = %v, want %v", got, want)
		}
		clearCommands()
	}
	base := time.Now()
	frame := 0
	elapsed := time.Duration(0)
	update := func(health, spirit int, distance int16, visible bool, step time.Duration) {
		frame++
		elapsed += step
		primarySession.draw.mu.Lock()
		primarySession.draw.current.descriptors = map[uint8]frameDescriptor{
			1: {Index: 1, Name: "Hero", Type: kDescPlayer},
			2: {Index: 2, Name: "Leader", Type: kDescPlayer},
		}
		primarySession.draw.current.liveMobs = []frameMobile{{Index: 1}}
		if visible {
			primarySession.draw.current.liveMobs = append(primarySession.draw.current.liveMobs, frameMobile{Index: 2, H: distance})
		}
		primarySession.draw.current.hp, primarySession.draw.current.hpMax = health, 100
		primarySession.draw.current.sp, primarySession.draw.current.spMax = spirit, 100
		primarySession.draw.current.logicalFrame = frame
		primarySession.draw.current.receivedAt = base.Add(elapsed)
		markWorldStateChanged()
		primarySession.draw.mu.Unlock()
		dispatchScriptChange(ChangeEvent{Type: ChangeWorld})
		sim.barrier(t)
	}
	const retry = 3 * time.Second
	addInventoryItem(501, -1, "Caduceus", true)
	addInventoryItem(502, -1, "Moonstone", false)
	update(100, 100, 80, true, 0)
	sim.command(t, "follow", "Leader")
	wantCommands() // Healing is opt-in.
	configure("heal-leader", true)
	update(100, 100, 80, true, retry)
	wantCommands("/useitem caduceus leader")
	update(100, 100, 80, true, 100*time.Millisecond)
	wantCommands() // No command per frame.
	update(100, 100, 160, true, retry)
	wantCommands("/useitem caduceus /off")
	if !applyScriptMovement(inputState{}, time.Now()).mouseDown {
		t.Fatal("healing range limit stopped catching up")
	}
	update(100, 100, 80, true, retry)
	wantCommands("/useitem caduceus leader")
	update(100, 100, 80, false, retry)
	wantCommands("/useitem caduceus /off")
	update(100, 100, 80, true, retry)
	wantCommands("/useitem caduceus leader")
	update(30, 100, 80, true, retry)
	wantCommands("/useitem caduceus /off")
	update(100, 20, 80, true, retry)
	wantCommands()

	configure("heal-self", true)
	update(80, 100, 80, true, retry)
	wantCommands("/useitem caduceus leader") // Exactly 80% does not self-heal.
	update(79, 100, 80, true, retry)
	wantCommands("/useitem caduceus /off", "/equip 502")
	update(79, 100, 80, true, 100*time.Millisecond)
	wantCommands() // Wait for the server to confirm equipment.
	resetInventory()
	addInventoryItem(501, -1, "Caduceus", false)
	addInventoryItem(502, -1, "Moonstone", true)
	update(79, 100, 80, true, retry)
	wantCommands("/use 10")
	update(79, 100, 80, true, 100*time.Millisecond)
	wantCommands()
	configure("self-heal-below", 70)
	update(79, 100, 80, true, retry)
	wantCommands("/equip 501")
	configure("heal-equip", false)
	update(79, 100, 80, true, retry)
	wantCommands()
	update(69, 100, 80, true, retry)
	wantCommands("/use 10")
	update(69, 20, 80, true, retry)
	wantCommands()
	resetInventory()
	update(69, 100, 80, true, retry)
	wantCommands() // Missing moonstone cannot fall through to a different /use.
	addInventoryItem(501, -1, "Caduceus", true)
	update(100, 100, 80, true, retry)
	wantCommands("/useitem caduceus leader")
	sim.command(t, "follow", "off")
	wantCommands("/useitem caduceus /off")
	update(20, 100, 80, true, retry)
	wantCommands()
	update(100, 100, 80, true, retry)
	sim.command(t, "follow", "Leader")
	wantCommands("/useitem caduceus leader")
	configure("heal-leader", false)
	wantCommands("/useitem caduceus /off")
	configure("heal-leader", true)
	update(100, 100, 80, true, retry)
	wantCommands("/useitem caduceus leader")
	primarySession.draw.mu.Lock()
	primarySession.draw.current.receivedAt = time.Now().Add(-2 * time.Second)
	markWorldStateChanged()
	primarySession.draw.mu.Unlock()
	sim.timers(t)
	wantCommands("/useitem caduceus /off")
	update(100, 100, 80, true, retry)
	wantCommands("/useitem caduceus leader")
	interruptScriptMovement(time.Now())
	sim.timers(t)
	wantCommands("/useitem caduceus /off")
	update(100, 100, 80, true, retry)
	wantCommands()
}
