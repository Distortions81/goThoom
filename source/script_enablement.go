package main

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	scriptEnablementDirName  = "Scripts"
	scriptEnablementFileName = "enabled.json"
)

// scriptEnablementSelection is deliberately separate from settings.json and
// from the executable-adjacent scripts directory. It records which script IDs
// are enabled globally or for each character.
type scriptEnablementSelection struct {
	Global  []string            `json:"global,omitempty"`
	Players map[string][]string `json:"players,omitempty"`
}

func scriptEnablementPath() string {
	return filepath.Join(dataDirPath, scriptEnablementDirName, scriptEnablementFileName)
}

func loadScriptEnablement() {
	if isWASM {
		return
	}
	data, err := os.ReadFile(scriptEnablementPath())
	if os.IsNotExist(err) {
		// Persist an empty selection so the location is discoverable and becomes
		// the sole source of truth.
		saveScriptEnablement()
		return
	}
	if err != nil {
		log.Printf("read script enablement: %v", err)
		return
	}
	selection := scriptEnablementSelection{}
	if err := json.Unmarshal(data, &selection); err != nil {
		log.Printf("parse script enablement: %v", err)
		return
	}
	selection.normalize()
	scriptMu.Lock()
	scriptEnabledFor = selection.scopes()
	scriptMu.Unlock()
}

func saveScriptEnablement() {
	if isWASM {
		return
	}
	scriptMu.RLock()
	selection := scriptEnablementFromScopes(scriptEnabledFor)
	scriptMu.RUnlock()
	selection.normalize()
	data, err := json.MarshalIndent(selection, "", "  ")
	if err != nil {
		log.Printf("encode script enablement: %v", err)
		return
	}
	path := scriptEnablementPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		log.Printf("create script enablement directory: %v", err)
		return
	}
	if err := legacyMacroAtomicWriteFile(path, append(data, '\n'), 0o644); err != nil {
		log.Printf("write script enablement: %v", err)
	}
}

func scriptEnablementFromScopes(scopes map[string]scriptScope) scriptEnablementSelection {
	selection := scriptEnablementSelection{}
	for id, scope := range scopes {
		if !validScriptEnablementID(id) {
			continue
		}
		if scope.All {
			selection.Global = append(selection.Global, id)
			continue
		}
		for character := range scope.Chars {
			if !validScriptEnablementCharacter(character) {
				continue
			}
			if selection.Players == nil {
				selection.Players = make(map[string][]string)
			}
			selection.Players[character] = append(selection.Players[character], id)
		}
	}
	return selection
}

func (selection scriptEnablementSelection) scopes() map[string]scriptScope {
	scopes := make(map[string]scriptScope, len(selection.Global))
	for _, id := range selection.Global {
		scopes[id] = scriptScope{All: true}
	}
	for character, ids := range selection.Players {
		for _, id := range ids {
			scope := scopes[id]
			if !scope.All {
				scope.addChar(character)
				scopes[id] = scope
			}
		}
	}
	return scopes
}

func (selection *scriptEnablementSelection) normalize() {
	selection.Global = normalizeScriptEnablementIDs(selection.Global)
	global := make(map[string]bool, len(selection.Global))
	for _, id := range selection.Global {
		global[id] = true
	}
	players := make(map[string][]string)
	for character, ids := range selection.Players {
		if !validScriptEnablementCharacter(character) {
			continue
		}
		filtered := make([]string, 0, len(ids))
		for _, id := range normalizeScriptEnablementIDs(ids) {
			if !global[id] {
				filtered = append(filtered, id)
			}
		}
		if len(filtered) > 0 {
			players[character] = filtered
		}
	}
	if len(players) == 0 {
		selection.Players = nil
	} else {
		selection.Players = players
	}
}

func normalizeScriptEnablementIDs(ids []string) []string {
	seen := make(map[string]bool, len(ids))
	result := make([]string, 0, len(ids))
	for _, id := range ids {
		if !validScriptEnablementID(id) || seen[id] {
			continue
		}
		seen[id] = true
		result = append(result, id)
	}
	sort.Strings(result)
	return result
}

func validScriptEnablementID(id string) bool {
	return id != "" && normalizeScriptID(id) == id
}

func validScriptEnablementCharacter(character string) bool {
	character = strings.TrimSpace(character)
	return character != "" && character != "." && character != ".." && filepath.Base(character) == character && !strings.ContainsAny(character, "/\\")
}
