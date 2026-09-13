package main

import (
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
