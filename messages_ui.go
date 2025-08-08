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
	messagesWin.Closable = false
	messagesWin.Resizable = false
	messagesWin.AutoSize = true
	messagesWin.Movable = true
	messagesWin.PinTo = eui.PIN_BOTTOM_LEFT
	messagesWin.Open = true

	messagesList = &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL}
	messagesWin.AddItem(messagesList)
	messagesWin.AddWindow(false)
	updateMessagesWindow()
}
