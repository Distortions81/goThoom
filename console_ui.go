//go:build !test

package main

import "go_client/eui"

var consoleWin *eui.WindowData
var consoleFlow *eui.ItemData
var messagesFlow *eui.ItemData
var inputFlow *eui.ItemData
var messagesDirty bool

func updateConsoleWindow() {
	if messagesFlow == nil || inputFlow == nil {
		return
	}
	msgs := getConsoleMessages()
	changed := false
	for i, msg := range msgs {
		if i < len(messagesFlow.Contents) {
			if messagesFlow.Contents[i].Text != msg || messagesFlow.Contents[i].FontSize != float32(gs.ConsoleFontSize) {
				messagesFlow.Contents[i].Text = msg
				messagesFlow.Contents[i].FontSize = float32(gs.ConsoleFontSize)
				changed = true
			}
		} else {
			t, _ := eui.NewText()
			t.Text = msg
			t.FontSize = float32(gs.ConsoleFontSize)
			messagesFlow.AddItem(t)
			changed = true
		}
	}
	if len(messagesFlow.Contents) > len(msgs) {
		for i := len(msgs); i < len(messagesFlow.Contents); i++ {
			messagesFlow.Contents[i] = nil
		}
		messagesFlow.Contents = messagesFlow.Contents[:len(msgs)]
		changed = true
	}

	inputFlow.Size.Y = float32(gs.ConsoleFontSize) + 8
	if consoleWin != nil {
		messagesFlow.Size.Y = consoleWin.GetSize().Y - inputFlow.Size.Y
	}
	inputMsg := "[Command Input Bar] (Press enter to switch to command mode)"
	if inputActive {
		inputMsg = string(inputText)
	}
	if len(inputFlow.Contents) == 0 {
		t, _ := eui.NewText()
		t.Text = inputMsg
		t.FontSize = float32(gs.ConsoleFontSize)
		inputFlow.AddItem(t)
		changed = true
	} else {
		if inputFlow.Contents[0].Text != inputMsg || inputFlow.Contents[0].FontSize != float32(gs.ConsoleFontSize) {
			inputFlow.Contents[0].Text = inputMsg
			inputFlow.Contents[0].FontSize = float32(gs.ConsoleFontSize)
			changed = true
		}
	}
	if changed {
		messagesDirty = true
		if consoleWin != nil {
			consoleWin.Refresh()
		}
	}
}

func makeConsoleWindow() {
	if consoleWin != nil {
		return
	}
	consoleWin = eui.NewWindow()
	consoleWin.Title = "Console"
	consoleWin.Size = eui.Point{X: 410, Y: 450}
	consoleWin.Closable = true
	consoleWin.Resizable = true
	consoleWin.Movable = true
	consoleWin.SetZone(eui.HZoneLeft, eui.VZoneBottom)

	consoleFlow = &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL, Fixed: true}
	consoleFlow.Size = consoleWin.GetSize()
	consoleWin.AddItem(consoleFlow)

	messagesFlow = &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL, Scrollable: true, Fixed: true}
	consoleFlow.AddItem(messagesFlow)

	inputFlow = &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL, Fixed: true}
	inputFlow.Color = eui.ColorVeryDarkGray
	consoleFlow.AddItem(inputFlow)

	consoleWin.AddWindow(false)
	updateConsoleWindow()
}
