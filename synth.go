package main

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"log"
	"math"
	"os"
	"path"
	"sync"
	"time"

	"github.com/hajimehoshi/ebiten/v2/audio"
	meltysynth "github.com/sinshu/go-meltysynth/meltysynth"
)

const (
	sampleRate = 44100
	// Use a small fixed render block that aligns with common synth effect
	// processing sizes to avoid internal ring-buffer edge cases.
	block = 64

    // tailSamples extends the rendered length to allow reverb to decay.
    // Keep this short to better match classic client perceived song length.
    tailSamples = sampleRate / 2 // ~0.5s
)

// Note represents a single MIDI note with a duration and start time.
type Note struct {
	// Key is the MIDI note number (e.g. 60 = middle C).
	Key int
	// Velocity is the MIDI velocity 1..127.
	Velocity int
	// Start is the time offset from the beginning when the note starts.
	Start time.Duration
	// Duration specifies how long the note should sound.
	Duration time.Duration
}

// synthesizer abstracts the subset of meltysynth.Synthesizer used by Play.
type synthesizer interface {
	ProcessMidiMessage(channel int32, command int32, data1, data2 int32)
	NoteOn(channel, key, vel int32)
	NoteOff(channel, key int32)
	Render(left, right []float32)
}

var (
	setupSynthOnce sync.Once
	sfntCached     *meltysynth.SoundFont
	synthSettings  *meltysynth.SynthesizerSettings

	musicPlayers   = make(map[*audio.Player]struct{})
	musicPlayersMu sync.Mutex
)

// newSynthesizer constructs a meltysynth synthesizer. Tests may override this to
// inject a mock implementation.
var newSynthesizer = func(sf *meltysynth.SoundFont, settings *meltysynth.SynthesizerSettings) (synthesizer, error) {
	return meltysynth.NewSynthesizer(sf, settings)
}

func stopAllMusic() {
	musicPlayersMu.Lock()
	defer musicPlayersMu.Unlock()
	for p := range musicPlayers {
		_ = p.Close()
		delete(musicPlayers, p)
	}
}

func setupSynth() {
	var err error

	sfPath := path.Join(dataDirPath, "soundfont.sf2")

	var sfData []byte
	sfData, err = os.ReadFile(sfPath)
	if err != nil {
		log.Printf("soundfont missing: %v", err)
		return
	}
	rs := bytes.NewReader(sfData)
	sfnt, err := meltysynth.NewSoundFont(rs)
	if err != nil {
		return
	}
	settings := meltysynth.NewSynthesizerSettings(sampleRate)
	// Align meltysynth internal block size with our render loop to reduce
	// chances of effect buffers overrunning on odd boundaries.
	settings.BlockSize = block
	sfntCached = sfnt
	synthSettings = settings
}

// renderSong renders the provided notes using the current SoundFont and returns
// the raw left and right channel samples. The caller can further process or mix
// these samples before playback.
func renderSong(program int, notes []Note) ([]float32, []float32, error) {
	setupSynthOnce.Do(setupSynth)
	if sfntCached == nil || synthSettings == nil {
		return nil, nil, errors.New("synth not initialized")
	}

	const ch = 0
	// Build a fresh synth per song to avoid concurrent use of internal state.
	syn, err := newSynthesizer(sfntCached, synthSettings)
	if err != nil {
		return nil, nil, err
	}
	syn.ProcessMidiMessage(ch, 0xC0, int32(program), 0)

	type event struct {
		key, vel   int
		start, end int
	}
	var events []event
	var maxEnd int
	for _, n := range notes {
		durSamples := int((n.Duration.Nanoseconds()*int64(sampleRate) + int64(time.Second/2)) / int64(time.Second))
		if durSamples <= 0 {
			continue
		}
		startSamples := int((n.Start.Nanoseconds()*int64(sampleRate) + int64(time.Second/2)) / int64(time.Second))
		ev := event{key: n.Key, vel: n.Velocity, start: startSamples, end: startSamples + durSamples}
		events = append(events, ev)
		if ev.end > maxEnd {
			maxEnd = ev.end
		}
	}
	totalSamples := maxEnd + tailSamples

	leftAll := make([]float32, 0, totalSamples)
	rightAll := make([]float32, 0, totalSamples)
	active := map[int]bool{}

	trigger := func(start, count int) {
		end := start + count
		for _, ev := range events {
			if ev.start >= start && ev.start < end && !active[ev.key] {
				syn.NoteOn(ch, int32(ev.key), int32(ev.vel))
				active[ev.key] = true
			}
			if ev.end >= start && ev.end < end && active[ev.key] {
				syn.NoteOff(ch, int32(ev.key))
				active[ev.key] = false
			}
		}
	}

	for pos := 0; pos < totalSamples; pos += block {
		// Render in fixed-size blocks to avoid triggering edge cases in the
		// underlying synth (e.g., effects processing relying on block size).
		n := block
		if pos+n > totalSamples {
			n = totalSamples - pos
		}
		trigger(pos, n)
		// Always ask the synth to render a full block, then trim to the
		// number of remaining samples we actually need to keep timing exact.
		left := make([]float32, block)
		right := make([]float32, block)
		if err := safeRender(syn, left, right); err != nil {
			return nil, nil, fmt.Errorf("synth render: %v", err)
		}
		leftAll = append(leftAll, left[:n]...)
		rightAll = append(rightAll, right[:n]...)
	}

	return leftAll, rightAll, nil
}

// safeRender calls the synthesizer Render method while protecting against
// panics from the underlying synth implementation. Any panic is recovered and
// returned as an error so callers can fail gracefully instead of crashing the
// entire client.
func safeRender(s synthesizer, left, right []float32) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic in synth.Render: %v", r)
		}
	}()
	s.Render(left, right)
	return nil
}

// mixPCM normalizes the provided samples and returns interleaved 16-bit PCM
// data suitable for audio playback.
func mixPCM(leftAll, rightAll []float32) []byte {
	// Normalize to avoid clipping and boost quiet audio
	var peak float32
	for i := range leftAll {
		if v := float32(math.Abs(float64(leftAll[i]))); v > peak {
			peak = v
		}
		if v := float32(math.Abs(float64(rightAll[i]))); v > peak {
			peak = v
		}
	}
	if peak > 0 {
		g := float32(0.99) / peak
		if g != 1 {
			for i := range leftAll {
				leftAll[i] *= g
				rightAll[i] *= g
			}
		}
	}

	pcm := make([]byte, len(leftAll)*4)
	for i := range leftAll {
		l := int16(leftAll[i] * 32767)
		r := int16(rightAll[i] * 32767)
		binary.LittleEndian.PutUint16(pcm[4*i:], uint16(l))
		binary.LittleEndian.PutUint16(pcm[4*i+2:], uint16(r))
	}
	return pcm
}

// Play renders the provided notes using the given SoundFont, mixes the entire
// song, and then plays it through the provided audio context. The function
// blocks until playback has finished.
func Play(ctx *audio.Context, program int, notes []Note) error {

	if ctx == nil {
		return errors.New("nil audio context")
	}

	if gs.Mute || !gs.Music || gs.MasterVolume <= 0 || gs.MusicVolume <= 0 {
		return errors.New("music muted")
	}

	leftAll, rightAll, err := renderSong(program, notes)
	if err != nil {
		return err
	}

	pcm := mixPCM(leftAll, rightAll)
	if dumpMusic {
		dumpPCMAsWAV(pcm)
	}
	player := ctx.NewPlayerFromBytes(pcm)

	vol := gs.MasterVolume * gs.MusicVolume
	if gs.Mute {
		vol = 0
	}
	player.SetVolume(vol)

	musicPlayersMu.Lock()
	musicPlayers[player] = struct{}{}
	musicPlayersMu.Unlock()

    player.Play()

    // Compute the logical song duration from note events (without long
    // reverb tail), then add a small grace period to avoid clipping endings.
    // This keeps playback length closer to the classic client.
    var lastEnd time.Duration
    for _, n := range notes {
        if e := n.Start + n.Duration; e > lastEnd {
            lastEnd = e
        }
    }
    grace := 300 * time.Millisecond
    if grace < 0 {
        grace = 0
    }
    target := time.Now().Add(lastEnd + grace)
    for time.Now().Before(target) {
        if !safeIsPlaying(player) {
            break
        }
        time.Sleep(25 * time.Millisecond)
    }

	musicPlayersMu.Lock()
	delete(musicPlayers, player)
	musicPlayersMu.Unlock()

	return player.Close()
}

// safeIsPlaying checks IsPlaying and recovers if the player has been closed.
func safeIsPlaying(p *audio.Player) (ok bool) {
	defer func() {
		if recover() != nil {
			ok = false
		}
	}()
	return p.IsPlaying()
}

// dumpPCMAsWAV writes the provided 16-bit stereo PCM data to a WAV file when
// the -dumpMusic flag is set. Files are named music_YYYYMMDD_HHMMSS.wav.
func dumpPCMAsWAV(pcm []byte) {
	ts := time.Now().Format("20060102_150405")
	name := "music_" + ts + ".wav"
	f, err := os.Create(name)
	if err != nil {
		log.Printf("dump music: %v", err)
		return
	}
	defer f.Close()

	dataLen := uint32(len(pcm))
	var header [44]byte
	copy(header[0:], []byte("RIFF"))
	binary.LittleEndian.PutUint32(header[4:], 36+dataLen)
	copy(header[8:], []byte("WAVE"))
	copy(header[12:], []byte("fmt "))
	binary.LittleEndian.PutUint32(header[16:], 16)
	binary.LittleEndian.PutUint16(header[20:], 1)
	binary.LittleEndian.PutUint16(header[22:], 2)
	binary.LittleEndian.PutUint32(header[24:], uint32(sampleRate))
	binary.LittleEndian.PutUint32(header[28:], uint32(sampleRate*4))
	binary.LittleEndian.PutUint16(header[32:], 4)
	binary.LittleEndian.PutUint16(header[34:], 16)
	copy(header[36:], []byte("data"))
	binary.LittleEndian.PutUint32(header[40:], dataLen)

	if _, err := f.Write(header[:]); err != nil {
		log.Printf("dump music header: %v", err)
		return
	}
	if _, err := f.Write(pcm); err != nil {
		log.Printf("dump music data: %v", err)
		return
	}
	log.Printf("wrote %s", name)
}
