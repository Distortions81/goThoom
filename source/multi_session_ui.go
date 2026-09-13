package main

import (
	"sync/atomic"

	"gothoom/eui"
)

var (
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
	sessionsToolbarButton.Text = "Sessions"
	ready := sessionsReady()
	multi := appSessions != nil && appSessions.multiEnabled()
	busy := multi && appSessions.anyBusy()
	sessionsToolbarButton.Disabled = !ready || busy
	switch {
	case !multi:
		sessionsToolbarButton.SetTooltip("Start multi-session mode with four independent character slots.")
	case busy:
		sessionsToolbarButton.SetTooltip("Log out of every session before returning to single-session mode.")
	default:
		sessionsToolbarButton.SetTooltip("Return to single-session mode.")
	}
	sessionsToolbarButton.Dirty = true
}
