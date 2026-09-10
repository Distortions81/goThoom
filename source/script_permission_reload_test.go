package main

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"gothoom/eui"
)

func TestScriptPermissionGrantUsesCurrentDiskSource(t *testing.T) {
	initFont()
	for _, timing := range []string{"reload", "before-review", "during-review"} {
		t.Run(timing, func(t *testing.T) {
			const owner = "permission-reload-disk"
			resetScriptCallbackTestState(t, owner)
			oldSettings, oldPackages := gs, scriptPackages
			gs.ScriptsPath = t.TempDir()
			path := filepath.Join(gs.ScriptsPath, "reload.go")
			scriptSessionLogin("Tester")
			scriptPermissionMu.Lock()
			scriptPermissionGrants[owner] = map[string]bool{"windows": true, "storage": true}
			scriptPermissionReviews[owner] = map[string]bool{"windows": true, "storage": true}
			scriptPermissionMu.Unlock()
			t.Cleanup(func() {
				disablescript(owner, "test cleanup")
				if scriptPermissionsWin != nil {
					scriptPermissionsWin.Close()
				}
				scriptPermissionRequests = nil
				drainScriptDispatcher()
				gs, scriptPackages = oldSettings, oldPackages
			})
			write := func(version string, hotkey bool) {
				t.Helper()
				binding := ""
				if hotkey {
					binding = `gt2.Bind("Alt-RightClick", func(e gt2.InputEvent) { gt2.Store("clicked", "` + version + `") })`
				}
				source := fmt.Sprintf(`package main
import "gt2"
const scriptID = "%s"
const scriptName = "Permission reload"
const scriptAPIVersion = 2
var panel gt2.Window
func Init() { panel = gt2.CreateWindow(gt2.WindowOptions{Title: "%s"}); gt2.Store("version", "%s"); %s }
`, owner, version, version, binding)
				if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			window := func() Window {
				t.Helper()
				value, err := currentScriptEventQueue(owner).interpreter.Eval("panel")
				if err != nil {
					t.Fatal(err)
				}
				return value.Interface().(Window)
			}
			grant := func() {
				t.Helper()
				if scriptPermissionsWin == nil {
					t.Fatal("permission dialog missing")
				}
				root := scriptPermissionsWin.Contents[0]
				buttons := root.Contents[len(root.Contents)-1]
				buttons.Contents[0].Handler.Emit(eui.UIEvent{Type: eui.EventClick})
				drainScriptDispatcher()
			}
			write("old", false)
			scriptPackages = scanscripts([]string{gs.ScriptsPath}, nil)
			enablescript(owner)
			drainScriptDispatcher()
			oldPanel := window()
			write("new", true)
			if timing == "reload" || timing == "during-review" {
				reloadscript(owner)
				drainScriptDispatcher()
			} else {
				// Enabling/reviewing from an older folder scan must read today's file.
				openScriptPermissionsWindow(owner)
			}
			if !oldPanel.Active() || !oldPanel.state.ui.IsOpen() {
				t.Fatal("pending review stopped the old version")
			}
			if timing == "during-review" {
				write("latest", true)
			}
			grant()
			if timing == "during-review" {
				// Source changed after the dialog opened: don't silently approve it.
				if scriptStorageGet(owner, "version") != "old" || scriptHasPermission(owner, "hotkeys") {
					t.Fatal("Grant approved a different source from the reviewed version")
				}
				openScriptPermissionsWindow(owner)
				grant()
			}
			want := "new"
			if timing == "during-review" {
				want = "latest"
			}
			if got := scriptStorageGet(owner, "version"); got != want {
				t.Fatalf("Grant loaded %v, want %s from disk", got, want)
			}
			if oldPanel.Active() || oldPanel.state.ui.IsOpen() || !window().state.ui.IsOpen() {
				t.Fatal("Grant did not replace the old window")
			}
			sim := scriptEventSimulator{owner: owner}
			sim.input(t, makeScriptInputEvent("Alt-RightClick"))
			sim.barrier(t)
			if got := scriptStorageGet(owner, "clicked"); got != want {
				t.Fatalf("Grant left an old or missing input callback: %v", got)
			}
		})
	}
}
