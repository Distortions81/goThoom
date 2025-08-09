//go:build !test

package main

import "github.com/Distortions81/EUI/eui"

var chatWin *eui.WindowData
var chatList *eui.ItemData

func updateChatWindow() {
	if chatList == nil {
		return
	}
	msgs := getChatMessages()
	chatList.Contents = chatList.Contents[:0]
	for _, msg := range msgs {
		t, _ := eui.NewText(&eui.ItemData{Text: msg, FontSize: 10, Size: eui.Point{X: 256, Y: 24}})
		chatList.AddItem(t)
	}
	if chatWin != nil {
		chatWin.Refresh()
	}
}

func openChatWindow() {
	if chatWin != nil {
		if chatWin.Open {
			return
		}
	}
	chatWin = eui.NewWindow(&eui.WindowData{})
	chatWin.Title = "Chat"
	if gs.ChatWindow.Size.X > 0 && gs.ChatWindow.Size.Y > 0 {
		chatWin.Size = eui.Point{X: float32(gs.ChatWindow.Size.X), Y: float32(gs.ChatWindow.Size.Y)}
	} else {
		chatWin.Size = eui.Point{X: 700, Y: 300}
	}
	chatWin.Closable = true
	chatWin.Resizable = true
	chatWin.AutoSize = false
	chatWin.Movable = false
	chatWin.Open = true
	chatWin.PinTo = eui.PIN_BOTTOM_RIGHT
	if gs.ChatWindow.Position.X != 0 || gs.ChatWindow.Position.Y != 0 {
		chatWin.Position = eui.Point{X: float32(gs.ChatWindow.Position.X), Y: float32(gs.ChatWindow.Position.Y)}
	}

	chatList = &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL}
	chatWin.AddItem(chatList)
	chatWin.AddWindow(false)

	updateChatWindow()
}
