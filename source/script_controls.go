package main

import (
	"fmt"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/hajimehoshi/ebiten/v2"
)

// Protected by scriptMu. Default is the stable identity supplied by the script;
// Value is the command currently exposed to the player.
type scriptCommandSetting struct {
	Owner, Default, Value string
}

var scriptCommandSettings = map[string]*scriptCommandSetting{}

func scriptControlValue(owner, kind, original string) string {
	return scriptStoredString(scriptStorageGet(owner, "__controls__:"+kind+":"+original), original)
}

func saveScriptControl(owner, kind, original, value string) {
	key := "__controls__:" + kind + ":" + original
	if original == value {
		scriptStorageDelete(owner, key)
	} else {
		scriptStorageSet(owner, key, value)
	}
	flushscriptStore(owner)
}

func scriptHotkeyDefault(hk Hotkey) string {
	if hk.defaultCombo != "" {
		return hk.defaultCombo
	}
	return hk.Combo
}

func scriptCommandSettingsFor(owner string) []scriptCommandSetting {
	return scriptCommandSettingsForSession(nil, owner)
}

func scriptCommandSettingsForSession(session *Session, owner string) []scriptCommandSetting {
	if session != nil {
		session.automation.scriptMu.RLock()
		var entries []scriptCommandSetting
		for value, entry := range session.automation.localCommands {
			if entry.owner == owner {
				entries = append(entries, scriptCommandSetting{Owner: owner, Default: entry.original, Value: value})
			}
		}
		session.automation.scriptMu.RUnlock()
		sort.Slice(entries, func(i, j int) bool { return entries[i].Default < entries[j].Default })
		return entries
	}
	scriptMu.RLock()
	defer scriptMu.RUnlock()
	var entries []scriptCommandSetting
	for key, entry := range scriptCommandSettings {
		if entry.Owner == owner && scriptCommandOwners[key] == owner && scriptCommands[key] != nil {
			entries = append(entries, *entry)
		}
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Default < entries[j].Default })
	return entries
}

func scriptHotkeysForSession(session *Session, owner string) []Hotkey {
	if session == nil {
		return scriptHotkeys(owner)
	}
	session.automation.scriptMu.RLock()
	bindings := session.automation.localHotkeys[owner]
	keys := make([]Hotkey, 0, len(bindings))
	for combo, binding := range bindings {
		keys = append(keys, Hotkey{Combo: combo, Script: owner, defaultCombo: binding.original})
	}
	session.automation.scriptMu.RUnlock()
	sort.Slice(keys, func(i, j int) bool { return keys[i].Combo < keys[j].Combo })
	return keys
}

func setScriptCommandName(owner, original, value string) error {
	return setScriptCommandNameForSession(nil, owner, original, value)
}

func setScriptCommandNameForSession(session *Session, owner, original, value string) error {
	if session != nil {
		value = normalizeScriptCommand(strings.TrimSpace(value))
		if value == "" || strings.ContainsAny(value, "/\\") || strings.ContainsFunc(value, unicode.IsSpace) {
			return fmt.Errorf("Enter one command name, without spaces or arguments.")
		}
		if value == "play" || value == "palette" || value == "setting" || strings.HasPrefix(value, "testhooks") {
			return fmt.Errorf("/%s is a built-in client command.", value)
		}
		session.automation.scriptMu.Lock()
		current := ""
		var entry sessionScriptCommand
		for command, candidate := range session.automation.localCommands {
			if candidate.owner == owner && candidate.original == original {
				current, entry = command, candidate
				break
			}
		}
		if current == "" {
			session.automation.scriptMu.Unlock()
			return fmt.Errorf("This command is no longer registered. Reopen Settings.")
		}
		if existing, exists := session.automation.localCommands[value]; exists && (value != current || existing.owner != owner) {
			session.automation.scriptMu.Unlock()
			return fmt.Errorf("/%s is already registered.", value)
		}
		delete(session.automation.localCommands, current)
		session.automation.localCommands[value] = entry
		session.automation.scriptMu.Unlock()
		saveScriptControl(owner, "command", original, value)
		return nil
	}
	value = normalizeScriptCommand(strings.TrimSpace(value))
	if value == "" || strings.ContainsAny(value, "/\\") || strings.ContainsFunc(value, unicode.IsSpace) {
		return fmt.Errorf("Enter one command name, without spaces or arguments.")
	}
	if value == "play" || value == "palette" || value == "setting" || strings.HasPrefix(value, "testhooks") {
		return fmt.Errorf("/%s is a built-in client command.", value)
	}
	scriptMu.Lock()
	var entry *scriptCommandSetting
	for key, candidate := range scriptCommandSettings {
		if candidate.Owner == owner && candidate.Default == original && scriptCommandOwners[key] == owner && scriptCommands[key] != nil {
			entry = candidate
			break
		}
	}
	if entry == nil {
		scriptMu.Unlock()
		return fmt.Errorf("This command is no longer registered. Reopen Settings.")
	}
	if entry.Value != value && scriptCommands[value] != nil {
		scriptMu.Unlock()
		return fmt.Errorf("/%s is already registered.", value)
	}
	handler := scriptCommands[entry.Value]
	delete(scriptCommands, entry.Value)
	delete(scriptCommandOwners, entry.Value)
	delete(scriptCommandSettings, entry.Value)
	entry.Value = value
	scriptCommands[value] = handler
	scriptCommandOwners[value] = owner
	scriptCommandSettings[value] = entry
	scriptMu.Unlock()
	saveScriptControl(owner, "command", original, value)
	return nil
}

func setScriptBinding(owner, original, value string) error {
	return setScriptBindingForSession(nil, owner, original, value)
}

func setScriptBindingForSession(session *Session, owner, original, value string) error {
	if session != nil {
		value = strings.TrimSpace(value)
		if !validScriptControlBinding(value) {
			return fmt.Errorf("Enter a key combination or use Record.")
		}
		session.automation.scriptMu.Lock()
		bindings := session.automation.localHotkeys[owner]
		current := ""
		var entry sessionScriptHotkey
		for combo, candidate := range bindings {
			if candidate.original == original {
				current, entry = combo, candidate
				break
			}
		}
		if current == "" {
			session.automation.scriptMu.Unlock()
			return fmt.Errorf("This binding is no longer registered. Reopen Settings.")
		}
		for registered := range bindings {
			if registered != current && sameCombo(registered, value) {
				session.automation.scriptMu.Unlock()
				return fmt.Errorf("%s is already registered.", value)
			}
		}
		delete(bindings, current)
		bindings[value] = entry
		session.automation.scriptMu.Unlock()
		saveScriptControl(owner, "binding", original, value)
		return nil
	}
	value = strings.TrimSpace(value)
	if !validScriptControlBinding(value) {
		return fmt.Errorf("Enter a key combination or use Record.")
	}
	hotkeysMu.Lock()
	index := -1
	for i, hk := range hotkeys {
		if hk.Script == owner && scriptHotkeyDefault(hk) == original {
			index = i
			break
		}
	}
	if index < 0 {
		hotkeysMu.Unlock()
		return fmt.Errorf("This binding is no longer registered. Reopen Settings.")
	}
	for i, hk := range hotkeys {
		if i != index && sameCombo(hk.Combo, value) {
			hotkeysMu.Unlock()
			return fmt.Errorf("%s is already assigned to another hotkey.", value)
		}
	}
	old := hotkeys[index].Combo
	scriptHotkeyFnMu.Lock()
	handlers := scriptHotkeyFns[owner]
	if handlers[old] == nil {
		scriptHotkeyFnMu.Unlock()
		hotkeysMu.Unlock()
		return fmt.Errorf("This binding is no longer active. Reopen Settings.")
	}
	handler := handlers[old]
	delete(handlers, old)
	handlers[value] = handler
	hotkeys[index].Combo = value
	scriptHotkeyFnMu.Unlock()
	hotkeysMu.Unlock()
	saveScriptControl(owner, "binding", original, value)
	saveHotkeys()
	refreshHotkeysList()
	refreshScriptToolbars()
	return nil
}

func validScriptControlBinding(value string) bool {
	if !validScriptBindingText(value) {
		return false
	}
	for _, part := range strings.Split(value, "-") {
		if isInputModifierName(part) {
			continue
		}
		switch strings.ToLower(part) {
		case "leftclick", "rightclick", "middleclick", "mouse4", "mouse5", "wheelup", "wheeldown", "wheelleft", "wheelright":
			continue
		}
		if utf8.RuneCountInString(part) == 1 {
			char, _ := utf8.DecodeRuneInString(part)
			if unicode.IsPrint(char) && !unicode.IsSpace(char) {
				continue
			}
		}
		var key ebiten.Key
		if err := key.UnmarshalText([]byte(part)); err != nil {
			return false
		}
	}
	return true
}
