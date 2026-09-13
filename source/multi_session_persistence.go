package main

import (
	"encoding/json"
	"errors"
	"log"
	"os"
	"path/filepath"

	"gothoom/eui"
)

const (
	multiSessionWorkspaceFile    = "multi_session.json"
	multiSessionWorkspaceVersion = 1
)

type multiSessionViewportPlacement struct {
	Position WindowPoint `json:"position"`
	Size     WindowPoint `json:"size"`
}

type multiSessionWorkspaceDocument struct {
	Version       int                                        `json:"version"`
	Selected      SessionID                                  `json:"selected_session"`
	MusicSource   SessionID                                  `json:"music_source_session"`
	ViewportSlots [maxSessions]multiSessionViewportPlacement `json:"viewports"`
}

var (
	multiSessionWorkspace      = defaultMultiSessionWorkspaceDocument()
	multiSessionWorkspaceUsed  bool
	multiSessionWorkspaceDirty bool
)

func defaultMultiSessionWorkspaceDocument() multiSessionWorkspaceDocument {
	return multiSessionWorkspaceDocument{
		Version:     multiSessionWorkspaceVersion,
		Selected:    primarySessionID,
		MusicSource: primarySessionID,
	}
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
	if loaded.Version != multiSessionWorkspaceVersion {
		log.Printf("load multi-session workspace: unsupported version %d", loaded.Version)
		return false
	}
	if !loaded.Selected.Valid() {
		loaded.Selected = primarySessionID
	}
	if !loaded.MusicSource.Valid() {
		loaded.MusicSource = primarySessionID
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
	if !multiSessionWorkspaceUsed || appSessions == nil || !appSessions.multiEnabled() {
		return false
	}
	changed := false
	if selected := appSessions.selectedID(); selected.Valid() && multiSessionWorkspace.Selected != selected {
		multiSessionWorkspace.Selected = selected
		changed = true
	}
	if source := musicSourceSessionID(); source.Valid() && multiSessionWorkspace.MusicSource != source {
		multiSessionWorkspace.MusicSource = source
		changed = true
	}
	if appViewports.layoutSnapshot() == viewportLayoutFreeform {
		screenWidth, screenHeight := eui.ScreenSize()
		if screenWidth > 0 && screenHeight > 0 {
			for slot, view := range appViewports.snapshot() {
				state := view.render
				if !view.Active || state == nil || state.window == nil || !state.window.IsOpen() {
					continue
				}
				pos := state.window.GetPos()
				size := state.window.GetSize()
				placement := multiSessionViewportPlacement{
					Position: WindowPoint{X: float64(pos.X) / float64(screenWidth), Y: float64(pos.Y) / float64(screenHeight)},
					Size:     WindowPoint{X: float64(size.X) / float64(screenWidth), Y: float64(size.Y) / float64(screenHeight)},
				}
				clampMultiSessionViewportPlacement(&placement)
				if multiSessionWorkspace.ViewportSlots[slot] != placement {
					multiSessionWorkspace.ViewportSlots[slot] = placement
					changed = true
				}
			}
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

func multiSessionViewportPlacementValid(placement multiSessionViewportPlacement) bool {
	return placement.Position.X >= 0 && placement.Position.X <= 1 &&
		placement.Position.Y >= 0 && placement.Position.Y <= 1 &&
		placement.Size.X > 0 && placement.Size.X <= 1 &&
		placement.Size.Y > 0 && placement.Size.Y <= 1
}

func clampMultiSessionViewportPlacement(placement *multiSessionViewportPlacement) {
	if placement == nil {
		return
	}
	state := WindowState{Position: placement.Position, Size: placement.Size}
	clampWindowState(&state)
	placement.Position = state.Position
	placement.Size = state.Size
}
