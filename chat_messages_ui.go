//go:build !test

package main

import "github.com/Distortions81/EUI/eui"

var chatWin *eui.WindowData
var chatList *eui.ItemData
var chatDirty bool

func updateChatWindow() {
	if chatList == nil {
		return
	}
	msgs := getChatMessages()
	changed := false
	for i, msg := range msgs {
		if i < len(chatList.Contents) {
			if chatList.Contents[i].Text != msg {
				chatList.Contents[i].Text = msg
				changed = true
			}
		} else {
			t, err := eui.NewText(&eui.ItemData{Text: msg, FontSize: 10, Size: eui.Point{X: 256, Y: 24}})
			if err != nil {
				logError("failed to create chat text: %v", err)
				continue
			}
			chatList.AddItem(t)
			changed = true
		}
	}
	if len(chatList.Contents) > len(msgs) {
		chatList.Contents = chatList.Contents[:len(msgs)]
		changed = true
	}
	if changed {
		chatDirty = true
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
	initWindow(chatWin, gs.ChatWindow, eui.PIN_BOTTOM_RIGHT)
	if chatWin.Size.X == 0 || chatWin.Size.Y == 0 {
		chatWin.Size = eui.Point{X: 700, Y: 300}
	}

	chatList = &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL}
	chatWin.AddItem(chatList)
	chatWin.AddWindow(false)

	updateChatWindow()
}
