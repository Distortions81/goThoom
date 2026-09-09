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

func setScriptCommandName(owner, original, value string) error {
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
