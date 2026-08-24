package main

import (
	"encoding/binary"
	"math"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	meltysynth "github.com/sinshu/go-meltysynth/meltysynth"
)

func TestMusicReverbCarriesAcrossChunks(t *testing.T) {
	whole := make([]float32, 2048)
	whole[0] = 1
	split := append([]float32(nil), whole...)

	onePass := newMusicReverb(sampleRate)
	onePass.Process(whole, append([]float32(nil), whole...))

	chunked := newMusicReverb(sampleRate)
	chunked.Process(split[:1024], append([]float32(nil), split[:1024]...))
	chunked.Process(split[1024:], append([]float32(nil), split[1024:]...))

	for i := range whole {
		if math.Abs(float64(whole[i]-split[i])) > 1e-6 {
			t.Fatalf("chunk boundary changed reverb at sample %d", i)
		}
	}
}

type bufferedStreamSynth struct{}

func (*bufferedStreamSynth) ProcessMidiMessage(int32, int32, int32, int32) {}
func (*bufferedStreamSynth) NoteOn(int32, int32, int32)                    {}
func (*bufferedStreamSynth) NoteOff(int32, int32)                          {}
func (*bufferedStreamSynth) Render(left, right []float32)                  {}

type sequentialStreamSynth struct{ next int }

func (*sequentialStreamSynth) ProcessMidiMessage(int32, int32, int32, int32) {}
func (*sequentialStreamSynth) NoteOn(int32, int32, int32)                    {}
func (*sequentialStreamSynth) NoteOff(int32, int32)                          {}
func (s *sequentialStreamSynth) Render(left, right []float32) {
	for i := range left {
		left[i] = float32(s.next)
		right[i] = float32(s.next)
		s.next++
	}
}

func TestMusicChunksPreserveSynthContinuity(t *testing.T) {
	if musicChunkFrames%block != 0 {
		t.Fatalf("music chunk size %d is not aligned to synth block %d", musicChunkFrames, block)
	}
	renderer := &songRenderer{
		syn:          &sequentialStreamSynth{},
		active:       make(map[int]bool),
		totalSamples: musicChunkFrames * 2,
	}
	first, _, err := renderer.render(musicChunkFrames)
	if err != nil {
		t.Fatalf("render first chunk: %v", err)
	}
	second, _, err := renderer.render(musicChunkFrames)
	if err != nil {
		t.Fatalf("render second chunk: %v", err)
	}
	if got, want := second[0], first[len(first)-1]+1; got != want {
		t.Fatalf("synth jumped at chunk boundary: got %v, want %v", got, want)
	}
}

func TestMusicStreamBuffersFiveOneSecondChunks(t *testing.T) {
	origSynth := newSynthesizer
	origFont := sfntCached
	origSettings := synthSettings
	origSettingsState := gs
	t.Cleanup(func() {
		newSynthesizer = origSynth
		setupSynthOnce = sync.Once{}
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

	buf := make([]byte, musicChunkFrames*4)
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

func TestMusicStreamRefillsConsumedChunk(t *testing.T) {
	origSynth := newSynthesizer
	origFont := sfntCached
	origSettings := synthSettings
	origSettingsState := gs
	t.Cleanup(func() {
		newSynthesizer = origSynth
		setupSynthOnce = sync.Once{}
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

	stream, err := newMusicStream(0, []Note{{Key: 60, Velocity: 100, Duration: 10 * time.Second}})
	if err != nil {
		t.Fatalf("newMusicStream: %v", err)
	}
	defer stream.Close()

	buf := make([]byte, musicChunkFrames*4)
	if n, err := stream.Read(buf); err != nil || n != len(buf) {
		t.Fatalf("consume buffered chunk: n=%d err=%v", n, err)
	}
	deadline := time.Now().Add(time.Second)
	for len(stream.chunks) < musicBufferSeconds && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if got := len(stream.chunks); got != musicBufferSeconds {
		t.Fatalf("buffered chunks after refill = %d, want %d", got, musicBufferSeconds)
	}
}

func TestMusicStreamProducesAudiblePCM(t *testing.T) {
	origFont, origSettings := sfntCached, synthSettings
	origDataDir, origSettingsState := dataDirPath, gs
	t.Cleanup(func() {
		setupSynthOnce = sync.Once{}
		sfntCached, synthSettings = origFont, origSettings
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

	pcm := make([]byte, musicChunkFrames*4)
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
