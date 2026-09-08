package main

import (
	"fmt"
	scriptapi "gt2"
	"reflect"
	"testing"
)

func isolateScriptScopeSelection(t *testing.T) {
	t.Helper()
	oldName, oldPlayer, oldLast := name, playerName, gs.LastCharacter
	oldCharacters := characters
	oldMovie, oldCLMovie, oldPCAP, oldFake := playingMovie, clmov, pcapPath, fake
	scriptSessionMu.Lock()
	oldActive, oldSession := scriptSessionActive, scriptSessionCharacter
	scriptSessionActive = false
	scriptSessionCharacter = ""
	scriptSessionMu.Unlock()
	name, playerName, gs.LastCharacter = "", "Previous", "Previous"
	characters = []Character{{Name: "Alpha"}, {Name: "Beta"}}
	playingMovie, clmov, pcapPath, fake = false, "", "", false
	t.Cleanup(func() {
		name, playerName, gs.LastCharacter = oldName, oldPlayer, oldLast
		characters = oldCharacters
		playingMovie, clmov, pcapPath, fake = oldMovie, oldCLMovie, oldPCAP, oldFake
		scriptSessionMu.Lock()
		scriptSessionActive, scriptSessionCharacter = oldActive, oldSession
		scriptSessionMu.Unlock()
	})
}

func TestScriptScopeRequiresActualSelection(t *testing.T) {
	isolateScriptScopeSelection(t)
	for _, selected := range []string{"", freeDemoSelection, "Removed character"} {
		name = selected
		if got := scriptScopeCharacter(); got != "" {
			t.Fatalf("selection %q used stale character %q", selected, got)
		}
	}
	name = "Alpha"
	if got := scriptScopeCharacter(); got != "Alpha" {
		t.Fatalf("selected character=%q", got)
	}
	scriptSessionMu.Lock()
	scriptSessionActive = true
	scriptSessionCharacter = "Beta"
	scriptSessionMu.Unlock()
	if got := scriptScopeCharacter(); got != "Beta" {
		t.Fatalf("login selection replaced active player: %q", got)
	}
}

func TestScriptSessionsStartFreshForEveryScope(t *testing.T) {
	initFont()
	const owner = "scope-player"
	const global = "scope-all"
	resetScriptCallbackTestState(t, owner)
	isolateScriptScopeSelection(t)
	grantScriptPermissionsForTest(t, global)
	disablescript(owner, "reloaded")
	oldPackages := scriptPackages
	oldUI, oldProfile, oldProfiles := uiReady, activeCharacterProfile, characterProfiles
	oldSettings, oldBase, oldBaseReady := gs, globalSettingsBase, globalSettingsBaseReady
	oldDetails := scriptDetails
	scriptDetails = nil
	uiReady = true
	activeCharacterProfile = ""
	characterProfiles = characterProfilesDocument{}
	globalSettingsBaseReady = false
	source := func(id string) []byte {
		return []byte(fmt.Sprintf(`package main
import ("gt2"; "time")
var count int
var panel gt2.Window
var read = gt2.Self
var send = gt2.Send
func Init(){
 panel=gt2.CreateWindow(gt2.WindowOptions{Title:"%s"})
 gt2.Command("%s",func(args string){count++;gt2.Store("count",count)})
 gt2.OnLogin(func(e gt2.LifecycleEvent){gt2.Store("login",e.Character)})
 gt2.OnLogout(func(e gt2.LifecycleEvent){gt2.Store("logout",e.Character)})
 gt2.Repeat(time.Hour,func(){count++})
}
func Terminate(){gt2.Store("terminated",true)}
`, id, id))
	}
	scriptMu.Lock()
	scriptPackages = map[string]scriptInfo{
		owner:  {id: owner, name: owner, path: owner + ".go", src: source(owner)},
		global: {id: global, name: global, path: global + ".go", src: source(global)},
	}
	scriptDisplayNames = map[string]string{owner: owner, global: global}
	scriptDisabled = map[string]bool{owner: true, global: true}
	scriptEnabledFor = map[string]scriptScope{owner: {Chars: map[string]bool{"Alpha": true}}, global: {All: true}}
	scriptMu.Unlock()
	t.Cleanup(func() {
		disablescript(owner, "test cleanup")
		disablescript(global, "test cleanup")
		drainScriptDispatcher()
		scriptPackages = oldPackages
		uiReady, activeCharacterProfile, characterProfiles = oldUI, oldProfile, oldProfiles
		gs, globalSettingsBase, globalSettingsBaseReady = oldSettings, oldBase, oldBaseReady
		scriptDetails = oldDetails
	})
	applyEnabledScripts()
	if scriptIsRunning(owner) || scriptIsRunning(global) {
		t.Fatal("scripts ran without a session or player selection")
	}
	setscriptEnabled(owner, true, false)
	if scope := scriptEnabledFor[owner]; len(scope.Chars) != 1 || !scope.Chars["Alpha"] {
		t.Fatalf("no-selection enable changed stored scope: %+v", scope)
	}
	name = "Alpha"
	switchCharacterProfile("Alpha")
	if scriptIsRunning(owner) || scriptIsRunning(global) {
		t.Fatal("selecting a player started scripts before login")
	}
	first := startSessionScripts("Alpha")
	queues := map[string]*scriptEventQueue{}
	panels := map[string]Window{}
	for _, id := range []string{owner, global} {
		sim := scriptEventSimulator{owner: id}
		sim.command(t, id, "")
		if scriptStorageGet(id, "login") != "Alpha" || scriptStorageGet(id, "count") != 1 {
			t.Fatalf("%s did not receive login and fresh state", id)
		}
		queues[id] = currentScriptEventQueue(id)
		value, err := queues[id].interpreter.Eval("panel")
		if err != nil {
			t.Fatal(err)
		}
		panels[id] = value.Interface().(Window)
	}
	oldRead, err := queues[global].interpreter.Eval("read")
	if err != nil {
		t.Fatal(err)
	}
	oldSend, err := queues[global].interpreter.Eval("send")
	if err != nil {
		t.Fatal(err)
	}
	endSessionScripts(first)
	drainScriptDispatcher()
	for _, id := range []string{owner, global} {
		if scriptIsRunning(id) || panels[id].Active() || len(scriptRepeats[id]) != 0 || currentScriptEventQueue(id) != nil {
			t.Fatalf("%s left live resources at logout", id)
		}
		if scriptStorageGet(id, "logout") != "Alpha" || scriptStorageGet(id, "terminated") != true {
			t.Fatalf("%s did not finish logout/Terminate", id)
		}
	}
	applyEnabledScripts()
	if scriptIsRunning(owner) || scriptIsRunning(global) {
		t.Fatal("logout restarted scripts")
	}
	second := startSessionScripts("Alpha")
	for _, id := range []string{owner, global} {
		(scriptEventSimulator{owner: id}).command(t, id, "")
		if currentScriptEventQueue(id) == queues[id] || scriptStorageGet(id, "count") != 1 {
			t.Fatalf("%s reused interpreter or variables on same-player relogin", id)
		}
	}
	endSessionScripts(first)
	if !scriptIsRunning(owner) || !scriptIsRunning(global) {
		t.Fatal("stale logout killed a new session")
	}
	if got := oldRead.Call(nil)[0].Interface().(scriptapi.Character); got.Name != "" {
		t.Fatal("stale API read new session data")
	}
	oldSend.Call([]reflect.Value{reflect.ValueOf("stale-command")})
	drainScriptDispatcher()
	if len(getQueuedCommands()) != 0 {
		t.Fatal("stale API sent a command in the new session")
	}
	endSessionScripts(second)
	name = "Beta"
	third := startSessionScripts("Beta")
	if scriptIsRunning(owner) || !scriptIsRunning(global) {
		t.Fatal("next character did not respect saved enablement")
	}
	(scriptEventSimulator{owner: global}).command(t, global, "")
	if scriptStorageGet(global, "count") != 1 {
		t.Fatal("all-player script leaked state across players")
	}
	endSessionScripts(third)
	if scope := scriptEnabledFor[owner]; !scope.Chars["Alpha"] || scope.Chars["Beta"] {
		t.Fatalf("session change lost enablement: %+v", scope)
	}
}

func TestScriptSessionDiscardsDeferredFailure(t *testing.T) {
	for _, failure := range []string{"panic", "timeout"} {
		t.Run(failure, func(t *testing.T) {
			const owner = "deferred-failure"
			resetScriptCallbackTestState(t, owner)
			oldQueue := currentScriptEventQueue(owner)
			released := false
			registerScriptResource(owner, func() { released = true })
			if failure == "panic" {
				handleScriptCallbackPanic(owner, "old", "test", "")
			} else {
				handleScriptExecutionLimit(owner, "old", false)
			}
			stopScripts("session ended")
			if !released || currentScriptEventQueue(owner) != nil {
				t.Fatal("logout retained resources from a failed script")
			}
			scriptMu.Lock()
			scriptDisabled[owner] = false
			scriptMu.Unlock()
			replacement := startScriptEventQueue(owner)
			t.Cleanup(func() { disablescript(owner, "test cleanup") })
			drainScriptDispatcher()
			if !scriptIsRunning(owner) || currentScriptEventQueue(owner) != replacement {
				t.Fatal("old deferred failure stopped the replacement")
			}
			if runScriptCallbackOnQueue(oldQueue, owner, "stale", func() { panic("stale") }) {
				t.Fatal("old callback executed against the replacement")
			}
		})
	}
}
