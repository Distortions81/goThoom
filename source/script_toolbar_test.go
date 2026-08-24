package main

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestPackagedScriptToolbarLoadsPNGAndUsesExistingHotkeys(t *testing.T) {
	const owner = "toolbar-test"
	resetScriptCallbackTestState(t, owner)
	scriptDisabled[owner] = true
	scriptToolbarMu = sync.RWMutex{}
	scriptToolbars = map[string][]*scriptToolbarRegistration{}
	scriptToolbarNext = map[string]int{}
	originalToolbarRoot := toolbarRoot
	toolbarRoot = nil
	t.Cleanup(func() {
		toolbarRoot = originalToolbarRoot
		scriptToolbarMu = sync.RWMutex{}
		scriptToolbars = map[string][]*scriptToolbarRegistration{}
		scriptToolbarNext = map[string]int{}
	})

	scriptsDir := t.TempDir()
	packageDir := filepath.Join(scriptsDir, owner)
	if err := os.MkdirAll(filepath.Join(packageDir, "icons", "actions"), 0o755); err != nil {
		t.Fatal(err)
	}
	source := []byte(`package main
import "gt2"
func Init() {
	gt2.AddToolbar(gt2.ToolbarOptions{
		Label: "Actions",
		Buttons: []gt2.ToolbarButton{{
			Label: "Heal", Tooltip: "Heal now",
			Icon: "icons/actions/heal.png", Key: "F3",
			OnClick: func() { gt2.Store("clicked", true) },
		}},
	})
}
`)
	if err := os.WriteFile(filepath.Join(packageDir, "main.go"), source, 0o644); err != nil {
		t.Fatal(err)
	}
	iconFile, err := os.Create(filepath.Join(packageDir, "icons", "actions", "heal.png"))
	if err != nil {
		t.Fatal(err)
	}
	icon := image.NewRGBA(image.Rect(0, 0, 2, 2))
	icon.Set(0, 0, color.RGBA{R: 255, A: 255})
	if err := png.Encode(iconFile, icon); err != nil {
		t.Fatal(err)
	}
	if err := iconFile.Close(); err != nil {
		t.Fatal(err)
	}

	info, ok := scanscripts([]string{scriptsDir}, nil)[owner]
	if !ok || info.invalid {
		t.Fatalf("packaged toolbar script = %+v", info)
	}
	if !loadscriptPackageSource(owner, owner, info.path, info.src, restrictedStdlib(), info.assets, info.fingerprint) {
		t.Fatal("toolbar script did not load")
	}
	scriptToolbarMu.RLock()
	registration := scriptToolbars[owner][0]
	scriptToolbarMu.RUnlock()
	if registration == nil || len(registration.buttons) != 1 || registration.buttons[0].image == nil {
		t.Fatalf("toolbar registration = %+v", registration)
	}
	hotkey, ok := scriptGetHotkeyFn(owner, "F3")
	if !ok {
		t.Fatal("toolbar key was not registered as a script hotkey")
	}
	hotkey(makeScriptInputEvent("F3"))
	if got := scriptStorageGet(owner, "clicked"); got != true {
		t.Fatalf("toolbar hotkey callback stored %v", got)
	}

	disablescript(owner, "test complete")
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		scriptToolbarMu.RLock()
		remaining := len(scriptToolbars[owner])
		scriptToolbarMu.RUnlock()
		if remaining == 0 {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("toolbar remained after script stopped")
}

func TestLooseScriptCannotReferenceToolbarAssets(t *testing.T) {
	source := []byte(`package main
import "gt2"
func Init() {
	gt2.AddToolbar(gt2.ToolbarOptions{Buttons: []gt2.ToolbarButton{{
		Icon: "icon.png", OnClick: func() {},
	}}})
}
`)
	prepared, err := prepareScriptSource("loose-toolbar", source, restrictedStdlib())
	if err == nil {
		err = scriptCandidateConflict("loose-toolbar", prepared.candidate)
	}
	disposePreparedScript(prepared)
	if err == nil {
		t.Fatal("loose script toolbar icon was accepted")
	}
}
