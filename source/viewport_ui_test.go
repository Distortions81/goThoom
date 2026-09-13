package main

import (
	"context"
	"image"
	"math"
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

func TestViewportLoginOverlayOwnsFullImageAndTracksConnectionState(t *testing.T) {
	if err := eui.Init(); err != nil {
		t.Fatalf("initialize EUI: %v", err)
	}
	state := &viewportRenderState{window: newGameRenderWindow()}
	state.window.Size = eui.Point{X: 640, Y: 420}
	oldCharacters := characters
	characters = []Character{
		{Name: "Alice", passHash: "0123456789abcdef0123456789abcdef"},
		{Name: "Bob", DontRemember: true},
	}
	t.Cleanup(func() {
		characters = oldCharacters
		if state.imageBacking != nil {
			state.imageBacking.Deallocate()
		}
	})

	session := mustNewSession(2)
	session.login.setRequest(sessionLoginRequest{host: "example.test:5010", character: "Test Hero"})
	makeViewportLoginOverlay(state, session)
	updateViewportImageSize(state, false)
	refreshViewportLoginOverlay(state, session, true)

	if state.loginOverlay == nil || state.loginForm == nil {
		t.Fatal("viewport login overlay was not created")
	}
	if len(state.window.Contents) != 2 || state.window.Contents[0] != state.imageItem || state.window.Contents[1] != state.loginOverlay {
		t.Fatal("login overlay does not render above the viewport image")
	}
	if state.loginOverlay.Invisible {
		t.Fatal("disconnected session login overlay is hidden")
	}
	if state.loginOverlay.Size != state.imageItem.Size || state.loginOverlay.Position != state.imageItem.Position {
		t.Fatalf("overlay geometry = %+v at %+v, image = %+v at %+v", state.loginOverlay.Size, state.loginOverlay.Position, state.imageItem.Size, state.imageItem.Position)
	}
	if state.loginCharacter != "Test Hero" || state.loginCharacterItem.Text != "Test Hero" {
		t.Fatalf("character input = %q / %q", state.loginCharacter, state.loginCharacterItem.Text)
	}
	if state.loginSavedChoice == nil || len(state.loginSavedChoice.Options) != 3 || state.loginSavedChoice.Selected != 0 {
		t.Fatalf("saved character choices = %+v", state.loginSavedChoice)
	}
	if !selectViewportLoginSavedCharacter(state, session, "Alice") {
		t.Fatal("saved character could not be selected")
	}
	if state.loginCharacter != "Alice" || state.loginCharacterItem.Text != "Alice" || !state.loginRemember || state.loginSavedChoice.Selected != 1 {
		t.Fatalf("saved character selection = name:%q input:%q remember:%v selected:%d", state.loginCharacter, state.loginCharacterItem.Text, state.loginRemember, state.loginSavedChoice.Selected)
	}
	if state.loginAction.Text != "Connect" || state.loginCharacterItem.Disabled {
		t.Fatal("disconnected overlay is not connect-ready")
	}

	_, cancel := context.WithCancel(context.Background())
	if !session.transport.begin(cancel) {
		t.Fatal("could not put session into connecting state")
	}
	t.Cleanup(session.transport.failConnect)
	refreshViewportLoginOverlay(state, session, true)
	if state.loginAction.Text != "Cancel" || !state.loginSavedChoice.Disabled || !state.loginCharacterItem.Disabled || !state.loginPasswordItem.Disabled {
		t.Fatal("connecting overlay did not disable credentials and offer cancellation")
	}

	refreshViewportLoginOverlay(state, session, false)
	if !state.loginOverlay.Invisible {
		t.Fatal("single-session mode left the embedded login overlay visible")
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
	appViewports.enableMulti(viewportLayoutFreeform)
	state := appViewports.renderStateForViewport(2)
	state.window = eui.NewWindow()

	if uiOwnsPointerPress(false, state.window, true) {
		t.Fatal("secondary playfield retained a world press")
	}
	if !isViewportWindow(state.window) {
		t.Fatal("secondary playfield window was not recognized")
	}
}

func TestMultiSessionTiledLayoutAndFreeformRestore(t *testing.T) {
	if err := eui.Init(); err != nil {
		t.Fatalf("initialize EUI: %v", err)
	}
	oldScreenWidth, oldScreenHeight := eui.ScreenSize()
	oldSessions := appSessions
	oldViewports := appViewports
	oldGameWindow := gameWin
	oldGameImageItem, oldGameImage, oldGameImageBacking := gameImageItem, gameImage, gameImageBacking
	oldSettings := gs
	oldWorkspace := multiSessionWorkspace
	oldUsed, oldDirty := multiSessionWorkspaceUsed, multiSessionWorkspaceDirty
	oldApplied := viewportWorkspaceAppliedLayout
	oldAppliedWidth, oldAppliedHeight := viewportWorkspaceScreenWidth, viewportWorkspaceScreenHeight
	t.Cleanup(func() {
		for _, view := range appViewports.snapshot() {
			if view.render == nil {
				continue
			}
			if view.render.window != nil {
				view.render.window.RemoveWindow()
			}
			if view.render.imageBacking != nil {
				view.render.imageBacking.Deallocate()
			}
		}
		eui.SetScreenSize(oldScreenWidth, oldScreenHeight)
		appSessions = oldSessions
		appViewports = oldViewports
		gameWin = oldGameWindow
		gameImageItem, gameImage, gameImageBacking = oldGameImageItem, oldGameImage, oldGameImageBacking
		gs = oldSettings
		multiSessionWorkspace = oldWorkspace
		multiSessionWorkspaceUsed, multiSessionWorkspaceDirty = oldUsed, oldDirty
		viewportWorkspaceAppliedLayout = oldApplied
		viewportWorkspaceScreenWidth, viewportWorkspaceScreenHeight = oldAppliedWidth, oldAppliedHeight
	})

	eui.SetScreenSize(1000, 800)
	primary := mustNewSession(primarySessionID)
	appSessions = newSessionManager(primary)
	appSessions.mu.Lock()
	appSessions.multi = true
	appSessions.selected = 2
	for slot := 1; slot < maxSessions; slot++ {
		id, _ := sessionIDForSlot(slot)
		appSessions.slots[slot] = mustNewSession(id)
	}
	appSessions.mu.Unlock()
	appViewports = newViewportManager()
	appViewports.enableMulti(viewportLayoutTiled)
	gs.GameWindow = WindowState{Open: true, Position: WindowPoint{X: 0.1, Y: 0.1}, Size: WindowPoint{X: 0.8, Y: 0.8}}
	multiSessionWorkspace = defaultMultiSessionWorkspaceDocument()
	multiSessionWorkspaceUsed = true
	for slot := range multiSessionWorkspace.ViewportSlots {
		multiSessionWorkspace.ViewportSlots[slot] = multiSessionViewportPlacement{
			Position: WindowPoint{X: 0.05 + float64(slot)*0.1, Y: 0.05 + float64(slot)*0.08},
			Size:     WindowPoint{X: 0.32, Y: 0.265},
		}
	}

	gameWin = newGameRenderWindow()
	gameWin.Size = eui.Point{X: 640, Y: 392}
	gameWin.MarkOpen()
	bindPrimaryViewportWindow()
	views := appViewports.snapshot()
	for slot := 1; slot < maxSessions; slot++ {
		configureSecondaryViewportWindow(views[slot], appSessions.slots[slot])
	}
	applyMultiSessionViewportLayout(true)

	wantRects := tiledViewportRects(image.Rect(100, 80, 900, 720))
	for slot, view := range appViewports.snapshot() {
		win := view.render.window
		pos, size := win.GetPos(), win.GetSize()
		want := wantRects[slot]
		if int(pos.X) != want.Min.X || int(pos.Y) != want.Min.Y || int(size.X) != want.Dx() || int(size.Y) != want.Dy() {
			t.Fatalf("tile %d = %.0f,%.0f %.0fx%.0f, want %v", slot, pos.X, pos.Y, size.X, size.Y, want)
		}
		if !win.Docked || win.Movable || win.Resizable {
			t.Fatalf("tile %d chrome is not fixed and docked", slot)
		}
	}
	selectedWindow := appViewports.renderStateForSession(2).window
	if !selectedWindow.Outlined || selectedWindow.BorderColor != eui.AccentColor() {
		t.Fatal("selected tile does not have the accent outline")
	}

	appViewports.setLayout(viewportLayoutFreeform)
	applyMultiSessionViewportLayout(true)
	for slot, view := range appViewports.snapshot() {
		win := view.render.window
		pos := win.GetPos()
		want := multiSessionWorkspace.ViewportSlots[slot]
		if int(pos.X) != int(math.Round(want.Position.X*1000)) || int(pos.Y) != int(math.Round(want.Position.Y*800)) {
			t.Fatalf("freeform slot %d position = %.0f,%.0f, want %+v", slot, pos.X, pos.Y, want.Position)
		}
		if win.Docked || !win.Movable || !win.Resizable {
			t.Fatalf("freeform slot %d chrome was not restored", slot)
		}
	}
}
