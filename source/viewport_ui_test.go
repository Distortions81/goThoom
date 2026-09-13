package main

import (
	"context"
	"net"
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

func TestViewportLoginPanelAndLogoutTrackConnectionState(t *testing.T) {
	if err := eui.Init(); err != nil {
		t.Fatalf("initialize EUI: %v", err)
	}
	state := &viewportRenderState{window: newGameRenderWindow()}
	state.window.Size = eui.Point{X: 640, Y: 420}
	oldCharacters := characters
	oldSettings := gs
	gs.ServerAddress = "example.test:5010"
	gs.ServerAddresses = []string{gs.ServerAddress}
	gs.LastCharacter = ""
	characters = []Character{
		{Name: "Alice", passHash: "0123456789abcdef0123456789abcdef"},
		{Name: "Bob", DontRemember: true},
	}
	t.Cleanup(func() {
		characters = oldCharacters
		gs = oldSettings
		if state.imageBacking != nil {
			state.imageBacking.Deallocate()
		}
	})

	session := mustNewSession(2)
	oldSessions, oldWorkspaceUpdate := appSessions, queueSessionWorkspaceUIUpdate
	appSessions = newSessionManager(mustNewSession(primarySessionID))
	appSessions.mu.Lock()
	appSessions.slots[1] = session
	appSessions.mu.Unlock()
	queueSessionWorkspaceUIUpdate = func() {}
	t.Cleanup(func() {
		appSessions = oldSessions
		queueSessionWorkspaceUIUpdate = oldWorkspaceUpdate
	})
	session.login.setRequest(sessionLoginRequest{host: "example.test:5010", character: "Test Hero"})
	makeViewportLoginOverlay(state, session)
	updateViewportImageSize(state, false)
	refreshViewportLoginOverlay(state, session, true)

	if state.loginOverlay == nil || state.loginForm == nil {
		t.Fatal("viewport login overlay was not created")
	}
	if len(state.window.Contents) != 3 || state.window.Contents[0] != state.imageItem || state.window.Contents[1] != state.loginOverlay || state.window.Contents[2] != state.sessionLogout {
		t.Fatal("login panel and logout action do not render above the viewport image")
	}
	if state.loginOverlay.Invisible {
		t.Fatal("disconnected session login overlay is hidden")
	}
	if state.loginOverlay.GetSize().X >= state.imageItem.GetSize().X || state.loginOverlay.GetSize().Y >= state.imageItem.GetSize().Y {
		t.Fatalf("login background covers the full viewport: panel %+v, image %+v", state.loginOverlay.GetSize(), state.imageItem.GetSize())
	}
	if state.loginOverlay.Color.A == 0 || state.loginOverlay.Color.A >= state.window.Theme.Window.BGColor.A {
		t.Fatalf("login background alpha = %d, theme alpha = %d", state.loginOverlay.Color.A, state.window.Theme.Window.BGColor.A)
	}
	if state.loginForm.Position != (eui.Point{X: viewportLoginPanelPadding, Y: viewportLoginPanelPadding}) {
		t.Fatalf("login form inset = %+v", state.loginForm.Position)
	}
	if state.sessionLogout == nil || !state.sessionLogout.Invisible {
		t.Fatal("disconnected session exposes its logout action")
	}
	if state.loginCharacter != "" {
		t.Fatalf("invalid saved-character selection was retained: %q", state.loginCharacter)
	}
	if state.loginCharacters == nil || len(state.loginCharacters.Contents) != 3 {
		t.Fatalf("saved character rows = %+v", state.loginCharacters)
	}
	if !state.loginAction.Disabled {
		t.Fatal("connect action is enabled without a character selection")
	}
	if got := state.loginServerChoice.Options[len(state.loginServerChoice.Options)-1]; got != editServerListOption {
		t.Fatalf("last server option = %q, want %q", got, editServerListOption)
	}
	if !selectViewportLoginSavedCharacter(state, session, "Alice") {
		t.Fatal("saved character could not be selected")
	}
	if state.loginCharacter != "Alice" || !loginCharacterRowChecked(state.loginCharacters, "Alice") {
		t.Fatalf("saved character selection = %q", state.loginCharacter)
	}
	if state.loginAction.Text != "Connect" || state.loginCharacters.Disabled {
		t.Fatal("disconnected overlay is not connect-ready")
	}

	_, cancel := context.WithCancel(context.Background())
	if !session.transport.begin(cancel) {
		t.Fatal("could not put session into connecting state")
	}
	t.Cleanup(session.transport.failConnect)
	refreshViewportLoginOverlay(state, session, true)
	if state.loginAction.Text != "Cancel" || !state.loginCharacters.Disabled || !state.loginAdd.Disabled || !state.loginEdit.Disabled || !state.loginDelete.Disabled {
		t.Fatal("connecting overlay did not disable credentials and offer cancellation")
	}

	refreshViewportLoginOverlay(state, session, false)
	if !state.loginOverlay.Invisible {
		t.Fatal("single-session mode left the embedded login overlay visible")
	}

	session.transport.failConnect()
	tcp, tcpPeer := net.Pipe()
	udp, udpPeer := net.Pipe()
	t.Cleanup(func() {
		session.transport.disconnect()
		_ = tcpPeer.Close()
		_ = udpPeer.Close()
	})
	if _, ok := session.transport.attach(tcp, udp); !ok {
		t.Fatal("could not put session into connected state")
	}
	refreshViewportLoginOverlay(state, session, true)
	if !state.loginOverlay.Invisible || state.sessionLogout.Invisible {
		t.Fatal("connected session did not replace login controls with its logout action")
	}
	bounds := state.image.Bounds()
	buttonSize := state.sessionLogout.GetSize()
	wantX := state.imageItem.Position.X + float32(bounds.Dx()) - buttonSize.X - viewportControlInset
	wantY := state.imageItem.Position.Y + float32(bounds.Dy()) - buttonSize.Y - viewportControlInset
	if state.sessionLogout.Position != (eui.Point{X: wantX, Y: wantY}) {
		t.Fatalf("logout position = %+v, want bottom-right %+v", state.sessionLogout.Position, eui.Point{X: wantX, Y: wantY})
	}
	state.sessionLogout.Handler.Handle(eui.UIEvent{Type: eui.EventClick})
	if session.transport.connected() {
		t.Fatal("viewport logout action left its session connected")
	}
}

func loginCharacterRowChecked(list *eui.ItemData, characterName string) bool {
	if list == nil {
		return false
	}
	for _, row := range list.Contents {
		if row == nil || len(row.Contents) < 3 {
			continue
		}
		radio := row.Contents[2]
		if radio != nil && radio.Text == characterName {
			return radio.Checked
		}
	}
	return false
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

func TestPrimaryViewportLoginTargetDoesNotReopenStandaloneLogin(t *testing.T) {
	if err := eui.Init(); err != nil {
		t.Fatalf("initialize EUI: %v", err)
	}
	oldLogin, oldSessions, oldViewports := loginWin, appSessions, appViewports
	oldName := name
	t.Cleanup(func() {
		if loginWin != nil && loginWin != oldLogin {
			loginWin.RemoveWindow()
		}
		loginWin, appSessions, appViewports, name = oldLogin, oldSessions, oldViewports, oldName
	})

	loginWin = eui.NewWindow()
	loginWin.AddWindow(false)
	appSessions = newSessionManager(primarySession)
	appViewports = newViewportManager()
	state := appViewports.renderStateForSession(primarySessionID)
	state.window = eui.NewWindow()
	state.loginOverlay = eui.NewColumn()
	state.window.AddWindow(false)
	state.window.MarkOpen()
	t.Cleanup(state.window.RemoveWindow)

	target := loginSurfaceTarget{sessionID: primarySessionID, viewport: true}
	restoreLoginTarget(target)
	if loginWin.IsOpen() {
		t.Fatal("returning to session 1's viewport reopened the standalone Login window")
	}
	name = "Standalone Hero"
	selectCharacterForLoginTarget(target, "Viewport Hero", "hash")
	if name != "Standalone Hero" || state.loginCharacter != "Viewport Hero" {
		t.Fatalf("primary viewport selection leaked into standalone login: name=%q viewport=%q", name, state.loginCharacter)
	}

	restoreLoginTarget(loginSurfaceTarget{sessionID: primarySessionID})
	if !loginWin.IsOpen() {
		t.Fatal("standalone login target did not reopen the Login window")
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
