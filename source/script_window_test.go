package main

import (
	"strings"
	"testing"
	"time"

	"gothoom/eui"
)

func TestScriptWindowLifecycle(t *testing.T) {
	initFont()
	const owner = "window_test"
	resetScriptCallbackTestState(t, owner)
	prepared, err := prepareScriptSource(owner, []byte(`package main
import "gt2"
var panel gt2.Window
func Init() {
 panel = gt2.CreateWindow(gt2.WindowOptions{
  Title: "Script controls", Text: "Initial",
  Buttons: []gt2.WindowButton{{ID:"go", Label:"Go", Disabled:true, OnClick:func(){panel.SetText("Clicked")}}},
  OnClose:func(){gt2.Store("closed",true)},
 })
 panel.SetText("Ready")
 gt2.Command("enable",func(args string){panel.SetButtonEnabled("go",true)})
}
`), restrictedStdlib())
	if err != nil {
		t.Fatal(err)
	}
	value, err := prepared.interpreter.Eval("panel")
	if err != nil {
		t.Fatal(err)
	}
	panel := value.Interface().(Window)
	if panel.Active() || panel.state.ui != nil {
		t.Fatal("validation opened a native window")
	}
	activatePreparedScript(owner, prepared)
	t.Cleanup(func() { disablescript(owner, "test cleanup"); drainScriptDispatcher() })
	sim := scriptEventSimulator{owner: owner}
	sim.barrier(t)
	if !panel.Active() || panel.state.text != "Ready" || !panel.state.ui.IsOpen() {
		t.Fatal("window was not activated with staged text")
	}
	native := panel.state.ui
	button := panel.state.buttons["go"]
	button.Handler.Handle(eui.UIEvent{Type: eui.EventClick})
	sim.barrier(t)
	if panel.state.text != "Ready" {
		t.Fatal("disabled button ran its callback")
	}
	sim.command(t, "enable", "")
	button.Handler.Handle(eui.UIEvent{Type: eui.EventClick})
	sim.barrier(t)
	if panel.state.text != "Clicked" {
		t.Fatal("window callback did not update status")
	}
	if panel.state.ui != native {
		t.Fatal("status update recreated the window")
	}
	panel.Hide()
	sim.barrier(t)
	sim.barrier(t) // Close callback is enqueued by the preceding UI dispatch.
	if native.IsOpen() || !panel.Active() || scriptStorageGet(owner, "closed") != true {
		t.Fatal("hide/OnClose lifecycle failed")
	}
	panel.Show()
	sim.barrier(t)
	if !native.IsOpen() {
		t.Fatal("Show did not reopen hidden window")
	}
	panel.Remove()
	sim.barrier(t)
	if panel.Active() || native.IsOpen() {
		t.Fatal("Remove left the window active")
	}
	panel.SetText("stale")
	panel.Show()
	button.Handler.Handle(eui.UIEvent{Type: eui.EventClick})
	sim.barrier(t)
	if native.IsOpen() || panel.state.text != "Clicked" {
		t.Fatal("removed window accepted a stale update or click")
	}
}

func TestScriptWindowDiscardAndStop(t *testing.T) {
	initFont()
	const owner = "window_discard"
	resetScriptCallbackTestState(t, owner)
	t.Cleanup(func() { disablescript(owner, "test cleanup"); drainScriptDispatcher() })
	src := []byte(`package main
import "gt2"
var panel gt2.Window
func Init(){panel=gt2.CreateWindow(gt2.WindowOptions{Title:"Temporary"})}
`)
	prepared, err := prepareScriptSource(owner, src, restrictedStdlib())
	if err != nil {
		t.Fatal(err)
	}
	value, _ := prepared.interpreter.Eval("panel")
	discarded := value.Interface().(Window)
	disposePreparedScript(prepared)
	discarded.Show()
	drainScriptDispatcher()
	if discarded.state.ui != nil || discarded.Active() {
		t.Fatal("discarded validation created UI")
	}
	prepared, err = prepareScriptSource(owner, src, restrictedStdlib())
	if err != nil {
		t.Fatal(err)
	}
	activatePreparedScript(owner, prepared)
	value, _ = prepared.interpreter.Eval("panel")
	old := value.Interface().(Window)
	disablescript(owner, "reloaded")
	if old.Active() {
		t.Fatal("script stop left window handle active")
	}
	replacement, err := prepareScriptSource(owner, src, restrictedStdlib())
	if err != nil {
		t.Fatal(err)
	}
	activatePreparedScript(owner, replacement)
	value, _ = replacement.interpreter.Eval("panel")
	current := value.Interface().(Window)
	old.Show()
	drainScriptDispatcher()
	if old.state.ui.IsOpen() || !current.Active() || !current.state.ui.IsOpen() {
		t.Fatal("old cleanup affected replacement window")
	}
}

func TestScriptWindowRejectsAmbiguousButtons(t *testing.T) {
	grantScriptPermissionsForTest(t, "invalid_window")
	prepared, err := prepareScriptSource("invalid_window", []byte(`package main
import "gt2"
func Init(){gt2.CreateWindow(gt2.WindowOptions{Buttons:[]gt2.WindowButton{
 {ID:"same",Label:"One",OnClick:func(){}}, {ID:"same",Label:"Two",OnClick:func(){}},
}})}
`), restrictedStdlib())
	disposePreparedScript(prepared)
	if err == nil || !strings.Contains(err.Error(), "unique") {
		t.Fatalf("duplicate button IDs accepted: %v", err)
	}
}

func TestFollowWindowControlsAndStatus(t *testing.T) {
	initFont()
	isolateScriptWorld(t)
	const owner = "follow_window"
	oldSelected := selectedPlayerName
	playersMu.Lock()
	oldPlayers := players
	players = map[string]*Player{"Leader": {Name: "Leader"}}
	playersMu.Unlock()
	selectedPlayerName = "Leader"
	t.Cleanup(func() {
		selectedPlayerName = oldSelected
		playersMu.Lock()
		players = oldPlayers
		playersMu.Unlock()
		drainScriptDispatcher()
	})
	sim := activateBundledProofScript(t, owner, "follow_player.go")
	sim.login(t, "Hero")
	stateMu.Lock()
	state.descriptors = map[uint8]frameDescriptor{1: {Index: 1, Name: "Hero", Type: kDescPlayer}, 2: {Index: 2, Name: "Leader", Type: kDescPlayer}}
	state.liveMobs = []frameMobile{{Index: 1}, {Index: 2, H: 100}}
	state.logicalFrame = 1
	state.receivedAt = time.Now()
	stateMu.Unlock()
	dispatchScriptChange(ChangeEvent{Type: ChangeWorld})
	sim.barrier(t)
	value, err := currentScriptEventQueue(owner).interpreter.Eval("followWindow")
	if err != nil {
		t.Fatal(err)
	}
	panel := value.Interface().(Window)
	if !panel.Active() || panel.state.buttons["follow"].Disabled || !panel.state.buttons["stop"].Disabled {
		t.Fatal("initial window button state incorrect")
	}
	panel.state.buttons["follow"].Handler.Handle(eui.UIEvent{Type: eui.EventClick})
	sim.barrier(t)
	if !strings.Contains(panel.state.text, "Following: Leader") || !strings.Contains(panel.state.text, "Status: Following") || panel.state.buttons["stop"].Disabled {
		t.Fatalf("follow control/status failed: %q", panel.state.text)
	}
	stateMu.Lock()
	state.liveMobs[1].H = 40
	state.logicalFrame++
	state.receivedAt = time.Now()
	stateMu.Unlock()
	dispatchScriptChange(ChangeEvent{Type: ChangeWorld})
	sim.barrier(t)
	if !strings.Contains(panel.state.text, "Status: Staying") {
		t.Fatalf("resting status missing: %q", panel.state.text)
	}
	panel.state.buttons["stop"].Handler.Handle(eui.UIEvent{Type: eui.EventClick})
	sim.barrier(t)
	if !strings.Contains(panel.state.text, "Status: Stopped") || !panel.state.buttons["stop"].Disabled || scriptMovementSnapshot(owner, time.Now()).Active {
		t.Fatal("Stop Follow did not stop movement and update status")
	}
	panel.state.ui.Close()
	sim.barrier(t)
	sim.command(t, "followui", "")
	if !panel.state.ui.IsOpen() {
		t.Fatal("/followui did not reopen the window")
	}
}
