package main

import (
	"errors"
	"image"
	"math"

	"gothoom/eui"

	"github.com/hajimehoshi/ebiten/v2"
)

func refreshViewportTitles() {
	views := appViewports.snapshot()
	sessions := appSessions.snapshot()
	for slot, view := range views {
		if view.render != nil && view.render.window != nil && sessions[slot] != nil {
			view.render.window.Title = gameWindowTitle()
			view.render.window.Dirty = true
		}
	}
	refreshViewportSelectionTreatment()
}

func bindSelectedViewportWindow() {
	state := appViewports.renderStateForSession(appSessions.selectedID())
	if state == nil {
		return
	}
	sharedLightingTmp := state.lightingTmp
	sharedPuddleReflectionTmp := state.puddleReflectionTmp
	for _, view := range appViewports.snapshot() {
		other := view.render
		if other == nil || other == state {
			continue
		}
		if other.window == gameWin {
			if other.lightingTmp != nil {
				if sharedLightingTmp != nil && sharedLightingTmp != other.lightingTmp {
					sharedLightingTmp.Deallocate()
				}
				sharedLightingTmp = other.lightingTmp
				other.lightingTmp = nil
			}
			if other.puddleReflectionTmp != nil {
				if sharedPuddleReflectionTmp != nil && sharedPuddleReflectionTmp != other.puddleReflectionTmp {
					sharedPuddleReflectionTmp.Deallocate()
				}
				sharedPuddleReflectionTmp = other.puddleReflectionTmp
				other.puddleReflectionTmp = nil
			}
			other.window = nil
			other.imageItem = nil
			other.image = nil
			other.imageBacking = nil
		}
	}
	state.window = gameWin
	state.imageItem = gameImageItem
	state.image = gameImage
	state.imageBacking = gameImageBacking
	state.lightingTmp = sharedLightingTmp
	state.puddleReflectionTmp = sharedPuddleReflectionTmp
}

func syncPrimaryViewportAliases(state *viewportRenderState) {
	if state == nil || state.window != gameWin {
		return
	}
	gameImageItem = state.imageItem
	gameImage = state.image
	gameImageBacking = state.imageBacking
}

func viewportStateForWindow(win *eui.WindowData) *viewportRenderState {
	if win == nil {
		return nil
	}
	for _, view := range appViewports.snapshot() {
		if view.render != nil && view.render.window == win {
			return view.render
		}
	}
	return nil
}

func isViewportWindow(win *eui.WindowData) bool {
	return win != nil && (win == gameWin || viewportStateForWindow(win) != nil)
}

func isViewportImageItem(item *eui.ItemData) bool {
	if item == nil {
		return false
	}
	if item == gameImageItem {
		return true
	}
	for _, view := range appViewports.snapshot() {
		if view.render != nil && view.render.imageItem == item {
			return true
		}
	}
	return false
}

func updateViewportImageSize(state *viewportRenderState, tiled bool) {
	if state == nil || state.window == nil {
		return
	}
	win := state.window
	size := win.GetSize()
	pad := float64(2 * win.Padding)
	title := float64(win.GetTitleSize())
	pixelW := int(size.X) &^ 1
	pixelH := int(size.Y) &^ 1
	edgeInset := 2
	if tiled {
		pixelW = int(math.Round(float64(size.X)))
		pixelH = int(math.Round(float64(size.Y)))
		edgeInset = 0
	}
	w := int(float64(pixelW)-pad) - 2*edgeInset
	tabHeight := 0
	if win == gameWin {
		tabHeight = sessionTabBarPixelHeight()
	}
	h := int(float64(pixelH)-pad-title) - 2*edgeInset - tabHeight
	if w <= 0 || h <= 0 {
		return
	}
	s := eui.UIScale()
	if state.imageItem == nil {
		item, backing := eui.NewImageFastItem(w, h)
		state.imageItem = item
		state.imageBacking = backing
		state.image = backing
		item.Image = state.image
		item.Size = eui.Point{X: float32(w) / s, Y: float32(h) / s}
		item.Position = eui.Point{X: float32(edgeInset) / s, Y: float32(edgeInset+tabHeight) / s}
		win.AddItem(item)
		syncPrimaryViewportAliases(state)
		return
	}
	iw, ih := 0, 0
	if state.imageBacking != nil {
		bounds := state.imageBacking.Bounds()
		iw, ih = bounds.Dx(), bounds.Dy()
	}
	if state.imageBacking == nil || iw < w || ih < h {
		_, replacement := eui.NewImageFastItem(w, h)
		if state.imageBacking != nil {
			state.imageBacking.Deallocate()
		}
		state.imageBacking = replacement
		win.Dirty = true
	}
	state.image = state.imageBacking.SubImage(image.Rect(0, 0, w, h)).(*ebiten.Image)
	state.imageItem.Image = state.image
	state.imageItem.Size = eui.Point{X: float32(w) / s, Y: float32(h) / s}
	state.imageItem.Position = eui.Point{X: float32(edgeInset) / s, Y: float32(edgeInset+tabHeight) / s}
	syncPrimaryViewportAliases(state)
}

func loginServerOptions(server string) ([]string, int) {
	options := serverAddresses()
	selected := 0
	found := false
	for index, option := range options {
		if sameServerAddress(option, server) {
			selected = index
			found = true
			break
		}
	}
	if server != "" && !found {
		options = append(options, server)
		selected = len(options) - 1
	}
	options = append(options, editServerListOption)
	return options, selected
}

func viewportLoginRememberPreference(session *Session, serverSlot int, characterName string) bool {
	if session != nil {
		if _, remember, staged := session.login.stagedPasswordSettings(serverSlot, characterName); staged {
			return remember
		}
	}
	if character, ok := characterForServerSlot(serverSlot, characterName); ok {
		return !character.DontRemember
	}
	return true
}

func startViewportLogin(state *viewportRenderState, session *Session) {
	if state == nil || session == nil {
		return
	}
	if session.connectionBusy() {
		appSessions.disconnectSession(session.ID())
		return
	}
	if state.loginCharacter == freeDemoSelection {
		startViewportDemoLogin(state, session)
		return
	}
	serverSlot := serverSlotForAddress(state.loginServer)
	character, ok := characterForServerSlot(serverSlot, state.loginCharacter)
	if !ok {
		session.login.setStatus("Disconnected", errors.New("select a saved character before connecting"))
		refreshViewportLoginOverlay(state, session, true)
		return
	}
	passwordHash := character.passHash
	if staged, ok := session.login.stagedPasswordHash(serverSlot, character.Name); ok {
		passwordHash = staged
	}
	if passwordHash == "" {
		showPasswordPromptForSession(session, state, false, viewportLoginRememberPreference(session, serverSlot, character.Name), loginConnectButton)
		return
	}
	startViewportLoginRequest(state, session, passwordHash)
}

func startViewportLoginRequest(state *viewportRenderState, session *Session, passwordHash string) {
	request := sessionLoginRequest{
		host:         state.loginServer,
		character:    state.loginCharacter,
		passwordHash: passwordHash,
	}
	loginVersion := clVersion
	if status.Version > loginVersion {
		loginVersion = status.Version
	}
	if _, err := appSessions.startLogin(gameCtx, session.ID(), request, loginVersion); err != nil {
		session.login.setStatus("Disconnected", err)
	}
	refreshViewportLoginOverlay(state, session, true)
}

func startViewportDemoLogin(state *viewportRenderState, session *Session) {
	if state == nil || session == nil || session.connectionBusy() || state.loginDemoLookup {
		return
	}
	state.loginDemoLookup = true
	loginVersion := clVersion
	if status.Version > loginVersion {
		loginVersion = status.Version
	}
	session.login.setRequest(sessionLoginRequest{host: state.loginServer})
	session.login.setStatus("Finding an available demo character...", nil)
	refreshViewportLoginOverlay(state, session, true)
	go func() {
		candidates, err := fetchSessionDemoCharacters(session, loginVersion)
		dispatchMainThread(func() {
			state.loginDemoLookup = false
			if err != nil {
				session.login.setStatus("Disconnected", err)
				refreshViewportLoginOverlay(state, session, true)
				return
			}
			if len(candidates) == 0 {
				session.login.setStatus("Disconnected", errors.New("no demo characters are available"))
				refreshViewportLoginOverlay(state, session, true)
				return
			}
			state.loginCharacter = freeDemoSelection
			request := sessionLoginRequest{host: state.loginServer, character: candidates[0], password: "demo"}
			if _, err := appSessions.startLogin(gameCtx, session.ID(), request, loginVersion); err != nil {
				session.login.setStatus("Disconnected", err)
			}
			refreshViewportLoginOverlay(state, session, true)
		})
	}()
}

func refreshViewportLoginOverlay(state *viewportRenderState, session *Session, multi bool) {
	if state == nil || session == nil {
		return
	}
	if multi && appSessions != nil && appSessions.selectedID() == session.ID() && loginWin != nil {
		updateCharacterButtons()
	}
}

func focusViewportLogin(id SessionID) {
	if appSessions == nil || !appSessions.selectSession(id) || loginWin == nil {
		return
	}
	updateCharacterButtons()
	centerLoginWindow()
	loginWin.MarkOpen()
}

func refreshViewportSelectionTreatment() {
	refreshSessionTabs()
}

func refreshViewportWorkspace() {
	if appSessions == nil || appViewports == nil || gameWin == nil {
		return
	}
	createdTabBar := ensureSessionTabBar()
	selected := appSessions.selectedID()
	appViewports.showSession(selected)
	bindSelectedViewportWindow()
	views := appViewports.snapshot()
	sessions := appSessions.snapshot()
	for slot, view := range views {
		state := view.render
		if state == nil {
			continue
		}
		if sessions[slot] == nil {
			if state.window != nil && state.window != gameWin {
				state.window.RemoveWindow()
			}
			if state.imageBacking != nil && state.imageBacking != gameImageBacking {
				state.imageBacking.Deallocate()
			}
			if state.lightingTmp != nil {
				state.lightingTmp.Deallocate()
			}
			if state.puddleReflectionTmp != nil {
				state.puddleReflectionTmp.Deallocate()
			}
			state.window = nil
			state.imageItem = nil
			state.image = nil
			state.imageBacking = nil
			state.lightingTmp = nil
			state.puddleReflectionTmp = nil
			state.nightTransition = nightTransitionState{}
			state.worldRenderValid = false
			continue
		}
		if view.SessionID != selected {
			if state.window != nil && state.window != gameWin {
				state.window.RemoveWindow()
			}
			state.window = nil
			state.imageItem = nil
			state.image = nil
			state.imageBacking = nil
			continue
		}
		state.window = gameWin
		state.imageItem = gameImageItem
		state.image = gameImage
		state.imageBacking = gameImageBacking
		state.worldRenderValid = false
		updateViewportImageSize(state, gs.TiledWindows)
	}
	refreshSessionTabs()
	if createdTabBar && !gs.TiledWindows {
		onGameWindowResize()
	}
	if !gameWin.IsOpen() {
		gameWin.MarkOpen()
	}
	if session, ok := appSessions.session(selected); ok && loginWin != nil && sessionTabsVisible() && clmov == "" && !movieMode && !playingMovie && !status.NeedImages && !status.NeedSounds {
		if session.transport.connected() {
			loginWin.Close()
		} else {
			updateCharacterButtons()
			centerLoginWindow()
			loginWin.MarkOpen()
		}
	}
	refreshViewportTitles()
}

func refreshViewportRectsFromDrawRects() {
	for _, view := range appViewports.snapshot() {
		state := view.render
		if !view.Active || state == nil || state.window == nil || !state.window.IsOpen() || state.imageItem == nil {
			appViewports.setRect(view.ID, image.Rectangle{})
			continue
		}
		r := state.imageItem.DrawRect
		rect := image.Rect(int(math.Round(float64(r.X0))), int(math.Round(float64(r.Y0))), int(math.Round(float64(r.X1))), int(math.Round(float64(r.Y1))))
		appViewports.setRect(view.ID, rect)
	}
}

func sessionViewportWorldAt(session *Session, point image.Point) (int16, int16, bool) {
	if session == nil {
		return 0, 0, false
	}
	for _, view := range appViewports.snapshot() {
		if view.SessionID != session.ID() {
			continue
		}
		return view.worldAt(point)
	}
	return 0, 0, false
}

func worldDrawInfoForSession(session *Session) (int, int, float64) {
	if session != nil {
		for _, view := range appViewports.snapshot() {
			if view.SessionID != session.ID() || !view.Active || view.Rect.Empty() {
				continue
			}
			viewRect, scale := fittedWorldView(view.Rect.Dx(), view.Rect.Dy())
			return view.Rect.Min.X + viewRect.Min.X, view.Rect.Min.Y + viewRect.Min.Y, scale
		}
	}
	return worldDrawInfo()
}
