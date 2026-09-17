package main

import (
	"math"
	"net"
	"testing"
	"time"

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
	oldMusicIndicators, oldMusicIndicatorRun := sessionMusicIndicators, lastSessionMusicIndicatorRun
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
		sessionMusicIndicators, lastSessionMusicIndicatorRun = oldMusicIndicators, oldMusicIndicatorRun
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
	if closeButton := sessionTabBar.Contents[0].Contents[2]; !closeButton.Disabled {
		t.Fatal("the disconnected main tab's disconnect button is enabled")
	}
	primary, _ := appSessions.session(primarySessionID)
	cancelled := false
	generation, ok := primary.login.beginSupervisor(func() { cancelled = true })
	if !ok {
		t.Fatal("could not start test login")
	}
	t.Cleanup(func() { primary.login.finishSupervisor(generation) })
	refreshSessionTabs()
	if sessionTabBar.Contents[0].Contents[2].Disabled {
		t.Fatal("the final tab cannot disconnect a pending login")
	}
	popup := confirmCloseSessionTab(primary)
	clickMacroEditorButton(t, popup, "Disconnect")
	if !cancelled || appSessions.count() != 1 || appSessions.selectedSession() != primary {
		t.Fatal("disconnect did not cancel login while preserving the final tab")
	}
	primary.login.finishSupervisor(generation)
	refreshSessionTabs()
	if !sessionTabBar.Contents[0].Contents[0].SelectionIndicator {
		t.Fatal("selected session tab is missing its highlight")
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
	selectButton, closeButton := firstTab.Contents[0], firstTab.Contents[2]
	if firstTab.FlowType != eui.FLOW_OVERLAY {
		t.Fatal("session actions are not contained by an overlay tab")
	}
	if got, want := selectButton.GetSize().X, firstTab.GetSize().X; math.Abs(float64(got-want)) > 0.01 {
		t.Fatalf("tab button width = %.2f, want enclosing tab width %.2f", got, want)
	}
	if got, want := closeButton.Position.X+closeButton.GetSize().X, firstTab.GetSize().X; math.Abs(float64(got-want)) > 0.01 {
		t.Fatalf("close button right edge = %.2f, want tab edge %.2f", got, want)
	}
	if !closeButton.NoSurface {
		t.Fatal("close action draws a separate button surface inside the tab")
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

	second, ok := appSessions.session(2)
	if !ok {
		t.Fatal("second session is unavailable")
	}
	now := time.Unix(100, 0)
	second.music.startTracks([]tuneJob{{notes: []Note{{Duration: 10 * time.Second}}}}, now)
	refreshSessionMusicIndicators(now.Add(time.Second))
	playingTab := sessionTabBar.Contents[1]
	if len(playingTab.Contents) < 3 || playingTab.Contents[2].ImageName != "music_note" {
		t.Fatal("playing tab does not contain a music-note icon before its actions")
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

func TestSessionTabsRemainVisibleDuringMoviePlayback(t *testing.T) {
	oldFake, oldMovie, oldPCAP, oldPreview := fake, clmov, pcapPath, setupWizardPreviewActive
	t.Cleanup(func() {
		fake, clmov, pcapPath, setupWizardPreviewActive = oldFake, oldMovie, oldPCAP, oldPreview
	})
	fake, clmov, pcapPath, setupWizardPreviewActive = false, "movie.clMov", "", false
	if !sessionTabsVisible() {
		t.Fatal("session tabs were hidden during movie playback")
	}
}

func TestSessionTabCyclingUsesOpenOrderAndWraps(t *testing.T) {
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
	if !appSessions.closeSession(2) || !appSessions.selectSession(1) {
		t.Fatal("could not arrange open sessions")
	}
	for _, want := range []SessionID{3, 4, 1} {
		if !selectAdjacentSessionTab(1) || appSessions.selectedID() != want {
			t.Fatalf("forward selected %d, want %d", appSessions.selectedID(), want)
		}
	}
	if !selectAdjacentSessionTab(-1) || appSessions.selectedID() != 4 {
		t.Fatalf("backward wrap selected %d, want 4", appSessions.selectedID())
	}
}

func TestSessionTabDisconnectKeepsMainAndLastTab(t *testing.T) {
	initFont()
	for _, scenario := range []string{"main", "main-with-other", "last-secondary"} {
		t.Run(scenario, func(t *testing.T) {
			oldSessions := appSessions
			t.Cleanup(func() { appSessions = oldSessions })
			primary := mustNewSession(primarySessionID)
			other := mustNewSession(2)
			appSessions = newSessionManager(primary)
			target := primary
			if scenario != "main" {
				appSessions.slots[1], appSessions.selected = other, 2
			}
			if scenario == "last-secondary" {
				appSessions.slots[0], target = nil, other
			}
			connect := func(session *Session) {
				tcp, tcpPeer := net.Pipe()
				udp, udpPeer := net.Pipe()
				t.Cleanup(func() { session.transport.disconnect(); tcpPeer.Close(); udpPeer.Close() })
				if _, ok := session.transport.attach(tcp, udp); !ok {
					t.Fatal("attach test transport")
				}
			}
			connect(target)
			if scenario == "main-with-other" {
				connect(other)
			}
			count, selected := appSessions.count(), appSessions.selectedID()
			popup := confirmCloseSessionTab(target)
			clickMacroEditorButton(t, popup, "Cancel")
			if !target.transport.connected() {
				t.Fatal("Cancel disconnected the session")
			}
			popup = confirmCloseSessionTab(target)
			clickMacroEditorButton(t, popup, "Disconnect")
			if target.transport.connected() || appSessions.count() != count || appSessions.selectedID() != selected {
				t.Fatal("disconnect changed open tabs or selection")
			}
			if retained, ok := appSessions.session(target.ID()); !ok || retained != target {
				t.Fatal("disconnect removed the session tab")
			}
			if scenario == "main-with-other" && !other.transport.connected() {
				t.Fatal("disconnect affected another session")
			}
		})
	}
}
