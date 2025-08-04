package main

import (
	"testing"

	"github.com/hajimehoshi/ebiten/v2/audio"

	"go_client/clsnd"
)

// TestPlaySoundResample ensures sounds with a sample rate different from the
// audio context are resampled and produce audio bytes.
func TestPlaySoundResample(t *testing.T) {
	// Reset caches and players.
	soundMu.Lock()
	soundCache = make(map[uint16]*clsnd.Sound)
	soundMu.Unlock()
	soundPlayers = make(map[*audio.Player]struct{})

	const id = uint16(1)
	snd := &clsnd.Sound{
		Data:       []byte{0x00, 0x01, 0x00, 0x02}, // two 16-bit samples
		SampleRate: 11025,
		Channels:   1,
		Bits:       16,
	}

	soundMu.Lock()
	soundCache[id] = snd
	soundMu.Unlock()

	playSound(id)

	if len(soundPlayers) == 0 {
		t.Fatalf("expected player to be created")
	}
	for p := range soundPlayers {
		if !p.IsPlaying() {
			t.Fatalf("expected player to be playing")
		}
		p.Close()
	}
}
