package main

import (
	"fmt"
	"image"
	"math"

	"gothoom/eui"

	"github.com/hajimehoshi/ebiten/v2"
	text "github.com/hajimehoshi/ebiten/v2/text/v2"
)

type activeGameOverlayPositions struct {
	recPlay      image.Rectangle
	recPlayLabel string
	fps          image.Rectangle
	fpsLabel     string
	activity     image.Rectangle
}

type itemOverlayReservation struct {
	item   *eui.ItemData
	handle int
}

// layoutActiveGameOverlays is the single registration point for overlays that
// hug an edge of the rendered game view. Each participant reserves its actual
// pixel size, so unrelated features can share an anchor without knowing about
// one another.
func layoutActiveGameOverlays(bounds image.Rectangle, activity clientActivity) activeGameOverlayPositions {
	positions := activeGameOverlayPositions{}
	if bounds.Empty() {
		return positions
	}

	layout := newGameOverlayLayout(bounds)
	recPlayHandle := -1
	fpsHandle := -1
	activityHandle := -1
	streamHandle := -1
	var itemReservations []itemOverlayReservation

	positions.recPlayLabel = activeRecPlayLabel()
	if positions.recPlayLabel != "" && mainFontBold != nil {
		recPlayHandle = layout.Add(gameOverlayTopLeft, badgeOverlaySize(positions.recPlayLabel))
	}
	for _, message := range thinkMessages {
		if message != nil && message.item != nil {
			itemReservations = append(itemReservations, itemOverlayReservation{
				item: message.item, handle: layout.Add(gameOverlayTopLeft, gameOverlayItemSize(message.item)),
			})
		}
	}

	if streamOutputRunning() && streamGameBadge != nil && !streamGameBadge.Invisible {
		streamHandle = layout.Add(gameOverlayTopRight, gameOverlayItemSize(streamGameBadge))
	}
	if fpsOverlayVisible() {
		positions.fpsLabel = fmt.Sprintf("%.0f FPS", ebiten.ActualFPS())
		fpsHandle = layout.Add(gameOverlayTopRight, textOverlaySize(positions.fpsLabel))
	}

	// Notifications are registered before the activity lights so the small,
	// persistent indicator row remains closest to the bottom edge.
	for _, notification := range notifications {
		if notification != nil && notification.item != nil {
			itemReservations = append(itemReservations, itemOverlayReservation{
				item: notification.item, handle: layout.Add(gameOverlayBottomRight, gameOverlayItemSize(notification.item)),
			})
		}
	}
	if activity != clientActivityNone {
		activityHandle = layout.Add(gameOverlayBottomRight, clientActivityOverlaySize())
	}

	rects := layout.Rects()
	if recPlayHandle >= 0 {
		positions.recPlay = rects[recPlayHandle]
	}
	if fpsHandle >= 0 {
		positions.fps = rects[fpsHandle]
	}
	if activityHandle >= 0 {
		positions.activity = rects[activityHandle]
	}
	for _, reservation := range itemReservations {
		positionGameOverlayItem(reservation.item, rects[reservation.handle])
	}
	if streamHandle >= 0 && positionGameOverlayItem(streamGameBadge, rects[streamHandle]) && gameWin != nil {
		gameWin.Refresh()
	}
	return positions
}

func activeRecPlayLabel() string {
	if recorder != nil || recordingMovie {
		return "REC"
	}
	if playingMovie && !setupWizardPreviewActive {
		return "PLAY"
	}
	return ""
}

func fpsOverlayVisible() bool {
	wizardOpen := setupWizardWin != nil && setupWizardWin.IsOpen()
	return mainFontBold != nil && (gs.ShowFPS || wizardOpen)
}

func badgeOverlaySize(label string) image.Point {
	w, h := text.Measure(label, mainFontBold, 0)
	return image.Pt(int(math.Ceil(w))+32, int(math.Ceil(h))+12)
}

func textOverlaySize(label string) image.Point {
	w, h := text.Measure(label, mainFontBold, 0)
	return image.Pt(int(math.Ceil(w))+1, int(math.Ceil(h))+1)
}

func gameOverlayItemSize(item *eui.ItemData) image.Point {
	size := item.GetSize()
	return image.Pt(int(math.Ceil(float64(size.X))), int(math.Ceil(float64(size.Y))))
}

func positionGameOverlayItem(item *eui.ItemData, bounds image.Rectangle) bool {
	if item == nil || gameImageItem == nil || bounds.Empty() {
		return false
	}
	scale := eui.UIScale()
	if gameWin != nil && gameWin.NoScale {
		scale = 1
	}
	position := eui.Point{
		X: gameImageItem.Position.X + float32(bounds.Min.X)/scale,
		Y: gameImageItem.Position.Y + float32(bounds.Min.Y)/scale,
	}
	if item.Position == position {
		return false
	}
	item.Position = position
	item.Dirty = true
	return true
}

func clientActivityOverlaySize() image.Point {
	radius := int(math.Ceil(float64(clientActivityIndicatorRadius + 2)))
	return image.Pt(radius*2+int(clientActivityIndicatorSpacing)*2, radius*2)
}
