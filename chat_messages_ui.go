//go:build !test

package main

import (
	"go_client/eui"
)

var chatWin *eui.WindowData
var chatList *eui.ItemData

func updateChatWindow() {
	if chatList == nil {
		return
	}
	msgs := getChatMessages()
	changed := false
	for i, msg := range msgs {
		if i < len(chatList.Contents) {
			if chatList.Contents[i].Text != msg || chatList.Contents[i].FontSize != float32(gs.ChatFontSize) {
				chatList.Contents[i].Text = msg
				chatList.Contents[i].FontSize = float32(gs.ChatFontSize)
				changed = true
			}
		} else {
			t, _ := eui.NewText()
			if t == nil {
				logError("create chat text: eui.NewText returned nil")
				continue
			}
			t.Text = msg
			t.FontSize = float32(gs.ChatFontSize)
			t.AutoSize = true
			chatList.AddItem(t)
			changed = true
		}
	}
	if len(chatList.Contents) > len(msgs) {
		for i := len(msgs); i < len(chatList.Contents); i++ {
			chatList.Contents[i] = nil
		}
		chatList.Contents = chatList.Contents[:len(msgs)]
		changed = true
	}
	if changed && chatWin != nil {
		chatWin.Refresh()
	}
}

func makeChatWindow() error {
	if chatWin != nil {
		return nil
	}
	chatWin = eui.NewWindow()
	chatWin.Size = eui.Point{X: 410, Y: 450}
	chatWin.Title = "Chat"
	chatWin.Closable = true
	chatWin.Resizable = true
	chatWin.Movable = true
	chatWin.SetZone(eui.HZoneRight, eui.VZoneBottom)

	chatList = &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL}
	chatWin.AddItem(chatList)
	chatWin.AddWindow(false)
	return nil
}
