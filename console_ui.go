//go:build !test

package main

import "github.com/Distortions81/EUI/eui"

var messagesWin *eui.WindowData
var messagesList *eui.ItemData
var messagesDirty bool

func updateMessagesWindow() {
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
	inputIdx := len(msgs)
	if inputIdx < len(messagesList.Contents) {
		if messagesList.Contents[inputIdx].Text != inputMsg {
			messagesList.Contents[inputIdx].Text = inputMsg
			changed = true
		}
	} else {
		t, _ := eui.NewText()
		t.Text = inputMsg
		t.FontSize = 10
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
	if messagesWin != nil {
		return
	}
	messagesWin = eui.NewWindow()
	sx, sy := eui.ScreenSize()
	if gs.MessagesWindow.Size.X > 0 && gs.MessagesWindow.Size.Y > 0 {
		messagesWin.Size = eui.Point{X: float32(gs.MessagesWindow.Size.X) * float32(sx), Y: float32(gs.MessagesWindow.Size.Y) * float32(sy)}
	} else {
		messagesWin.Size = eui.Point{X: 425, Y: 350}
	}
	messagesWin.Title = "Console"
	messagesWin.Closable = true
	messagesWin.Resizable = true
	messagesWin.Movable = true
	messagesWin.Position = BOTTOM_LEFT
	if gs.MessagesWindow.Position.X != 0 || gs.MessagesWindow.Position.Y != 0 {
		messagesWin.Position = eui.Point{X: float32(gs.MessagesWindow.Position.X) * float32(sx), Y: float32(gs.MessagesWindow.Position.Y) * float32(sy)}
	}

	messagesList = &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL}
	messagesWin.AddItem(messagesList)
	messagesWin.AddWindow(false)
	updateMessagesWindow()
}
