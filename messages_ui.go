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
		t, _ := eui.NewText(&eui.ItemData{Text: msg, FontSize: 10, Size: eui.Point{X: 256, Y: 24}})
		messagesList.AddItem(t)
	}
	inputMsg := "[Command Input Bar] (Press enter to switch to command mode)"
	if inputActive {
		inputMsg = string(inputText)
	}
	t, _ := eui.NewText(&eui.ItemData{Text: inputMsg, FontSize: 10, Size: eui.Point{X: 256, Y: 24}})
	messagesList.AddItem(t)
	if messagesWin != nil {
		messagesWin.Refresh()
	}
}

func openMessagesWindow() {
	if messagesWin != nil {
		return
	}
	messagesWin = eui.NewWindow(&eui.WindowData{})
	messagesWin.Title = "Messages"
	messagesWin.Closable = true
	messagesWin.Resizable = true
	messagesWin.AutoSize = false
	messagesWin.Movable = true
	messagesWin.Open = true

	messagesList = &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL}
	messagesWin.AddItem(messagesList)
	messagesWin.AddWindow(false)
	updateMessagesWindow()
}
