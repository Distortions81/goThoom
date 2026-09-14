package main

import (
	"encoding/json"
	"errors"
	"log"
	"os"
	"path/filepath"
)

const (
	multiSessionWorkspaceFile    = "multi_session.json"
	multiSessionWorkspaceVersion = 2
)

type multiSessionWorkspaceDocument struct {
	Version               int               `json:"version"`
	Selected              SessionID         `json:"selected_session"`
	OpenTabs              [maxSessions]bool `json:"open_tabs"`
	TabHotkeysInitialized bool              `json:"tab_hotkeys_initialized,omitempty"`
	TabCycleInitialized   bool              `json:"tab_cycle_hotkeys_initialized,omitempty"`
	ClientHotkeysVersion  int               `json:"client_hotkeys_version,omitempty"`
}

var (
	multiSessionWorkspace      = defaultMultiSessionWorkspaceDocument()
	multiSessionWorkspaceUsed  bool
	multiSessionWorkspaceDirty bool
)

func defaultMultiSessionWorkspaceDocument() multiSessionWorkspaceDocument {
	document := multiSessionWorkspaceDocument{
		Version:  multiSessionWorkspaceVersion,
		Selected: primarySessionID,
	}
	document.OpenTabs[0] = true
	return document
}

func loadMultiSessionWorkspace() bool {
	multiSessionWorkspace = defaultMultiSessionWorkspaceDocument()
	multiSessionWorkspaceUsed = false
	multiSessionWorkspaceDirty = false
	data, err := os.ReadFile(filepath.Join(dataDirPath, multiSessionWorkspaceFile))
	if errors.Is(err, os.ErrNotExist) {
		return false
	}
	if err != nil {
		log.Printf("load multi-session workspace: %v", err)
		return false
	}
	var loaded multiSessionWorkspaceDocument
	if err := json.Unmarshal(data, &loaded); err != nil {
		log.Printf("load multi-session workspace: %v", err)
		return false
	}
	if loaded.Version != 1 && loaded.Version != multiSessionWorkspaceVersion {
		log.Printf("load multi-session workspace: unsupported version %d", loaded.Version)
		return false
	}
	if loaded.Version == 1 {
		// The previous workspace always materialized four panes at once.
		for slot := 0; slot < 4 && slot < maxSessions; slot++ {
			loaded.OpenTabs[slot] = true
		}
		loaded.Version = multiSessionWorkspaceVersion
		multiSessionWorkspaceDirty = true
	}
	if !anyOpenSessionTabs(loaded.OpenTabs) {
		loaded.OpenTabs[0] = true
	}
	if !loaded.Selected.Valid() {
		loaded.Selected = primarySessionID
	}
	multiSessionWorkspace = loaded
	multiSessionWorkspaceUsed = true
	return true
}

func markMultiSessionWorkspaceUsed() {
	if !multiSessionWorkspaceUsed {
		multiSessionWorkspaceUsed = true
		multiSessionWorkspaceDirty = true
	}
}

func syncMultiSessionWorkspace() bool {
	if !multiSessionWorkspaceUsed || appSessions == nil {
		return false
	}
	changed := false
	sessions := appSessions.snapshot()
	if selected := appSessions.selectedID(); selected.Valid() && multiSessionWorkspace.Selected != selected {
		multiSessionWorkspace.Selected = selected
		changed = true
	}
	for slot, session := range sessions {
		open := session != nil
		if multiSessionWorkspace.OpenTabs[slot] != open {
			multiSessionWorkspace.OpenTabs[slot] = open
			changed = true
		}
	}
	if changed {
		multiSessionWorkspaceDirty = true
	}
	return changed
}

func saveMultiSessionWorkspace() {
	if isWASM || !multiSessionWorkspaceUsed {
		return
	}
	syncMultiSessionWorkspace()
	if !multiSessionWorkspaceDirty {
		return
	}
	multiSessionWorkspace.Version = multiSessionWorkspaceVersion
	data, err := json.MarshalIndent(multiSessionWorkspace, "", "  ")
	if err != nil {
		log.Printf("save multi-session workspace: %v", err)
		return
	}
	data = append(data, '\n')
	path := filepath.Join(dataDirPath, multiSessionWorkspaceFile)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		log.Printf("save multi-session workspace: %v", err)
		return
	}
	if err := writeFileAtomic(path, data, 0o644); err != nil {
		log.Printf("save multi-session workspace: %v", err)
		return
	}
	multiSessionWorkspaceDirty = false
}

func anyOpenSessionTabs(open [maxSessions]bool) bool {
	for _, value := range open {
		if value {
			return true
		}
	}
	return false
}
