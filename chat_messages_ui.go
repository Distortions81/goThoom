//go:build !test

package main

import (
	"go_client/eui"
)

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
			t, _ := eui.NewText()
			if t == nil {
				logError("create chat text: eui.NewText returned nil")
				continue
			}
			t.Text = msg
			t.FontSize = 10
			t.Size = eui.Point{X: 256, Y: 24}
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
	if changed {
		chatDirty = true
	}
}

func makeChatWindow() error {
	if chatWin != nil {
		return nil
	}
	chatWin = eui.NewWindow()
	chatWin.Size = eui.Point{X: 450, Y: 450}
	chatWin.Title = "Chat"
	chatWin.Closable = true
	chatWin.Resizable = true
	chatWin.Movable = true

	chatList = &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL}
	chatWin.AddItem(chatList)
	chatWin.AddWindow(false)
	return nil
}
