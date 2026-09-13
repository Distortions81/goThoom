package main

import (
	"math"
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
	oldScale := eui.UIScale()
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
		eui.SetUIScale(oldScale)
		multiSessionWorkspace = oldWorkspace
		multiSessionWorkspaceUsed, multiSessionWorkspaceDirty = oldWorkspaceUsed, oldWorkspaceDirty
		appMusicSource.mu.Lock()
		appMusicSource.source, appMusicSource.generation = oldMusicSource, oldMusicGeneration
		appMusicSource.mu.Unlock()
	})

	fake, clmov, pcapPath = false, "", ""
	eui.SetUIScale(2)
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
	wantWidth := gameWin.GetSize().X - 2*(gameWin.Padding+gameWin.BorderPad)
	if got := sessionTabBar.GetSize().X; math.Abs(float64(got-wantWidth)) > 0.01 {
		t.Fatalf("ten-tab strip width = %.2f, want %.2f", got, wantWidth)
	}
	if got := sessionTabBar.GetSize().Y; math.Abs(float64(got-sessionTabBarHeight)) > 0.01 {
		t.Fatalf("tab strip height = %.2f, want %d; controls introduced vertical overflow", got, sessionTabBarHeight)
	}
	var childWidth float32
	for _, item := range sessionTabBar.Contents {
		childWidth += item.GetSize().X
	}
	if math.Abs(float64(childWidth-wantWidth)) > 0.01 {
		t.Fatalf("tab and add widths = %.2f, want %.2f", childWidth, wantWidth)
	}
	firstTab := sessionTabBar.Contents[0]
	selectButton, closeButton := firstTab.Contents[0], firstTab.Contents[1]
	if got, want := selectButton.GetSize().X, firstTab.GetSize().X; math.Abs(float64(got-want)) > 0.01 {
		t.Fatalf("tab button width = %.2f, want enclosing tab width %.2f", got, want)
	}
	if got, want := closeButton.Position.X, -closeButton.Size.X; math.Abs(float64(got-want)) > 0.01 {
		t.Fatalf("close button x offset = %.2f, want %.2f so it sits inside its tab", got, want)
	}

	originalTabWidth := firstTab.GetSize().X
	gameWin.Size.X /= 2
	refreshSessionTabs()
	wantWidth = gameWin.GetSize().X - 2*(gameWin.Padding+gameWin.BorderPad)
	if got := sessionTabBar.GetSize().X; math.Abs(float64(got-wantWidth)) > 0.01 {
		t.Fatalf("resized tab strip width = %.2f, want available %.2f", got, wantWidth)
	}
	if got := sessionTabBar.Contents[0].GetSize().X; got >= originalTabWidth {
		t.Fatalf("tab width did not shrink with available space: %.2f >= %.2f", got, originalTabWidth)
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
