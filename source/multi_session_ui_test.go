package main

import (
	"testing"

	"gothoom/eui"
)

func TestSessionTabBarAddsUpToTenSessions(t *testing.T) {
	if err := eui.Init(); err != nil {
		t.Fatalf("initialize EUI: %v", err)
	}
	initFont()
	oldSessions, oldViewports := appSessions, appViewports
	oldGameWindow, oldTabBar := gameWin, sessionTabBar
	oldFake, oldMovie, oldPCAP := fake, clmov, pcapPath
	oldWorkspace := multiSessionWorkspace
	oldWorkspaceUsed, oldWorkspaceDirty := multiSessionWorkspaceUsed, multiSessionWorkspaceDirty
	appMusicSource.mu.RLock()
	oldMusicSource, oldMusicGeneration := appMusicSource.source, appMusicSource.generation
	appMusicSource.mu.RUnlock()
	t.Cleanup(func() {
		if gameWin != nil && gameWin != oldGameWindow {
			gameWin.RemoveWindow()
		}
		appSessions, appViewports = oldSessions, oldViewports
		gameWin, sessionTabBar = oldGameWindow, oldTabBar
		fake, clmov, pcapPath = oldFake, oldMovie, oldPCAP
		multiSessionWorkspace = oldWorkspace
		multiSessionWorkspaceUsed, multiSessionWorkspaceDirty = oldWorkspaceUsed, oldWorkspaceDirty
		appMusicSource.mu.Lock()
		appMusicSource.source, appMusicSource.generation = oldMusicSource, oldMusicGeneration
		appMusicSource.mu.Unlock()
	})

	fake, clmov, pcapPath = false, "", ""
	appSessions = newSessionManager(mustNewSession(primarySessionID))
	appViewports = newViewportManager()
	gameWin = newGameRenderWindow()
	gameWin.Size = eui.Point{X: 900, Y: 600}
	gameWin.AddWindow(false)
	sessionTabBar = nil

	refreshSessionTabs()
	if sessionTabBar == nil || len(sessionTabBar.Contents) != 2 {
		t.Fatalf("initial tab bar has %d items, want one tab and add", len(sessionTabBar.Contents))
	}
	if closeButton := sessionTabBar.Contents[0].Contents[1]; !closeButton.Disabled {
		t.Fatal("the final tab's close button is enabled")
	}
	sessionTabBar.Contents[1].Handler.Handle(eui.UIEvent{Type: eui.EventClick})
	if appSessions.count() != 2 || appSessions.selectedID() != 2 {
		t.Fatalf("add button produced count/selection %d/%d", appSessions.count(), appSessions.selectedID())
	}
	for appSessions.count() < maxSessions {
		if _, ok := appSessions.addSession(); !ok {
			t.Fatal("could not add session before reaching limit")
		}
	}
	refreshSessionTabs()
	if got := appSessions.count(); got != 10 {
		t.Fatalf("session count = %d, want 10", got)
	}
	if got := len(sessionTabBar.Contents); got != maxSessions+1 {
		t.Fatalf("tab bar item count = %d, want %d", got, maxSessions+1)
	}
	addButton := sessionTabBar.Contents[len(sessionTabBar.Contents)-1]
	if !addButton.Disabled {
		t.Fatal("add button is enabled at the ten-session limit")
	}
	if _, ok := appSessions.addSession(); ok {
		t.Fatal("added an eleventh session")
	}
}

func TestSessionTabPositionUsesOpenTabOrder(t *testing.T) {
	oldSessions := appSessions
	oldWorkspace := multiSessionWorkspace
	oldWorkspaceUsed, oldWorkspaceDirty := multiSessionWorkspaceUsed, multiSessionWorkspaceDirty
	appMusicSource.mu.RLock()
	oldMusicSource, oldMusicGeneration := appMusicSource.source, appMusicSource.generation
	appMusicSource.mu.RUnlock()
	t.Cleanup(func() {
		appSessions = oldSessions
		multiSessionWorkspace = oldWorkspace
		multiSessionWorkspaceUsed, multiSessionWorkspaceDirty = oldWorkspaceUsed, oldWorkspaceDirty
		appMusicSource.mu.Lock()
		appMusicSource.source, appMusicSource.generation = oldMusicSource, oldMusicGeneration
		appMusicSource.mu.Unlock()
	})
	appSessions = newSessionManager(mustNewSession(primarySessionID))
	for range 3 {
		appSessions.addSession()
	}
	if !appSessions.closeSession(2) {
		t.Fatal("could not close the second session")
	}
	if !selectSessionTabPosition(2) {
		t.Fatal("could not select the second open tab")
	}
	if got := appSessions.selectedID(); got != 3 {
		t.Fatalf("selected session = %d, want session 3", got)
	}
}
