package main

import "sync"

const (
	maxMessages = 5
)

var (
	messageMu sync.Mutex
	messages  []string
)

func addMessage(msg string) {
	if msg == "" {
		return
	}

	messageMu.Lock()
	messages = append(messages, msg)
	if len(messages) > maxMessages {
		messages = messages[len(messages)-maxMessages:]
	}
	messageMu.Unlock()

	updateMessagesWindow()
}

func getMessages() []string {
	messageMu.Lock()
	defer messageMu.Unlock()

	out := make([]string, len(messages))
	copy(out, messages)
	return out
}
