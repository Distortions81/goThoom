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
		return
	}
	chatWin = eui.NewWindow(&eui.WindowData{})
	chatWin.Title = "Chat"
	chatWin.Closable = false
	chatWin.Resizable = false
	chatWin.AutoSize = true
	chatWin.Movable = true
	chatWin.Open = true

	chatList = &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL}
	chatWin.AddItem(chatList)
	chatWin.AddWindow(false)

	w := float32(256)
	h := float32(150)
	chatWin.Position = eui.Point{X: float32(float64(gameAreaSizeX)*gs.Scale) - w, Y: float32(float64(gameAreaSizeY)*gs.Scale) - h}
	updateChatWindow()
}
