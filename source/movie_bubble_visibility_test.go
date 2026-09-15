package main

import (
	"encoding/binary"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
)

// This stock recording contains an off-screen yell that previously reached
// layout but vanished at the final overlap gate for one update.
func TestLoreMovieSpeechBubblesReachDrawBatch(t *testing.T) {
	initFont()
	setupBubbleLayoutTest(t)
	oldMovieMode, oldImages, oldBlockMusic := movieMode, clImages, blockMusic
	clImages = testCLImages(nil)
	movieMode = true
	blockMusic = true
	t.Cleanup(func() {
		movieMode, clImages = oldMovieMode, oldImages
		blockMusic = oldBlockMusic
		resetDrawState()
	})
	gs.BubbleWhisper, gs.BubbleYell, gs.BubbleThought = true, true, true
	gs.BubbleRealAction, gs.BubbleMonster, gs.BubblePlayerAction = true, true, true
	gs.BubblePonder, gs.BubbleNarrate, gs.BubbleMonsters, gs.BubbleNarration = true, true, true, true
	frames, err := parseMovie(movieFixturePath(t, "lore1.clMov"), baseVersion)
	if err != nil {
		t.Fatal(err)
	}
	screen := ebiten.NewImage(800, 600)
	defer screen.Dispose()
	var snap drawSnapshot
	for _, frame := range frames {
		if frame.index > 11838 {
			break
		}
		if len(frame.data) < 2 || binary.BigEndian.Uint16(frame.data[:2]) != 2 {
			continue
		}
		if !handleDrawState(frame.data, false) {
			continue
		}
		if frame.index != 11838 {
			continue
		}
		captureDrawSnapshot(&snap)
		drawSpeechBubbles(screen, snap, 1, speechBubbleWindowScale(gs.GameScale))
		if got, want := len(bubbleFrameScratch.prepared), 5; got != want {
			t.Fatalf("movie frame %d prepared %d speech bubbles, want %d", frame.index, got, want)
		}
		if got, want := len(bubbleFrameScratch.drawRequests), len(bubbleFrameScratch.prepared); got != want {
			t.Fatalf("movie frame %d drew %d of %d prepared speech bubbles", frame.index, got, want)
		}
		return
	}
	t.Fatal("movie frame 11838 was not replayed")
}
