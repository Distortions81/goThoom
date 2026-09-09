package main

import (
	"reflect"
	"testing"
	"time"

	"gothoom/eui"
	scriptapi "gt2"
)

func activateExtensionTest(t *testing.T, owner, source string) *preparedScript {
	t.Helper()
	resetScriptCallbackTestState(t, owner)
	prepared, err := prepareScriptSource(owner, []byte(source), restrictedStdlib())
	if err != nil {
		t.Fatal(err)
	}
	activatePreparedScript(owner, prepared)
	t.Cleanup(func() { disablescript(owner, "test cleanup"); drainScriptDispatcher(); clearCommands() })
	return prepared
}
func waitExtensionTest(t *testing.T, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for !condition() {
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for script")
		}
		time.Sleep(time.Millisecond)
	}
}
func TestScriptCharacterStore(t *testing.T) {
	const owner = "character-store-test"
	resetScriptCallbackTestState(t, owner)
	t.Cleanup(func() {
		stopScriptEventQueue(owner)
		scriptSessionMu.Lock()
		scriptSessionActive = false
		scriptSessionCharacter = ""
		scriptSessionMu.Unlock()
	})
	exports := exportsForscript(owner)["gt2/gt2"]
	create := exports["CharacterStore"].Interface().(func() Storage)
	if create().Active() {
		t.Fatal("unknown character has storage")
	}
	selectCharacter := func(name string) {
		scriptSessionMu.Lock()
		scriptSessionActive = true
		scriptSessionCharacter = name
		scriptSessionMu.Unlock()
	}
	selectCharacter(" Pebble Pockets ")
	first := create()
	first.Store("count", 7)
	first.Store("names", []string{"one"})
	names := first.LoadStrings("names", nil)
	names[0] = "changed"
	if first.LoadStrings("names", nil)[0] != "one" {
		t.Fatal("storage slice alias")
	}
	selectCharacter("Other")
	if first.Active() || first.LoadInteger("count", 0) != 0 {
		t.Fatal("stale character handle accessed storage")
	}
	second := create()
	second.Store("count", 3)
	selectCharacter("PEBBLE POCKETS")
	if create().LoadInteger("count", 0) != 7 {
		t.Fatal("normalized character storage was not retained")
	}
	scriptPermissionMu.Lock()
	scriptPermissionGrants[owner]["storage"] = false
	scriptPermissionMu.Unlock()
	first.Store("count", 9)
	if first.Active() {
		t.Fatal("revoked storage handle is active")
	}
	scriptPermissionMu.Lock()
	scriptPermissionGrants[owner]["storage"] = true
	scriptPermissionMu.Unlock()
	if first.LoadInteger("count", 0) != 7 {
		t.Fatal("revoked storage handle wrote data")
	}
}
func TestScriptCharacterStoreInterpreterAndStaging(t *testing.T) {
	const owner = "character-store-interpreter"
	resetScriptCallbackTestState(t, owner)
	scriptSessionMu.Lock()
	scriptSessionActive = true
	scriptSessionCharacter = "Pebble"
	scriptSessionMu.Unlock()
	t.Cleanup(func() {
		disablescript(owner, "test cleanup")
		scriptSessionMu.Lock()
		scriptSessionActive = false
		scriptSessionCharacter = ""
		scriptSessionMu.Unlock()
	})
	src := []byte(`package main
import "gt2"
var saved gt2.Storage
func Init(){ saved=gt2.CharacterStore(); saved.Store("count",4); if saved.LoadInteger("count",0)!=4 { panic("staged read") } }
`)
	prepared, err := prepareScriptSource(owner, src, restrictedStdlib())
	if err != nil {
		t.Fatal(err)
	}
	value, _ := prepared.interpreter.Eval("saved")
	saved := value.Interface().(Storage)
	if scriptStorageGet(owner, saved.key("count")) != nil {
		t.Fatal("validation wrote storage")
	}
	disposePreparedScript(prepared)
	if saved.Active() {
		t.Fatal("discarded storage handle active")
	}
	prepared, err = prepareScriptSource(owner, src, restrictedStdlib())
	if err != nil {
		t.Fatal(err)
	}
	activatePreparedScript(owner, prepared)
	value, _ = prepared.interpreter.Eval("saved")
	saved = value.Interface().(Storage)
	if saved.LoadInteger("count", 0) != 4 {
		t.Fatal("activation lost storage")
	}
}
func TestScriptTaskCancellationDuringWait(t *testing.T) {
	for _, wait := range []string{`gt2.Wait(time.Hour)`, `gt2.WaitTicks(10000)`, `gt2.WaitForInventory("NoSuchItem",true,time.Hour)`, `gt2.WaitForEquipment("NoSuchItem",true,time.Hour)`} {
		t.Run(wait, func(t *testing.T) {
			const owner = "task-cancel-test"
			clearCommands()
			prepared := activateExtensionTest(t, owner, `package main
import("gt2";"time")
var job gt2.Task
var ticket gt2.CommandTicket
var cleanup bool
func Init(){
 job=gt2.StartTask(func(){
  defer func(){cleanup=true}()
  ticket=gt2.QueueCommand("/pose sit")
  gt2.Store("started",true)
  `+wait+`
  gt2.Store("after",true)
 })
 gt2.Command("canceljob",func(args string){job.Cancel()})
}
`)
			waitExtensionTest(t, func() bool { return scriptStorageGet(owner, "started") == true })
			sim := scriptEventSimulator{owner: owner}
			sim.command(t, "canceljob", "")
			waitExtensionTest(t, func() bool {
				q := currentScriptEventQueue(owner)
				q.mu.Lock()
				defer q.mu.Unlock()
				return q.activeTask == nil
			})
			sim.barrier(t)
			v, err := prepared.interpreter.Eval("cleanup")
			if err != nil || !v.Bool() {
				t.Fatalf("task defer: %v %v", v, err)
			}
			if scriptStorageGet(owner, "after") != nil {
				t.Fatal("cancelled task continued after wait")
			}
			v, _ = prepared.interpreter.Eval("ticket")
			if v.Interface().(CommandTicket).Status().State != scriptapi.CommandCancelled {
				t.Fatal("task command was not cancelled")
			}
			if !commandQueueIsIdle() {
				t.Fatal("cancelled task left unsent commands")
			}
		})
	}
}
func TestScriptTaskCompletionAndAfter(t *testing.T) {
	const owner = "task-complete-test"
	prepared := activateExtensionTest(t, owner, `package main
import("gt2";"time")
var job gt2.Task
var ticket gt2.CommandTicket
var once gt2.Timer
func Init(){
 job=gt2.StartTask(func(){ticket=gt2.QueueCommand("/pose sit");gt2.Wait(time.Millisecond);gt2.Store("done",true)})
 once=gt2.After(time.Millisecond,func(){gt2.Store("fired",gt2.LoadInteger("fired",0)+1)})
 stopped:=gt2.After(0,func(){gt2.Store("bad",true)});stopped.Stop()
}
`)
	waitExtensionTest(t, func() bool { return scriptStorageGet(owner, "done") == true && scriptStorageGet(owner, "fired") == 1 })
	(scriptEventSimulator{owner}).barrier(t)
	value, _ := prepared.interpreter.Eval("ticket")
	ticket := value.Interface().(CommandTicket)
	if ticket.Status().State != scriptapi.CommandQueued {
		t.Fatalf("successful task cancelled its command: %+v", ticket.Status())
	}
	value, _ = prepared.interpreter.Eval("job")
	if value.Interface().(Task).Active() {
		t.Fatal("finished task active")
	}
	value, _ = prepared.interpreter.Eval("once")
	if value.Interface().(Timer).Active() {
		t.Fatal("fired timer active")
	}
	if scriptStorageGet(owner, "bad") != nil {
		t.Fatal("stopped timer fired")
	}
}
func TestScriptPlayerTransitions(t *testing.T) {
	const owner = "player-events-test"
	resetScriptCallbackTestState(t, owner)
	var got []scriptapi.PlayerChangeEvent
	handle := registerScriptPlayerChange(owner, func(e scriptapi.PlayerChangeEvent) { got = append(got, e); e.Player.Colors[0] = 99 })
	t.Cleanup(func() { handle.release(); stopScriptEventQueue(owner) })
	before := []scriptapi.Player{{Name: "Pebble", Colors: []byte{1}}}
	after := []scriptapi.Player{{Name: "Pebble", Colors: []byte{2}, Dead: true, Offline: true, Sharing: true}}
	dispatchScriptPlayerChanges(before, after)
	(scriptEventSimulator{owner}).barrier(t)
	kinds := []string{}
	for _, e := range got {
		kinds = append(kinds, e.Type)
	}
	if !reflect.DeepEqual(kinds, []string{scriptapi.PlayerLogout, scriptapi.PlayerFallen, scriptapi.PlayerSharing}) {
		t.Fatalf("events: %v", kinds)
	}
	if after[0].Colors[0] != 2 {
		t.Fatal("event shared player data")
	}
	got = nil
	dispatchScriptPlayerChanges(after, before)
	(scriptEventSimulator{owner}).barrier(t)
	if got[0].Type != scriptapi.PlayerLogin || got[1].Type != scriptapi.PlayerRecovered {
		t.Fatalf("recovery events: %v", got)
	}
	handle.release()
	got = nil
	dispatchScriptPlayerChanges(before, after)
	(scriptEventSimulator{owner}).barrier(t)
	if len(got) != 0 {
		t.Fatal("removed listener fired")
	}
}
func TestScriptWindowRichControls(t *testing.T) {
	initFont()
	const owner = "rich-controls-test"
	prepared := activateExtensionTest(t, owner, `package main
import "gt2"
var panel gt2.Window
func changed(e gt2.WindowControlEvent){gt2.Store(e.ID,e.Text);gt2.Store("edits",gt2.LoadInteger("edits",0)+1)}
func colorChanged(e gt2.WindowControlEvent){gt2.Store("color",int(e.Color));gt2.Store("edits",gt2.LoadInteger("edits",0)+1)}
func Init(){panel=gt2.CreateWindow(gt2.WindowOptions{Title:"Controls",Controls:[]gt2.WindowControl{
 {ID:"name",Label:"Name",Kind:gt2.ControlText,Text:"Pebble",OnChange:changed},
 {ID:"enabled",Label:"Enabled",Kind:gt2.ControlCheckbox,OnChange:changed},
 {ID:"mode",Label:"Mode",Kind:gt2.ControlDropdown,Options:[]string{"One","Two"},OnChange:changed},
 {ID:"players",Label:"Players",Kind:gt2.ControlList,Options:[]string{"Pebble","Other"},OnChange:changed},
 {ID:"accent",Label:"Accent",Kind:gt2.ControlColor,Color:0x11223344,OnChange:colorChanged},
}})}
`)
	v, _ := prepared.interpreter.Eval("panel")
	panel := v.Interface().(Window)
	sim := scriptEventSimulator{owner}
	sim.barrier(t)
	controls := panel.state.controls
	oldPicker := colorPickerWin
	colorPickerWin = nil
	t.Cleanup(func() {
		if colorPickerWin != nil {
			colorPickerWin.Close()
		}
		colorPickerWin = oldPicker
	})
	controls["accent"].item.Handler.Emit(eui.UIEvent{Type: eui.EventClick})
	if colorPickerWin == nil || !controls["accent"].item.ColorSwatch {
		t.Fatal("color control did not open the existing color picker")
	}
	colorPickerWin.Close()
	controls["name"].item.Handler.Emit(eui.UIEvent{Type: eui.EventInputChanged, Text: "Edited"})
	controls["enabled"].item.Handler.Emit(eui.UIEvent{Type: eui.EventCheckboxChanged, Checked: true})
	controls["mode"].item.Handler.Emit(eui.UIEvent{Type: eui.EventDropdownSelected, Index: 1})
	oldRow := controls["players"].item.Contents[1]
	oldRow.Handler.Emit(eui.UIEvent{Type: eui.EventRadioSelected})
	panel.state.colorPicked(controls["accent"], 0xaabbccdd, controls["accent"].revision)
	sim.barrier(t)
	if scriptStorageGet(owner, "edits") != 5 || scriptStorageGet(owner, "players") != "Other" || scriptStorageGet(owner, "color") != int(0xaabbccdd) {
		t.Fatalf("control callbacks failed: edits=%v players=%v color=%v", scriptStorageGet(owner, "edits"), scriptStorageGet(owner, "players"), scriptStorageGet(owner, "color"))
	}
	panel.SetControlText("name", "Programmatic")
	panel.SetControlChecked("enabled", false)
	panel.SetControlSelected("mode", 0)
	panel.SetControlOptions("players", []string{"New"})
	panel.SetControlColor("accent", 0x102030ff)
	panel.SetControlEnabled("mode", false)
	sim.barrier(t)
	oldRow.Handler.Emit(eui.UIEvent{Type: eui.EventRadioSelected})
	controls["mode"].item.Handler.Emit(eui.UIEvent{Type: eui.EventDropdownSelected, Index: 1})
	sim.barrier(t)
	if scriptStorageGet(owner, "edits") != 5 {
		t.Fatal("programmatic/stale/disabled update fired callback")
	}
	if controls["name"].item.Text != "Programmatic" || controls["enabled"].item.Checked || controls["mode"].item.Selected != 0 || len(controls["players"].item.Contents) != 1 || packScriptWindowColor(controls["accent"].item.WheelColor) != 0x102030ff {
		t.Fatal("programmatic update failed")
	}
	panel.Remove()
	sim.barrier(t)
	controls["name"].item.Handler.Emit(eui.UIEvent{Type: eui.EventInputChanged, Text: "stale"})
	sim.barrier(t)
	if scriptStorageGet(owner, "edits") != 5 {
		t.Fatal("removed control fired")
	}
}

func TestScriptExtensionsDiscardReloadAndPermissions(t *testing.T) {
	const owner = "extension-lifecycle"
	resetScriptCallbackTestState(t, owner)
	clearCommands()
	t.Cleanup(func() { disablescript(owner, "test cleanup"); drainScriptDispatcher(); clearCommands() })
	src := []byte(`package main
import("gt2";"time")
var job gt2.Task
var timer gt2.Timer
var ticket gt2.CommandTicket
func Init(){
 if !gt2.HasPermission("timers") || gt2.HasPermission("unknown") {panic("permission query")}
 job=gt2.StartTask(func(){gt2.Store("started",true);gt2.Wait(time.Hour)})
 timer=gt2.After(time.Hour,func(){gt2.Store("bad",true)})
 ticket=gt2.QueueCommand("/pose sit")
}
`)
	discarded, err := prepareScriptSource(owner, src, restrictedStdlib())
	if err != nil {
		t.Fatal(err)
	}
	disposePreparedScript(discarded)
	drainScriptDispatcher()
	if !commandQueueIsIdle() || scriptStorageGet(owner, "started") != nil {
		t.Fatal("discarded validation ran actions")
	}
	prepared, err := prepareScriptSource(owner, src, restrictedStdlib())
	if err != nil {
		t.Fatal(err)
	}
	activatePreparedScript(owner, prepared)
	waitExtensionTest(t, func() bool { return scriptStorageGet(owner, "started") == true })
	var job Task
	var timer Timer
	var ticket CommandTicket
	// Read handles on the serialized callback queue while the task has yielded.
	queueScriptCallbackWait(owner, "inspect", func() {
		v, _ := prepared.interpreter.Eval("job")
		job = v.Interface().(Task)
		v, _ = prepared.interpreter.Eval("timer")
		timer = v.Interface().(Timer)
		v, _ = prepared.interpreter.Eval("ticket")
		ticket = v.Interface().(CommandTicket)
	})
	disablescript(owner, "reloaded")
	drainScriptDispatcher()
	if job.Active() || timer.Active() || ticket.Status().State != scriptapi.CommandCancelled {
		t.Fatalf("reload leaked handles: %v %v %+v", job.Active(), timer.Active(), ticket.Status())
	}
	if !commandQueueIsIdle() {
		t.Fatal("reload left commands")
	}
}

func TestScriptWindowControlValidation(t *testing.T) {
	for _, options := range []scriptapi.WindowOptions{
		{Controls: []scriptapi.WindowControl{{ID: "same", Label: "First", Kind: scriptapi.ControlText}, {ID: "same", Label: "Second", Kind: scriptapi.ControlCheckbox}}},
		{Controls: []scriptapi.WindowControl{{ID: "unknown", Label: "Unknown", Kind: "bogus"}}},
		{Controls: []scriptapi.WindowControl{{ID: "bad", Label: "Bad", Kind: scriptapi.ControlList, Options: []string{"one"}, Selected: 2}}},
		{Controls: []scriptapi.WindowControl{{ID: "", Label: "Empty", Kind: scriptapi.ControlText}}},
		{Rows: make([]scriptapi.WindowRow, 257)},
		{Controls: []scriptapi.WindowControl{{ID: "same", Label: "Top", Kind: scriptapi.ControlText}}, Rows: []scriptapi.WindowRow{{Controls: []scriptapi.WindowControl{{ID: "same", Label: "Row", Kind: scriptapi.ControlText}}}}},
		{Rows: []scriptapi.WindowRow{{Controls: []scriptapi.WindowControl{{ID: "bad", Label: "Bad", Kind: "unknown"}}}}},
	} {
		if validateScriptWindowOptions(options) == nil {
			t.Fatalf("accepted invalid controls: %+v", options)
		}
	}
}

func TestScriptWindowRowsCloneInputs(t *testing.T) {
	options := scriptapi.WindowOptions{Rows: []scriptapi.WindowRow{{
		Controls: []scriptapi.WindowControl{{ID: "choices", Label: "Choices", Kind: scriptapi.ControlList, Options: []string{"one"}, OptionImages: []uint16{22}}},
		Buttons:  []scriptapi.WindowButton{{ID: "delete", Label: "×", OnClick: func() {}}},
	}}}
	cloned := cloneScriptWindowOptions(options)
	options.Rows[0].Controls[0].Options[0] = "changed"
	options.Rows[0].Controls[0].OptionImages[0] = 71
	options.Rows[0].Buttons[0].Label = "changed"
	if cloned.Rows[0].Controls[0].Options[0] != "one" || cloned.Rows[0].Controls[0].OptionImages[0] != 22 || cloned.Rows[0].Buttons[0].Label != "×" {
		t.Fatal("row options retain mutable aliases")
	}
}

func TestScriptPlayerChangeBaselineAndDiscovery(t *testing.T) {
	const owner = "player-baseline-test"
	resetScriptCallbackTestState(t, owner)
	playersMu.Lock()
	original := players
	players = map[string]*Player{"Pebble": {Name: "Pebble"}}
	playersMu.Unlock()
	t.Cleanup(func() { disablescript(owner, "test cleanup"); playersMu.Lock(); players = original; playersMu.Unlock() })
	if _, observed := captureScriptPlayerChanges(); observed {
		t.Fatal("player snapshots without listeners")
	}
	pollScriptChangeEvents()
	var got []scriptapi.PlayerChangeEvent
	registerScriptPlayerChange(owner, func(e scriptapi.PlayerChangeEvent) { got = append(got, e) })
	pollScriptChangeEvents()
	(scriptEventSimulator{owner}).barrier(t)
	if len(got) != 0 {
		t.Fatal("initial player baseline emitted events")
	}
	playersMu.Lock()
	players["Other"] = &Player{Name: "Other", Dead: true}
	playersMu.Unlock()
	pollScriptChangeEvents()
	(scriptEventSimulator{owner}).barrier(t)
	if len(got) != 1 || got[0].Type != scriptapi.PlayerDiscovered || got[0].Player.Name != "Other" {
		t.Fatalf("discovery events: %+v", got)
	}
	got = nil
	playersMu.Lock()
	delete(players, "Other")
	playersMu.Unlock()
	pollScriptChangeEvents()
	(scriptEventSimulator{owner}).barrier(t)
	if len(got) != 1 || got[0].Type != scriptapi.PlayerRemoved || got[0].Previous.Name != "Other" || got[0].Player.Name != "" {
		t.Fatalf("removal events: %+v", got)
	}
}

func TestScriptExtendedHandleEditorMethods(t *testing.T) {
	// Native handles have private runtime state, but their public methods must
	// agree with the editor package in argument and result types.
	for _, pair := range [][2]any{
		{Task{}, scriptapi.Task{}}, {Timer{}, scriptapi.Timer{}}, {Storage{}, scriptapi.Storage{}},
		{CommandTicket{}, scriptapi.CommandTicket{}}, {Window{}, scriptapi.Window{}},
	} {
		runtimeType, editorType := reflect.TypeOf(pair[0]), reflect.TypeOf(pair[1])
		if runtimeType.NumMethod() != editorType.NumMethod() {
			t.Fatalf("%s method count differs", runtimeType.Name())
		}
		for i := 0; i < editorType.NumMethod(); i++ {
			want := editorType.Method(i)
			got, ok := runtimeType.MethodByName(want.Name)
			if !ok || got.Type.NumIn() != want.Type.NumIn() || got.Type.NumOut() != want.Type.NumOut() {
				t.Fatalf("%s.%s signature differs", runtimeType.Name(), want.Name)
			}
			for j := 1; j < want.Type.NumIn(); j++ {
				if got.Type.In(j) != want.Type.In(j) {
					t.Errorf("%s.%s argument %d differs", runtimeType.Name(), want.Name, j)
				}
			}
			for j := 0; j < want.Type.NumOut(); j++ {
				if got.Type.Out(j) != want.Type.Out(j) {
					t.Errorf("%s.%s result %d differs", runtimeType.Name(), want.Name, j)
				}
			}
		}
	}
}

func TestScriptCharacterStoreSavesAtLogout(t *testing.T) {
	const owner = "storage-logout-test"
	resetScriptCallbackTestState(t, owner)
	scriptSessionLogin("Pebble")
	t.Cleanup(func() {
		disablescript(owner, "test cleanup")
		scriptSessionMu.Lock()
		scriptSessionActive = false
		scriptSessionCharacter = ""
		scriptSessionMu.Unlock()
	})
	prepared, err := prepareScriptSource(owner, []byte(`package main
import "gt2"
var saved gt2.Storage
func Init(){saved=gt2.CharacterStore();gt2.OnLogout(func(e gt2.LifecycleEvent){saved.Store("logout",true)})}
func Terminate(){saved.Store("terminated",true)}
`), restrictedStdlib())
	if err != nil {
		t.Fatal(err)
	}
	activatePreparedScript(owner, prepared)
	value, _ := prepared.interpreter.Eval("saved")
	saved := value.Interface().(Storage)
	scriptSessionLogout("Pebble")
	(scriptEventSimulator{owner}).barrier(t)
	if scriptStorageGet(owner, saved.key("logout")) != true {
		t.Fatal("logout could not save character state")
	}
	disablescript(owner, "session ended")
	if scriptStorageGet(owner, saved.key("terminated")) != true || saved.Active() {
		t.Fatal("Terminate storage/lifetime failed")
	}
}
