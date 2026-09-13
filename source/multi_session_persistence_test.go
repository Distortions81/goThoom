package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestMultiSessionWorkspaceIsLazyAndRoundTrips(t *testing.T) {
	oldDataDir := dataDirPath
	oldSessions := appSessions
	oldWorkspace := multiSessionWorkspace
	oldUsed := multiSessionWorkspaceUsed
	oldDirty := multiSessionWorkspaceDirty
	t.Cleanup(func() {
		dataDirPath = oldDataDir
		appSessions = oldSessions
		multiSessionWorkspace = oldWorkspace
		multiSessionWorkspaceUsed = oldUsed
		multiSessionWorkspaceDirty = oldDirty
	})

	dataDirPath = t.TempDir()
	appSessions = newSessionManager(mustNewSession(primarySessionID))
	multiSessionWorkspace = defaultMultiSessionWorkspaceDocument()
	multiSessionWorkspaceUsed = false
	multiSessionWorkspaceDirty = false

	saveMultiSessionWorkspace()
	path := filepath.Join(dataDirPath, multiSessionWorkspaceFile)
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("unused workspace file exists: %v", err)
	}

	multiSessionWorkspace = multiSessionWorkspaceDocument{
		Version:     multiSessionWorkspaceVersion,
		Selected:    3,
		MusicSource: 4,
	}
	multiSessionWorkspace.ViewportSlots[0] = multiSessionViewportPlacement{
		Position: WindowPoint{X: 0.1, Y: 0.2},
		Size:     WindowPoint{X: 0.4, Y: 0.5},
	}
	multiSessionWorkspaceUsed = true
	multiSessionWorkspaceDirty = true
	saveMultiSessionWorkspace()
	saved, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read workspace: %v", err)
	}
	if bytes.Contains(saved, []byte(`"layout"`)) {
		t.Fatalf("workspace retained an independent layout preference: %s", saved)
	}

	multiSessionWorkspace = defaultMultiSessionWorkspaceDocument()
	multiSessionWorkspaceUsed = false
	if !loadMultiSessionWorkspace() {
		t.Fatal("saved workspace did not load")
	}
	if multiSessionWorkspace.Selected != 3 || multiSessionWorkspace.MusicSource != 4 {
		t.Fatalf("loaded workspace = %+v", multiSessionWorkspace)
	}
	want := multiSessionViewportPlacement{Position: WindowPoint{X: 0.1, Y: 0.2}, Size: WindowPoint{X: 0.4, Y: 0.5}}
	if got := multiSessionWorkspace.ViewportSlots[0]; got != want {
		t.Fatalf("loaded viewport = %+v, want %+v", got, want)
	}
}
