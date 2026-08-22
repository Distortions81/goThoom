package main

import (
	"bytes"
	"testing"
)

func TestMacRomanEscapedRoundTrip(t *testing.T) {
	want := "Méme ☺ 🚀 unknown \\q and slash \\"
	encoded, err := EncodeMacRomanEscaped(want)
	if err != nil {
		t.Fatal(err)
	}
	got, err := DecodeMacRomanEscaped(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("round-trip = %q, want %q", got, want)
	}
}

func TestEncodeMacRomanEscapedUsesMacRomanAndUnicodeEscapes(t *testing.T) {
	got, err := EncodeMacRomanEscaped("café e\u0301 ☺ 🚀 literal \\u263A")
	if err != nil {
		t.Fatal(err)
	}
	want := append([]byte{'c', 'a', 'f', 0x8e, ' '}, []byte(`e\u0301 \u263A \U0001F680 literal \u263A`)...)
	if !bytes.Equal(got, want) {
		t.Fatalf("encoded bytes = % x, want % x", got, want)
	}
}

func TestDecodeMacRomanEscapedLeavesMalformedAndUnknownEscapesLiteral(t *testing.T) {
	want := `\u12G4 \q \uD800 \U00110000 trailing\`
	got, err := DecodeMacRomanEscaped([]byte(want))
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("decoded malformed text = %q, want %q", got, want)
	}
}

func TestDecodeMacRomanChatText(t *testing.T) {
	bubble := []byte{1, kBubbleNormal, 'c', 'a', 'f', 0x8e, ' '}
	bubble = append(bubble, []byte(`\u263A \U0001F680 literal \\u263A`)...)
	bubble = append(bubble, 0)
	_, got, _, _, _, _, _ := decodeBubble(bubble)
	want := `café ☺ 🚀 literal \u263A`
	if got != want {
		t.Fatalf("decoded chat = %q, want %q", got, want)
	}
}
