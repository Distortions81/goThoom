//go:build !test

package main

import "gothoom/eui"

var chatWin *eui.WindowData
var chatList *eui.ItemData
var chatPrevCount int

func updateChatWindow() {
    msgs := getChatMessages()
    updateTextWindow(chatWin, chatList, nil, msgs, gs.ChatFontSize, "")
    if chatList != nil && len(msgs) > chatPrevCount {
        // Auto-scroll list to bottom on new messages
        chatList.Scroll.Y = 1e9
        if chatWin != nil {
            chatWin.Refresh()
        }
    }
    chatPrevCount = len(msgs)
}

func makeChatWindow() error {
	if chatWin != nil {
		return nil
	}
	chatWin, chatList, _ = makeTextWindow("Chat", eui.HZoneRight, eui.VZoneBottom, false)
	// Rewrap and refresh on window resize
	chatWin.OnResize = func() { updateChatWindow() }
	updateChatWindow()
	return nil
}
