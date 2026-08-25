package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"unicode"
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
	shortcutMaps          = map[string]map[string]string{}
	shortcutRegistrations = map[string]scriptRegistrationHandle{}
)

const (
	shortcutsDir        = "shortcuts"
	globalShortcutsFile = "global-shortcuts.json"
)

// scriptAddShortcut registers a single shortcut for the script identified by owner.
// Typing short text in the chat box will expand into the full string before
// being sent.  For example, adding ("pp", "/ponder ") means that typing
// "pp" or "pp hello" becomes "/ponder " or "/ponder hello" respectively.
func addShortcut(owner, short, full string) {
	short = strings.ToLower(short)
	shortcutMu.Lock()
	m := shortcutMaps[owner]
	if m == nil {
		m = map[string]string{}
		shortcutMaps[owner] = m
		if owner != "user" && owner != "global" {
			var registration scriptRegistrationHandle
			registration = registerScriptResource(owner, func() {
				shortcutMu.Lock()
				if shortcutRegistrations[owner] == registration {
					delete(shortcutMaps, owner)
					delete(shortcutRegistrations, owner)
				}
				shortcutMu.Unlock()
				refreshShortcutsList()
			})
			shortcutRegistrations[owner] = registration
		}
	}
	m[short] = full
	shortcutMu.Unlock()
	refreshShortcutsList()
}

func scriptAddShortcut(owner, short, full string) {
	if scriptIsDisabled(owner) {
		return
	}
	addShortcut(owner, short, full)
	scriptLogEvent(owner, "Registered shortcut", fmt.Sprintf("%s -> %s", short, full))
}

// expandShortcut replaces one leading shortcut before input is sent. User
// shortcuts take precedence over global shortcuts, followed by script owners
// in name order, so duplicate prefixes always resolve the same way.
func expandShortcut(text string) string {
	shortcutMu.RLock()
	owners := make([]string, 0, len(shortcutMaps))
	for owner := range shortcutMaps {
		if owner != "user" && owner != "global" {
			owners = append(owners, owner)
		}
	}
	sort.Strings(owners)
	owners = append([]string{"user", "global"}, owners...)
	maps := make([]map[string]string, 0, len(owners))
	for _, owner := range owners {
		local := shortcutMaps[owner]
		copyOfLocal := make(map[string]string, len(local))
		for short, full := range local {
			copyOfLocal[short] = full
		}
		maps = append(maps, copyOfLocal)
	}
	shortcutMu.RUnlock()

	lower := strings.ToLower(text)
	for _, shortcuts := range maps {
		for short, full := range shortcuts {
			if lower == short {
				return full
			}
			if strings.HasPrefix(lower, short+" ") {
				return full + text[len(short)+1:]
			}
		}
	}
	return text
}

func removeShortcut(owner, short string) {
	short = strings.ToLower(short)
	shortcutMu.Lock()
	if m := shortcutMaps[owner]; m != nil {
		delete(m, short)
		if len(m) == 0 {
			delete(shortcutMaps, owner)
		}
	}
	shortcutMu.Unlock()
	refreshShortcutsList()
	if owner == "user" || owner == "global" {
		saveShortcuts()
	}
}

func addUserShortcut(short, full string) {
	addShortcut("user", short, full)
	saveShortcuts()
}

func addGlobalShortcut(short, full string) {
	addShortcut("global", short, full)
	saveShortcuts()
}

// loadShortcuts loads persisted user and global shortcuts from disk.
func loadShortcuts() {
	// Clear existing user/global maps so we replace previous character’s shortcuts.
	shortcutMu.Lock()
	delete(shortcutMaps, "user")
	delete(shortcutMaps, "global")
	shortcutMu.Unlock()

	dir := filepath.Join(dataDirPath, shortcutsDir)
	// Load global shortcuts
	if data, err := os.ReadFile(filepath.Join(dir, globalShortcutsFile)); err == nil {
		var m map[string]string
		if json.Unmarshal(data, &m) == nil {
			for k, v := range m {
				if k != "" && v != "" {
					addShortcut("global", k, v)
				}
			}
		}
	}
	// Load user shortcuts for the effective character
	eff := effectiveCharacterName()
	if eff != "" {
		name := sanitizeName(eff)
		if name != "" {
			if data, err := os.ReadFile(filepath.Join(dir, name+"-shortcuts.json")); err == nil {
				var m map[string]string
				if json.Unmarshal(data, &m) == nil {
					for k, v := range m {
						if k != "" && v != "" {
							addShortcut("user", k, v)
						}
					}
				}
			}
		}
	}
}

// saveShortcuts persists user and global shortcuts to disk.
func saveShortcuts() {
	if isWASM {
		return
	}
	_ = os.MkdirAll(filepath.Join(dataDirPath, shortcutsDir), 0o755)
	dir := filepath.Join(dataDirPath, shortcutsDir)
	shortcutMu.RLock()
	// Save global
	if gm := shortcutMaps["global"]; gm != nil {
		if data, err := json.MarshalIndent(gm, "", "  "); err == nil {
			_ = os.WriteFile(filepath.Join(dir, globalShortcutsFile), data, 0o644)
		}
	}
	// Save user for effective character
	eff := effectiveCharacterName()
	if eff != "" {
		name := sanitizeName(eff)
		if name != "" {
			if um := shortcutMaps["user"]; um != nil {
				if data, err := json.MarshalIndent(um, "", "  "); err == nil {
					_ = os.WriteFile(filepath.Join(dir, name+"-shortcuts.json"), data, 0o644)
				}
			}
		}
	}
	shortcutMu.RUnlock()
}

// effectiveCharacterName returns the current player name or last used character.
func effectiveCharacterName() string {
	if playerName != "" {
		return playerName
	}
	return gs.LastCharacter
}

// sanitizeName keeps letters/digits and converts spaces to underscores.
func sanitizeName(in string) string {
	var b strings.Builder
	for _, r := range in {
		if r == ' ' {
			b.WriteByte('_')
			continue
		}
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}
