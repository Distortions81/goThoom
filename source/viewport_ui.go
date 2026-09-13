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

const viewportLoginPanelWidth float32 = 360

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
	if session.transport.busy() {
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
	if state == nil || session == nil || session.transport.busy() || state.loginDemoLookup {
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
	refreshViewportLoginCharacterList(state, session)
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
	busy := session.transport.busy() || state.loginDemoLookup
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
	layoutViewportLoginOverlay(state)
	state.window.Refresh()
}

func focusViewportLogin(id SessionID) {
	state := appViewports.renderStateForSession(id)
	if state == nil || state.window == nil || state.loginOverlay == nil || state.loginOverlay.Invisible {
		return
	}
	state.window.BringForward()
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

func desiredMultiSessionViewportLayout() viewportLayout {
	if gs.TiledWindows {
		return viewportLayoutTiled
	}
	return viewportLayoutFreeform
}

func applyMultiSessionViewportLayoutIfNeeded() {
	if appSessions == nil || !appSessions.multiEnabled() {
		return
	}
	screenWidth, screenHeight := eui.ScreenSize()
	layout := desiredMultiSessionViewportLayout()
	if current := appViewports.layoutSnapshot(); current != layout {
		if current == viewportLayoutFreeform {
			syncMultiSessionWorkspace()
		}
		appViewports.setLayout(layout)
	}
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
	state.loginCharacters = nil
	state.loginAdd = nil
	state.loginEdit = nil
	state.loginDelete = nil
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
