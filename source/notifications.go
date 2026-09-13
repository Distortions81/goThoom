package main

import (
	"fmt"
	"time"

	"gothoom/eui"

	text "github.com/hajimehoshi/ebiten/v2/text/v2"
)

type notification struct {
	item   *eui.ItemData
	expiry time.Time
	source SessionID
}

var notifications []*notification

// showNotification displays msg in the Clan Lord window if notifications are
// enabled. Messages disappear after a timeout or when clicked.
// Optional note keys can be provided to customize the notification sound.
func showNotification(msg string, keys ...int) {
	showNotificationForSource(nil, msg, keys...)
}

func showSessionNotification(session *Session, msg string, keys ...int) {
	if session == nil {
		showNotification(msg, keys...)
		return
	}
	label := session.characterName()
	if label == "" {
		label = fmt.Sprintf("Session %d", session.ID())
	}
	msg = label + ": " + msg
	session.publishEvent(sessionEvent{Kind: sessionEventNotification, Text: msg})
	showNotificationForSource(session, msg, keys...)
}

func showNotificationForSource(session *Session, msg string, keys ...int) {
	if isWASM || seekingMov {
		return
	}
	if !gs.Notifications || gameWin == nil {
		return
	}
	if gs.NotificationBeep {
		if len(keys) == 0 {
			// middle C harp beep
			playBeep(46, 60)
		} else {
			playHarpNotes(keys...)
		}
	}
	// If app is unfocused and user enabled background desktop notifications,
	// mirror this in-game notification to the OS.
	if gs.NotifyWhenBackground && !windowIsFocused() {
		notifyDesktop("goThoom", msg)
	}
	if gs.ChatTTSNotifications {
		speakTTSMessage(msg)
	}

	btn, _ := eui.NewButton()
	btn.Text = msg
	btn.FontSize = float32(gs.ChatFontSize)
	btn.Filled = true
	btn.Outlined = false
	alpha := uint8(gs.BubbleOpacity * 255)
	btn.Color = eui.NewColor(0, 0, 0, alpha)
	btn.TextColor = eui.NewColor(255, 255, 255, 255)
	btn.HoverColor = btn.Color
	btn.ClickColor = btn.Color
	btn.Fillet = 6
	btn.Padding = 4
	btn.Margin = 0

	textSize := (btn.FontSize * eui.UIScale()) + 2
	face := &text.GoTextFace{Source: eui.FontSource(), Size: float64(textSize)}
	w, h := text.Measure(msg, face, 0)
	btn.Size = eui.Point{
		X: float32(w)/eui.UIScale() + btn.Padding*2 + btn.BorderPad*2,
		Y: float32(h)/eui.UIScale() + btn.Padding*2 + btn.BorderPad*2,
	}

	dur := time.Duration(gs.NotificationDuration * float64(time.Second))
	if dur <= 0 {
		dur = 6 * time.Second
	}
	source := SessionID(0)
	if session != nil {
		source = session.ID()
	}
	notifications = append(notifications, &notification{item: btn, expiry: time.Now().Add(dur), source: source})
	layoutNotifications()
}

func removeNotification(item *eui.ItemData) {
	for i, n := range notifications {
		if n.item == item {
			notifications = append(notifications[:i], notifications[i+1:]...)
			break
		}
	}
}

func clearNotifications() {
	notifications = nil
	markWorldRenderChanged()
}

func layoutNotifications() {
	if gameWin == nil {
		return
	}
	// Positions are assigned with every other game overlay in Draw, after the
	// current world-view bounds and all active reservations are known.
	markWorldRenderChanged()
}

func updateNotifications() {
	if len(notifications) == 0 {
		return
	}
	now := time.Now()
	changed := false
	for i := 0; i < len(notifications); {
		if now.After(notifications[i].expiry) {
			removeNotification(notifications[i].item)
			changed = true
		} else {
			i++
		}
	}
	if changed {
		layoutNotifications()
	}
}
