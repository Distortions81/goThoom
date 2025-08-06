package main

import (
	"sync"
	"time"
)

const (
	maxMessages     = 5
	messageLifetime = 15 * time.Second
)

type message struct {
	text   string
	expire time.Time
}

var (
	messageMu sync.Mutex
	messages  []message

	clmovCaching    bool
	clmovCacheFrame int
	clmovCacheMsgs  [][]string
)

func addMessage(msg string) {
	if msg == "" {
		return
	}

	if clmovCaching && clmovCacheFrame < len(clmovCacheMsgs) {
		clmovCacheMsgs[clmovCacheFrame] = append(clmovCacheMsgs[clmovCacheFrame], msg)
		return
	}

	messageMu.Lock()
	defer messageMu.Unlock()
	messages = append(messages, message{text: msg, expire: time.Now().Add(messageLifetime)})
	if len(messages) > maxMessages {
		messages = messages[len(messages)-maxMessages:]
	}
}

func getMessages() []string {
	messageMu.Lock()
	defer messageMu.Unlock()

	now := time.Now()
	var out []string
	var keep []message
	for _, m := range messages {
		if now.After(m.expire) {
			continue
		}
		out = append(out, m.text)
		keep = append(keep, m)
	}
	messages = keep
	return out
}
