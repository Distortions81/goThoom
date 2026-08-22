package main

import (
	"time"

	"gothoom/eui"

	"github.com/hajimehoshi/ebiten/v2"
	text "github.com/hajimehoshi/ebiten/v2/text/v2"
)

type notification struct {
	item   *eui.ItemData
	expiry time.Time
}

var notifications []*notification

// showNotification displays msg in the Clan Lord window if notifications are
// enabled. Messages disappear after a timeout or when clicked.
// Optional note keys can be provided to customize the notification sound.
func showNotification(msg string, keys ...int) {
	if isWASM {
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
	if gs.NotifyWhenBackground && !ebiten.IsFocused() {
		notifyDesktop("goThoom", msg)
	}

	btn, events := eui.NewButton()
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

	events.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			removeNotification(btn)
		}
	}

	dur := time.Duration(gs.NotificationDuration * float64(time.Second))
	if dur <= 0 {
		dur = 6 * time.Second
	}
	notifications = append(notifications, &notification{item: btn, expiry: time.Now().Add(dur)})
	gameWin.AddItem(btn)
	layoutNotifications()
}

func removeNotification(item *eui.ItemData) {
	for i, n := range notifications {
		if n.item == item {
			notifications = append(notifications[:i], notifications[i+1:]...)
			break
		}
	}
	if gameWin != nil {
		for i, it := range gameWin.Contents {
			if it == item {
				gameWin.Contents = append(gameWin.Contents[:i], gameWin.Contents[i+1:]...)
				break
			}
		}
		gameWin.Refresh()
	}
}

func clearNotifications() {
	for _, n := range notifications {
		if gameWin != nil {
			for i, it := range gameWin.Contents {
				if it == n.item {
					gameWin.Contents = append(gameWin.Contents[:i], gameWin.Contents[i+1:]...)
					break
				}
			}
		}
	}
	notifications = nil
	if gameWin != nil {
		gameWin.Refresh()
	}
}

func layoutNotifications() {
	if gameWin == nil {
		return
	}
	margin := float32(8)
	spacer := float32(4)
	winSz := gameWin.GetSize()
	y := winSz.Y - margin - 100
	scale := eui.UIScale()
	if gameWin.NoScale {
		scale = 1
	}
	for i := len(notifications) - 1; i >= 0; i-- {
		it := notifications[i].item
		sz := it.GetSize()
		y -= sz.Y
		x := winSz.X - sz.X - margin
		it.Position = eui.Point{X: x / scale, Y: y / scale}
		y -= spacer
		it.Dirty = true
	}
	gameWin.Refresh()
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
