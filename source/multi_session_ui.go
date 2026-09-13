package main

import (
	"sync/atomic"

	"gothoom/eui"
)

var (
	sessionsWin                  *eui.WindowData
	sessionsToolbarButton        *eui.ItemData
	sessionWorkspaceUpdateQueued atomic.Bool
)

func init() {
	queueSessionWorkspaceUIUpdate = func() {
		if !sessionWorkspaceUpdateQueued.CompareAndSwap(false, true) {
			return
		}
		dispatchMainThread(func() {
			sessionWorkspaceUpdateQueued.Store(false)
			refreshViewportWorkspace()
			refreshSessionsWindow()
			refreshSessionsToolbarButton()
			refreshMusicSourceControls()
		})
	}
}

func sessionsReady() bool {
	return uiReady && !fake && clmov == "" && pcapPath == "" &&
		!status.NeedImages && !status.NeedSounds &&
		(setupWizardWin == nil || !setupWizardWin.IsOpen())
}

func refreshSessionsToolbarButton() {
	if sessionsToolbarButton == nil {
		return
	}
	sessionsToolbarButton.Disabled = !sessionsReady()
	if appSessions.multiEnabled() {
		sessionsToolbarButton.Text = "Sessions"
		sessionsToolbarButton.SetTooltip("Open session controls and switch the selected character.")
	} else {
		sessionsToolbarButton.Text = "Multi-session"
		sessionsToolbarButton.SetTooltip("Open four independent character session slots.")
	}
	sessionsToolbarButton.Dirty = true
}

func makeSessionsWindow() {
	if sessionsWin != nil {
		refreshSessionsWindow()
		return
	}
	sessionsWin = eui.NewWindow()
	sessionsWin.Title = "Sessions"
	sessionsWin.Closable = true
	sessionsWin.Resizable = false
	sessionsWin.AutoSize = true
	sessionsWin.Movable = true
	sessionsWin.OnClose = func() { sessionsWin = nil }
	refreshSessionsWindow()
	sessionsWin.AddWindow(false)
}

func refreshSessionsWindow() {
	if sessionsWin == nil {
		return
	}
	root := eui.NewColumn()
	for _, session := range appSessions.snapshot() {
		if session == nil {
			continue
		}
		row := eui.NewRow()
		label, _ := eui.NewText()
		label.Text = musicSourceLabel(session.ID())
		label.Size = eui.Point{X: 180, Y: 28}
		row.AddItem(label)

		_, _, connection := session.transport.connections()
		statusText, lastErr := session.login.statusSnapshot()
		if statusText == "" {
			statusText = "Disconnected"
		}
		if lastErr != "" {
			statusText = "Error: " + lastErr
		}
		statusItem, _ := eui.NewText()
		statusItem.Text = statusText
		statusItem.Size = eui.Point{X: 240, Y: 28}
		row.AddItem(statusItem)

		selectButton, selectEvents := eui.NewButton()
		if appSessions.selectedID() == session.ID() {
			selectButton.Text = "Selected"
			selectButton.Disabled = true
		} else {
			selectButton.Text = "Select"
		}
		selectButton.Size = eui.Point{X: 80, Y: 28}
		sessionID := session.ID()
		selectEvents.Handle = func(event eui.UIEvent) {
			if event.Type == eui.EventClick {
				appSessions.selectSession(sessionID)
				refreshSessionsWindow()
			}
		}
		row.AddItem(selectButton)

		actionButton, actionEvents := eui.NewButton()
		actionButton.Size = eui.Point{X: 100, Y: 28}
		if connection == sessionDisconnected {
			if session.transport.busy() {
				actionButton.Text = "Disconnecting"
				actionButton.Disabled = true
			} else {
				actionButton.Text = "Connect"
				actionEvents.Handle = func(event eui.UIEvent) {
					if event.Type == eui.EventClick {
						appSessions.selectSession(sessionID)
						focusViewportLogin(sessionID)
					}
				}
			}
		} else {
			actionButton.Text = "Disconnect"
			actionEvents.Handle = func(event eui.UIEvent) {
				if event.Type == eui.EventClick {
					appSessions.disconnectSession(sessionID)
				}
			}
		}
		row.AddItem(actionButton)
		root.AddItem(row)
	}

	actions := eui.NewRow()
	quitAll, quitEvents := eui.NewButton()
	quitAll.Text = "Quit All Sessions"
	quitAll.Size = eui.Point{X: 180, Y: 28}
	quitAll.Disabled = !appSessions.anyBusy()
	quitEvents.Handle = func(event eui.UIEvent) {
		if event.Type == eui.EventClick {
			go func() {
				_, _ = appSessions.disconnectAllAndWait(gameCtx)
				queueSessionWorkspaceUIUpdate()
			}()
		}
	}
	actions.AddItem(quitAll)
	single, singleEvents := eui.NewButton()
	single.Text = "Return to Single Session"
	single.Size = eui.Point{X: 200, Y: 28}
	single.Disabled = appSessions.anyBusy()
	singleEvents.Handle = func(event eui.UIEvent) {
		if event.Type == eui.EventClick && appSessions.disableMulti() {
			sessionsWin.Close()
		}
	}
	actions.AddItem(single)
	root.AddItem(actions)

	if len(sessionsWin.Contents) == 0 {
		sessionsWin.AddItem(root)
	} else {
		sessionsWin.ReplaceItem(0, root)
	}
	sessionsWin.Refresh()
}
