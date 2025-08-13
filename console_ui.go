//go:build !test

package main

import "go_client/eui"

var consoleWin *eui.WindowData
var messagesList *eui.ItemData
var messagesDirty bool

func updateConsoleWindow() {
	if messagesList == nil {
		return
	}
	msgs := getConsoleMessages()
	inputMsg := "[Command Input Bar] (Press enter to switch to command mode)"
	if inputActive {
		inputMsg = string(inputText)
	}
	changed := false
	for i, msg := range msgs {
		if i < len(messagesList.Contents) {
			if messagesList.Contents[i].Text != msg || messagesList.Contents[i].FontSize != float32(gs.ConsoleFontSize) {
				messagesList.Contents[i].Text = msg
				messagesList.Contents[i].FontSize = float32(gs.ConsoleFontSize)
				changed = true
			}
		} else {
			t, _ := eui.NewText()
			t.Text = msg
			t.FontSize = float32(gs.ConsoleFontSize)
			t.Size = eui.Point{X: 500, Y: 24}
			messagesList.AddItem(t)
			changed = true
		}
	}
	inputIdx := len(msgs)
	if inputIdx < len(messagesList.Contents) {
		if messagesList.Contents[inputIdx].Text != inputMsg || messagesList.Contents[inputIdx].FontSize != float32(gs.ConsoleFontSize) {
			messagesList.Contents[inputIdx].Text = inputMsg
			messagesList.Contents[inputIdx].FontSize = float32(gs.ConsoleFontSize)
			changed = true
		}
	} else {
		t, _ := eui.NewText()
		t.Text = inputMsg
		t.FontSize = float32(gs.ConsoleFontSize)
		t.Size = eui.Point{X: 500, Y: 24}
		messagesList.AddItem(t)
		changed = true
	}
	if len(messagesList.Contents) > inputIdx+1 {
		for i := inputIdx + 1; i < len(messagesList.Contents); i++ {
			messagesList.Contents[i] = nil
		}
		messagesList.Contents = messagesList.Contents[:inputIdx+1]
		changed = true
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

	messagesList = &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL}
	consoleWin.AddItem(messagesList)
	consoleWin.AddWindow(false)
	updateConsoleWindow()
}
