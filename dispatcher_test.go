package main

import (
	"encoding/binary"
	"testing"
)

// helper to reset messages before each test
func resetMessages() {
	messageMu.Lock()
	messages = nil
	messageMu.Unlock()
}

func TestDispatchMessageDecode(t *testing.T) {
	resetMessages()
	// Build a packet with a simple bubble message "hi"
	data := []byte{0x00, byte(kBubbleNormal), 'h', 'i', 0}
	pkt := make([]byte, 16+len(data))
	binary.BigEndian.PutUint16(pkt[0:2], 1) // arbitrary tag
	copy(pkt[16:], data)
	dispatchMessage(pkt, true)
	msgs := getMessages()
	if len(msgs) != 1 || msgs[0] != "hi" {
		t.Fatalf("got messages %#v", msgs)
	}
}

func TestDispatchMessageInfoText(t *testing.T) {
	resetMessages()
	dispatchMessage([]byte("hello"), false)
	msgs := getMessages()
	if len(msgs) != 1 || msgs[0] != "handleInfoText: bepstrip: hello" {
		t.Fatalf("got messages %#v", msgs)
	}
}
