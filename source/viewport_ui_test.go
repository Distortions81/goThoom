package main

import (
	"testing"

	"gothoom/eui"
)

func TestViewportImageBuffersAreIndependent(t *testing.T) {
	first := &viewportRenderState{window: newGameRenderWindow()}
	second := &viewportRenderState{window: newGameRenderWindow()}
	first.window.Size = eui.Point{X: 400, Y: 300}
	second.window.Size = eui.Point{X: 400, Y: 300}

	updateViewportImageSize(first, false)
	updateViewportImageSize(second, false)
	t.Cleanup(func() {
		if first.imageBacking != nil {
			first.imageBacking.Deallocate()
		}
		if second.imageBacking != nil {
			second.imageBacking.Deallocate()
		}
	})

	if first.image == nil || second.image == nil {
		t.Fatal("viewport images were not allocated")
	}
	if first.image == second.image || first.imageBacking == second.imageBacking || first.imageItem == second.imageItem {
		t.Fatal("viewport image resources alias each other")
	}

	secondBacking := second.imageBacking
	first.window.Size = eui.Point{X: 800, Y: 600}
	updateViewportImageSize(first, false)
	if second.imageBacking != secondBacking {
		t.Fatal("resizing one viewport replaced another viewport's backing image")
	}
}

func TestSecondarySessionUsesStandaloneLoginWindow(t *testing.T) {
	if err := eui.Init(); err != nil {
		t.Fatalf("initialize EUI: %v", err)
	}
	oldLogin := loginWin
	oldCharactersList := charactersList
	oldEdit, oldDelete, oldConnect, oldServer := editCharBtn, deleteCharBtn, loginConnectButton, loginServerDropdown
	oldCharacters, oldSettings := characters, gs
	oldSessions, oldViewports := appSessions, appViewports
	loginWin = nil
	charactersList = nil
	gs.ServerAddress = "example.test:5010"
	gs.ServerAddresses = []string{gs.ServerAddress}
	gs.LastCharacter = "Alice"
	characters = []Character{
		{Name: "Alice", ServerSlot: 1, passHash: "0123456789abcdef0123456789abcdef"},
	}
	t.Cleanup(func() {
		if loginWin != nil && loginWin != oldLogin {
			loginWin.RemoveWindow()
		}
		loginWin, charactersList = oldLogin, oldCharactersList
		editCharBtn, deleteCharBtn, loginConnectButton, loginServerDropdown = oldEdit, oldDelete, oldConnect, oldServer
		characters, gs = oldCharacters, oldSettings
		appSessions, appViewports = oldSessions, oldViewports
	})

	session := mustNewSession(2)
	appSessions = newSessionManager(mustNewSession(primarySessionID))
	appSessions.mu.Lock()
	appSessions.slots[1] = session
	appSessions.selected = 2
	appSessions.mu.Unlock()
	appViewports = newViewportManager()
	makeLoginWindow()
	updateCharacterButtons()
	state := appViewports.renderStateForSession(2)
	if loginWin.Title != "Login — Session 2" {
		t.Fatalf("login title = %q", loginWin.Title)
	}
	if state.loginCharacter != "Alice" || selectedLoginSelection() != "Alice" {
		t.Fatalf("secondary login selection = %q", state.loginCharacter)
	}
	if len(charactersList.Contents) != 2 {
		t.Fatalf("standalone character rows = %d, want saved character and demo", len(charactersList.Contents))
	}
	if loginConnectButton.Text != "Connect" || loginConnectButton.Disabled {
		t.Fatal("standalone secondary login is not ready to connect")
	}
}

func TestSecondaryViewportWindowIsTransparentToWorldInput(t *testing.T) {
	oldViewports := appViewports
	oldGameWindow := gameWin
	t.Cleanup(func() {
		appViewports = oldViewports
		gameWin = oldGameWindow
	})

	appViewports = newViewportManager()
	appViewports.showSession(2)
	state := appViewports.renderStateForViewport(2)
	state.window = eui.NewWindow()

	if uiOwnsPointerPress(false, state.window, true) {
		t.Fatal("secondary playfield retained a world press")
	}
	if !isViewportWindow(state.window) {
		t.Fatal("secondary playfield window was not recognized")
	}
}

func TestRestoreLoginTargetReopensStandaloneLogin(t *testing.T) {
	if err := eui.Init(); err != nil {
		t.Fatalf("initialize EUI: %v", err)
	}
	oldLogin, oldSessions, oldViewports := loginWin, appSessions, appViewports
	t.Cleanup(func() {
		if loginWin != nil && loginWin != oldLogin {
			loginWin.RemoveWindow()
		}
		loginWin, appSessions, appViewports = oldLogin, oldSessions, oldViewports
	})

	loginWin = eui.NewWindow()
	loginWin.AddWindow(false)
	appSessions = newSessionManager(primarySession)
	appViewports = newViewportManager()
	restoreLoginTarget(loginSurfaceTarget{sessionID: primarySessionID})
	if !loginWin.IsOpen() {
		t.Fatal("login target did not reopen the standalone Login window")
	}
}

func TestTabSwitchRebindsOneSharedGameSurface(t *testing.T) {
	if err := eui.Init(); err != nil {
		t.Fatalf("initialize EUI: %v", err)
	}
	oldSessions, oldViewports := appSessions, appViewports
	oldGameWindow, oldLoginWindow := gameWin, loginWin
	oldImageItem, oldImage, oldBacking := gameImageItem, gameImage, gameImageBacking
	oldTabBar := sessionTabBar
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
		if gameImageBacking != nil && gameImageBacking != oldBacking {
			gameImageBacking.Deallocate()
		}
		appSessions, appViewports = oldSessions, oldViewports
		gameWin, loginWin = oldGameWindow, oldLoginWindow
		gameImageItem, gameImage, gameImageBacking = oldImageItem, oldImage, oldBacking
		sessionTabBar = oldTabBar
		fake, clmov, pcapPath = oldFake, oldMovie, oldPCAP
		multiSessionWorkspace = oldWorkspace
		multiSessionWorkspaceUsed, multiSessionWorkspaceDirty = oldWorkspaceUsed, oldWorkspaceDirty
		appMusicSource.mu.Lock()
		appMusicSource.source, appMusicSource.generation = oldMusicSource, oldMusicGeneration
		appMusicSource.mu.Unlock()
	})

	fake, clmov, pcapPath = false, "", ""
	loginWin = nil
	appSessions = newSessionManager(mustNewSession(primarySessionID))
	open := [maxSessions]bool{}
	open[0], open[1] = true, true
	appSessions.restoreTabs(open, 2)
	appViewports = newViewportManager()
	gameWin = newGameRenderWindow()
	gameWin.Size = eui.Point{X: 640, Y: 420}
	gameImageItem, gameImageBacking = eui.NewImageFastItem(640, 360)
	gameImage = gameImageBacking
	gameImageItem.Image = gameImage
	gameWin.AddItem(gameImageItem)
	gameWin.AddWindow(false)
	sessionTabBar = nil

	refreshViewportWorkspace()
	first := appViewports.renderStateForSession(1)
	second := appViewports.renderStateForSession(2)
	if second.window != gameWin || second.imageBacking != gameImageBacking || first.window != nil {
		t.Fatalf("session 2 binding = first window %p, second window %p backing %p", first.window, second.window, second.imageBacking)
	}
	if !appViewports.snapshot()[1].Active || appViewports.snapshot()[0].Active {
		t.Fatal("more than the selected viewport is active")
	}

	if !appSessions.selectSession(1) {
		t.Fatal("could not select session 1")
	}
	refreshViewportWorkspace()
	if first.window != gameWin || first.imageBacking != gameImageBacking || second.window != nil || second.imageBacking != nil {
		t.Fatalf("session 1 binding = first window %p backing %p, second window %p backing %p", first.window, first.imageBacking, second.window, second.imageBacking)
	}
}
