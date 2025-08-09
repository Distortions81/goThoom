package main

import "sync"

const (
	maxChatMessages = 5
)

var (
	chatMsgMu sync.Mutex
	chatMsgs  []string
)

func addChatMessage(msg string) {
	if msg == "" {
		return
	}
	chatMsgMu.Lock()
	chatMsgs = append(chatMsgs, msg)
	if len(chatMsgs) > maxChatMessages {
		chatMsgs = chatMsgs[len(chatMsgs)-maxChatMessages:]
	}
	chatMsgMu.Unlock()

	updateChatWindow()
}

func getChatMessages() []string {
	chatMsgMu.Lock()
	defer chatMsgMu.Unlock()

	out := make([]string, len(chatMsgs))
	copy(out, chatMsgs)
	return out
}
