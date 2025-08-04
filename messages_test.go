package main

import (
	"testing"
	"time"
)

func TestExpiredMessagesRemoved(t *testing.T) {
	messages = nil
	addMessage("hi")
	if len(getMessages()) != 1 {
		t.Fatalf("message not added")
	}
	messageMu.Lock()
	messages[0].expire = time.Now().Add(-time.Second)
	messageMu.Unlock()
	if len(getMessages()) != 0 {
		t.Fatalf("expired message not removed")
	}
}
