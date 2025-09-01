//go:build integration
// +build integration

package main

import (
	"sync"
	"testing"
	"time"
)

// Test that a single macro expands input text and matches case-insensitively.
func TestPluginAddShortcutExpandsInput(t *testing.T) {
	// Reset shared state.
	shortcutMu = sync.RWMutex{}
	shortcutMaps = map[string]map[string]string{}
	inputHandlersMu = sync.RWMutex{}
	pluginInputHandlers = nil

	pluginAddShortcut("tester", "pp", "/ponder ")

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
func TestPluginAddShortcuts(t *testing.T) {
	shortcutMu = sync.RWMutex{}
	shortcutMaps = map[string]map[string]string{}
	inputHandlersMu = sync.RWMutex{}
	pluginInputHandlers = nil

	pluginAddShortcuts("bulk", map[string]string{"pp": "/ponder ", "hi": "/hello "})

	if got, want := runInputHandlers("pp there"), "/ponder there"; got != want {
		t.Fatalf("pp macro failed: got %q, want %q", got, want)
	}
	if got, want := runInputHandlers("hi you"), "/hello you"; got != want {
		t.Fatalf("hi macro failed: got %q, want %q", got, want)
	}
}

// Test that calling pluginAddMacro multiple times for the same plugin
// installs only one input handler while all macros still expand.
func TestPluginAddShortcutSingleHandler(t *testing.T) {
	shortcutMu = sync.RWMutex{}
	shortcutMaps = map[string]map[string]string{}
	inputHandlersMu = sync.RWMutex{}
	pluginInputHandlers = nil

	owner := "dup"
	pluginAddShortcut(owner, "pp", "/ponder ")
	pluginAddShortcut(owner, "hi", "/hello ")

	handlers := 0
	for _, h := range pluginInputHandlers {
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

// Test that AutoReply triggers the specified command when the message starts with the trigger.
func TestPluginAutoReplyRunsCommand(t *testing.T) {
	shortcutMu = sync.RWMutex{}
	shortcutMaps = map[string]map[string]string{}
	inputHandlersMu = sync.RWMutex{}
	pluginInputHandlers = nil
	triggerHandlersMu = sync.RWMutex{}
	pluginTriggers = map[string][]triggerHandler{}
	pluginConsoleTriggers = map[string][]triggerHandler{}
	consoleLog = messageLog{max: maxMessages}
	commandQueue = nil
	pendingCommand = ""
	pluginSendHistory = map[string][]time.Time{}

	pluginAutoReply("bot", "hi", "/wave")
	runChatTriggers("Hi there")

	if msgs := getConsoleMessages(); len(msgs) != 0 {
		t.Fatalf("unexpected console messages %v", msgs)
	}
	if pendingCommand != "/wave" {
		t.Fatalf("pending command %q, want %q", pendingCommand, "/wave")
	}
}

// Test that disabling a plugin removes any macros it registered.
func TestPluginRemoveShortcutsOnDisable(t *testing.T) {
	// Reset shared state.
	shortcutMu = sync.RWMutex{}
	shortcutMaps = map[string]map[string]string{}
	inputHandlersMu = sync.RWMutex{}
	pluginInputHandlers = nil
	pluginMu = sync.RWMutex{}
	pluginDisabled = map[string]bool{}
	pluginInvalid = map[string]bool{}
	pluginEnabledFor = map[string]pluginScope{}
	pluginDisplayNames = map[string]string{}
	pluginCategories = map[string]string{}
	pluginSubCategories = map[string]string{}
	pluginTerminators = map[string]func(){}
	pluginCommandOwners = map[string]string{}
	pluginCommands = map[string]PluginCommandHandler{}
	pluginSendHistory = map[string][]time.Time{}
	hotkeysMu = sync.RWMutex{}
	hotkeys = nil
	pluginHotkeyEnabled = map[string]map[string]bool{}
	consoleLog = messageLog{max: maxMessages}

	owner := "plug"
	pluginAddShortcut(owner, "pp", "/ponder ")
	if got, want := runInputHandlers("pp hello"), "/ponder hello"; got != want {
		t.Fatalf("macro not added: got %q, want %q", got, want)
	}

	disablePlugin(owner, "testing")

	if got, want := runInputHandlers("pp hello"), "pp hello"; got != want {
		t.Fatalf("macro not removed: got %q, want %q", got, want)
	}
}
