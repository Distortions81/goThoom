//go:build integration
// +build integration

package main

import (
	"sync"
	"testing"
	"time"
)

// Test that a single macro expands input text and matches case-insensitively.
func TestscriptAddShortcutExpandsInput(t *testing.T) {
	// Reset shared state.
	shortcutMu = sync.RWMutex{}
	shortcutMaps = map[string]map[string]string{}
	inputHandlersMu = sync.RWMutex{}
	scriptInputHandlers = nil

	scriptAddShortcut("tester", "pp", "/ponder ")

	if got, want := runInputHandlers("pp"), "/ponder "; got != want {
		t.Fatalf("bare macro failed: got %q, want %q", got, want)
	}

	if got, want := runInputHandlers("pp hello"), "/ponder hello"; got != want {
		t.Fatalf("lowercase macro with space failed: got %q, want %q", got, want)
	}

	if got, want := runInputHandlers("PP Hello"), "/ponder Hello"; got != want {
		t.Fatalf("uppercase macro failed: got %q, want %q", got, want)
	}

	if got, want := runInputHandlers("pphi"), "pphi"; got != want {
		t.Fatalf("macro should not expand within word: got %q, want %q", got, want)
	}
}

// Test that multiple macros can be registered at once.
func TestscriptAddShortcuts(t *testing.T) {
	shortcutMu = sync.RWMutex{}
	shortcutMaps = map[string]map[string]string{}
	inputHandlersMu = sync.RWMutex{}
	scriptInputHandlers = nil

	scriptAddShortcuts("bulk", map[string]string{"pp": "/ponder ", "hi": "/hello "})

	if got, want := runInputHandlers("pp there"), "/ponder there"; got != want {
		t.Fatalf("pp macro failed: got %q, want %q", got, want)
	}
	if got, want := runInputHandlers("hi you"), "/hello you"; got != want {
		t.Fatalf("hi macro failed: got %q, want %q", got, want)
	}
}

// Test that calling scriptAddMacro multiple times for the same script
// installs only one input handler while all macros still expand.
func TestscriptAddShortcutSingleHandler(t *testing.T) {
	shortcutMu = sync.RWMutex{}
	shortcutMaps = map[string]map[string]string{}
	inputHandlersMu = sync.RWMutex{}
	scriptInputHandlers = nil

	owner := "dup"
	scriptAddShortcut(owner, "pp", "/ponder ")
	scriptAddShortcut(owner, "hi", "/hello ")

	handlers := 0
	for _, h := range scriptInputHandlers {
		if h.owner == owner {
			handlers++
		}
	}
	if handlers != 1 {
		t.Fatalf("unexpected handler count: %d", handlers)
	}
	if got, want := runInputHandlers("pp there"), "/ponder there"; got != want {
		t.Fatalf("pp macro failed: got %q, want %q", got, want)
	}
	if got, want := runInputHandlers("hi you"), "/hello you"; got != want {
		t.Fatalf("hi macro failed: got %q, want %q", got, want)
	}
}

// Test that disabling a script removes any macros it registered.
func TestscriptRemoveShortcutsOnDisable(t *testing.T) {
	// Reset shared state.
	shortcutMu = sync.RWMutex{}
	shortcutMaps = map[string]map[string]string{}
	inputHandlersMu = sync.RWMutex{}
	scriptInputHandlers = nil
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
	scriptAddShortcut(owner, "pp", "/ponder ")
	if got, want := runInputHandlers("pp hello"), "/ponder hello"; got != want {
		t.Fatalf("macro not added: got %q, want %q", got, want)
	}

	disablescript(owner, "testing")

	if got, want := runInputHandlers("pp hello"), "pp hello"; got != want {
		t.Fatalf("macro not removed: got %q, want %q", got, want)
	}
}
