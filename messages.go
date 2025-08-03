package main

import (
	"fmt"
	"sync"
)

const maxMessages = 5

var (
	messageMu sync.Mutex
	messages  []string
)

func addMessage(msg string) {
	if msg == "" {
		return
	}

	messageMu.Lock()
	defer messageMu.Unlock()
	messages = append(messages, msg)
	if len(messages) > maxMessages {
		messages = messages[len(messages)-maxMessages:]
	}

	fmt.Println(msg)
}

func getMessages() []string {
	messageMu.Lock()
	defer messageMu.Unlock()
	return append([]string(nil), messages...)
}
