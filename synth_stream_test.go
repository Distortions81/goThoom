package main

import (
	"encoding/binary"
	"path/filepath"
	"runtime"
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

func TestMusicStreamProducesAudiblePCM(t *testing.T) {
	origOnce, origFont, origSettings := setupSynthOnce, sfntCached, synthSettings
	origDataDir, origSettingsState := dataDirPath, gs
	t.Cleanup(func() {
		setupSynthOnce, sfntCached, synthSettings = origOnce, origFont, origSettings
		dataDirPath, gs = origDataDir, origSettingsState
	})

	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("find test source path")
	}
	dataDirPath = filepath.Join(filepath.Dir(thisFile), "data")
	setupSynthOnce = sync.Once{}
	sfntCached = nil
	synthSettings = nil
	gs.MusicEnhancement = false

	stream, err := newMusicStream(instruments[0].program, []Note{{Key: 60, Velocity: 100, Duration: time.Second}})
	if err != nil {
		t.Fatalf("newMusicStream: %v", err)
	}
	defer stream.Close()

	pcm := make([]byte, sampleRate*4)
	if n, err := stream.Read(pcm); err != nil || n != len(pcm) {
		t.Fatalf("read first PCM second: n=%d err=%v", n, err)
	}
	var peak int16
	for i := 0; i+1 < len(pcm); i += 2 {
		v := int16(binary.LittleEndian.Uint16(pcm[i:]))
		if v < 0 {
			v = -v
		}
		if v > peak {
			peak = v
		}
	}
	if peak == 0 {
		t.Fatal("rendered music PCM is silent")
	}
	t.Logf("first-second music peak: %d", peak)
}
