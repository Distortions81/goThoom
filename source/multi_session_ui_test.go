package main

import (
	"context"
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

func TestSessionsToolbarButtonExplainsModeExit(t *testing.T) {
	if err := eui.Init(); err != nil {
		t.Fatalf("initialize EUI: %v", err)
	}
	initFont()
	oldSessions, oldButton := appSessions, sessionsToolbarButton
	t.Cleanup(func() {
		appSessions, sessionsToolbarButton = oldSessions, oldButton
	})

	appSessions = newSessionManager(mustNewSession(primarySessionID))
	button, _ := eui.NewButton()
	sessionsToolbarButton = button
	refreshSessionsToolbarButton()
	if !strings.Contains(button.Tooltip, "Start multi-session") {
		t.Fatalf("single-session tooltip = %q", button.Tooltip)
	}

	appSessions.mu.Lock()
	appSessions.multi = true
	appSessions.mu.Unlock()
	refreshSessionsToolbarButton()
	if !strings.Contains(button.Tooltip, "Return to single-session") {
		t.Fatalf("idle multi-session tooltip = %q", button.Tooltip)
	}

	_, cancel := context.WithCancel(context.Background())
	if !appSessions.slots[0].transport.begin(cancel) {
		t.Fatal("could not put session into connecting state")
	}
	t.Cleanup(appSessions.slots[0].transport.failConnect)
	refreshSessionsToolbarButton()
	if !button.Disabled || !strings.Contains(button.Tooltip, "Log out of every session") {
		t.Fatalf("busy multi-session button = disabled %v, tooltip %q", button.Disabled, button.Tooltip)
	}
}
