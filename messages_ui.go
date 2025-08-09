//go:build !test

package main

import "github.com/Distortions81/EUI/eui"

var messagesWin *eui.WindowData
var messagesList *eui.ItemData

func updateMessagesWindow() {
	if messagesList == nil {
		return
	}
	msgs := getMessages()
	messagesList.Contents = messagesList.Contents[:0]
	for _, msg := range msgs {
		t, _ := eui.NewText(&eui.ItemData{Text: msg, FontSize: 10, Size: eui.Point{X: 500, Y: 24}})
		messagesList.AddItem(t)
	}
	inputMsg := "[Command Input Bar] (Press enter to switch to command mode)"
	if inputActive {
		inputMsg = string(inputText)
	}
	t, _ := eui.NewText(&eui.ItemData{Text: inputMsg, FontSize: 10, Size: eui.Point{X: 500, Y: 24}})
	messagesList.AddItem(t)
	if messagesWin != nil {
		messagesWin.Refresh()
	}
}

func openMessagesWindow() {
	if messagesWin != nil {
		if messagesWin.Open {
			return
		}
	}
	messagesWin = eui.NewWindow(&eui.WindowData{})
	if gs.MessagesWindow.Size.X > 0 && gs.MessagesWindow.Size.Y > 0 {
		messagesWin.Size = eui.Point{X: float32(gs.MessagesWindow.Size.X), Y: float32(gs.MessagesWindow.Size.Y)}
	} else {
		messagesWin.Size = eui.Point{X: 700, Y: 300}
	}
	messagesWin.Title = "Console"
	messagesWin.Closable = true
	messagesWin.Resizable = true
	messagesWin.AutoSize = false
	messagesWin.Movable = false
	messagesWin.Open = true
	messagesWin.PinTo = eui.PIN_BOTTOM_LEFT
	if gs.MessagesWindow.Position.X != 0 || gs.MessagesWindow.Position.Y != 0 {
		messagesWin.Position = eui.Point{X: float32(gs.MessagesWindow.Position.X), Y: float32(gs.MessagesWindow.Position.Y)}
	}

	messagesList = &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL}
	messagesWin.AddItem(messagesList)
	messagesWin.AddWindow(false)

	updateMessagesWindow()
}
