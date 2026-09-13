package main

import (
	"fmt"
	"image"
	"math"
	"strings"

	"gothoom/eui"

	"github.com/hajimehoshi/ebiten/v2"
)

func viewportTitle(session *Session) string {
	if session == nil {
		return "Session"
	}
	name := session.characterName()
	if session == primarySession {
		name = playerName
	}
	if name == "" {
		return fmt.Sprintf("Session %d", session.ID())
	}
	return fmt.Sprintf("Session %d -- %s", session.ID(), name)
}

func refreshViewportTitles() {
	views := appViewports.snapshot()
	sessions := appSessions.snapshot()
	for slot, view := range views {
		if view.render != nil && view.render.window != nil && sessions[slot] != nil {
			title := viewportTitle(sessions[slot])
			if appSessions.multiEnabled() && appSessions.selectedID() == sessions[slot].ID() {
				title += " [Selected]"
			}
			view.render.window.Title = title
			view.render.window.Dirty = true
		}
	}
}

func bindPrimaryViewportWindow() {
	state := appViewports.renderStateForViewport(1)
	if state == nil {
		return
	}
	state.window = gameWin
	state.imageItem = gameImageItem
	state.image = gameImage
	state.imageBacking = gameImageBacking
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
	h := int(float64(pixelH)-pad-title) - 2*edgeInset
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
		item.Position = eui.Point{X: float32(edgeInset) / s, Y: float32(edgeInset) / s}
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
	state.imageItem.Position = eui.Point{X: float32(edgeInset) / s, Y: float32(edgeInset) / s}
	syncPrimaryViewportAliases(state)
	layoutViewportLoginOverlay(state)
}

func viewportLoginServerOptions(server string) ([]string, int) {
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
	return options, selected
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
	state.loginRemember = passwordRememberPreference(state.loginCharacter)

	heading, _ := eui.NewText()
	heading.Text = fmt.Sprintf("Connect Session %d", session.ID())
	heading.FontSize = 18
	heading.Size = eui.Point{X: 360, Y: 32}

	serverChoice, serverEvents := eui.NewDropdown()
	serverChoice.Label = "Server"
	serverChoice.Size = eui.Point{X: 360, Y: 32}
	serverChoice.Options, serverChoice.Selected = viewportLoginServerOptions(state.loginServer)
	serverEvents.Handle = func(event eui.UIEvent) {
		if event.Type != eui.EventDropdownSelected || event.Index < 0 || event.Index >= len(serverChoice.Options) {
			return
		}
		state.loginServer = serverChoice.Options[event.Index]
		appSessions.selectSession(session.ID())
	}
	state.loginServerChoice = serverChoice

	characterInput, characterEvents := eui.NewInput()
	characterInput.Label = "Character"
	characterInput.TextPtr = &state.loginCharacter
	characterInput.Text = state.loginCharacter
	characterInput.Size = eui.Point{X: 360, Y: 32}
	characterEvents.Handle = func(event eui.UIEvent) {
		if event.Type == eui.EventInputChanged {
			state.loginRemember = passwordRememberPreference(state.loginCharacter)
			if state.loginRememberItem != nil {
				state.loginRememberItem.Checked = state.loginRemember
				state.loginRememberItem.Dirty = true
			}
			appSessions.selectSession(session.ID())
		}
	}
	state.loginCharacterItem = characterInput

	passwordInput, passwordEvents := eui.NewInput()
	passwordInput.Label = "Password"
	passwordInput.TextPtr = &state.loginPassword
	passwordInput.HideText = true
	passwordInput.Size = eui.Point{X: 360, Y: 32}
	passwordEvents.Handle = func(event eui.UIEvent) {
		if event.Type == eui.EventInputChanged {
			appSessions.selectSession(session.ID())
		}
	}
	state.loginPasswordItem = passwordInput

	remember, rememberEvents := eui.NewCheckbox()
	remember.Text = "Remember Password"
	remember.Size = eui.Point{X: 360, Y: 28}
	remember.Checked = state.loginRemember
	rememberEvents.Handle = func(event eui.UIEvent) {
		if event.Type != eui.EventCheckboxChanged {
			return
		}
		state.loginRemember = event.Checked
		if !event.Checked {
			forgetSavedPassword(state.loginCharacter)
			session.login.discardStagedPasswordFor(state.loginCharacter)
		}
		appSessions.selectSession(session.ID())
	}
	state.loginRememberItem = remember

	statusItem, _ := eui.NewText()
	statusItem.Size = eui.Point{X: 360, Y: 48}
	statusItem.FontSize = 13
	state.loginStatus = statusItem

	action, actionEvents := eui.NewButton()
	action.Text = "Connect"
	action.Size = eui.Point{X: 160, Y: 36}
	action.Outlined = true
	action.Border = 2
	action.OutlineColor = eui.ColorGreen
	actionEvents.Handle = func(event eui.UIEvent) {
		if event.Type != eui.EventClick {
			return
		}
		appSessions.selectSession(session.ID())
		if session.transport.busy() {
			appSessions.disconnectSession(session.ID())
			return
		}
		request := sessionLoginRequest{
			host:      state.loginServer,
			character: state.loginCharacter,
			password:  state.loginPassword,
		}
		if request.password == "" {
			if staged, ok := session.login.stagedPasswordHash(request.character); ok {
				request.passwordHash = staged
			} else if saved, ok := selectedCharacter(request.character); ok {
				request.passwordHash = saved.passHash
			}
		}
		if err := request.normalized().validate(); err != nil {
			session.login.setStatus("Disconnected", err)
			refreshViewportLoginOverlay(state, session, true)
			queueSessionWorkspaceUIUpdate()
			return
		}
		if request.password != "" {
			request.passwordHash = session.login.stagePassword(request.character, request.password, state.loginRemember)
		}
		loginVersion := clVersion
		if status.Version > loginVersion {
			loginVersion = status.Version
		}
		if _, err := appSessions.startLogin(gameCtx, session.ID(), request, loginVersion); err != nil {
			session.login.setStatus("Disconnected", err)
			refreshViewportLoginOverlay(state, session, true)
			queueSessionWorkspaceUIUpdate()
			return
		}
		clearPasswordInput(passwordInput, &state.loginPassword)
		refreshViewportLoginOverlay(state, session, true)
	}
	state.loginAction = action

	actions := eui.NewRow(action)
	form := eui.NewColumn(heading, serverChoice, characterInput, passwordInput, remember, statusItem, actions)
	state.loginForm = form
	overlay := eui.NewColumn(form)
	overlay.Fixed = true
	overlay.Filled = true
	overlay.Scrollable = true
	state.loginOverlay = overlay
	state.window.AddItem(overlay)
	layoutViewportLoginOverlay(state)
}

func layoutViewportLoginOverlay(state *viewportRenderState) {
	if state == nil || state.loginOverlay == nil || state.loginForm == nil || state.imageItem == nil || state.image == nil {
		return
	}
	bounds := state.image.Bounds()
	state.loginOverlay.Position = state.imageItem.Position
	state.loginOverlay.Size = state.imageItem.Size
	scale := eui.UIScale()
	if scale <= 0 {
		scale = 1
	}
	controlWidth := float32(math.Min(360, float64(float32(max(120, bounds.Dx()-24))/scale)))
	for index, item := range state.loginForm.Contents {
		if index >= 6 {
			break
		}
		item.Size.X = controlWidth
		item.Dirty = true
	}
	state.loginAction.Size.X = float32(math.Min(160, float64(controlWidth)))
	formSize := state.loginForm.GetSize()
	state.loginForm.Position = eui.Point{
		X: float32(max(12, (bounds.Dx()-int(math.Round(float64(formSize.X))))/2)),
		Y: float32(max(12, (bounds.Dy()-int(math.Round(float64(formSize.Y))))/2)),
	}
	state.loginOverlay.Dirty = true
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
	visible := multi && !session.transport.connected()
	state.loginOverlay.Invisible = !visible
	if !visible {
		eui.ClearFocus(state.loginCharacterItem)
		eui.ClearFocus(state.loginPasswordItem)
		state.window.Refresh()
		return
	}
	statusText, lastErr := session.login.statusSnapshot()
	if statusText == "" {
		statusText = "Disconnected"
	}
	if lastErr != "" {
		statusText = "Error: " + lastErr
	}
	state.loginStatus.Text = statusText
	state.loginStatus.Dirty = true
	busy := session.transport.busy()
	state.loginServerChoice.Disabled = busy
	state.loginCharacterItem.Disabled = busy
	state.loginPasswordItem.Disabled = busy
	state.loginRememberItem.Disabled = busy
	if busy {
		state.loginAction.Text = "Cancel"
		state.loginAction.OutlineColor = eui.ColorDarkRed
	} else {
		state.loginAction.Text = "Connect"
		state.loginAction.OutlineColor = eui.ColorGreen
	}
	state.loginAction.Dirty = true
	layoutViewportLoginOverlay(state)
	state.window.Refresh()
}

func focusViewportLogin(id SessionID) {
	state := appViewports.renderStateForSession(id)
	if state == nil || state.window == nil || state.loginOverlay == nil || state.loginOverlay.Invisible {
		return
	}
	state.window.BringForward()
	if strings.TrimSpace(state.loginCharacter) == "" {
		eui.Focus(state.loginCharacterItem)
	} else {
		eui.Focus(state.loginPasswordItem)
	}
}

func resizeViewportWindow(state *viewportRenderState) {
	if state == nil || state.window == nil {
		return
	}
	win := state.window
	if state.inAspectResize {
		updateViewportImageSize(state, false)
		return
	}
	size := win.GetSize()
	if size.X <= 0 || size.Y <= 0 {
		return
	}
	pad := float64(2 * win.Padding)
	title := float64(win.GetTitleSize())
	availW := float64(int(size.X)&^1) - pad
	availH := float64(int(size.Y)&^1) - pad - title
	if availW <= 0 || availH <= 0 {
		updateViewportImageSize(state, false)
		return
	}
	scale := math.Min(availW/float64(gameAreaSizeX), availH/float64(gameAreaSizeY))
	if scale < 0.25 {
		scale = 0.25
	}
	newSize := eui.Point{
		X: float32(math.Round(float64(gameAreaSizeX)*scale + pad)),
		Y: float32(math.Round(float64(gameAreaSizeY)*scale + pad + title)),
	}
	if math.Abs(float64(size.X-newSize.X)) > 0.5 || math.Abs(float64(size.Y-newSize.Y)) > 0.5 {
		state.inAspectResize = true
		_ = win.SetSize(newSize)
		state.inAspectResize = false
	}
	updateViewportImageSize(state, false)
}

func configureSecondaryViewportWindow(view Viewport, session *Session) {
	state := view.render
	if state == nil || state.window != nil {
		return
	}
	win := newGameRenderWindow()
	win.Title = viewportTitle(session)
	win.Closable = false
	win.Resizable = true
	win.Movable = true
	win.Maximizable = false
	state.window = win
	win.OnResize = func() { resizeViewportWindow(state) }

	screenW, screenH := eui.ScreenSize()
	width := max(320, min(640, screenW/2-24))
	height := max(220, min(420, screenH/2-24))
	win.Size = eui.Point{X: float32(width), Y: float32(height)}
	col := (int(view.ID) - 1) % 2
	row := (int(view.ID) - 1) / 2
	_ = win.SetPos(eui.Point{X: float32(12 + col*(width+12)), Y: float32(12 + row*(height+12))})
	updateViewportImageSize(state, false)
	win.MarkOpen()
}

func refreshViewportWorkspace() {
	bindPrimaryViewportWindow()
	views := appViewports.snapshot()
	sessions := appSessions.snapshot()
	multi := appSessions.multiEnabled()
	if multi && loginWin != nil && loginWin.IsOpen() {
		loginWin.Close()
	}
	for slot, view := range views {
		state := view.render
		if state == nil {
			continue
		}
		if slot == 0 {
			refreshViewportLoginOverlay(state, sessions[slot], multi)
			continue
		}
		if view.Active && multi {
			configureSecondaryViewportWindow(view, sessions[slot])
			refreshViewportLoginOverlay(state, sessions[slot], true)
			if state.window != nil && !state.window.IsOpen() {
				state.window.MarkOpen()
			}
			continue
		}
		if state.window != nil {
			state.window.RemoveWindow()
			if state.imageBacking != nil {
				state.imageBacking.Deallocate()
			}
			state.window = nil
			state.imageItem = nil
			state.image = nil
			state.imageBacking = nil
			state.worldRenderValid = false
			clearViewportLoginUI(state)
		}
	}
	if !multi && primarySession != nil && !primarySession.transport.connected() && !primarySession.transport.busy() && loginWin != nil && !loginWin.IsOpen() {
		loginWin.MarkOpen()
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
	state.loginCharacterItem = nil
	state.loginPasswordItem = nil
	state.loginRememberItem = nil
	state.loginAction = nil
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
