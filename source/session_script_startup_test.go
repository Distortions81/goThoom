package main

import (
	"crypto/sha256"
	"net"
	"testing"
	"time"
)

func installSessionScriptPackageForTest(t *testing.T, owner, character, marker string) {
	t.Helper()
	source := []byte("package main\nvar Marker = \"" + marker + "\"\nfunc Init(){}\n")
	grantScriptPermissionsForTest(t, owner)
	scriptMu.Lock()
	oldPackage, hadPackage := scriptPackages[owner]
	oldName, hadName := scriptDisplayNames[owner]
	oldScope, hadScope := scriptEnabledFor[owner]
	oldInvalid, hadInvalid := scriptInvalid[owner]
	scriptPackages[owner] = scriptInfo{id: owner, name: owner, src: source, fingerprint: sha256.Sum256(source)}
	scriptDisplayNames[owner] = owner
	if character == "" {
		scriptEnabledFor[owner] = scriptScope{All: true}
	} else {
		scriptEnabledFor[owner] = scriptScope{Chars: map[string]bool{character: true}}
	}
	delete(scriptInvalid, owner)
	scriptMu.Unlock()
	t.Cleanup(func() {
		scriptMu.Lock()
		if hadPackage {
			scriptPackages[owner] = oldPackage
		} else {
			delete(scriptPackages, owner)
		}
		if hadName {
			scriptDisplayNames[owner] = oldName
		} else {
			delete(scriptDisplayNames, owner)
		}
		if hadScope {
			scriptEnabledFor[owner] = oldScope
		} else {
			delete(scriptEnabledFor, owner)
		}
		if hadInvalid {
			scriptInvalid[owner] = oldInvalid
		} else {
			delete(scriptInvalid, owner)
		}
		scriptMu.Unlock()
	})
}

func TestSyncEnabledSessionScriptsUsesGlobalAndCharacterScopes(t *testing.T) {
	const globalOwner = "session-sync-global"
	const alphaOwner = "session-sync-alpha"
	const betaOwner = "session-sync-beta"
	installSessionScriptPackageForTest(t, globalOwner, "", "global-v1")
	installSessionScriptPackageForTest(t, alphaOwner, "Alpha", "alpha-v1")
	installSessionScriptPackageForTest(t, betaOwner, "Beta", "beta-v1")

	session := mustNewSession(2)
	session.setCharacterName("Alpha")
	t.Cleanup(func() { session.automation.stopSessionScripts("test cleanup") })
	if errs := session.syncEnabledSessionScripts("Alpha"); len(errs) != 0 {
		t.Fatalf("sync enabled scripts: %v", errs)
	}
	globalQueue := currentSessionScriptEventQueue(session, globalOwner)
	if globalQueue == nil || currentSessionScriptEventQueue(session, alphaOwner) == nil {
		t.Fatal("global or matching character script did not start")
	}
	if currentSessionScriptEventQueue(session, betaOwner) != nil {
		t.Fatal("script for another character started")
	}

	updated := []byte("package main\nvar Marker = \"global-v2\"\nfunc Init(){}\n")
	scriptMu.Lock()
	info := scriptPackages[globalOwner]
	info.src = updated
	info.fingerprint = sha256.Sum256(updated)
	scriptPackages[globalOwner] = info
	scriptEnabledFor[alphaOwner] = scriptScope{Chars: map[string]bool{"Beta": true}}
	scriptMu.Unlock()
	if errs := session.syncEnabledSessionScripts("Alpha"); len(errs) != 0 {
		t.Fatalf("resync enabled scripts: %v", errs)
	}
	if currentSessionScriptEventQueue(session, globalOwner) == globalQueue {
		t.Fatal("changed script source did not replace the session interpreter")
	}
	if currentSessionScriptEventQueue(session, alphaOwner) != nil {
		t.Fatal("script disabled for this character kept running")
	}
	queue := currentSessionScriptEventQueue(session, globalOwner)
	value, err := queue.interpreter.Eval("Marker")
	if err != nil || value.String() != "global-v2" {
		t.Fatalf("reloaded marker = %q, %v", value.String(), err)
	}
}

func TestSecondarySessionScriptMovementIsSessionOwned(t *testing.T) {
	const owner = "secondary-session-movement"
	grantScriptPermissionsForTest(t, owner)
	session := mustNewSession(2)
	session.setCharacterName("Mover")
	tcp, tcpPeer := net.Pipe()
	udp, udpPeer := net.Pipe()
	if _, ok := session.transport.attach(tcp, udp); !ok {
		t.Fatal("attach session transport")
	}
	t.Cleanup(func() {
		session.transport.disconnect()
		_ = tcpPeer.Close()
		_ = udpPeer.Close()
	})
	now := time.Now()
	session.draw.mu.Lock()
	session.draw.current.receivedAt = now
	session.draw.mu.Unlock()
	source := []byte(`package main
import "gt2"
var Moved bool
func Init(){gt2.OnLogin(func(_ gt2.LifecycleEvent){Moved=gt2.Move(32000,-32000)})}
`)
	if err := session.startSessionScript(owner, source, restrictedStdlib(), nil); err != nil {
		t.Fatalf("start movement script: %v", err)
	}
	t.Cleanup(func() { session.stopSessionScript(owner, "test cleanup") })
	queue := currentSessionScriptEventQueue(session, owner)
	value, err := queue.interpreter.Eval("Moved")
	if err != nil || !value.Bool() {
		t.Fatalf("session movement result = %v, %v", value, err)
	}
	input := applyScriptMovementForSession(session, inputState{}, now)
	if !input.mouseDown || input.mouseX != int16(fieldCenterX) || input.mouseY != -int16(fieldCenterY) {
		t.Fatalf("secondary movement was not routed to its session: %+v", input)
	}
	if applyScriptMovement(inputState{}, now).mouseDown {
		t.Fatal("secondary movement crossed into the primary session")
	}
	session.stopSessionScript(owner, "test stop")
	if applyScriptMovementForSession(session, inputState{}, now).mouseDown {
		t.Fatal("stopped secondary script retained movement")
	}
}

func TestSecondarySessionScriptRegistrationsIgnorePrimaryRuntimeState(t *testing.T) {
	const owner = "secondary-session-registrations"
	grantScriptPermissionsForTest(t, owner)
	scriptMu.Lock()
	oldDisabled, hadDisabled := scriptDisabled[owner]
	scriptDisabled[owner] = true
	scriptMu.Unlock()
	shortcutMu.Lock()
	oldShortcuts, hadShortcuts := shortcutMaps[owner]
	oldShortcutRegistration, hadShortcutRegistration := shortcutRegistrations[owner]
	oldSessionShortcutRegistrations, hadSessionShortcutRegistrations := shortcutSessionRegistrations[owner]
	delete(shortcutMaps, owner)
	delete(shortcutRegistrations, owner)
	delete(shortcutSessionRegistrations, owner)
	shortcutMu.Unlock()
	t.Cleanup(func() {
		scriptMu.Lock()
		if hadDisabled {
			scriptDisabled[owner] = oldDisabled
		} else {
			delete(scriptDisabled, owner)
		}
		scriptMu.Unlock()
		shortcutMu.Lock()
		if hadShortcuts {
			shortcutMaps[owner] = oldShortcuts
		} else {
			delete(shortcutMaps, owner)
		}
		if hadShortcutRegistration {
			shortcutRegistrations[owner] = oldShortcutRegistration
		} else {
			delete(shortcutRegistrations, owner)
		}
		if hadSessionShortcutRegistrations {
			shortcutSessionRegistrations[owner] = oldSessionShortcutRegistrations
		} else {
			delete(shortcutSessionRegistrations, owner)
		}
		shortcutMu.Unlock()
	})
	session := mustNewSession(2)
	source := []byte(`package main
import "gt2"
func Init(){
	gt2.Command("local-session",func(string){})
	gt2.Bind("F8",func(gt2.InputEvent){})
	gt2.AddToolbar(gt2.ToolbarOptions{Buttons:[]gt2.ToolbarButton{{Label:"Local",OnClick:func(){}}}})
	gt2.AddShortcut("ss","/say ")
}
`)
	if err := session.startSessionScript(owner, source, restrictedStdlib(), nil); err != nil {
		t.Fatalf("start registration script: %v", err)
	}
	t.Cleanup(func() { session.stopSessionScript(owner, "test cleanup") })
	if _, ok := session.sessionScriptCommand("local-session"); !ok {
		t.Fatal("secondary command depended on primary enabled state")
	}
	if _, ok, _ := session.sessionScriptHotkey("F8"); !ok {
		t.Fatal("secondary hotkey depended on primary enabled state")
	}
	session.automation.scriptMu.RLock()
	toolbars := len(session.automation.localToolbars[owner])
	session.automation.scriptMu.RUnlock()
	if toolbars != 1 {
		t.Fatalf("secondary toolbar registrations = %d, want 1", toolbars)
	}
	if got := expandShortcut("ss hello"); got != "/say hello" {
		t.Fatalf("secondary shortcut expansion = %q", got)
	}
	session.stopSessionScript(owner, "test stop")
	if got := expandShortcut("ss hello"); got != "ss hello" {
		t.Fatalf("stopped secondary shortcut remained registered: %q", got)
	}
}

func TestSecondarySessionScriptWindowAndPrintUseOwningSession(t *testing.T) {
	initFont()
	const owner = "secondary-session-window-print"
	grantScriptPermissionsForTest(t, owner)
	session := mustNewSession(2)
	source := []byte(`package main
import "gt2"
var Panel gt2.Window
func Init(){
	gt2.Print("secondary output")
	Panel=gt2.CreateWindow(gt2.WindowOptions{Title:"Secondary"})
}
`)
	if err := session.startSessionScript(owner, source, restrictedStdlib(), nil); err != nil {
		t.Fatalf("start window script: %v", err)
	}
	queue := currentSessionScriptEventQueue(session, owner)
	value, err := queue.interpreter.Eval("Panel")
	if err != nil {
		t.Fatalf("read window handle: %v", err)
	}
	panel := value.Interface().(Window)
	if !panel.Active() || panel.state.handle.queue != queue {
		t.Fatal("secondary script window was not owned by its session queue")
	}
	found := false
	for _, event := range session.events.log.snapshot() {
		if event.Kind == sessionEventConsole && event.Text == "secondary output" {
			found = true
		}
	}
	if !found {
		t.Fatal("secondary Print output was not recorded on its session")
	}
	session.automation.scriptMu.RLock()
	runtime := session.automation.scripts[owner]
	session.automation.scriptMu.RUnlock()
	dispatched := false
	runtime.prepared.candidate.dispatch(owner, func() { dispatched = true })
	if dispatched {
		t.Fatal("active session client action ran outside the main-thread dispatcher")
	}
	drainMainThreadDispatcher()
	if !dispatched {
		t.Fatal("active session client action did not reach the main thread")
	}
	staleDispatched := false
	runtime.prepared.candidate.dispatch(owner, func() { staleDispatched = true })
	session.stopSessionScript(owner, "test stop")
	drainMainThreadDispatcher()
	if staleDispatched {
		t.Fatal("stopped session ran a queued client action")
	}
	if panel.Active() {
		t.Fatal("secondary script window survived its owning runtime")
	}
}

func TestSecondarySessionScriptPanicDoesNotDisableOtherRuntimes(t *testing.T) {
	const owner = "secondary-session-panic"
	grantScriptPermissionsForTest(t, owner)
	scriptMu.Lock()
	oldDisabled, hadDisabled := scriptDisabled[owner]
	oldScope, hadScope := scriptEnabledFor[owner]
	scriptDisabled[owner] = false
	scriptEnabledFor[owner] = scriptScope{All: true}
	scriptMu.Unlock()
	t.Cleanup(func() {
		scriptMu.Lock()
		if hadDisabled {
			scriptDisabled[owner] = oldDisabled
		} else {
			delete(scriptDisabled, owner)
		}
		if hadScope {
			scriptEnabledFor[owner] = oldScope
		} else {
			delete(scriptEnabledFor, owner)
		}
		scriptMu.Unlock()
	})
	session := mustNewSession(2)
	source := []byte(`package main
import "gt2"
func Init(){gt2.Command("panic-here",func(string){panic("session failure")})}
`)
	if err := session.startSessionScript(owner, source, restrictedStdlib(), nil); err != nil {
		t.Fatalf("start panic script: %v", err)
	}
	command, ok := session.sessionScriptCommand("panic-here")
	if !ok {
		t.Fatal("panic command not registered")
	}
	command.handler("")
	deadline := time.Now().Add(time.Second)
	for currentSessionScriptEventQueue(session, owner) != nil && time.Now().Before(deadline) {
		drainScriptDispatcher()
		time.Sleep(time.Millisecond)
	}
	if currentSessionScriptEventQueue(session, owner) != nil {
		t.Fatal("panicking secondary script did not stop")
	}
	scriptMu.RLock()
	disabled := scriptDisabled[owner]
	scope := scriptEnabledFor[owner]
	scriptMu.RUnlock()
	if disabled || !scope.All {
		t.Fatal("secondary panic disabled the shared script selection")
	}
}
