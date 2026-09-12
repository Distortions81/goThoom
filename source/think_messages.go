package main

import (
	"math"
	"strings"
	"time"

	"gothoom/eui"

	text "github.com/hajimehoshi/ebiten/v2/text/v2"
)

// sndThinkTo matches the original client's notification sound for think messages.
const sndThinkTo = 58

type thinkMessage struct {
	item   *eui.ItemData
	expiry time.Time
}

var thinkMessages []*thinkMessage

// showThinkMessage displays a temporary think message at the top of the screen.
// msg should already include the sender's name.
func showThinkMessage(msg string) {
	if seekingMov || gameWin == nil {
		return
	}
	playSound([]uint16{sndThinkTo})
	btn, _ := eui.NewButton()
	btn.Text = msg
	btn.FontSize = float32(gs.ChatFontSize)
	btn.Filled = true
	btn.Outlined = false
	btn.Color = eui.NewColor(0, 0, 0, 160)
	btn.TextColor = eui.NewColor(255, 255, 255, 255)
	btn.HoverColor = btn.Color
	btn.ClickColor = btn.Color
	btn.Fillet = 6
	btn.Padding = 4
	btn.Margin = 0

	textSize := (btn.FontSize * eui.UIScale()) + 2
	face := &text.GoTextFace{Source: eui.FontSource(), Size: float64(textSize)}

	metrics := face.Metrics()
	lineHeight := math.Ceil(metrics.HAscent) + math.Ceil(metrics.HDescent) + math.Ceil(metrics.HLineGap)

	winSize := gameWin.GetSize()
	pad := float64((btn.Padding + btn.BorderPad) * eui.UIScale())
	maxWidth := float64(winSize.X)/4 - 2*pad
	usedWidth, lines := eui.WrapText(msg, face, maxWidth)
	btn.Text = strings.Join(lines, "\n")
	btn.Size = eui.Point{
		X: float32(usedWidth)/eui.UIScale() + btn.Padding*2 + btn.BorderPad*2,
		Y: float32(lineHeight*float64(len(lines)))/eui.UIScale() + btn.Padding*2 + btn.BorderPad*2,
	}

	dur := time.Duration(gs.NotificationDuration * float64(time.Second))
	if dur <= 0 {
		dur = 6 * time.Second
	}
	thinkMessages = append(thinkMessages, &thinkMessage{item: btn, expiry: time.Now().Add(dur)})
	layoutThinkMessages()
}

func removeThinkMessage(item *eui.ItemData) {
	for i, m := range thinkMessages {
		if m.item == item {
			thinkMessages = append(thinkMessages[:i], thinkMessages[i+1:]...)
			break
		}
	}
}

func layoutThinkMessages() {
	if gameWin == nil {
		return
	}
	// Positions are assigned with every other game overlay in Draw, after the
	// current world-view bounds and all active reservations are known.
	markWorldRenderChanged()
}

func updateThinkMessages() {
	if len(thinkMessages) == 0 {
		return
	}
	now := time.Now()
	changed := false
	for i := 0; i < len(thinkMessages); {
		if now.After(thinkMessages[i].expiry) {
			removeThinkMessage(thinkMessages[i].item)
			changed = true
		} else {
			i++
		}
	}
	if changed {
		layoutThinkMessages()
	}
}
