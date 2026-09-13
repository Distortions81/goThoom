package main

import (
	"context"
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
	t.Cleanup(func() {
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
	if state.loginAction.Text != "Connect" || state.loginCharacterItem.Disabled {
		t.Fatal("disconnected overlay is not connect-ready")
	}

	_, cancel := context.WithCancel(context.Background())
	if !session.transport.begin(cancel) {
		t.Fatal("could not put session into connecting state")
	}
	t.Cleanup(session.transport.failConnect)
	refreshViewportLoginOverlay(state, session, true)
	if state.loginAction.Text != "Cancel" || !state.loginCharacterItem.Disabled || !state.loginPasswordItem.Disabled {
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
