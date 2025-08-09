package main

import (
	"sync"
	"time"
)

const (
	maxChatMessages     = 5
	chatMessageLifetime = 15 * time.Second
)

type chatMessage struct {
	text   string
	expire time.Time
}

var (
	chatMsgMu sync.Mutex
	chatMsgs  []chatMessage
)

func addChatMessage(msg string) {
	if msg == "" {
		return
	}
	chatMsgMu.Lock()
	defer chatMsgMu.Unlock()
	chatMsgs = append(chatMsgs, chatMessage{text: msg, expire: time.Now().Add(chatMessageLifetime)})
	if len(chatMsgs) > maxChatMessages {
		chatMsgs = chatMsgs[len(chatMsgs)-maxChatMessages:]
	}
	chatDirty.Store(true)
}

func getChatMessages() []string {
	chatMsgMu.Lock()
	defer chatMsgMu.Unlock()
	now := time.Now()
	var out []string
	var keep []chatMessage
	for _, m := range chatMsgs {
		if now.After(m.expire) {
			continue
		}
		out = append(out, m.text)
		keep = append(keep, m)
	}
	chatMsgs = keep
	return out
}
