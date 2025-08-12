package main

import "testing"

func TestDecodeMessage_NonChatPacket(t *testing.T) {
	m := append(make([]byte, 16), []byte{0x7f, 0, 1, 2, 3}...)
	if s := decodeMessage(m); s != "" {
		t.Fatalf("expected empty string, got %q", s)
	}
}

func TestDecodeMessage_ShortBubble(t *testing.T) {
	m := append(make([]byte, 16), []byte{kMsgBubble, 0, 'h', 'i', 0}...)
	if s := decodeMessage(m); s != "hi" {
		t.Fatalf("expected 'hi', got %q", s)
	}
}
