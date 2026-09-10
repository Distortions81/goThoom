package main

import (
	"testing"

	"gothoom/eui"
)

func TestDismissGameMessageOverlayAtRemovesTopmostMessage(t *testing.T) {
	originalWindow, originalImageItem := gameWin, gameImageItem
	originalNotifications, originalThinkMessages := notifications, thinkMessages
	t.Cleanup(func() {
		gameWin, gameImageItem = originalWindow, originalImageItem
		notifications, thinkMessages = originalNotifications, originalThinkMessages
	})

	gameWin = eui.NewWindow()
	gameWin.NoScale = true
	gameWin.Margin, gameWin.Border, gameWin.BorderPad, gameWin.Padding, gameWin.TitleHeight = 0, 0, 0, 0, 0
	_ = gameWin.SetPos(eui.Point{X: 100, Y: 50})
	gameImageItem = &eui.ItemData{Position: eui.Point{X: 2, Y: 3}}

	think := &eui.ItemData{DrawRect: eui.Rect{X0: 10, Y0: 10, X1: 50, Y1: 30}}
	notice := &eui.ItemData{DrawRect: eui.Rect{X0: 10, Y0: 10, X1: 50, Y1: 30}}
	thinkMessages = []*thinkMessage{{item: think}}
	notifications = []*notification{{item: notice}}

	if !dismissGameMessageOverlayAt(112, 63) {
		t.Fatal("click inside message overlay was not handled")
	}
	if len(notifications) != 0 || len(thinkMessages) != 1 {
		t.Fatalf("click removed wrong overlay: notifications=%d think=%d", len(notifications), len(thinkMessages))
	}
	if dismissGameMessageOverlayAt(400, 400) {
		t.Fatal("click outside message overlays was handled")
	}
}
