package main

import (
	"encoding/binary"
	"errors"
	"io"
	"math"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
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
	onePass.Process(whole, append([]float32(nil), whole...), 1)

	chunked := newMusicReverb(sampleRate)
	chunked.Process(split[:1024], append([]float32(nil), split[:1024]...), 1)
	chunked.Process(split[1024:], append([]float32(nil), split[1024:]...), 1)

	for i := range whole {
		if math.Abs(float64(whole[i]-split[i])) > 1e-6 {
			t.Fatalf("chunk boundary changed reverb at sample %d", i)
		}
	}
}

func TestMusicReverbAmountControlsDryWetMix(t *testing.T) {
	lowAmount := make([]float32, 256)
	highAmount := make([]float32, 256)
	lowAmount[0], highAmount[0] = 1, 1

	newMusicReverb(sampleRate).Process(lowAmount, append([]float32(nil), lowAmount...), 0.1)
	newMusicReverb(sampleRate).Process(highAmount, append([]float32(nil), highAmount...), 2)

	if lowAmount[0] <= highAmount[0] {
		t.Fatalf("reverb first sample low=%v high=%v, want lower dry level at stronger ambience", lowAmount[0], highAmount[0])
	}
}

type bufferedStreamSynth struct{}

func (*bufferedStreamSynth) ProcessMidiMessage(int32, int32, int32, int32) {}
func (*bufferedStreamSynth) NoteOn(int32, int32, int32)                    {}
func (*bufferedStreamSynth) NoteOff(int32, int32)                          {}
func (*bufferedStreamSynth) Render(left, right []float32)                  {}

type panickingStreamSynth struct{}

func (*panickingStreamSynth) ProcessMidiMessage(int32, int32, int32, int32) {}
func (*panickingStreamSynth) NoteOn(int32, int32, int32)                    {}
func (*panickingStreamSynth) NoteOff(int32, int32)                          {}
func (*panickingStreamSynth) Render([]float32, []float32)                   { panic("render failed") }

type timingStreamSynth struct {
	rendered int
	noteOns  []int
	noteOffs []int
}

func (*timingStreamSynth) ProcessMidiMessage(int32, int32, int32, int32) {}
func (s *timingStreamSynth) NoteOn(int32, int32, int32) {
	s.noteOns = append(s.noteOns, s.rendered)
}
func (s *timingStreamSynth) NoteOff(int32, int32) {
	s.noteOffs = append(s.noteOffs, s.rendered)
}
func (s *timingStreamSynth) Render(left, _ []float32) { s.rendered += len(left) }

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

func TestSafeRenderConvertsSynthPanicToError(t *testing.T) {
	err := safeRender(&panickingStreamSynth{}, make([]float32, block), make([]float32, block))
	if err == nil || !strings.Contains(err.Error(), "render failed") {
		t.Fatalf("safeRender error = %v, want recovered panic", err)
	}
}

func TestMusicStreamReadFillsAcrossBufferedChunks(t *testing.T) {
	stream := &musicStream{
		chunks: make(chan []byte, 2),
		done:   make(chan struct{}),
	}
	stream.chunks <- []byte{1, 2}
	stream.chunks <- []byte{3, 4}
	close(stream.chunks)

	buf := make([]byte, 4)
	n, err := stream.Read(buf)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if n != len(buf) || !slices.Equal(buf, []byte{1, 2, 3, 4}) {
		t.Fatalf("Read = %d, %v; want 4, [1 2 3 4]", n, buf)
	}
}

func TestMusicStreamRenderErrorBecomesCleanEOF(t *testing.T) {
	stream := &musicStream{
		chunks:    make(chan []byte),
		done:      make(chan struct{}),
		exhausted: make(chan struct{}),
	}
	stream.setRenderError(errors.New("render failed"))
	close(stream.chunks)

	if n, err := stream.Read(make([]byte, 4)); n != 0 || !errors.Is(err, io.EOF) {
		t.Fatalf("Read = %d, %v; want clean EOF", n, err)
	}
	if !stream.isExhausted() {
		t.Fatal("stream did not record exhaustion")
	}
	if err := stream.renderError(); err == nil || err.Error() != "render failed" {
		t.Fatalf("renderError = %v", err)
	}
}

func TestMusicPlaybackMonitorDoesNotFinishFromWallClock(t *testing.T) {
	started := time.Unix(1, 0)
	monitor := newMusicPlaybackMonitor(started, 10*time.Second)
	complete, resume, err := monitor.update(started.Add(time.Hour), true, 5*time.Second, true, nil)
	if err != nil || complete || resume {
		t.Fatalf("update = complete:%t resume:%t err:%v, want active playback", complete, resume, err)
	}
}

func TestMusicPlaybackMonitorResumesBeforeBufferedEnd(t *testing.T) {
	started := time.Unix(1, 0)
	monitor := newMusicPlaybackMonitor(started, 10*time.Second)
	monitor.update(started.Add(time.Second), true, time.Second, false, nil)
	complete, resume, err := monitor.update(started.Add(2*time.Second), false, time.Second, true, nil)
	if err != nil || complete || !resume {
		t.Fatalf("update = complete:%t resume:%t err:%v, want resume", complete, resume, err)
	}
}

func TestMusicPlaybackMonitorFinishesAfterBufferedEnd(t *testing.T) {
	started := time.Unix(1, 0)
	monitor := newMusicPlaybackMonitor(started, 10*time.Second)
	complete, resume, err := monitor.update(started.Add(10*time.Second), false, 10*time.Second, true, nil)
	if err != nil || !complete || resume {
		t.Fatalf("update = complete:%t resume:%t err:%v, want complete", complete, resume, err)
	}
}

func TestMusicPlaybackMonitorResetsStallAfterDelayedStart(t *testing.T) {
	started := time.Unix(1, 0)
	monitor := newMusicPlaybackMonitor(started, 10*time.Second)
	monitor.update(started.Add(9*time.Second), false, 0, false, nil)
	monitor.update(started.Add(11*time.Second), true, 0, false, nil)
	complete, _, err := monitor.update(started.Add(12*time.Second), false, 0, false, nil)
	if err != nil || complete {
		t.Fatalf("update = complete:%t err:%v, want recovery window after delayed start", complete, err)
	}
}

func TestMusicNoteOffRoundsToLaterRenderBoundary(t *testing.T) {
	origSynth := newSynthesizer
	origFont := sfntCached
	origSettings := synthSettings
	t.Cleanup(func() {
		newSynthesizer = origSynth
		setupSynthOnce = sync.Once{}
		sfntCached = origFont
		synthSettings = origSettings
	})

	timing := &timingStreamSynth{}
	newSynthesizer = func(*meltysynth.SoundFont, *meltysynth.SynthesizerSettings) (synthesizer, error) {
		return timing, nil
	}
	setupSynthOnce = sync.Once{}
	sfntCached = &meltysynth.SoundFont{}
	synthSettings = meltysynth.NewSynthesizerSettings(sampleRate)

	originalEnd := block + block/2
	duration := time.Duration(originalEnd) * time.Second / sampleRate
	renderer, err := newSongRenderer(0, []Note{{Key: 60, Velocity: 100, Duration: duration}})
	if err != nil {
		t.Fatal(err)
	}
	if got := renderer.events[0].end; got < originalEnd || got%block != 0 {
		t.Fatalf("aligned note end = %d, want a render boundary at or after %d", got, originalEnd)
	}
	if _, _, err := renderer.render(renderer.totalSamples); err != nil {
		t.Fatal(err)
	}
	if len(timing.noteOffs) != 1 || timing.noteOffs[0] < originalEnd {
		t.Fatalf("note offs = %v, want no release before sample %d", timing.noteOffs, originalEnd)
	}
}

func TestMusicNoteOffRoundingPreservesSameKeyRetrigger(t *testing.T) {
	origSynth := newSynthesizer
	origFont := sfntCached
	origSettings := synthSettings
	t.Cleanup(func() {
		newSynthesizer = origSynth
		setupSynthOnce = sync.Once{}
		sfntCached = origFont
		synthSettings = origSettings
	})

	timing := &timingStreamSynth{}
	newSynthesizer = func(*meltysynth.SoundFont, *meltysynth.SynthesizerSettings) (synthesizer, error) {
		return timing, nil
	}
	setupSynthOnce = sync.Once{}
	sfntCached = &meltysynth.SoundFont{}
	synthSettings = meltysynth.NewSynthesizerSettings(sampleRate)

	toDuration := func(samples int) time.Duration {
		return time.Duration((int64(samples)*int64(time.Second) + sampleRate/2) / sampleRate)
	}
	renderer, err := newSongRenderer(25, []Note{
		{Key: 60, Velocity: 100, Duration: toDuration(block + block/2)},
		{Key: 60, Velocity: 100, Start: toDuration(block + 3*block/4), Duration: toDuration(block)},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := renderer.render(renderer.totalSamples); err != nil {
		t.Fatal(err)
	}
	if len(timing.noteOns) != 2 {
		t.Fatalf("note ons = %v, want both same-key notes retriggered", timing.noteOns)
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
	defer func() {
		_ = stream.Close()
		stream.waitForProducer()
	}()

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
	defer func() {
		_ = stream.Close()
		stream.waitForProducer()
	}()

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
	defer func() {
		_ = stream.Close()
		stream.waitForProducer()
	}()

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
