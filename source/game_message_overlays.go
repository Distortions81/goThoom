package main

import (
	"gothoom/eui"

	"github.com/hajimehoshi/ebiten/v2"
)

// drawGameMessageOverlays keeps transient thoughts and notices in the game
// framebuffer. Game-only captures then include the same messages players see.
func drawGameMessageOverlays(dst *ebiten.Image) {
	if dst == nil || gameImageItem == nil {
		return
	}

	scale := eui.UIScale()
	if gameWin != nil && gameWin.NoScale {
		scale = 1
	}
	origin := gameImageItem.Position
	draw := func(item *eui.ItemData) {
		if item == nil {
			return
		}
		eui.DrawItemAt(dst, item, eui.Point{
			X: (item.Position.X - origin.X) * scale,
			Y: (item.Position.Y - origin.Y) * scale,
		})
	}
	for _, message := range thinkMessages {
		draw(message.item)
	}
	for _, message := range notifications {
		draw(message.item)
	}
}

// dismissGameMessageOverlayAt removes the topmost transient message at a
// screen position. The messages are rendered into gameImage rather than added
// to gameWin, so their input needs the matching local-coordinate hit test.
func dismissGameMessageOverlayAt(x, y int) bool {
	item := gameMessageOverlayAt(x, y)
	if item == nil {
		return false
	}
	for _, message := range notifications {
		if message.item == item {
			removeNotification(item)
			markWorldRenderChanged()
			return true
		}
	}
	for _, message := range thinkMessages {
		if message.item == item {
			removeThinkMessage(item)
			markWorldRenderChanged()
			return true
		}
	}
	return false
}

// gameMessageOverlayAt returns the topmost visible transient message at a
// screen position. DrawRect is local to gameImage after DrawItemAt, while
// input arrives in screen coordinates.
func gameMessageOverlayAt(x, y int) *eui.ItemData {
	if gameWin == nil || gameImageItem == nil {
		return nil
	}
	scale := windowInputScale(gameWin)
	originX, originY := gameWindowOrigin()
	localX := float32(x-originX) - gameImageItem.Position.X*scale
	localY := float32(y-originY) - gameImageItem.Position.Y*scale
	contains := func(item *eui.ItemData) bool {
		return item != nil && !item.Invisible && localX >= item.DrawRect.X0 && localX <= item.DrawRect.X1 && localY >= item.DrawRect.Y0 && localY <= item.DrawRect.Y1
	}
	for i := len(notifications) - 1; i >= 0; i-- {
		if contains(notifications[i].item) {
			return notifications[i].item
		}
	}
	for i := len(thinkMessages) - 1; i >= 0; i-- {
		if contains(thinkMessages[i].item) {
			return thinkMessages[i].item
		}
	}
	return nil
}
