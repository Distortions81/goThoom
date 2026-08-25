//go:build integration
// +build integration

package main

import (
	"sync"
	"testing"
	"time"
)

// Test that a single macro expands input text and matches case-insensitively.
func TestScriptAddShortcutExpandsInput(t *testing.T) {
	// Reset shared state.
	shortcutMu = sync.RWMutex{}
	shortcutMaps = map[string]map[string]string{}
	shortcutRegistrations = map[string]scriptRegistrationHandle{}
	startScriptEventQueue("tester")

	scriptAddShortcut("tester", "pp", "/ponder ")

	if got, want := expandShortcut("pp"), "/ponder "; got != want {
		t.Fatalf("bare macro failed: got %q, want %q", got, want)
	}

	if got, want := expandShortcut("pp hello"), "/ponder hello"; got != want {
		t.Fatalf("lowercase macro with space failed: got %q, want %q", got, want)
	}

	if got, want := expandShortcut("PP Hello"), "/ponder Hello"; got != want {
		t.Fatalf("uppercase macro failed: got %q, want %q", got, want)
	}

	if got, want := expandShortcut("pphi"), "pphi"; got != want {
		t.Fatalf("macro should not expand within word: got %q, want %q", got, want)
	}
}

// Test that multiple shortcuts can be registered with the canonical one-at-a-time API.
func TestScriptAddMultipleShortcuts(t *testing.T) {
	shortcutMu = sync.RWMutex{}
	shortcutMaps = map[string]map[string]string{}
	shortcutRegistrations = map[string]scriptRegistrationHandle{}
	startScriptEventQueue("bulk")

	scriptAddShortcut("bulk", "pp", "/ponder ")
	scriptAddShortcut("bulk", "hi", "/hello ")

	if got, want := expandShortcut("pp there"), "/ponder there"; got != want {
		t.Fatalf("pp macro failed: got %q, want %q", got, want)
	}
	if got, want := expandShortcut("hi you"), "/hello you"; got != want {
		t.Fatalf("hi macro failed: got %q, want %q", got, want)
	}
}

// Test that multiple shortcuts for one script share one cleanup registration.
func TestScriptAddShortcutSingleRegistration(t *testing.T) {
	shortcutMu = sync.RWMutex{}
	shortcutMaps = map[string]map[string]string{}
	shortcutRegistrations = map[string]scriptRegistrationHandle{}

	owner := "dup"
	startScriptEventQueue(owner)
	scriptAddShortcut(owner, "pp", "/ponder ")
	scriptAddShortcut(owner, "hi", "/hello ")

	if registration := shortcutRegistrations[owner]; registration.id == 0 {
		t.Fatal("script shortcuts have no cleanup registration")
	}
	if got, want := expandShortcut("pp there"), "/ponder there"; got != want {
		t.Fatalf("pp macro failed: got %q, want %q", got, want)
	}
	if got, want := expandShortcut("hi you"), "/hello you"; got != want {
		t.Fatalf("hi macro failed: got %q, want %q", got, want)
	}
}

// Test that disabling a script removes any macros it registered.
func TestScriptRemoveShortcutsOnDisable(t *testing.T) {
	// Reset shared state.
	shortcutMu = sync.RWMutex{}
	shortcutMaps = map[string]map[string]string{}
	shortcutRegistrations = map[string]scriptRegistrationHandle{}
	scriptMu = sync.RWMutex{}
	scriptDisabled = map[string]bool{}
	scriptInvalid = map[string]bool{}
	scriptEnabledFor = map[string]scriptScope{}
	scriptDisplayNames = map[string]string{}
	scriptCategories = map[string]string{}
	scriptSubCategories = map[string]string{}
	scriptTerminators = map[string]func(){}
	scriptCommandOwners = map[string]string{}
	scriptCommands = map[string]scriptCommandHandler{}
	scriptSendHistory = map[string][]time.Time{}
	hotkeysMu = sync.RWMutex{}
	hotkeys = nil
	scriptHotkeyEnabled = map[string]map[string]bool{}
	consoleLog = messageLog{max: maxMessages}

	owner := "plug"
	startScriptEventQueue(owner)
	scriptAddShortcut(owner, "pp", "/ponder ")
	if got, want := expandShortcut("pp hello"), "/ponder hello"; got != want {
		t.Fatalf("macro not added: got %q, want %q", got, want)
	}

	disablescript(owner, "testing")

	if got, want := expandShortcut("pp hello"), "pp hello"; got != want {
		t.Fatalf("macro not removed: got %q, want %q", got, want)
	}
}
