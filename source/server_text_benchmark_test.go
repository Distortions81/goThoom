package main

import (
	"runtime"
	"testing"
)

func BenchmarkDecodeBubbleText(b *testing.B) {
	for _, test := range []struct{ name, text string }{
		{"SingleLine", "Hello there!"},
		{"MultipleLines", "First line\r\rSecond line\rThird line\r"},
	} {
		b.Run(test.name, func(b *testing.B) {
			packet := append([]byte{1, kBubbleNormal}, test.text...)
			packet = append(packet, 0)
			buffer := make([]byte, len(packet))
			var decoded string
			b.ReportAllocs()
			for b.Loop() {
				copy(buffer, packet)
				_, decoded, _, _, _, _, _ = decodeBubble(buffer)
			}
			runtime.KeepAlive(decoded)
		})
	}
}

func BenchmarkDecodeInfoText(b *testing.B) {
	session := &Session{}
	packet := []byte("/ignored command\r/another ignored command\r")
	buffer := make([]byte, len(packet))
	b.ReportAllocs()
	for b.Loop() {
		copy(buffer, packet)
		handleSessionInfoText(session, buffer)
	}
}
