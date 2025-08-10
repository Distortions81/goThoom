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
			t, _ := eui.NewText()
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

func makeChatWindow() {
	if chatWin != nil {
		return
	}
	chatWin = eui.NewWindow()
	chatWin.Title = "Chat"
	if gs.ChatWindow.Size.X > 0 && gs.ChatWindow.Size.Y > 0 {
		chatWin.Size = eui.Point{X: float32(gs.ChatWindow.Size.X), Y: float32(gs.ChatWindow.Size.Y)}
	} else {
		chatWin.Size = eui.Point{X: 480, Y: 350}
	}
	chatWin.Closable = true
	chatWin.Resizable = true
	chatWin.Movable = true
	chatWin.Position = BOTTOM_RIGHT
	if gs.ChatWindow.Position.X != 0 || gs.ChatWindow.Position.Y != 0 {
		chatWin.Position = eui.Point{X: float32(gs.ChatWindow.Position.X), Y: float32(gs.ChatWindow.Position.Y)}
	}

	chatList = &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL}
	chatWin.AddItem(chatList)
	chatWin.AddWindow(false)
}
