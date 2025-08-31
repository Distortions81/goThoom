package main

import (
	"fmt"
	"log"
	"strings"
	"sync"
)

// This file implements helpers for chat shortcuts registered by scripts.
// Shortcuts replace a short bit of text with a longer command, making it easy
// to type common actions. The helpers below manage shortcuts on behalf of a
// script.

var (
    // shortcutMu guards access to shortcutMaps so that multiple scripts can add
    // shortcuts safely.
    shortcutMu sync.RWMutex
    // shortcutMaps keeps shortcuts separate for each script by name.
    shortcutMaps = map[string]map[string]string{}
)

// pluginAddShortcut registers a single shortcut for the script identified by owner.
// Typing short text in the chat box will expand into the full string before
// being sent.  For example, adding ("pp", "/ponder ") means that typing
// "pp" or "pp hello" becomes "/ponder " or "/ponder hello" respectively.
func pluginAddShortcut(owner, short, full string) {
    if pluginIsDisabled(owner) {
        return
    }
    short = strings.ToLower(short)
    shortcutMu.Lock()
    m := shortcutMaps[owner]
    if m == nil {
        m = map[string]string{}
        shortcutMaps[owner] = m
        // Install an input handler the first time this script adds a
        // macro.  It runs whenever the user submits chat text and
        // replaces any macro prefixes.
        pluginRegisterInputHandler(owner, func(txt string) string {
            shortcutMu.RLock()
            local := shortcutMaps[owner]
            shortcutMu.RUnlock()
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
    shortcutMu.Unlock()
    refreshShortcutsList()
    name := pluginDisplayNames[owner]
    if name == "" {
        name = owner
    }
    msg := fmt.Sprintf("[plugin:%s] shortcut added: %s -> %s", name, short, full)
    if gs.pluginOutputDebug {
        consoleMessage(msg)
    }
    log.Print(msg)
}

// pluginAddShortcuts registers many shortcuts at once for the given script.
func pluginAddShortcuts(owner string, shortcuts map[string]string) {
    if pluginIsDisabled(owner) {
        return
    }
    for k, v := range shortcuts {
        pluginAddShortcut(owner, k, v)
    }
}

// pluginRemoveShortcuts deletes all shortcuts registered by the specified script.
// It is typically called when a script is disabled or unloaded so that any
// previously registered shortcut prefixes no longer expand.
func pluginRemoveShortcuts(owner string) {
    shortcutMu.Lock()
    delete(shortcutMaps, owner)
    shortcutMu.Unlock()
    refreshShortcutsList()
    name := pluginDisplayNames[owner]
    if name == "" {
        name = owner
    }
    msg := fmt.Sprintf("[plugin:%s] shortcuts removed", name)
    if gs.pluginOutputDebug {
        consoleMessage(msg)
    }
    log.Print(msg)
}
