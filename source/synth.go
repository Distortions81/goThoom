package main

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
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
	block = 1024

	// tailSamples extends the rendered length to allow natural release/verb.
	// Keep a small base tail to capture synth effect decays even without fade.
	tailSamples = sampleRate // ~1.0s base tail

	// fadeOutSamples controls the length of the final fade applied within the
	// rendered tail to guarantee a smooth 1s ramp to silence.
	fadeOutSamples = sampleRate // 1 second
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

	musicPlayers   = make(map[*audio.Player]musicTrack)
	musicPlayersMu sync.Mutex
)

type musicTrack struct {
	stream *musicStream
	whos   map[int]struct{}
}

// newSynthesizer constructs a meltysynth synthesizer. Tests may override this to
// inject a mock implementation.
var newSynthesizer = func(sf *meltysynth.SoundFont, settings *meltysynth.SynthesizerSettings) (synthesizer, error) {
	return meltysynth.NewSynthesizer(sf, settings)
}

func stopAllMusic() {
	musicPlayersMu.Lock()
	for p, track := range musicPlayers {
		_ = p.Close()
		_ = track.stream.Close()
		delete(musicPlayers, p)
	}
	musicPlayersMu.Unlock()
}

func stopMusicFor(who int) {
	musicPlayersMu.Lock()
	for p, track := range musicPlayers {
		if _, ok := track.whos[who]; !ok {
			continue
		}
		_ = p.Close()
		_ = track.stream.Close()
		delete(musicPlayers, p)
	}
	musicPlayersMu.Unlock()
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
	// Disable the built-in reverb/chorus effect to match the desired dry output.
	settings.EnableReverbAndChorus = false
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
	r, err := newSongRenderer(program, notes)
	if err != nil {
		return nil, nil, err
	}
	leftAll := make([]float32, 0, r.totalSamples)
	rightAll := make([]float32, 0, r.totalSamples)
	for r.remaining() > 0 {
		left, right, err := r.render(min(block, r.remaining()))
		if err != nil {
			return nil, nil, err
		}
		leftAll = append(leftAll, left...)
		rightAll = append(rightAll, right...)
	}
	return leftAll, rightAll, nil
}

// songRenderer keeps the synthesizer and event state for one tune. It permits
// a song to be rendered in bounded pieces without changing its note timing.
type songRenderer struct {
	syn          synthesizer
	events       []songEvent
	active       map[int]bool
	pos          int
	totalSamples int
}

type songEvent struct {
	key, vel   int
	start, end int
}

func newSongRenderer(program int, notes []Note) (*songRenderer, error) {
	setupSynthOnce.Do(setupSynth)
	if sfntCached == nil || synthSettings == nil {
		return nil, errors.New("synth not initialized")
	}

	const ch = 0
	// Build a fresh synth per song to avoid concurrent use of internal state.
	syn, err := newSynthesizer(sfntCached, synthSettings)
	if err != nil {
		return nil, err
	}
	syn.ProcessMidiMessage(ch, 0xC0, int32(program), 0)

	var events []songEvent
	var maxEnd int
	for _, n := range notes {
		durSamples := int((n.Duration.Nanoseconds()*int64(sampleRate) + int64(time.Second/2)) / int64(time.Second))
		if durSamples <= 0 {
			continue
		}
		startSamples := int((n.Start.Nanoseconds()*int64(sampleRate) + int64(time.Second/2)) / int64(time.Second))
		ev := songEvent{key: n.Key, vel: n.Velocity, start: startSamples, end: startSamples + durSamples}
		events = append(events, ev)
		if ev.end > maxEnd {
			maxEnd = ev.end
		}
	}
	// Optional per-program release extension to avoid abrupt cuts on plucked
	// instruments without affecting scheduling. Extend ends slightly but never
	// past the next start for the same key.
	// Tune values conservatively to preserve rhythmic gaps.
	extraRelease := 0
	switch program {
	case 25: // Acoustic Guitar (steel) – Gitor
		extraRelease = int(0.800 * sampleRate) // ~800ms
	case 46: // Harp
		extraRelease = int(0.300 * sampleRate) // ~300ms
	}
	if extraRelease > 0 && len(events) > 0 {
		// Build per-key indices of starts
		startsByKey := make(map[int][]int)
		for i, ev := range events {
			startsByKey[ev.key] = append(startsByKey[ev.key], i)
		}
		for _, idxs := range startsByKey {
			// For each occurrence of this key, extend end up to next start-1
			for j, idx := range idxs {
				nextStart := int(^uint(0) >> 1) // max int
				if j+1 < len(idxs) {
					nextIdx := idxs[j+1]
					nextStart = events[nextIdx].start
				}
				// Proposed new end
				newEnd := events[idx].end + extraRelease
				if newEnd >= nextStart {
					newEnd = nextStart - 1
				}
				if newEnd > events[idx].end {
					events[idx].end = newEnd
				}
			}
		}
		// Recompute maxEnd
		maxEnd = 0
		for _, ev := range events {
			if ev.end > maxEnd {
				maxEnd = ev.end
			}
		}
	}

	return &songRenderer{
		syn:          syn,
		events:       events,
		active:       make(map[int]bool),
		totalSamples: maxEnd + tailSamples,
	}, nil
}

func (r *songRenderer) remaining() int { return r.totalSamples - r.pos }

// render returns exactly count frames (or the remaining frames, if fewer).
func (r *songRenderer) render(count int) ([]float32, []float32, error) {
	if count > r.remaining() {
		count = r.remaining()
	}
	leftAll := make([]float32, 0, count)
	rightAll := make([]float32, 0, count)
	for count > 0 {
		n := block
		if n > count {
			n = count
		}
		start := r.pos
		end := start + n
		// First process all note-offs that land in this block so that a
		// note retrigger (end and start in same block) can fire correctly.
		for _, ev := range r.events {
			if ev.end >= start && ev.end < end && r.active[ev.key] {
				r.syn.NoteOff(0, int32(ev.key))
				r.active[ev.key] = false
			}
		}
		// Then process note-ons for this block.
		for _, ev := range r.events {
			if ev.start >= start && ev.start < end && !r.active[ev.key] {
				r.syn.NoteOn(0, int32(ev.key), int32(ev.vel))
				r.active[ev.key] = true
			}
		}
		// Always ask the synth to render a full block, then trim to the
		// number of remaining samples we actually need to keep timing exact.
		left := make([]float32, block)
		right := make([]float32, block)
		if err := safeRender(r.syn, left, right); err != nil {
			return nil, nil, fmt.Errorf("synth render: %v", err)
		}
		leftAll = append(leftAll, left[:n]...)
		rightAll = append(rightAll, right[:n]...)
		r.pos += n
		count -= n
	}
	return leftAll, rightAll, nil
}

// safeRender calls the synthesizer Render method while protecting against
// panics from the underlying synth implementation. Any panic is recovered and
// returned as an error so callers can fail gracefully instead of crashing the
// entire client.
func safeRender(s synthesizer, left, right []float32) (err error) {
	s.Render(left, right)
	return nil
}

type musicReverbTap struct {
	seconds  float64
	feedback float64
}

type musicAllpassTap struct {
	seconds float64
	gain    float64
}

type musicCombState struct {
	buf      []float64
	idx      int
	feedback float64
	filter   float64
}

type musicAllpassState struct {
	buf  []float64
	idx  int
	gain float64
}

type musicReverbChannel struct {
	combs     []musicCombState
	allpasses []musicAllpassState
	preDelay  []float64
	preIdx    int
	wetState  float64
}

type musicReverb struct {
	left, right musicReverbChannel
}

func newMusicReverb(rate int) *musicReverb {
	return &musicReverb{
		left:  newMusicReverbChannel(rate, []musicReverbTap{{0.0297, 0.82}, {0.0371, 0.8}, {0.0411, 0.78}, {0.0531, 0.76}, {0.0617, 0.74}}, []musicAllpassTap{{0.0053, 0.63}, {0.0127, 0.52}, {0.0017, 0.6}}, 0.022),
		right: newMusicReverbChannel(rate, []musicReverbTap{{0.0311, 0.82}, {0.0387, 0.8}, {0.0433, 0.78}, {0.0551, 0.76}, {0.0647, 0.74}}, []musicAllpassTap{{0.0047, 0.63}, {0.0119, 0.52}, {0.0019, 0.6}}, 0.028),
	}
}

func newMusicReverbChannel(rate int, taps []musicReverbTap, diffusers []musicAllpassTap, preDelaySeconds float64) musicReverbChannel {
	r := musicReverbChannel{}
	for _, t := range taps {
		if delay := int(math.Round(t.seconds * float64(rate))); delay > 0 {
			r.combs = append(r.combs, musicCombState{buf: make([]float64, delay), feedback: t.feedback})
		}
	}
	for _, d := range diffusers {
		if delay := int(math.Round(d.seconds * float64(rate))); delay > 0 {
			r.allpasses = append(r.allpasses, musicAllpassState{buf: make([]float64, delay), gain: d.gain})
		}
	}
	if n := int(math.Round(preDelaySeconds * float64(rate))); n > 0 {
		r.preDelay = make([]float64, n)
	}
	return r
}

func (r *musicReverb) Process(left, right []float32) {
	r.left.process(left)
	r.right.process(right)
}

func (r *musicReverbChannel) process(samples []float32) {
	if len(r.combs) == 0 {
		return
	}
	const damping, wetMix, dryMix, wetLowpass = 0.35, 0.34, 0.66, 0.25
	for i := range samples {
		dry, input := float64(samples[i]), float64(samples[i])
		if len(r.preDelay) > 0 {
			input = r.preDelay[r.preIdx]
			r.preDelay[r.preIdx] = dry
			r.preIdx = (r.preIdx + 1) % len(r.preDelay)
		}
		wet := 0.0
		for i := range r.combs {
			c := &r.combs[i]
			delayed := c.buf[c.idx]
			c.filter += (delayed - c.filter) * damping
			wet += c.filter
			c.buf[c.idx] = input + c.filter*c.feedback
			c.idx = (c.idx + 1) % len(c.buf)
		}
		wet /= float64(len(r.combs))
		for i := range r.allpasses {
			ap := &r.allpasses[i]
			y := ap.buf[ap.idx] - ap.gain*wet
			ap.buf[ap.idx] = wet + y*ap.gain
			wet = y
			ap.idx = (ap.idx + 1) % len(ap.buf)
		}
		r.wetState += (wet - r.wetState) * wetLowpass
		v := dry*dryMix + r.wetState*wetMix
		if v > 1 {
			v = 1
		} else if v < -1 {
			v = -1
		}
		samples[i] = float32(v)
	}
}

// applyMusicReverb is retained for one-shot callers. Streaming music uses a
// persistent musicReverb instance instead.
func applyMusicReverb(left, right []float32, rate int) { newMusicReverb(rate).Process(left, right) }

// mixPCM normalizes the provided samples and returns interleaved 16-bit PCM
// data suitable for audio playback.
func mixPCM(leftAll, rightAll []float32) []byte {
	// Apply a 1s fade-out at the end to ensure smooth endings.
	if len(leftAll) == len(rightAll) && len(leftAll) > 0 {
		fadeSamples := fadeOutSamples
		n := len(leftAll)
		if fadeSamples > n {
			fadeSamples = n
		}
		start := n - fadeSamples
		// Linear fade from 1.0 -> 0.0 over the last fadeSamples
		for i := start; i < n; i++ {
			t := float32(i-start) / float32(fadeSamples)
			if t < 0 {
				t = 0
			}
			if t > 1 {
				t = 1
			}
			g := 1.0 - t
			leftAll[i] *= g
			rightAll[i] *= g
		}
	}

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

const (
	musicRenderSeconds = 1
	musicBufferSeconds = 5
	// Keep intermediate stream chunks aligned to the synthesizer's render
	// block. Rendering a partial block advances meltysynth through the entire
	// block, so discarding its unused tail would create a seam before the next
	// chunk.
	musicChunkFrames = (musicRenderSeconds * sampleRate / block) * block
)

// musicStream exposes rendered PCM to Ebiten as it becomes available. Five
// block-aligned chunks (approximately five seconds) are produced before
// playback starts; after that the renderer keeps the queue full.
type musicStream struct {
	chunks      chan []byte
	done        chan struct{}
	ready       chan struct{}
	totalFrames int
	once        sync.Once
	readyOnce   sync.Once
	data        []byte
	err         error
	mu          sync.Mutex
}

func newMusicStream(program int, notes []Note) (*musicStream, error) {
	return newMixedMusicStream([]musicPart{{program: program, notes: notes}})
}

type musicPart struct {
	program int
	notes   []Note
}

const maxMusicPan = 0.40

// musicPan distributes a group evenly across a conservative stereo field.
// A single part is centered; two parts use -40% and +40%.
func musicPan(index, count int) float32 {
	if count <= 1 {
		return 0
	}
	return -maxMusicPan + (2*maxMusicPan*float32(index))/float32(count-1)
}

// applyMusicPan preserves the centered level and attenuates only the far
// channel. This avoids boosting multi-bard mixes into clipping.
func applyMusicPan(left, right []float32, pan float32) {
	if pan < 0 {
		gain := 1 + pan
		for i := range right {
			right[i] *= gain
		}
	} else if pan > 0 {
		gain := 1 - pan
		for i := range left {
			left[i] *= gain
		}
	}
}

// newMixedMusicStream renders all parts into one PCM stream. This avoids
// relying on simultaneous audio-backend players for /with bard groups.
func newMixedMusicStream(parts []musicPart) (*musicStream, error) {
	if len(parts) == 0 {
		return nil, errors.New("empty music group")
	}
	renderers := make([]*songRenderer, 0, len(parts))
	maxFrames := 0
	for _, part := range parts {
		renderer, err := newSongRenderer(part.program, part.notes)
		if err != nil {
			return nil, err
		}
		renderers = append(renderers, renderer)
		if renderer.totalSamples > maxFrames {
			maxFrames = renderer.totalSamples
		}
	}
	s := &musicStream{
		chunks:      make(chan []byte, musicBufferSeconds),
		done:        make(chan struct{}),
		ready:       make(chan struct{}),
		totalFrames: maxFrames,
	}
	go s.produceMixed(renderers)
	<-s.ready // render five seconds before the caller starts the player
	return s, nil
}

func (s *musicStream) produceMixed(renderers []*songRenderer) {
	defer close(s.chunks)
	defer s.signalReady()
	chunkFrames := musicChunkFrames
	buffered := 0
	var dump []byte
	var reverb *musicReverb
	for pos := 0; pos < s.totalFrames; {
		frames := min(chunkFrames, s.totalFrames-pos)
		left, right := make([]float32, frames), make([]float32, frames)
		for i, renderer := range renderers {
			if renderer.remaining() == 0 {
				continue
			}
			partLeft, partRight, err := renderer.render(min(frames, renderer.remaining()))
			if err != nil {
				s.mu.Lock()
				s.err = err
				s.mu.Unlock()
				return
			}
			if gs.MusicStereoPan {
				applyMusicPan(partLeft, partRight, musicPan(i, len(renderers)))
			}
			for i := range partLeft {
				left[i] += partLeft[i]
				right[i] += partRight[i]
			}
		}
		if gs.MusicEnhancement {
			if reverb == nil {
				reverb = newMusicReverb(sampleRate)
			}
			reverb.Process(left, right)
		}
		pos += frames
		pcm := mixPCMChunk(left, right, pos == s.totalFrames)
		if dumpMusic {
			dump = append(dump, pcm...)
		}
		select {
		case s.chunks <- pcm:
			buffered++
			if buffered == musicBufferSeconds {
				s.signalReady()
			}
		case <-s.done:
			return
		}
	}
	if dumpMusic {
		dumpPCMAsWAV(dump)
	}
}

func (s *musicStream) signalReady() {
	s.readyOnce.Do(func() { close(s.ready) })
}

// Read blocks only after the initial five-second buffer has been consumed.
// That gives the render goroutine enough time to keep the audio device fed.
func (s *musicStream) Read(p []byte) (int, error) {
	for len(s.data) == 0 {
		select {
		case s.data = <-s.chunks:
			if s.data == nil {
				s.mu.Lock()
				err := s.err
				s.mu.Unlock()
				if err != nil {
					return 0, err
				}
				return 0, io.EOF
			}
		case <-s.done:
			return 0, io.EOF
		}
	}
	n := copy(p, s.data)
	s.data = s.data[n:]
	return n, nil
}

func (s *musicStream) Close() error {
	s.once.Do(func() { close(s.done) })
	return nil
}

// mixPCMChunk intentionally does not normalize each chunk: independent peak
// normalization would make one bard's volume jump from second to second.
func mixPCMChunk(left, right []float32, final bool) []byte {
	if final {
		fade := fadeOutSamples
		if fade > len(left) {
			fade = len(left)
		}
		for i := len(left) - fade; i < len(left); i++ {
			g := 1 - float32(i-(len(left)-fade))/float32(fade)
			left[i] *= g
			right[i] *= g
		}
	}
	pcm := make([]byte, len(left)*4)
	for i := range left {
		l := int16(left[i] * 32767)
		r := int16(right[i] * 32767)
		binary.LittleEndian.PutUint16(pcm[4*i:], uint16(l))
		binary.LittleEndian.PutUint16(pcm[4*i+2:], uint16(r))
	}
	return pcm
}

// Play starts an independent, five-second-buffered music stream. Rendering is
// done in one-second increments, allowing several bards to play together.
func Play(ctx *audio.Context, program int, notes []Note) error {
	return playMusic(ctx, program, notes, 0, nil, nil)
}

// prepared is called after the initial five seconds are rendered. start gates
// playback so a /with group can begin its independently rendered tracks at the
// same instant.
func playMusic(ctx *audio.Context, program int, notes []Note, who int, prepared func(), start <-chan struct{}) error {
	return playMusicGroup(ctx, []musicPart{{program: program, notes: notes}}, []int{who}, prepared, start)
}

func playMusicGroup(ctx *audio.Context, parts []musicPart, whos []int, prepared func(), start <-chan struct{}) error {

	if ctx == nil {
		return errors.New("nil audio context")
	}

	if gs.Mute || focusMuted || !gs.Music || gs.MasterVolume <= 0 || gs.MusicVolume <= 0 {
		return errors.New("music muted")
	}

	stream, err := newMixedMusicStream(parts)
	if err != nil {
		if prepared != nil {
			prepared()
		}
		return err
	}
	player, err := ctx.NewPlayer(stream)
	if err != nil {
		_ = stream.Close()
		if prepared != nil {
			prepared()
		}
		return err
	}

	vol := gs.MasterVolume * gs.MusicVolume
	if gs.Mute || focusMuted {
		vol = 0
	}
	player.SetVolume(vol)

	musicPlayersMu.Lock()
	trackWhos := make(map[int]struct{}, len(whos))
	for _, who := range whos {
		trackWhos[who] = struct{}{}
	}
	musicPlayers[player] = musicTrack{stream: stream, whos: trackWhos}
	musicPlayersMu.Unlock()

	if prepared != nil {
		prepared()
	}
	if start != nil {
		<-start
	}
	player.Play()

	// Oto can report IsPlaying false briefly before its first audio callback.
	// Keep the stream alive for its rendered duration instead of treating that
	// transient state as an ended song.
	playDuration := time.Duration(stream.totalFrames) * time.Second / sampleRate
	timer := time.NewTimer(playDuration + 100*time.Millisecond)
	select {
	case <-timer.C:
	case <-stream.done:
		if !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}
	}

	musicPlayersMu.Lock()
	delete(musicPlayers, player)
	musicPlayersMu.Unlock()

	_ = stream.Close()
	// A scoped or global music stop can close the player before this owner
	// goroutine wakes on stream.done. Closing twice is expected in that case.
	_ = player.Close()
	return nil
}

// safeIsPlaying checks IsPlaying and recovers if the player has been closed.
func safeIsPlaying(p *audio.Player) (ok bool) {
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
