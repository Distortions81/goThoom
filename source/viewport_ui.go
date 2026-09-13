package main

import (
	"fmt"
	"image"
	"math"
	"strings"

	"gothoom/eui"

	"github.com/hajimehoshi/ebiten/v2"
)

var (
	viewportWorkspaceAppliedLayout viewportLayout
	viewportWorkspaceScreenWidth   int
	viewportWorkspaceScreenHeight  int
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
	multi := appSessions.multiEnabled()
	for slot, view := range views {
		if view.render != nil && view.render.window != nil && sessions[slot] != nil {
			title := viewportTitle(sessions[slot])
			if !multi && slot == 0 {
				title = gameWindowTitle()
			}
			if multi && appSessions.selectedID() == sessions[slot].ID() {
				title += " [Selected]"
			}
			view.render.window.Title = title
			view.render.window.Dirty = true
		}
	}
	refreshViewportSelectionTreatment()
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
	captureViewportFreeformChrome(state)
}

func captureViewportFreeformChrome(state *viewportRenderState) {
	if state == nil || state.window == nil || state.freeformChromeSet {
		return
	}
	win := state.window
	state.freeformTitle = win.GetRawTitleSize()
	state.freeformPadding = win.Padding
	state.freeformMargin = win.Margin
	state.freeformBorder = win.Border
	state.freeformOutlined = win.Outlined
	if win == gameWin {
		if gameWindowFreeformTitleHeight > 0 {
			state.freeformTitle = gameWindowFreeformTitleHeight
		}
		if gameWindowFreeformPadding > 0 {
			state.freeformPadding = gameWindowFreeformPadding
		}
		if gameWindowFreeformMargin > 0 {
			state.freeformMargin = gameWindowFreeformMargin
		}
	}
	state.freeformChromeSet = true
}

func primaryViewportUsesTiledSizing() bool {
	if appSessions != nil && appSessions.multiEnabled() {
		return appViewports.layoutSnapshot() == viewportLayoutTiled
	}
	return gs.TiledWindows
}

func viewportUsesTiledSizing(id ViewportID) bool {
	if appSessions != nil && appSessions.multiEnabled() {
		return appViewports.layoutSnapshot() == viewportLayoutTiled
	}
	return id == 1 && gs.TiledWindows
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

const viewportLoginSavedCharacterPrompt = "Choose a saved character..."

func viewportLoginSavedCharacterOptions(characterName string) ([]string, int) {
	options := make([]string, 1, len(characters)+1)
	options[0] = viewportLoginSavedCharacterPrompt
	selected := 0
	for _, character := range characters {
		options = append(options, character.Name)
		if strings.EqualFold(character.Name, strings.TrimSpace(characterName)) {
			selected = len(options) - 1
		}
	}
	return options, selected
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
	state.loginPassword = ""
	state.loginRemember = viewportLoginRememberPreference(session, character.Name)
	if state.loginCharacterItem != nil {
		state.loginCharacterItem.Text = state.loginCharacter
		state.loginCharacterItem.Dirty = true
	}
	if state.loginPasswordItem != nil {
		clearPasswordInput(state.loginPasswordItem, &state.loginPassword)
	}
	if state.loginRememberItem != nil {
		state.loginRememberItem.Checked = state.loginRemember
		state.loginRememberItem.Dirty = true
	}
	if state.loginSavedChoice != nil {
		state.loginSavedChoice.Options, state.loginSavedChoice.Selected = viewportLoginSavedCharacterOptions(state.loginCharacter)
		state.loginSavedChoice.Dirty = true
	}
	appSessions.selectSession(session.ID())
	return true
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
	state.loginRemember = viewportLoginRememberPreference(session, state.loginCharacter)

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

	savedChoice, savedEvents := eui.NewDropdown()
	savedChoice.Label = "Saved Character"
	savedChoice.Size = eui.Point{X: 360, Y: 32}
	savedChoice.Options, savedChoice.Selected = viewportLoginSavedCharacterOptions(state.loginCharacter)
	savedChoice.Disabled = len(characters) == 0
	savedChoice.SetTooltip("Choose a character saved on this computer, or type another name below.")
	savedEvents.Handle = func(event eui.UIEvent) {
		if event.Type != eui.EventDropdownSelected || event.Index <= 0 || event.Index >= len(savedChoice.Options) {
			return
		}
		selectViewportLoginSavedCharacter(state, session, savedChoice.Options[event.Index])
	}
	state.loginSavedChoice = savedChoice

	characterInput, characterEvents := eui.NewInput()
	characterInput.Label = "Character"
	characterInput.TextPtr = &state.loginCharacter
	characterInput.Text = state.loginCharacter
	characterInput.Size = eui.Point{X: 360, Y: 32}
	characterEvents.Handle = func(event eui.UIEvent) {
		if event.Type == eui.EventInputChanged {
			state.loginRemember = viewportLoginRememberPreference(session, state.loginCharacter)
			if state.loginSavedChoice != nil {
				state.loginSavedChoice.Options, state.loginSavedChoice.Selected = viewportLoginSavedCharacterOptions(state.loginCharacter)
				state.loginSavedChoice.Dirty = true
			}
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
	form := eui.NewColumn(heading, serverChoice, savedChoice, characterInput, passwordInput, remember, statusItem, actions)
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
		if index >= len(state.loginForm.Contents)-1 {
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
	state.loginSavedChoice.Options, state.loginSavedChoice.Selected = viewportLoginSavedCharacterOptions(state.loginCharacter)
	state.loginSavedChoice.Disabled = busy || len(characters) == 0
	state.loginSavedChoice.Dirty = true
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
	if appViewports.layoutSnapshot() == viewportLayoutTiled {
		updateViewportImageSize(state, true)
		return
	}
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
	captureViewportFreeformChrome(state)
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

func setMultiSessionViewportLayout(layout viewportLayout) bool {
	if appSessions == nil || !appSessions.multiEnabled() || (layout != viewportLayoutFreeform && layout != viewportLayoutTiled) {
		return false
	}
	if appViewports.layoutSnapshot() == layout {
		return true
	}
	if appViewports.layoutSnapshot() == viewportLayoutFreeform {
		syncMultiSessionWorkspace()
	}
	appViewports.setLayout(layout)
	markMultiSessionWorkspaceUsed()
	multiSessionWorkspace.Layout = viewportLayoutName(layout)
	multiSessionWorkspaceDirty = true
	applyMultiSessionViewportLayout(true)
	refreshViewportTitles()
	refreshSessionsWindow()
	return true
}

func viewportLayoutName(layout viewportLayout) string {
	if layout == viewportLayoutTiled {
		return "tiled"
	}
	return "freeform"
}

func applyMultiSessionViewportLayoutIfNeeded() {
	if appSessions == nil || !appSessions.multiEnabled() {
		return
	}
	screenWidth, screenHeight := eui.ScreenSize()
	layout := appViewports.layoutSnapshot()
	if viewportWorkspaceAppliedLayout == layout && viewportWorkspaceScreenWidth == screenWidth && viewportWorkspaceScreenHeight == screenHeight {
		return
	}
	applyMultiSessionViewportLayout(false)
}

func applyMultiSessionViewportLayout(force bool) {
	if appSessions == nil || !appSessions.multiEnabled() {
		return
	}
	layout := appViewports.layoutSnapshot()
	screenWidth, screenHeight := eui.ScreenSize()
	if !force && viewportWorkspaceAppliedLayout == layout && viewportWorkspaceScreenWidth == screenWidth && viewportWorkspaceScreenHeight == screenHeight {
		return
	}
	if layout == viewportLayoutTiled {
		applyTiledSessionViewports()
	} else {
		applyFreeformSessionViewports()
	}
	viewportWorkspaceAppliedLayout = layout
	viewportWorkspaceScreenWidth = screenWidth
	viewportWorkspaceScreenHeight = screenHeight
	refreshViewportSelectionTreatment()
}

func applyFreeformSessionViewports() {
	screenWidth, screenHeight := eui.ScreenSize()
	for slot, view := range appViewports.snapshot() {
		state := view.render
		if !view.Active || state == nil || state.window == nil {
			continue
		}
		configureFreeformViewportChrome(state)
		placement := multiSessionWorkspace.ViewportSlots[slot]
		if multiSessionViewportPlacementValid(placement) && screenWidth > 0 && screenHeight > 0 {
			state.inAspectResize = true
			_ = state.window.SetSize(eui.Point{X: float32(placement.Size.X * float64(screenWidth)), Y: float32(placement.Size.Y * float64(screenHeight))})
			state.inAspectResize = false
			_ = state.window.SetPos(eui.Point{X: float32(placement.Position.X * float64(screenWidth)), Y: float32(placement.Position.Y * float64(screenHeight))})
		}
		resizeViewportWindow(state)
		state.window.MarkOpen()
	}
}

func configureFreeformViewportChrome(state *viewportRenderState) {
	if state == nil || state.window == nil {
		return
	}
	captureViewportFreeformChrome(state)
	win := state.window
	win.SetDocked(false)
	win.TitleHeight = state.freeformTitle
	win.Padding = state.freeformPadding
	win.Margin = state.freeformMargin
	win.Border = state.freeformBorder
	win.Outlined = state.freeformOutlined
	win.BorderColor = eui.Color{}
	win.Closable = false
	win.Movable = true
	win.Resizable = true
	win.Maximizable = false
	win.Dirty = true
}

func applyTiledSessionViewports() {
	area := multiSessionGameArea()
	if area.Empty() {
		return
	}
	appViewports.tile(area)
	rects := tiledViewportRects(area)
	for slot, view := range appViewports.snapshot() {
		state := view.render
		if !view.Active || state == nil || state.window == nil {
			continue
		}
		configureTiledViewportChrome(state)
		state.inAspectResize = true
		state.window.Resizable = true
		_ = state.window.SetPos(eui.Point{X: float32(rects[slot].Min.X), Y: float32(rects[slot].Min.Y)})
		_ = state.window.SetSize(eui.Point{X: float32(rects[slot].Dx()), Y: float32(rects[slot].Dy())})
		state.window.Resizable = false
		state.inAspectResize = false
		updateViewportImageSize(state, true)
		state.window.MarkOpen()
	}
}

func configureTiledViewportChrome(state *viewportRenderState) {
	if state == nil || state.window == nil {
		return
	}
	captureViewportFreeformChrome(state)
	win := state.window
	win.SetDocked(true)
	win.TitleHeight = state.freeformTitle
	win.Padding = 0
	win.Margin = 0
	win.Closable = false
	win.Movable = false
	win.Maximizable = false
	win.Dirty = true
}

func multiSessionGameArea() image.Rectangle {
	screenWidth, screenHeight := eui.ScreenSize()
	if screenWidth <= 0 || screenHeight <= 0 {
		return image.Rectangle{}
	}
	state := gs.GameWindow
	if normalizedWindowStateValid(state, true) {
		x0 := int(math.Round(state.Position.X * float64(screenWidth)))
		y0 := int(math.Round(state.Position.Y * float64(screenHeight)))
		x1 := x0 + int(math.Round(state.Size.X*float64(screenWidth)))
		y1 := y0 + int(math.Round(state.Size.Y*float64(screenHeight)))
		area := image.Rect(max(0, x0), max(0, y0), min(screenWidth, x1), min(screenHeight, y1))
		if !area.Empty() {
			return area
		}
	}
	if gameWin == nil {
		return image.Rectangle{}
	}
	pos, size := gameWin.GetPos(), gameWin.GetSize()
	return image.Rect(int(pos.X), int(pos.Y), int(pos.X+size.X), int(pos.Y+size.Y))
}

func refreshViewportSelectionTreatment() {
	selected := SessionID(0)
	if appSessions != nil {
		selected = appSessions.selectedID()
	}
	tiled := appSessions != nil && appSessions.multiEnabled() && appViewports.layoutSnapshot() == viewportLayoutTiled
	for _, view := range appViewports.snapshot() {
		state := view.render
		if state == nil || state.window == nil {
			continue
		}
		if tiled && view.SessionID == selected {
			state.window.Outlined = true
			state.window.Border = 3
			state.window.BorderColor = eui.AccentColor()
		} else if tiled {
			state.window.Outlined = false
			state.window.Border = state.freeformBorder
			state.window.BorderColor = eui.Color{}
		} else {
			state.window.Outlined = state.freeformOutlined
			state.window.Border = state.freeformBorder
			state.window.BorderColor = eui.Color{}
		}
		state.window.Dirty = true
	}
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
	if multi {
		applyMultiSessionViewportLayoutIfNeeded()
	} else if viewportWorkspaceAppliedLayout != viewportLayoutSingle {
		viewportWorkspaceAppliedLayout = viewportLayoutSingle
		viewportWorkspaceScreenWidth, viewportWorkspaceScreenHeight = eui.ScreenSize()
		applyManagedWindowLayout()
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
	state.loginSavedChoice = nil
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
