package main

import (
	"sync"
	"testing"
	"time"

	meltysynth "github.com/sinshu/go-meltysynth/meltysynth"
)

type bufferedStreamSynth struct{}

func (*bufferedStreamSynth) ProcessMidiMessage(int32, int32, int32, int32) {}
func (*bufferedStreamSynth) NoteOn(int32, int32, int32)                    {}
func (*bufferedStreamSynth) NoteOff(int32, int32)                          {}
func (*bufferedStreamSynth) Render(left, right []float32)                  {}

func TestMusicStreamBuffersFiveOneSecondChunks(t *testing.T) {
	origSynth := newSynthesizer
	origOnce := setupSynthOnce
	origFont := sfntCached
	origSettings := synthSettings
	origSettingsState := gs
	t.Cleanup(func() {
		newSynthesizer = origSynth
		setupSynthOnce = origOnce
		sfntCached = origFont
		synthSettings = origSettings
		gs = origSettingsState
	})

	newSynthesizer = func(*meltysynth.SoundFont, *meltysynth.SynthesizerSettings) (synthesizer, error) {
		return &bufferedStreamSynth{}, nil
	}
	setupSynthOnce = sync.Once{}
	sfntCached = &meltysynth.SoundFont{}
	synthSettings = meltysynth.NewSynthesizerSettings(sampleRate)
	gs.MusicEnhancement = false

	stream, err := newMusicStream(0, []Note{{Key: 60, Velocity: 100, Duration: 6 * time.Second}})
	if err != nil {
		t.Fatalf("newMusicStream: %v", err)
	}
	defer stream.Close()

	buf := make([]byte, sampleRate*4)
	for i := 0; i < musicBufferSeconds; i++ {
		n, err := stream.Read(buf)
		if err != nil {
			t.Fatalf("chunk %d: %v", i, err)
		}
		if n != len(buf) {
			t.Fatalf("chunk %d length = %d, want %d", i, n, len(buf))
		}
	}
}
