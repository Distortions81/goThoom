//go:build !test

package main

import "go_client/eui"

var consoleWin *eui.WindowData
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
			t.AutoSize = true
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

	inputMsg := "[Command Input Bar] (Press enter to switch to command mode)"
	if inputActive {
		inputMsg = string(inputText)
	}
	if len(inputFlow.Contents) == 0 {
		t, _ := eui.NewText()
		t.Text = inputMsg
		t.FontSize = float32(gs.ConsoleFontSize)
		t.AutoSize = true
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

	messagesFlow = &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL, Scrollable: true}
	consoleWin.AddItem(messagesFlow)
	inputFlow = &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL, Fixed: true, PinTo: eui.PIN_BOTTOM_LEFT}
	consoleWin.AddItem(inputFlow)
	consoleWin.AddWindow(false)
	updateConsoleWindow()
}
