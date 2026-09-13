package main

import (
	"errors"
	"fmt"
	"image"
	"math"
	"strings"

	"gothoom/eui"

	"github.com/hajimehoshi/ebiten/v2"
)

const (
	viewportLoginPanelWidth   float32 = 360
	viewportLoginPanelPadding float32 = 12
	viewportLogoutButtonSize  float32 = 28
	viewportControlInset      float32 = 6
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
		if state.loginOverlay != nil {
			win.PrependItem(item)
		} else {
			win.AddItem(item)
		}
		syncPrimaryViewportAliases(state)
		layoutViewportLoginOverlay(state)
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
	layoutViewportLoginOverlay(state)
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

func refreshAllViewportLoginServerChoices() {
	if appViewports == nil {
		return
	}
	for _, view := range appViewports.snapshot() {
		state := view.render
		if state == nil || state.loginServerChoice == nil {
			continue
		}
		state.loginServerChoice.Options, state.loginServerChoice.Selected = loginServerOptions(state.loginServer)
		state.loginServerChoice.Dirty = true
	}
}

func viewportLoginRememberPreference(session *Session, characterName string) bool {
	if session != nil {
		if _, remember, staged := session.login.stagedPasswordSettings(characterName); staged {
			return remember
		}
	}
	for _, character := range characters {
		if strings.EqualFold(character.Name, strings.TrimSpace(characterName)) {
			return !character.DontRemember
		}
	}
	return true
}

func selectViewportLoginSavedCharacter(state *viewportRenderState, session *Session, characterName string) bool {
	if state == nil || session == nil {
		return false
	}
	character, ok := selectedCharacter(characterName)
	if !ok {
		return false
	}
	state.loginCharacter = character.Name
	appSessions.selectSession(session.ID())
	refreshViewportLoginCharacterList(state, session)
	return true
}

func refreshViewportLoginCharacterList(state *viewportRenderState, session *Session) {
	if state == nil || session == nil || state.loginCharacters == nil {
		return
	}
	if state.loginCharacter != "" && !validLoginCharacterSelection(state.loginCharacter) {
		state.loginCharacter = ""
	}
	if state.loginCharacter == "" {
		if session == primarySession && validLoginCharacterSelection(name) {
			state.loginCharacter = name
		} else if gs.LastCharacter != "" {
			if saved, ok := selectedCharacter(gs.LastCharacter); ok {
				state.loginCharacter = saved.Name
			}
		}
		if state.loginCharacter == "" && len(characters) == 1 {
			state.loginCharacter = characters[0].Name
		}
		if state.loginCharacter == "" && len(characters) == 0 {
			state.loginCharacter = freeDemoSelection
		}
	}
	refreshLoginCharacterList(loginCharacterListConfig{
		list:       state.loginCharacters,
		width:      viewportLoginPanelWidth,
		radioGroup: fmt.Sprintf("characters-session-%d", session.ID()),
		selection:  state.loginCharacter,
		onSelect: func(choice loginCharacterChoice) {
			state.loginCharacter = choice.selection
			appSessions.selectSession(session.ID())
			refreshViewportLoginCharacterList(state, session)
			refreshViewportLoginOverlay(state, session, true)
		},
	})
	disabled := state.loginCharacter == "" || state.loginCharacter == freeDemoSelection
	for _, button := range []*eui.ItemData{state.loginEdit, state.loginDelete} {
		if button != nil {
			button.Disabled = disabled
			button.Dirty = true
		}
	}
}

func refreshAllViewportLoginCharacterLists() {
	if appSessions == nil || appViewports == nil {
		return
	}
	sessions := appSessions.snapshot()
	for slot, view := range appViewports.snapshot() {
		if view.render != nil && sessions[slot] != nil {
			refreshViewportLoginCharacterList(view.render, sessions[slot])
		}
	}
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
	character, ok := selectedCharacter(state.loginCharacter)
	if !ok {
		session.login.setStatus("Disconnected", errors.New("select a saved character before connecting"))
		refreshViewportLoginOverlay(state, session, true)
		return
	}
	passwordHash := character.passHash
	if staged, ok := session.login.stagedPasswordHash(character.Name); ok {
		passwordHash = staged
	}
	if passwordHash == "" {
		showPasswordPromptForSession(session, state, false, viewportLoginRememberPreference(session, character.Name), state.loginAction)
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

func makeViewportLoginOverlay(state *viewportRenderState, session *Session) {
	if state == nil || state.window == nil || session == nil || state.loginOverlay != nil {
		return
	}
	request := session.login.requestSnapshot()
	state.loginServer = request.host
	state.loginCharacter = request.character
	if state.loginServer == "" {
		state.loginServer = strings.TrimSpace(gs.ServerAddress)
	}
	if session == primarySession && state.loginCharacter == "" {
		state.loginCharacter = strings.TrimSpace(name)
	}
	heading, _ := eui.NewText()
	heading.Text = fmt.Sprintf("Connect Session %d", session.ID())
	heading.FontSize = 18
	heading.Size = eui.Point{X: viewportLoginPanelWidth, Y: 32}

	controls := newSessionLoginControls(sessionLoginControlsConfig{
		sessionID:    session.ID(),
		viewport:     true,
		width:        viewportLoginPanelWidth,
		listHeight:   224,
		connectWidth: 140,
		selection:    func() string { return state.loginCharacter },
		server:       func() string { return state.loginServer },
		onServer: func(address string) {
			state.loginServer = address
			appSessions.selectSession(session.ID())
		},
		onConnect: func(_ *eui.ItemData) {
			appSessions.selectSession(session.ID())
			startViewportLogin(state, session)
		},
	})
	state.loginServerChoice = controls.server
	state.loginCharacters = controls.characters
	state.loginAdd = controls.add
	state.loginEdit = controls.edit
	state.loginDelete = controls.delete
	state.loginAction = controls.connect

	statusItem, _ := eui.NewText()
	statusItem.Size = eui.Point{X: viewportLoginPanelWidth, Y: 48}
	statusItem.FontSize = 13
	state.loginStatus = statusItem

	form := eui.NewColumn(heading, controls.characterActions, controls.characters, controls.connectRow, statusItem)
	state.loginForm = form
	overlay := eui.NewColumn(form)
	overlay.Fixed = true
	overlay.Filled = true
	overlay.Scrollable = true
	state.loginOverlay = overlay
	state.window.AddItem(overlay)

	logout, logoutEvents := eui.NewButton()
	setMaterialIconOnly(logout, "logout", "X")
	logout.SetTooltip(fmt.Sprintf("Log out of Session %d.", session.ID()))
	logout.Size = eui.Point{X: viewportLogoutButtonSize, Y: viewportLogoutButtonSize}
	logout.Color = eui.ColorDarkRed
	logout.Invisible = true
	logoutEvents.Handle = func(event eui.UIEvent) {
		if event.Type == eui.EventClick {
			appSessions.disconnectSession(session.ID())
		}
	}
	state.sessionLogout = logout
	state.window.AddItem(logout)

	refreshViewportLoginCharacterList(state, session)
	layoutViewportLoginOverlay(state)
}

func layoutViewportLoginOverlay(state *viewportRenderState) {
	if state == nil || state.imageItem == nil || state.image == nil {
		return
	}
	bounds := state.image.Bounds()
	scale := eui.UIScale()
	if scale <= 0 {
		scale = 1
	}
	if state.loginOverlay != nil && state.loginForm != nil {
		controlRoom := max(120, bounds.Dx()-int(4*viewportLoginPanelPadding))
		controlWidth := float32(math.Min(float64(viewportLoginPanelWidth), float64(float32(controlRoom)/scale)))
		for index, item := range state.loginForm.Contents {
			if index >= len(state.loginForm.Contents)-1 {
				break
			}
			item.Size.X = controlWidth
			item.Dirty = true
		}

		formSize := state.loginForm.GetSize()
		padding := viewportLoginPanelPadding
		panelWidth := float32(math.Min(float64(formSize.X+2*padding), float64(float32(bounds.Dx())-2*padding)))
		panelHeight := float32(math.Min(float64(formSize.Y+2*padding), float64(float32(bounds.Dy())-2*padding)))
		panelWidth = float32(math.Max(1, float64(panelWidth)))
		panelHeight = float32(math.Max(1, float64(panelHeight)))
		state.loginOverlay.Size = eui.Point{X: panelWidth / scale, Y: panelHeight / scale}
		state.loginOverlay.Position = eui.Point{
			X: state.imageItem.Position.X + float32(math.Max(float64(padding), float64((float32(bounds.Dx())-panelWidth)/2))),
			Y: state.imageItem.Position.Y + float32(math.Max(float64(padding), float64((float32(bounds.Dy())-panelHeight)/2))),
		}
		state.loginForm.Position = eui.Point{X: padding, Y: padding}
		background := eui.NewColor(32, 32, 32, 255)
		if state.window != nil && state.window.Theme != nil {
			background = state.window.Theme.Window.BGColor
		}
		background.A = uint8(uint16(background.A) * 9 / 10)
		state.loginOverlay.Color = background
		state.loginOverlay.Dirty = true
	}
	if state.sessionLogout != nil {
		buttonSize := state.sessionLogout.GetSize()
		state.sessionLogout.Position = eui.Point{
			X: state.imageItem.Position.X + float32(math.Max(0, float64(float32(bounds.Dx())-buttonSize.X-viewportControlInset))),
			Y: state.imageItem.Position.Y + float32(math.Max(0, float64(float32(bounds.Dy())-buttonSize.Y-viewportControlInset))),
		}
		state.sessionLogout.Dirty = true
	}
}

func refreshViewportLoginOverlay(state *viewportRenderState, session *Session, multi bool) {
	if state == nil || session == nil || state.window == nil {
		return
	}
	if multi {
		makeViewportLoginOverlay(state, session)
	}
	if state.loginOverlay == nil {
		return
	}
	connected := session.transport.connected()
	visible := multi && !connected
	state.loginOverlay.Invisible = !visible
	if state.sessionLogout != nil {
		state.sessionLogout.Invisible = !multi || !connected
		state.sessionLogout.Dirty = true
	}
	layoutViewportLoginOverlay(state)
	if !visible {
		state.window.Refresh()
		return
	}
	statusText, lastErr := session.login.statusSnapshot()
	if statusText == "" {
		statusText = "Disconnected"
	}
	if lastErr != "" {
		if strings.HasPrefix(statusText, "Reconnecting") {
			statusText += "\n" + lastErr
		} else {
			statusText = "Error: " + lastErr
		}
	}
	state.loginStatus.Text = statusText
	state.loginStatus.Dirty = true
	busy := session.connectionBusy() || state.loginDemoLookup
	state.loginServerChoice.Disabled = busy
	state.loginCharacters.Disabled = busy
	state.loginCharacters.Dirty = true
	for _, button := range []*eui.ItemData{state.loginAdd, state.loginEdit, state.loginDelete} {
		if button != nil {
			button.Disabled = busy || (button != state.loginAdd && (state.loginCharacter == "" || state.loginCharacter == freeDemoSelection))
			button.Dirty = true
		}
	}
	if state.loginDemoLookup {
		state.loginAction.Text = "Finding Demo..."
		state.loginAction.Disabled = true
		state.loginAction.OutlineColor = eui.ColorGreen
	} else if busy {
		state.loginAction.Text = "Cancel"
		state.loginAction.Disabled = false
		state.loginAction.OutlineColor = eui.ColorDarkRed
	} else {
		state.loginAction.Text = "Connect"
		state.loginAction.Disabled = state.loginCharacter == ""
		state.loginAction.OutlineColor = eui.ColorGreen
		if state.loginAction.Disabled {
			state.loginAction.SetTooltip("Select a character before connecting.")
		} else {
			state.loginAction.SetTooltip("Connect as the selected character.")
		}
	}
	state.loginAction.Dirty = true
	state.window.Refresh()
}

func focusViewportLogin(id SessionID) {
	state := appViewports.renderStateForSession(id)
	if state == nil || state.window == nil || state.loginOverlay == nil || state.loginOverlay.Invisible {
		return
	}
	state.window.BringForward()
}

func refreshViewportSelectionTreatment() {
	refreshSessionTabs()
}

func refreshViewportWorkspace() {
	if appSessions == nil || appViewports == nil || gameWin == nil {
		return
	}
	ensureSessionTabBar()
	selected := appSessions.selectedID()
	appViewports.showSession(selected)
	bindSelectedViewportWindow()
	views := appViewports.snapshot()
	sessions := appSessions.snapshot()
	if loginWin != nil && loginWin.IsOpen() {
		loginWin.Close()
	}
	for slot, view := range views {
		state := view.render
		if state == nil {
			continue
		}
		if state.loginOverlay != nil {
			state.loginOverlay.Invisible = true
		}
		if state.sessionLogout != nil {
			state.sessionLogout.Invisible = true
		}
		if sessions[slot] == nil {
			if state.window != nil && state.window != gameWin {
				state.window.RemoveWindow()
			}
			if gameWin != nil {
				if state.loginOverlay != nil {
					gameWin.RemoveItem(state.loginOverlay)
				}
				if state.sessionLogout != nil {
					gameWin.RemoveItem(state.sessionLogout)
				}
			}
			if state.imageBacking != nil && state.imageBacking != gameImageBacking {
				state.imageBacking.Deallocate()
			}
			if state.lightingTmp != nil {
				state.lightingTmp.Deallocate()
			}
			state.window = nil
			state.imageItem = nil
			state.image = nil
			state.imageBacking = nil
			state.lightingTmp = nil
			state.nightTransition = nightTransitionState{}
			state.worldRenderValid = false
			clearViewportLoginUI(state)
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
		refreshViewportLoginOverlay(state, sessions[slot], sessionTabsVisible())
	}
	refreshSessionTabs()
	if !gameWin.IsOpen() {
		gameWin.MarkOpen()
	}
	refreshViewportTitles()
}

func clearViewportLoginUI(state *viewportRenderState) {
	if state == nil {
		return
	}
	state.loginOverlay = nil
	state.loginForm = nil
	state.loginStatus = nil
	state.loginServerChoice = nil
	state.loginCharacters = nil
	state.loginAdd = nil
	state.loginEdit = nil
	state.loginDelete = nil
	state.loginAction = nil
	state.sessionLogout = nil
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
