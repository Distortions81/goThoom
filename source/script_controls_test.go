package main

import (
	"gothoom/eui"
	"testing"
)

func TestScriptControlEditsPersistAndReleaseOriginalHandles(t *testing.T) {
	const owner = "editable_controls"
	resetScriptCallbackTestState(t, owner)
	t.Cleanup(func() { disablescript(owner, "test cleanup") })
	gotArgs, gotChord := "", ""
	command := scriptRegisterCommand(owner, "hello", func(args string) { gotArgs = args })
	binding := scriptAddHotkeyFn(owner, "Ctrl-F3", func(event InputEvent) { gotChord = event.Chord; event.Consume() })
	if err := setScriptCommandName(owner, "hello", "/Greetings"); err != nil {
		t.Fatal(err)
	}
	if err := setScriptBinding(owner, "Ctrl-F3", "Alt-F4"); err != nil {
		t.Fatal(err)
	}
	if scriptCommands["hello"] != nil {
		t.Fatal("old command remains callable")
	}
	if _, ok := scriptGetHotkeyFn(owner, "Ctrl-F3"); ok {
		t.Fatal("old binding remains callable")
	}
	sim := scriptEventSimulator{owner: owner}
	sim.command(t, "greetings", "one two")
	if sim.input(t, makeScriptInputEvent("Alt-F4")) {
		t.Fatal("remapped handler did not consume input")
	}
	if gotArgs != "one two" || gotChord != "Alt-F4" {
		t.Fatalf("callbacks = %q / %q", gotArgs, gotChord)
	}
	// Disabled state follows the script's original binding identity.
	hotkeys[0].Disabled = true
	saveHotkeys()
	command.release()
	binding.release()
	if scriptCommands["greetings"] != nil || len(scriptHotkeys(owner)) != 0 {
		t.Fatal("subscription release left remapped controls")
	}
	if _, ok := scriptGetHotkeyFn(owner, "Alt-F4"); ok {
		t.Fatal("released handler remains callable")
	}
	// Discard in-memory state to verify the files are sufficient on restart.
	scriptStores = map[string]*scriptStore{}
	loadHotkeys()
	command = scriptRegisterCommand(owner, "hello", func(args string) { gotArgs = args })
	binding = scriptAddHotkeyFn(owner, "Ctrl-F3", func(InputEvent) {})
	if scriptCommands["greetings"] == nil {
		t.Fatal("command rename did not survive reload")
	}
	keys := scriptHotkeys(owner)
	if len(keys) != 1 || keys[0].Combo != "Alt-F4" || !keys[0].Disabled {
		t.Fatalf("reloaded keys = %+v", keys)
	}
	if err := setScriptCommandName(owner, "hello", "hello"); err != nil {
		t.Fatal(err)
	}
	if err := setScriptBinding(owner, "Ctrl-F3", "Ctrl-F3"); err != nil {
		t.Fatal(err)
	}
	if scriptStorageGet(owner, "__controls__:command:hello") != nil || scriptStorageGet(owner, "__controls__:binding:Ctrl-F3") != nil {
		t.Fatal("reset kept overrides")
	}
	command.release()
	binding.release()
}

func TestScriptControlEditsRejectConflictsWithoutChangingRegistrations(t *testing.T) {
	const owner = "control_conflicts"
	resetScriptCallbackTestState(t, owner)
	t.Cleanup(func() { disablescript(owner, "test cleanup"); disablescript("other_controls", "test cleanup") })
	scriptRegisterCommand(owner, "first", func(string) {})
	scriptRegisterCommand(owner, "second", func(string) {})
	scriptRegisterCommand("other_controls", "taken", func(string) {})
	for _, value := range []string{"", "two words", "first/arg", "second", "taken", "palette", "play", "setting", "testhooks"} {
		if err := setScriptCommandName(owner, "first", value); err == nil {
			t.Errorf("accepted command %q", value)
		}
	}
	scriptAddHotkeyFn(owner, "Ctrl-F3", func(InputEvent) {})
	scriptAddHotkeyFn(owner, "Shift-F5", func(InputEvent) {})
	hotkeys = append(hotkeys, Hotkey{Combo: "Control-Shift-F6"})
	for _, value := range []string{"", "Ctrl-", "Ctrl", "BogusKey", "garbage-F3", "Shift-F5", "Shift-Ctrl-F6"} {
		if err := setScriptBinding(owner, "Ctrl-F3", value); err == nil {
			t.Errorf("accepted binding %q", value)
		}
	}
	if scriptCommands["first"] == nil || scriptHotkeys(owner)[0].Combo != "Ctrl-F3" {
		t.Fatal("rejected edit changed registration")
	}
	if scriptControlValue(owner, "command", "first") != "first" || scriptControlValue(owner, "binding", "Ctrl-F3") != "Ctrl-F3" {
		t.Fatal("rejected edit persisted")
	}
}

func TestScriptControlOverridesParticipateInReloadPreflight(t *testing.T) {
	const owner = "control_reload"
	resetScriptCallbackTestState(t, owner)
	t.Cleanup(func() { disablescript(owner, "test cleanup"); disablescript("other_controls", "test cleanup") })
	scriptRegisterCommand("other_controls", "taken", func(string) {})
	saveScriptControl(owner, "command", "original", "taken")
	candidate := &scriptCandidate{commands: map[string]struct{}{"original": {}}}
	if err := scriptCandidateConflict(owner, candidate); err == nil {
		t.Fatal("preflight missed remapped command conflict")
	}
	saveScriptControl(owner, "command", "original", "different")
	candidate.commands["different"] = struct{}{}
	if err := scriptCandidateConflict(owner, candidate); err == nil {
		t.Fatal("preflight missed same-script command conflict")
	}
	candidate.commands = nil
	saveScriptControl(owner, "binding", "F3", "F4")
	candidate.bindings = []string{"F3", "F4"}
	if err := scriptCandidateConflict(owner, candidate); err == nil {
		t.Fatal("preflight missed same-script binding conflict")
	}
	candidate.bindings = []string{"F3"}
	hotkeys = append(hotkeys, Hotkey{Combo: "F4"})
	if err := scriptCandidateConflict(owner, candidate); err == nil {
		t.Fatal("preflight missed global binding conflict")
	}
}

func TestScriptSettingsEditsCommandsWithoutPreferences(t *testing.T) {
	initFont()
	const owner = "settings_controls_ui"
	resetScriptCallbackTestState(t, owner)
	t.Cleanup(func() { disablescript(owner, "test cleanup") })
	scriptRegisterCommand(owner, "hello", func(string) {})
	scriptAddHotkeyFn(owner, "F3", func(InputEvent) {})
	openscriptConfigWindow(owner)
	if scriptConfigWin == nil || !scriptConfigWin.Open {
		t.Fatal("missing settings window")
	}
	tabs := scriptConfigWin.Contents[0].Tabs
	if len(tabs) != 3 || tabs[0].Name != "Preferences" || tabs[1].Name != "Key bindings" || tabs[2].Name != "Commands" {
		t.Fatal("missing settings tabs")
	}
	row := tabs[2].Contents[1].Contents[1]
	input, apply, reset := row.Contents[0], row.Contents[1], row.Contents[2]
	input.Text = "greeting"
	apply.Handler.Emit(eui.UIEvent{Item: apply, Type: eui.EventClick})
	if scriptCommands["greeting"] == nil {
		t.Fatal("Apply did not rename command")
	}
	reset.Handler.Emit(eui.UIEvent{Item: reset, Type: eui.EventClick})
	if input.Text != "hello" || scriptCommands["hello"] == nil {
		t.Fatal("Reset did not restore default")
	}
	input.Text = "two words"
	apply.Handler.Emit(eui.UIEvent{Item: apply, Type: eui.EventClick})
	if tabs[2].Contents[1].Contents[2].Text == "Saved." {
		t.Fatal("invalid edit reported success")
	}
}

func TestScriptControlOverridesSurviveInterpreterReload(t *testing.T) {
	const owner = "control_interpreter_reload"
	resetScriptCallbackTestState(t, owner)
	t.Cleanup(func() { disablescript(owner, "test cleanup") })
	source := []byte(`package main
import "gt2"
func Init() {
 gt2.Command("original",func(args string){gt2.Store("args",args)})
 gt2.Bind("F3",func(event gt2.InputEvent){gt2.Store("chord",event.Chord)})
}`)
	activate := func() {
		t.Helper()
		prepared, err := prepareScriptSource(owner, source, restrictedStdlib())
		if err != nil {
			t.Fatal(err)
		}
		if err := scriptCandidateConflict(owner, prepared.candidate); err != nil {
			disposePreparedScript(prepared)
			t.Fatal(err)
		}
		activatePreparedScript(owner, prepared)
	}
	activate()
	if err := setScriptCommandName(owner, "original", "renamed"); err != nil {
		t.Fatal(err)
	}
	if err := setScriptBinding(owner, "F3", "F4"); err != nil {
		t.Fatal(err)
	}
	oldQueue := currentScriptEventQueue(owner)
	disablescript(owner, "reloaded")
	activate()
	if currentScriptEventQueue(owner) == oldQueue {
		t.Fatal("reload retained old interpreter queue")
	}
	sim := scriptEventSimulator{owner: owner}
	sim.command(t, "renamed", "after reload")
	sim.input(t, makeScriptInputEvent("F4"))
	if scriptStorageGet(owner, "args") != "after reload" || scriptStorageGet(owner, "chord") != "F4" {
		t.Fatal("reloaded controls did not call new handlers")
	}
	if scriptCommands["original"] != nil {
		t.Fatal("reload restored original command")
	}
}

func TestScriptSettingsClosesItsRecorder(t *testing.T) {
	initFont()
	const owner = "control_recorder_close"
	resetScriptCallbackTestState(t, owner)
	t.Cleanup(func() { disablescript(owner, "test cleanup") })
	scriptAddHotkeyFn(owner, "F3", func(InputEvent) {})
	openscriptConfigWindow(owner)
	row := scriptConfigWin.Contents[0].Tabs[1].Contents[1].Contents[1]
	record := row.Contents[1]
	record.Handler.Emit(eui.UIEvent{Item: record, Type: eui.EventClick})
	if !recording || recordTarget != row.Contents[0] {
		t.Fatal("Record did not start binding capture")
	}
	scriptConfigWin.Close()
	if recording || recordTarget != nil {
		t.Fatal("closed settings left recording active")
	}
}
