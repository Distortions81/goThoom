//go:build !test

package main

import "gothoom/eui"

var chatWin *eui.WindowData
var chatList *eui.ItemData

func updateChatWindow() {
	updateTextWindow(chatWin, chatList, nil, getChatMessages(), gs.ChatFontSize, "")
}

func makeChatWindow() error {
	if chatWin != nil {
		return nil
	}
	chatWin, chatList, _ = makeTextWindow("Chat", eui.HZoneRight, eui.VZoneBottom, false)
	updateChatWindow()
	return nil
}
