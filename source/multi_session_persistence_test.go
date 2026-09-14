package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMultiSessionWorkspaceIsLazyAndRoundTripsTabs(t *testing.T) {
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

	open := [maxSessions]bool{}
	open[0], open[2], open[9] = true, true, true
	appSessions.restoreTabs(open, 3)
	multiSessionWorkspace.TabHotkeysInitialized = true
	multiSessionWorkspace.TabCycleInitialized = true
	multiSessionWorkspace.ClientHotkeysVersion = clientHotkeysVersion
	multiSessionWorkspaceUsed = true
	multiSessionWorkspaceDirty = true
	saveMultiSessionWorkspace()

	multiSessionWorkspace = defaultMultiSessionWorkspaceDocument()
	multiSessionWorkspaceUsed = false
	if !loadMultiSessionWorkspace() {
		t.Fatal("saved workspace did not load")
	}
	if multiSessionWorkspace.Selected != 3 || multiSessionWorkspace.OpenTabs != open {
		t.Fatalf("loaded workspace = %+v", multiSessionWorkspace)
	}
	if !multiSessionWorkspace.TabHotkeysInitialized {
		t.Fatal("tab hotkey migration marker was not restored")
	}
	if !multiSessionWorkspace.TabCycleInitialized {
		t.Fatal("tab cycle hotkey migration marker was not restored")
	}
	if multiSessionWorkspace.ClientHotkeysVersion != clientHotkeysVersion {
		t.Fatalf("client hotkey version = %d", multiSessionWorkspace.ClientHotkeysVersion)
	}
}

func TestMultiSessionWorkspaceV1MigratesFourPanesToTabs(t *testing.T) {
	oldDataDir := dataDirPath
	oldWorkspace := multiSessionWorkspace
	oldUsed, oldDirty := multiSessionWorkspaceUsed, multiSessionWorkspaceDirty
	t.Cleanup(func() {
		dataDirPath = oldDataDir
		multiSessionWorkspace = oldWorkspace
		multiSessionWorkspaceUsed, multiSessionWorkspaceDirty = oldUsed, oldDirty
	})
	dataDirPath = t.TempDir()
	data := []byte(`{"version":1,"selected_session":4,"music_source_session":2}`)
	if err := os.WriteFile(filepath.Join(dataDirPath, multiSessionWorkspaceFile), data, 0o644); err != nil {
		t.Fatalf("write v1 workspace: %v", err)
	}
	if !loadMultiSessionWorkspace() {
		t.Fatal("v1 workspace did not load")
	}
	for slot, open := range multiSessionWorkspace.OpenTabs {
		if open != (slot < 4) {
			t.Fatalf("migrated open tab %d = %v", slot+1, open)
		}
	}
	if multiSessionWorkspace.Selected != 4 || !multiSessionWorkspaceDirty {
		t.Fatalf("migration result = %+v dirty=%v", multiSessionWorkspace, multiSessionWorkspaceDirty)
	}
}
