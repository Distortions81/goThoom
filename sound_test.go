package main

import (
	"reflect"
	"testing"
	"unsafe"

	"github.com/hajimehoshi/ebiten/v2/audio"

	"go_client/clsnd"
)

func setTestSounds(m map[uint16]*clsnd.Sound) {
	clSounds = &clsnd.CLSounds{}
	cache := make(map[uint32]*clsnd.Sound, len(m))
	for id, snd := range m {
		cache[uint32(id)] = snd
	}
	v := reflect.ValueOf(clSounds).Elem().FieldByName("cache")
	reflect.NewAt(v.Type(), unsafe.Pointer(v.UnsafeAddr())).Elem().Set(reflect.ValueOf(cache))
}

// TestPlaySoundResample ensures sounds with a sample rate different from the
// audio context are resampled and produce audio bytes.
func TestPlaySoundResample(t *testing.T) {
	initSoundContext()
	// Reset caches and players.
	soundMu.Lock()
	pcmCache = make(map[uint16][]byte)
	soundMu.Unlock()
	soundPlayers = make(map[*audio.Player]struct{})

	const id = uint16(1)
	snd := &clsnd.Sound{
		Data:       []byte{0x00, 0x01, 0x00, 0x02}, // two 16-bit samples
		SampleRate: 11025,
		Channels:   1,
		Bits:       16,
	}

	setTestSounds(map[uint16]*clsnd.Sound{id: snd})

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

// TestFastSoundContext verifies that enabling fastSound switches to a lower
// sample rate and linear resampling.
func TestFastSoundContext(t *testing.T) {
	fastSound = true
	initSoundContext()
	if audioContext.SampleRate() != 22050 {
		t.Fatalf("expected audio context sample rate 22050, got %d", audioContext.SampleRate())
	}
	src := []int16{0, 1000}
	if got, want := resample(src, 44100, 22050), resampleLinear(src, 44100, 22050); !reflect.DeepEqual(got, want) {
		t.Fatalf("expected linear resampler when fastSound is enabled")
	}

	fastSound = false
	initSoundContext()
}

// TestMaxSoundsLimit verifies that playSound respects the maxSounds limit and
// does not create more than the specified number of concurrent players.
func TestMaxSoundsLimit(t *testing.T) {
	maxSounds = 2
	initSoundContext()

	// Reset caches and players.
	soundMu.Lock()
	pcmCache = make(map[uint16][]byte)
	soundMu.Unlock()
	soundPlayers = make(map[*audio.Player]struct{})

	// Create a 1-second sound so players remain active during the test.
	data := make([]byte, audioContext.SampleRate()*2)
	snd := &clsnd.Sound{
		Data:       data,
		SampleRate: uint32(audioContext.SampleRate() * 2),
		Channels:   1,
		Bits:       16,
	}

	setTestSounds(map[uint16]*clsnd.Sound{1: snd, 2: snd, 3: snd})

	playSound(1)
	playSound(2)
	playSound(3)

	if len(soundPlayers) != 2 {
		t.Fatalf("expected 2 sound players, got %d", len(soundPlayers))
	}
	for p := range soundPlayers {
		p.Close()
	}
	maxSounds = 32
}
