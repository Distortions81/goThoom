package main

import (
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"gothoom/eui"
)

func grantScriptPermissionsForTest(t *testing.T, owner string) {
	t.Helper()
	scriptPermissionMu.Lock()
	old := scriptPermissionGrants[owner]
	oldReview := scriptPermissionReviews[owner]
	grants := map[string]bool{}
	for _, permission := range scriptPermissionCatalog {
		grants[permission.ID] = true
	}
	scriptPermissionGrants[owner] = grants
	scriptPermissionReviews[owner] = grants
	scriptPermissionMu.Unlock()
	t.Cleanup(func() {
		scriptPermissionMu.Lock()
		if oldReview == nil {
			delete(scriptPermissionReviews, owner)
		} else {
			scriptPermissionReviews[owner] = oldReview
		}
		if old == nil {
			delete(scriptPermissionGrants, owner)
		} else {
			scriptPermissionGrants[owner] = old
		}
		scriptPermissionMu.Unlock()
	})
}

func TestScriptPermissionDiscovery(t *testing.T) {
	for _, test := range []struct {
		source string
		want   map[string]bool
	}{
		{`package main; import "gt2"; func Init(){gt2.Command("hi",func(args string){gt2.Send(args)});gt2.Bind("F1",func(e gt2.InputEvent){});gt2.Print("gt2.Move")}`, map[string]bool{"commands": true, "hotkeys": true, "send": true}},
		{`package main; import api "gt2"; var move=api.Move; func Init(){api.OnWorld(func(w api.World){});api.CreateWindow(api.WindowOptions{});api.OnChat(api.ChatFilter{},func(e api.ChatEvent){})}`, map[string]bool{"data": true, "movement": true, "windows": true, "messages": true}},
		{`package main; import . "gt2"; var read=LoadString; func Init(){OnLogin(func(e LifecycleEvent){});Repeat(1,func(){})}`, map[string]bool{"storage": true, "session": true, "timers": true}},
	} {
		got, err := scriptRequiredPermissions([]byte(test.source))
		if err != nil || !reflect.DeepEqual(got, test.want) {
			t.Fatalf("permissions = %v, %v; want %v", got, err, test.want)
		}
	}
}

func TestScriptPermissionExportsAreClassified(t *testing.T) {
	for name, value := range exportsForscript("permission-contract")["gt2/gt2"] {
		if value.Kind() == reflect.Func {
			permission, ok := scriptFunctionPermissions[name]
			if !ok {
				t.Errorf("unclassified exported function %s", name)
			}
			if permission != "" {
				found := false
				for _, known := range scriptPermissionCatalog {
					found = found || permission == known.ID
				}
				if !found {
					t.Errorf("unknown permission %s for %s", permission, name)
				}
			}
		}
	}
}

func TestScriptPermissionsDenyBeforeInitialization(t *testing.T) {
	const owner = "permission-denied"
	source := []byte(`package main; import "gt2"; var self=gt2.Self(); func Init(){gt2.Send("should not run")}`)
	prepared, err := prepareScriptSource(owner, source, restrictedStdlib())
	if prepared != nil || err == nil || !strings.Contains(err.Error(), "permissions required:") {
		t.Fatalf("prepare = %v, %v", prepared, err)
	}
	// Basic command and hotkey scripts need review too.
	_, err = prepareScriptSource(owner, []byte(`package main; import "gt2"; func Init(){gt2.Command("permission-basic",func(args string){gt2.Send(args)});gt2.Bind("F12",func(e gt2.InputEvent){})}`), restrictedStdlib())
	if err == nil {
		t.Fatal("command/hotkey script ran before review")
	}

}

func TestScriptPermissionRuntimeGuard(t *testing.T) {
	const owner = "permission-runtime"
	grantScriptPermissionsForTest(t, owner)
	exports := exportsForscript(owner)["gt2/gt2"]
	// A captured function must recheck grants, including after a revoke.
	read := exports["CurrentWorld"]
	scriptPermissionMu.Lock()
	delete(scriptPermissionGrants[owner], "data")
	scriptPermissionMu.Unlock()
	result := read.Call(nil)
	if !result[0].IsZero() {
		t.Fatal("captured read bypassed revoked permission")
	}

}

func TestScriptPermissionPersistence(t *testing.T) {
	const owner = "permission-persist"
	oldDir := dataDirPath
	dataDirPath = t.TempDir()
	t.Cleanup(func() { dataDirPath = oldDir })
	scriptPermissionMu.Lock()
	old := scriptPermissionGrants
	oldReviews := scriptPermissionReviews
	scriptPermissionGrants = map[string]map[string]bool{}
	scriptPermissionReviews = map[string]map[string]bool{}
	scriptPermissionMu.Unlock()
	t.Cleanup(func() {
		scriptPermissionMu.Lock()
		scriptPermissionGrants = old
		scriptPermissionReviews = oldReviews
		scriptPermissionMu.Unlock()
	})
	if err := saveScriptPermissions(map[string]map[string]bool{owner: {"movement": true, "windows": true, "unknown": true}}, map[string]map[string]bool{owner: {"movement": true, "windows": true, "storage": true}, "empty-review": {}}); err != nil {
		t.Fatal(err)
	}
	loadScriptPermissions()
	if !scriptPermissionsReviewed("empty-review") || scriptHasPermission(owner, "storage") {
		t.Fatal("empty review or denied decision did not persist")
	}
	if !scriptHasPermission(owner, "movement") || scriptHasPermission(owner, "data") || scriptHasPermission("other", "movement") || scriptHasPermission(owner, "unknown") {
		t.Fatal("grants were not isolated or default denied")
	}
	if err := os.WriteFile(scriptPermissionsPath(), []byte(`{broken`), 0600); err != nil {
		t.Fatal(err)
	}
	loadScriptPermissions()
	if scriptHasPermission(owner, "movement") {
		t.Fatal("malformed grants did not fail closed")
	}
}

func TestScriptPermissionDialogAndRevocation(t *testing.T) {
	initFont()
	isolateScriptWorld(t)
	const owner = "permission-follow"
	sim := activateBundledProofScript(t, owner, "follow_player.go")
	source, err := scriptScripts.ReadFile(bundledScriptDir + "/follow_player.go")
	if err != nil {
		t.Fatal(err)
	}
	scriptMu.Lock()
	previousPackages := scriptPackages
	scriptPackages = map[string]scriptInfo{owner: {id: owner, name: "Follow Player", path: "follow_player.go", src: source}}
	scriptMu.Unlock()
	t.Cleanup(func() {
		if scriptPermissionsWin != nil {
			scriptPermissionsWin.Close()
			scriptPermissionsWin = nil
		}
		scriptMu.Lock()
		scriptPackages = previousPackages
		scriptMu.Unlock()
	})
	value, err := currentScriptEventQueue(owner).interpreter.Eval("followWindow")
	if err != nil {
		t.Fatal(err)
	}
	panel := value.Interface().(Window)
	// Give the script a live movement lease; revoking must release it immediately.
	scriptSessionLogin("Hero")
	stateMu.Lock()
	state.receivedAt = time.Now()
	stateMu.Unlock()
	if !scriptMove(owner, 100, 0, time.Now()) {
		t.Fatal("movement setup failed")
	}
	sim.barrier(t)
	openScriptPermissionsWindow(owner)
	root := scriptPermissionsWin.Contents[0]
	var movement, unused *eui.ItemData
	for _, item := range root.Contents {
		if item.Text == "Automatic movement" {
			movement = item
		}
		if item.Text == "Chat and server messages" {
			unused = item
		}
	}
	if movement == nil || movement.Disabled || !movement.Checked {
		t.Fatal("used permission not editable and checked")
	}
	if unused == nil || !unused.Disabled || unused.Checked {
		t.Fatal("unused permission not greyed out")
	}
	movement.Handler.Handle(eui.UIEvent{Type: eui.EventCheckboxChanged, Checked: false})
	buttons := root.Contents[len(root.Contents)-1]
	buttons.Contents[0].Handler.Handle(eui.UIEvent{Type: eui.EventClick})
	drainScriptDispatcher()
	if scriptHasPermission(owner, "movement") || !scriptIsRunning(owner) || panel.Active() || panel.state.ui.IsOpen() || scriptMovementSnapshot(owner, time.Now()).Active {
		t.Fatal("revocation did not restart the script and release its old window/movement")
	}
	scriptMu.RLock()
	scope := scriptEnabledFor[owner]
	message := scriptErrors[owner]
	scriptMu.RUnlock()
	if !scope.All || message != "" {
		t.Fatalf("pending script lost enablement or error: %+v %s", scope, message)
	}
	required, err := scriptRequiredPermissions(source)
	if err != nil {
		t.Fatal(err)
	}
	if err := setScriptPermissions(owner, required); err != nil {
		t.Fatal(err)
	}
	if !scriptIsRunning(owner) {
		t.Fatal("granting missing permission did not restart selected script")
	}
	if panel.Active() {
		t.Fatal("restart revived an old window")
	}
}

func TestScriptPermissionsUpdatedSourceNeedsNewGrant(t *testing.T) {
	const owner = "permission-update"
	resetScriptCallbackTestState(t, owner)
	scriptPermissionMu.Lock()
	scriptPermissionGrants[owner] = map[string]bool{"commands": true}
	scriptPermissionReviews[owner] = map[string]bool{"commands": true}
	scriptPermissionMu.Unlock()
	t.Cleanup(func() { disablescript(owner, "test cleanup"); drainScriptDispatcher() })
	original := []byte(`package main; import "gt2"; func Init(){gt2.Command("permission-update",func(args string){gt2.Print(args)})}`)
	if !loadscriptSource(owner, owner, "permission.go", original, restrictedStdlib()) {
		t.Fatal("basic script did not start")
	}
	queue := currentScriptEventQueue(owner)
	update := []byte(`package main; import "gt2"; func Init(){gt2.Command("permission-update",func(args string){gt2.Move(100,0)})}`)
	if loadscriptSource(owner, owner, "permission.go", update, restrictedStdlib()) {
		t.Fatal("updated script gained movement without consent")
	}
	if currentScriptEventQueue(owner) != queue || !scriptIsRunning(owner) {
		t.Fatal("denied update replaced previous working script")
	}
}

func TestScriptPermissionsFirstEnableGrantAndBlock(t *testing.T) {
	initFont()
	const owner = "permission-first-enable"
	resetScriptCallbackTestState(t, owner)
	disablescript(owner, "reloaded")
	scriptPermissionMu.Lock()
	delete(scriptPermissionGrants, owner)
	delete(scriptPermissionReviews, owner)
	scriptPermissionMu.Unlock()
	source := []byte(`package main; import "gt2"
 var first=sendOnLoad()
 func sendOnLoad() bool {gt2.Send("global-send");return true}
 func Init(){gt2.Command("permission-first",func(args string){gt2.Send(args)})}`)
	scriptMu.Lock()
	previousPackages := scriptPackages
	scriptPackages = map[string]scriptInfo{owner: {id: owner, name: "First review", path: "first.go", src: source}}
	scriptMu.Unlock()
	t.Cleanup(func() {
		disablescript(owner, "test cleanup")
		if scriptPermissionsWin != nil {
			scriptPermissionsWin.Close()
		}
		scriptPermissionRequests = nil
		drainScriptDispatcher()
		scriptMu.Lock()
		scriptPackages = previousPackages
		scriptMu.Unlock()
		clearCommands()
	})
	clearCommands()
	setscriptEnabled(owner, false, true)
	if scriptIsRunning(owner) {
		t.Fatal("unreviewed script started")
	}
	if len(getQueuedCommands()) != 0 {
		t.Fatal("globals ran before review")
	}
	drainScriptDispatcher()
	if scriptPermissionsWin == nil || scriptPermissionsOwner != owner {
		t.Fatal("first enable did not open review")
	}
	root := scriptPermissionsWin.Contents[0]
	used := 0
	for _, item := range root.Contents {
		if item.ItemType != eui.ITEM_CHECKBOX {
			continue
		}
		if item.Disabled {
			if item.Checked {
				t.Fatal("unused permission checked")
			}
			continue
		}
		used++
		if !item.Checked {
			t.Fatal("requested permission not checked by default")
		}
		if item.Text == "Send server commands" {
			item.Handler.Handle(eui.UIEvent{Type: eui.EventCheckboxChanged, Checked: false})
		}
	}
	if used != 2 {
		t.Fatalf("requested permissions = %d", used)
	}
	buttons := root.Contents[len(root.Contents)-1]
	if buttons.Contents[0].Text != "Grant" || buttons.Contents[1].Text != "Block all" {
		t.Fatal("missing grant/block actions")
	}
	buttons.Contents[0].Handler.Handle(eui.UIEvent{Type: eui.EventClick})
	drainScriptDispatcher()
	if scriptIsRunning(owner) || !scriptHasPermission(owner, "commands") || scriptHasPermission(owner, "send") {
		t.Fatal("offline grant did not preserve selected access without running")
	}
	session := startSessionScripts("Hero")
	t.Cleanup(func() { endSessionScripts(session) })
	if !scriptIsRunning(owner) {
		t.Fatal("reviewed script did not start at login")
	}
	sim := scriptEventSimulator{owner: owner}
	sim.command(t, "permission-first", "denied-send")
	if len(getQueuedCommands()) != 0 {
		t.Fatal("denied Send executed from globals or command")
	}
	openScriptPermissionsWindow(owner)
	root = scriptPermissionsWin.Contents[0]
	buttons = root.Contents[len(root.Contents)-1]
	buttons.Contents[1].Handler.Handle(eui.UIEvent{Type: eui.EventClick})
	drainScriptDispatcher()
	if scriptIsRunning(owner) || scriptHasPermission(owner, "commands") || !scriptPermissionsReviewed(owner) {
		t.Fatal("Block all did not persist denial and stop script")
	}
	scriptMu.RLock()
	scope := scriptEnabledFor[owner]
	scriptMu.RUnlock()
	if !scope.empty() {
		t.Fatal("Block all left script enabled")
	}
}
