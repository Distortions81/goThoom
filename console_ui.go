//go:build !test

package main

import "github.com/Distortions81/EUI/eui"

var messagesWin *eui.WindowData
var messagesList *eui.ItemData
var inputBar *eui.ItemData
var messagesDirty bool

func updateMessagesWindow() {
	if messagesList == nil || inputBar == nil {
		return
	}
	msgs := getConsoleMessages()
	changed := false
	for i, msg := range msgs {
		if i < len(messagesList.Contents) {
			if messagesList.Contents[i].Text != msg {
				messagesList.Contents[i].Text = msg
				changed = true
			}
		} else {
			t, _ := eui.NewText()
			t.Text = msg
			t.FontSize = 10
			t.Size = eui.Point{X: 500, Y: 24}
			messagesList.AddItem(t)
			changed = true
		}
	}
	if len(messagesList.Contents) > len(msgs) {
		for i := len(msgs); i < len(messagesList.Contents); i++ {
			messagesList.Contents[i] = nil
		}
		messagesList.Contents = messagesList.Contents[:len(msgs)]
		changed = true
	}
	inputMsg := "[Command Input Bar] (Press enter to switch to command mode)"
	if inputActive {
		inputMsg = string(inputText)
	}
	if inputBar.Text != inputMsg {
		inputBar.Text = inputMsg
		messagesWin.Refresh()
	}
	if changed {
		messagesDirty = true
	}
}

func makeConsoleWindow() {
	if messagesWin != nil {
		return
	}
	messagesWin = eui.NewWindow()
	if gs.MessagesWindow.Size.X > 0 && gs.MessagesWindow.Size.Y > 0 {
		size := eui.NormToScreen(eui.Point{X: float32(gs.MessagesWindow.Size.X), Y: float32(gs.MessagesWindow.Size.Y)})
		messagesWin.Size = eui.ScreenToNorm(size)
	} else {
		messagesWin.Size = eui.ScreenToNorm(eui.Point{X: 425, Y: 350})
	}
	messagesWin.Title = "Console"
	messagesWin.Closable = true
	messagesWin.Resizable = true
	messagesWin.Movable = true
	messagesWin.Position = BOTTOM_LEFT
	if gs.MessagesWindow.Position.X != 0 || gs.MessagesWindow.Position.Y != 0 {
		pos := eui.NormToScreen(eui.Point{X: float32(gs.MessagesWindow.Position.X), Y: float32(gs.MessagesWindow.Position.Y)})
		messagesWin.Position = eui.ScreenToNorm(pos)
	}

	messagesList = &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL}
	messagesWin.AddItem(messagesList)
	inputBar, _ = eui.NewText()
	inputBar.Text = "[Command Input Bar] (Press enter to switch to command mode)"
	inputBar.FontSize = 10
	inputBar.Size = eui.Point{X: 500, Y: 24}
	messagesWin.AddItem(inputBar)
	messagesWin.AddWindow(false)
	updateMessagesWindow()
}
