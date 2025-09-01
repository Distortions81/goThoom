package main

import (
	"image/color"
	"strings"
	"time"

	"gothoom/eui"
)

var labelColors = []color.NRGBA{
	{0xff, 0x00, 0x00, 0xff}, // red
	{0xff, 0x80, 0x00, 0xff}, // orange
	{0xff, 0xff, 0x00, 0xff}, // yellow
	{0x00, 0xff, 0x00, 0xff}, // green
	{0x00, 0x80, 0xff, 0xff}, // blue
	{0x80, 0x00, 0xff, 0xff}, // purple
	{0xff, 0x00, 0xff, 0xff}, // pink
	{0x00, 0xff, 0xff, 0xff}, // teal
	{0x80, 0x40, 0x00, 0xff}, // brown
	{0x80, 0x80, 0x80, 0xff}, // gray
}

var defaultLabelNames = []string{
	"Red", "Orange", "Yellow", "Green", "Blue",
	"Purple", "Pink", "Teal", "Brown", "Gray",
}

var labelNames = append([]string(nil), defaultLabelNames...)

var labelEditWin *eui.WindowData

func labelName(i int) string {
	if i <= 0 || i > len(labelColors) {
		return ""
	}
	if i-1 < len(labelNames) && labelNames[i-1] != "" {
		return labelNames[i-1]
	}
	return defaultLabelNames[i-1]
}

func applyPlayerLabel(p *Player) {
	lbl := p.GlobalLabel
	if lbl == 0 {
		lbl = p.LocalLabel
	}
	p.FriendLabel = lbl
	switch lbl {
	case 6:
		p.Blocked = true
		p.Ignored = false
		p.Friend = false
	case 7:
		p.Ignored = true
		p.Blocked = false
		p.Friend = false
	default:
		p.Blocked = false
		p.Ignored = false
		p.Friend = lbl > 0
	}
}

func setPlayerLabel(name string, label int, global bool) {
	p := getPlayer(name)
	playersMu.Lock()
	if global {
		p.GlobalLabel = label
	} else {
		p.LocalLabel = label
		for i := range characters {
			if strings.EqualFold(characters[i].Name, playerName) {
				if characters[i].Labels == nil {
					characters[i].Labels = make(map[string]int)
				}
				if label == 0 {
					delete(characters[i].Labels, name)
				} else {
					characters[i].Labels[name] = label
				}
				saveCharacters()
				break
			}
		}
	}
	applyPlayerLabel(p)
	playerCopy := *p
	playersMu.Unlock()
	playersDirty = true
	if global {
		playersPersistDirty = true
	}
	killNameTagCacheFor(name)
	notifyPlayerHandlers(playerCopy)
}

func showLabelMenu(name string, pos eui.Point, global bool) {
	opts := []string{"None"}
	for i := range labelColors {
		opts = append(opts, labelName(i+1))
	}
	opts = append(opts, "Edit labels…")
	time.AfterFunc(0, func() {
		eui.ShowContextMenu(opts, pos.X, pos.Y, func(i int) {
			if i == 0 {
				setPlayerLabel(name, 0, global)
			} else if i == len(opts)-1 {
				openLabelEditWindow()
			} else {
				setPlayerLabel(name, i, global)
			}
		})
	})
}

func openLabelEditWindow() {
	if labelEditWin == nil {
		labelEditWin = eui.NewWindow()
		labelEditWin.Title = "Edit Labels"
		labelEditWin.AutoSize = true
		labelEditWin.Closable = true
		labelEditWin.Movable = true
		labelEditWin.Resizable = false
		labelEditWin.NoScroll = true
	}
	labelEditWin.Contents = nil
	flow := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL, Fixed: true}
	for i := range labelColors {
		row := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_HORIZONTAL, Fixed: true}
		swatch := &eui.ItemData{ItemType: eui.ITEM_FLOW, Fixed: true, Filled: true}
		swatch.Size = eui.Point{X: 16, Y: 16}
		swatch.Color = eui.Color(labelColors[i])
		row.AddItem(swatch)
		input, _ := eui.NewInput()
		input.Size = eui.Point{X: 120, Y: 20}
		if i < len(labelNames) {
			input.Text = labelNames[i]
			input.TextPtr = &labelNames[i]
		}
		row.AddItem(input)
		flow.AddItem(row)
	}
	saveBtn, _ := eui.NewButton()
	saveBtn.Text = "Save"
	saveBtn.Action = func() {
		playersPersistDirty = true
		labelEditWin.Close()
	}
	flow.AddItem(saveBtn)
	labelEditWin.AddItem(flow)
	labelEditWin.MarkOpen()
	labelEditWin.Refresh()
}

func applyLocalLabels() {
	if playerName == "" {
		return
	}
	for i := range characters {
		if strings.EqualFold(characters[i].Name, playerName) {
			for n, lbl := range characters[i].Labels {
				p := getPlayer(n)
				p.LocalLabel = lbl
				if p.GlobalLabel == 0 {
					applyPlayerLabel(p)
				}
			}
			break
		}
	}
}

func labelColor(i int) eui.Color {
	if i <= 0 || i > len(labelColors) {
		return eui.Color{}
	}
	c := labelColors[i-1]
	return eui.Color{R: c.R, G: c.G, B: c.B, A: c.A}
}
