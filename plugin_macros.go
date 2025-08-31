package main

import (
	"fmt"
	"log"
	"strings"
	"sync"
)

// This file implements small helpers for working with chat macros in plugin
// scripts.  Macros replace a short bit of text with a longer command, making it
// easy to type common actions.  The helpers below manage macros on behalf of a
// plugin.

var (
	// macroMu guards access to macroMaps so that multiple plugins can add
	// macros safely.
	macroMu sync.RWMutex
	// macroMaps keeps macros separate for each plugin by name.
	macroMaps = map[string]map[string]string{}
)

// pluginAddMacro registers a single macro for the plugin identified by owner.
// Typing short text in the chat box will expand into the full string before
// being sent.  For example, adding ("pp", "/ponder ") means that typing
// "pp" or "pp hello" becomes "/ponder " or "/ponder hello" respectively.
func pluginAddMacro(owner, short, full string) {
	if pluginIsDisabled(owner) {
		return
	}
	short = strings.ToLower(short)
	macroMu.Lock()
	m := macroMaps[owner]
	if m == nil {
		m = map[string]string{}
		macroMaps[owner] = m
		// Install an input handler the first time this plugin adds a
		// macro.  It runs whenever the user submits chat text and
		// replaces any macro prefixes.
		pluginRegisterInputHandler(owner, func(txt string) string {
			macroMu.RLock()
			local := macroMaps[owner]
			macroMu.RUnlock()
			lower := strings.ToLower(txt)
			for k, v := range local {
				if strings.HasPrefix(lower, k) {
					if len(lower) == len(k) {
						return v
					}
					if len(lower) > len(k) && lower[len(k)] == ' ' {
						return v + txt[len(k)+1:]
					}
				}
			}
			return txt
		})
	}
	m[short] = full
	macroMu.Unlock()
	refreshMacrosList()
	name := pluginDisplayNames[owner]
	if name == "" {
		name = owner
	}
    msg := fmt.Sprintf("[plugin:%s] macro added: %s -> %s", name, short, full)
    if gs.pluginOutputDebug {
        consoleMessage(msg)
    }
    log.Print(msg)
}

// pluginAddMacros registers many macros at once for the given plugin.
func pluginAddMacros(owner string, macros map[string]string) {
	if pluginIsDisabled(owner) {
		return
	}
	for k, v := range macros {
		pluginAddMacro(owner, k, v)
	}
}

// pluginRemoveMacros deletes all macros registered by the specified plugin.
// It is typically called when a plugin is disabled or unloaded so that any
// previously registered macro prefixes no longer expand.
func pluginRemoveMacros(owner string) {
	macroMu.Lock()
	delete(macroMaps, owner)
	macroMu.Unlock()
	refreshMacrosList()
	name := pluginDisplayNames[owner]
	if name == "" {
		name = owner
	}
    msg := fmt.Sprintf("[plugin:%s] macros removed", name)
    if gs.pluginOutputDebug {
        consoleMessage(msg)
    }
    log.Print(msg)
}
