package main

import (
	"fmt"
	"sync/atomic"

	"gothoom/eui"
)

var (
	sessionTabBar                *eui.ItemData
	sessionWorkspaceUpdateQueued atomic.Bool
)

const sessionTabBarHeight = 30

func init() {
	previousSelected := queueSelectedSessionUIUpdate
	queueSelectedSessionUIUpdate = func() {
		previousSelected()
		queueSessionWorkspaceUIUpdate()
	}
	queueSessionWorkspaceUIUpdate = func() {
		if !sessionWorkspaceUpdateQueued.CompareAndSwap(false, true) {
			return
		}
		dispatchMainThread(func() {
			sessionWorkspaceUpdateQueued.Store(false)
			refreshViewportWorkspace()
		})
	}
}

func sessionTabsVisible() bool {
	return !fake && pcapPath == "" && !setupWizardPreviewActive
}

func sessionTabBarPixelHeight() int {
	if sessionTabBar == nil || sessionTabBar.Invisible || !sessionTabsVisible() {
		return 0
	}
	return sessionTabBarHeight
}

func ensureSessionTabBar() bool {
	if sessionTabBar != nil || gameWin == nil {
		return false
	}
	sessionTabBar = eui.NewRow()
	sessionTabBar.Fixed = true
	sessionTabBar.ConstrainToSize = true
	gameWin.PrependItem(sessionTabBar)
	return true
}

func sessionTabWidths(available float32, count int) (tab, closeButton, addButton float32) {
	if count < 1 || available <= 0 {
		return 0, 0, 0
	}
	addButton = minFloat32(28, available/float32(count+1))
	tab = (available - addButton) / float32(count)
	closeButton = minFloat32(22, tab/2)
	return tab, closeButton, addButton
}

func sessionTabLabel(session *Session, available float32) string {
	if session == nil {
		return ""
	}
	if session == moviePlaybackSession {
		name := session.characterName()
		if name == "" || available < 100 {
			return "Movie"
		}
		return "Movie: " + name
	}
	if available < 56 {
		return fmt.Sprintf("%d", session.ID())
	}
	name := session.characterName()
	if name == "" {
		return fmt.Sprintf("Session %d", session.ID())
	}
	if available < 100 {
		runes := []rune(name)
		if len(runes) > 8 {
			runes = runes[:8]
		}
		return fmt.Sprintf("%d: %s", session.ID(), string(runes))
	}
	return fmt.Sprintf("%d: %s", session.ID(), name)
}

func selectSessionTabPosition(position int) bool {
	if appSessions == nil || position < 1 {
		return false
	}
	seen := 0
	for _, session := range appSessions.snapshot() {
		if session == nil {
			continue
		}
		seen++
		if seen == position {
			return appSessions.selectSession(session.ID())
		}
	}
	return false
}

func selectAdjacentSessionTab(direction int) bool {
	if appSessions == nil || direction == 0 {
		return false
	}
	var open []SessionID
	for _, session := range appSessions.snapshot() {
		if session != nil {
			open = append(open, session.ID())
		}
	}
	if len(open) == 0 {
		return false
	}
	selected := appSessions.selectedID()
	index := 0
	for candidate, id := range open {
		if id == selected {
			index = candidate
			break
		}
	}
	if direction > 0 {
		index = (index + 1) % len(open)
	} else {
		index = (index - 1 + len(open)) % len(open)
	}
	return appSessions.selectSession(open[index])
}

func keepSessionTabOpen(session *Session) bool {
	return session.ID() == primarySessionID || appSessions.count() <= 1
}

func confirmCloseSessionTab(session *Session) *eui.WindowData {
	if session == nil || appSessions == nil {
		return nil
	}
	if session == moviePlaybackSession && movieWin != nil {
		movieWin.Close()
		return nil
	}
	name := session.characterName()
	if name == "" {
		name = fmt.Sprintf("Session %d", session.ID())
	}
	title, action := "Close Session Tab", "Close"
	message := fmt.Sprintf("Close %s? Any active connection will be disconnected.", name)
	if keepSessionTabOpen(session) {
		title, action = "Disconnect Session", "Disconnect"
		message = fmt.Sprintf("Disconnect %s and return to login? The session tab will stay open.", name)
	}
	return eui.ShowPopup(title, message, []eui.PopupButton{
		{Text: "Cancel"},
		{Text: action, Color: &eui.ColorDarkRed, HoverColor: &eui.ColorRed, Action: func() {
			if current, ok := appSessions.session(session.ID()); !ok || current != session {
				return
			}
			if keepSessionTabOpen(session) {
				handleSessionDisconnect(session)
			} else {
				appSessions.closeSession(session.ID())
			}
		}},
	})
}

func refreshSessionTabs() {
	ensureSessionTabBar()
	if sessionTabBar == nil || gameWin == nil {
		return
	}
	visible := sessionTabsVisible()
	sessionTabBar.Invisible = !visible
	if !visible {
		return
	}
	scale := eui.UIScale()
	if scale <= 0 {
		scale = 1
	}
	sessions := appSessions.snapshot()
	count := appSessions.count()
	contentWidth := gameWin.GetSize().X - 2*(gameWin.Padding+gameWin.BorderPad)
	width := maxFloat32(1, contentWidth) / scale
	tabWidth, actionWidth, plusWidth := sessionTabWidths(width, count)
	items := make([]*eui.ItemData, 0, count+1)
	selected := appSessions.selectedID()
	position := 0
	for _, session := range sessions {
		if session == nil {
			continue
		}
		position++
		session := session
		segment := eui.NewOverlay()
		segment.Fixed = true
		segment.ConstrainToSize = true
		segment.Size = eui.Point{X: tabWidth, Y: sessionTabBarHeight / scale}
		// The action hit targets live inside the full-width tab surface. The game
		// window does not scale positions, while item sizes retain the global UI
		// scale, so overlay positions are expressed in final screen pixels.
		tabPixelWidth := tabWidth * scale
		actionPixelWidth := actionWidth * scale
		selectWidth := tabWidth
		selectButton, selectEvents := eui.NewButton()
		musicVisible := false
		if slot, ok := session.ID().Slot(); ok {
			musicVisible = sessionMusicIndicators[slot]
		}
		labelWidth := tabWidth - actionWidth
		if musicVisible {
			labelWidth -= actionWidth
		}
		selectButton.Text = ""
		selectButton.Size = eui.Point{X: selectWidth, Y: sessionTabBarHeight / scale}
		selectButton.Position = eui.Point{}
		selectButton.SetTooltip(hotkeyComboForCommand(fmt.Sprintf("/tab %d", position)))
		selectButton.SelectionIndicator = session.ID() == selected
		selectEvents.Handle = func(event eui.UIEvent) {
			if event.Type == eui.EventClick {
				appSessions.selectSession(session.ID())
			}
		}
		segment.AddItem(selectButton)

		labelButton, labelEvents := eui.NewButton()
		labelButton.Text = sessionTabLabel(session, labelWidth)
		labelButton.NoSurface = true
		labelButton.ConstrainToSize = true
		labelButton.Size = eui.Point{X: maxFloat32(1, labelWidth), Y: sessionTabBarHeight / scale}
		labelButton.Position = eui.Point{}
		if musicVisible {
			labelButton.Position.X = actionPixelWidth
		}
		labelButton.SetTooltip(selectButton.Tooltip)
		labelEvents.Handle = selectEvents.Handle
		segment.AddItem(labelButton)

		if musicVisible {
			musicButton, musicEvents := eui.NewButton()
			setMaterialIconOnly(musicButton, "music_note", "")
			musicButton.ImageName = "music_note"
			musicButton.NoSurface = true
			musicButton.Size = eui.Point{X: actionWidth, Y: sessionTabBarHeight / scale}
			musicButton.Position = eui.Point{}
			musicButton.SetTooltip(selectButton.Tooltip)
			musicEvents.Handle = selectEvents.Handle
			segment.AddItem(musicButton)
		}

		closeButton, closeEvents := eui.NewButton()
		setMaterialIconOnly(closeButton, "close", "X")
		// Material icons load on the first draw. Do not let their temporary text
		// fallback enlarge a narrow tab before the icon is available.
		closeButton.Text = ""
		closeButton.NoSurface = true
		closeButton.Size = eui.Point{X: actionWidth, Y: sessionTabBarHeight / scale}
		closeButton.Position = eui.Point{X: tabPixelWidth - actionPixelWidth}
		closeButton.Disabled = keepSessionTabOpen(session) && !session.connectionBusy()
		if closeButton.Disabled {
			closeButton.SetTooltip("This session is already disconnected.")
		} else if keepSessionTabOpen(session) {
			closeButton.SetTooltip(fmt.Sprintf("Disconnect Session %d and keep its tab open.", session.ID()))
		} else {
			closeButton.SetTooltip(fmt.Sprintf("Close Session %d.", session.ID()))
		}
		closeEvents.Handle = func(event eui.UIEvent) {
			if event.Type == eui.EventClick && !closeButton.Disabled {
				confirmCloseSessionTab(session)
			}
		}
		segment.AddItem(closeButton)
		items = append(items, segment)
	}

	addButton, addEvents := eui.NewButton()
	setMaterialIconOnly(addButton, "add", "+")
	addButton.Size = eui.Point{X: plusWidth, Y: sessionTabBarHeight / scale}
	addButton.Position = eui.Point{}
	addButton.Disabled = count >= maxSessions
	if addButton.Disabled {
		addButton.SetTooltip(fmt.Sprintf("The %d-session limit is reached.", maxSessions))
	} else {
		addButton.SetTooltip("Open another session tab.")
	}
	addEvents.Handle = func(event eui.UIEvent) {
		if event.Type == eui.EventClick && !addButton.Disabled {
			appSessions.addSession()
		}
	}
	items = append(items, addButton)
	sessionTabBar.Position = eui.Point{}
	sessionTabBar.Size = eui.Point{X: width, Y: sessionTabBarHeight / scale}
	sessionTabBar.SetItems(items)
	gameWin.Refresh()
}

func resizeSessionTabs() {
	if sessionTabBar != nil {
		refreshSessionTabs()
	}
}
