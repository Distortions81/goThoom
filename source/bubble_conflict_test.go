package main

import (
	"image"
	"testing"
)

func TestBubbleConflictShortcutMatchesFlags(t *testing.T) {
	for count := 0; count < 30; count++ {
		for spacing := 0; spacing < 25; spacing++ {
			items := make([]jointBubbleLayoutItem, count)
			for i := range items {
				items[i] = jointBubbleLayoutItem{rect: image.Rect(i*spacing, 0, i*spacing+20, 20), margin: i % 3}
			}
			want := false
			for _, flag := range bubbleLayoutConflictFlags(items) {
				want = want || flag
			}
			if got := bubbleLayoutHasConflict(items); got != want {
				t.Fatalf("count=%d spacing=%d: got %v want %v", count, spacing, got, want)
			}
		}
	}
}

func BenchmarkBubbleConflictCheck(b *testing.B) {
	items := make([]jointBubbleLayoutItem, 32)
	for i := range items {
		items[i].rect = image.Rect(i, 0, i+20, 20)
	}
	b.Run("Flags", func(b *testing.B) {
		for b.Loop() {
			bubbleLayoutConflictFlags(items)
		}
	})
	b.Run("Any", func(b *testing.B) {
		for b.Loop() {
			bubbleLayoutHasConflict(items)
		}
	})
}
