package main

import (
	"strings"
	"testing"
)

func TestExpandEmojiShortcodes(t *testing.T) {
	tests := []struct{ input, want string }{
		{"", ""},
		{"smile and rocket", "smile and rocket"},
		{"Hello :smile:!", "Hello 😄!"},
		{":thumbs_up: :thumbs up: :thumbsup: :+1:", "👍 👍 👍 👍"},
		{":ROCKET:", "🚀"},
		{":heart::woman_technologist:", "❤️👩‍💻"},
		{"café :smile: 🚀", "café 😄 🚀"},
		{":not_an_emoji:", ":not_an_emoji:"},
		{":unknown::smile: :rocket::unknown:", ":unknown:😄 🚀:unknown:"},
		{":smile", ":smile"},
		{"smile:", "smile:"},
		{":: :::", ":: :::"},
		{"https://example.com at 12:30", "https://example.com at 12:30"},
		{"/think ready :rocket:", "/think ready 🚀"},
	}
	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			if got := expandEmojiShortcodes(test.input); got != test.want {
				t.Fatalf("expandEmojiShortcodes(%q) = %q, want %q", test.input, got, test.want)
			}
		})
	}
}

func TestEmojiNamesAcceptUnderscores(t *testing.T) {
	for name, emoji := range emojiByName {
		input := ":" + strings.ReplaceAll(name, " ", "_") + ":"
		if got := expandEmojiShortcodes(input); got != emoji {
			t.Errorf("expandEmojiShortcodes(%q) = %q, want %q", input, got, emoji)
		}
	}
}

func TestEncodeEmojiShortcodes(t *testing.T) {
	for _, test := range []struct{ input, want string }{
		{"Hello 😄 🚀", "Hello :smile: :rocket:"},
		{"Hello :smile: :thumbs_up: :unknown:", "Hello :smile: :thumbs_up: :unknown:"},
		{"café 123 https://example.com", "café 123 https://example.com"},
		{"❤️", ":heart:"},
	} {
		if got := encodeEmojiShortcodes(test.input); got != test.want {
			t.Errorf("encodeEmojiShortcodes(%q) = %q, want %q", test.input, got, test.want)
		}
	}
	for _, emoji := range emojiByName {
		wire := encodeEmojiShortcodes(emoji)
		if got := expandEmojiShortcodes(wire); got != emoji {
			t.Errorf("emoji %q encoded as %q decoded to %q", emoji, wire, got)
		}
		if strings.Count(wire, ":") != 2 {
			t.Errorf("emoji sequence %q split into multiple shortcodes: %q", emoji, wire)
		}
	}
}

func TestDecodeEmojiChatAndBubbles(t *testing.T) {
	for _, wire := range []string{":smile: :rocket:", `\U0001F604 \U0001F680`} {
		for _, sender := range []string{"Self", "Another exile"} {
			for _, kind := range []int{kBubbleNormal, kBubbleThought} {
				body := wire
				if kind == kBubbleThought {
					body = sender + ": " + body
				}
				bubble := append([]byte{1, byte(kind)}, body...)
				bubble = append(bubble, 0)
				_, got, name, _, _, _, _ := decodeBubble(append([]byte(nil), bubble...))
				if formatTimedMessage(timedMessage{Text: got}, "", false) != "😄 🚀" {
					t.Errorf("bubble %q: got %q", body, got)
				}
				if kind == kBubbleThought && name != sender {
					t.Errorf("thought sender = %q, want %q", name, sender)
				}
				message := append(make([]byte, 16), bubble...)
				if got := formatTimedMessage(timedMessage{Text: decodeMessage(message)}, "", false); got != "😄 🚀" {
					t.Errorf("chat %q: got %q", body, got)
				}
			}
		}
	}
}
