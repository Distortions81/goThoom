//go:build integration

package main

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestDuplicateCommandsAndBindingsAreRejectedBeforeActivation(t *testing.T) {
	withoutConsoleTimestamps(t)
	origDataDir := dataDirPath
	origPlayerName := playerName
	origSettings := gs
	dataDirPath = t.TempDir()
	playerName = "Tester"
	t.Cleanup(func() {
		dataDirPath = origDataDir
		playerName = origPlayerName
		gs = origSettings
	})

	scriptMu = sync.RWMutex{}
	scriptNames = map[string]bool{}
	scriptDisplayNames = map[string]string{}
	scriptAuthors = map[string]string{}
	scriptCategories = map[string]string{}
	scriptSubCategories = map[string]string{}
	scriptInvalid = map[string]bool{}
	scriptDisabled = map[string]bool{}
	scriptEnabledFor = map[string]scriptScope{}
	scriptPaths = map[string]string{}
	scriptTerminators = map[string]func(){}
	scriptCommands = map[string]scriptCommandHandler{}
	scriptCommandOwners = map[string]string{}
	scriptSendHistory = map[string][]time.Time{}
	scriptRepeats = map[string][]*scriptRepeatRegistration{}
	scriptTickWaiters = map[string][]*tickWaiter{}
	hotkeysMu = sync.RWMutex{}
	hotkeys = nil
	scriptHotkeyMu = sync.RWMutex{}
	scriptHotkeyEnabled = map[string]map[string]bool{}
	scriptHotkeyFnMu = sync.RWMutex{}
	scriptHotkeyFns = map[string]map[string]func(InputEvent) bool{}
	scriptStoreMu = sync.Mutex{}
	scriptStores = map[string]*scriptStore{}
	consoleLog = messageLog{max: maxMessages}

	dir := t.TempDir()
	writeScript := func(file, name, initBody string) {
		t.Helper()
		src := `package main
import "gt2"
const scriptName = "` + name + `"
const scriptAuthor = "Test"
const scriptCategory = "Tests"
const scriptAPIVersion = 2
func Init() {
` + initBody + `
}
`
		if err := os.WriteFile(filepath.Join(dir, file), []byte(src), 0o644); err != nil {
			t.Fatalf("write %s: %v", file, err)
		}
	}

	writeScript("gamma.go", "Gamma", `
	gt2.Bind("Shift-Ctrl-F3", func(gt2.InputEvent) {})
	gt2.Store("started", "gamma")`)
	writeScript("beta.go", "Beta", `
	gt2.Command("shared", func(string) {})
	gt2.Store("started", "beta")`)
	writeScript("alpha.go", "Alpha", `
	gt2.Command("shared", func(string) {})
	gt2.Bind("Control-Shift-F3", func(gt2.InputEvent) {})
	gt2.Store("started", "alpha")`)

	for _, owner := range []string{"gamma", "beta", "alpha"} {
		scriptEnabledFor[owner] = scriptScope{All: true}
	}
	rescanScripts([]string{dir})

	if got := scriptCommandOwners["shared"]; got != "alpha" {
		t.Fatalf("deterministic command owner = %q, want alpha", got)
	}
	if !scriptIsRunning("alpha") || scriptIsRunning("beta") || scriptIsRunning("gamma") {
		t.Fatalf("running states: alpha=%v beta=%v gamma=%v", scriptIsRunning("alpha"), scriptIsRunning("beta"), scriptIsRunning("gamma"))
	}
	if got := scriptStorageGet("alpha", "started"); got != "alpha" {
		t.Fatalf("winning script did not activate: %v", got)
	}
	if got := scriptStorageGet("beta", "started"); got != nil {
		t.Fatalf("duplicate-command script activated staged storage: %v", got)
	}
	if got := scriptStorageGet("gamma", "started"); got != nil {
		t.Fatalf("duplicate-binding script activated staged storage: %v", got)
	}
	hotkeysMu.RLock()
	if len(hotkeys) != 1 || hotkeys[0].Script != "alpha" {
		hotkeysMu.RUnlock()
		t.Fatalf("binding winner was not deterministic: %+v", hotkeys)
	}
	hotkeysMu.RUnlock()

	messages := strings.Join(getConsoleMessages(), "\n")
	if !strings.Contains(messages, "duplicate command /shared already owned by alpha") {
		t.Fatalf("missing command conflict report: %s", messages)
	}
	if !strings.Contains(messages, "duplicate binding Shift-Ctrl-F3 already owned by alpha") {
		t.Fatalf("missing binding conflict report: %s", messages)
	}
}
