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
	gs.Enabledscripts = nil
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
	scriptTimers = map[string][]*time.Timer{}
	scriptTickerStops = map[string][]chan struct{}{}
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
import "gt"
const scriptName = "` + name + `"
const scriptAuthor = "Test"
const scriptCategory = "Tests"
const scriptAPIVersion = 1
func Init() {
` + initBody + `
}
`
		if err := os.WriteFile(filepath.Join(dir, file), []byte(src), 0o644); err != nil {
			t.Fatalf("write %s: %v", file, err)
		}
	}

	writeScript("gamma.go", "Gamma", `
	gt.Key("Shift-Ctrl-F3", func() {})
	gt.Save("started", "gamma")`)
	writeScript("beta.go", "Beta", `
	gt.RegisterCommand("shared", func(string) {})
	gt.Save("started", "beta")`)
	writeScript("alpha.go", "Alpha", `
	gt.RegisterCommand("shared", func(string) {})
	gt.Key("Control-Shift-F3", func() {})
	gt.Save("started", "alpha")`)

	for _, owner := range []string{"Gamma_gamma", "Beta_beta", "Alpha_alpha"} {
		scriptEnabledFor[owner] = scriptScope{All: true}
	}
	rescanScripts([]string{dir})

	if got := scriptCommandOwners["shared"]; got != "Alpha_alpha" {
		t.Fatalf("deterministic command owner = %q, want Alpha_alpha", got)
	}
	if !scriptIsRunning("Alpha_alpha") || scriptIsRunning("Beta_beta") || scriptIsRunning("Gamma_gamma") {
		t.Fatalf("running states: alpha=%v beta=%v gamma=%v", scriptIsRunning("Alpha_alpha"), scriptIsRunning("Beta_beta"), scriptIsRunning("Gamma_gamma"))
	}
	if got := scriptStorageGet("Alpha_alpha", "started"); got != "alpha" {
		t.Fatalf("winning script did not activate: %v", got)
	}
	if got := scriptStorageGet("Beta_beta", "started"); got != nil {
		t.Fatalf("duplicate-command script activated staged storage: %v", got)
	}
	if got := scriptStorageGet("Gamma_gamma", "started"); got != nil {
		t.Fatalf("duplicate-binding script activated staged storage: %v", got)
	}
	hotkeysMu.RLock()
	if len(hotkeys) != 1 || hotkeys[0].Script != "Alpha_alpha" {
		hotkeysMu.RUnlock()
		t.Fatalf("binding winner was not deterministic: %+v", hotkeys)
	}
	hotkeysMu.RUnlock()

	messages := strings.Join(getConsoleMessages(), "\n")
	if !strings.Contains(messages, "duplicate command /shared already owned by Alpha_alpha") {
		t.Fatalf("missing command conflict report: %s", messages)
	}
	if !strings.Contains(messages, "duplicate binding Shift-Ctrl-F3 already owned by Alpha_alpha") {
		t.Fatalf("missing binding conflict report: %s", messages)
	}
}
