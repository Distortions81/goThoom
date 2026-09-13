package main

import (
	"strings"
	"testing"

	"gothoom/eui"
)

func TestSessionsToolbarButtonAlwaysHasLabel(t *testing.T) {
	if err := eui.Init(); err != nil {
		t.Fatalf("initialize EUI: %v", err)
	}
	initFont()
	oldButton := sessionsToolbarButton
	t.Cleanup(func() { sessionsToolbarButton = oldButton })

	button, _ := eui.NewButton()
	sessionsToolbarButton = button
	refreshSessionsToolbarButton()
	if button.Text != "Sessions" {
		t.Fatalf("toolbar label = %q, want Sessions", button.Text)
	}
}

func TestSessionsWindowDoesNotChooseWorkspaceLayout(t *testing.T) {
	if err := eui.Init(); err != nil {
		t.Fatalf("initialize EUI: %v", err)
	}
	oldSessions, oldViewports, oldWindow := appSessions, appViewports, sessionsWin
	t.Cleanup(func() {
		appSessions, appViewports, sessionsWin = oldSessions, oldViewports, oldWindow
	})

	appSessions = newSessionManager(mustNewSession(primarySessionID))
	appSessions.mu.Lock()
	appSessions.multi = true
	for slot := 1; slot < maxSessions; slot++ {
		id, _ := sessionIDForSlot(slot)
		appSessions.slots[slot] = mustNewSession(id)
	}
	appSessions.mu.Unlock()
	appViewports = newViewportManager()
	sessionsWin = eui.NewWindow()
	refreshSessionsWindow()

	text := sessionWindowText(sessionsWin.Contents)
	for _, unwanted := range []string{"Session layout", "Freeform", "Tiled 2×2"} {
		if strings.Contains(text, unwanted) {
			t.Fatalf("Sessions window contains independent layout control %q", unwanted)
		}
	}
}

func sessionWindowText(items []*eui.ItemData) string {
	var builder strings.Builder
	var walk func([]*eui.ItemData)
	walk = func(items []*eui.ItemData) {
		for _, item := range items {
			if item == nil {
				continue
			}
			builder.WriteString(item.Text)
			builder.WriteByte('\n')
			walk(item.Contents)
		}
	}
	walk(items)
	return builder.String()
}
