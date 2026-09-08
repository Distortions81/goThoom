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
	"path/filepath"
	"sort"
	"strings"
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
	setupSynthOnce  sync.Once
	sfntCached      *meltysynth.SoundFont
	sfntFallback    *meltysynth.SoundFont
	synthSettings   *meltysynth.SynthesizerSettings
	synthCacheMu    sync.RWMutex
	synthGeneration uint64

	programGainMu    sync.Mutex
	programGainCache = make(map[programGainKey]*programGainCacheEntry)

	missingProgramMu       sync.Mutex
	missingProgramReported = make(map[programGainKey]struct{})

	musicPlayers   = make(map[*audio.Player]musicTrack)
	musicPlayersMu sync.Mutex
)

type musicTrack struct {
	stream     *musicStream
	whos       map[int]struct{}
	ctx        *audio.Context
	parts      []musicPart
	startFrame int
}

type programGainKey struct {
	generation uint64
	program    int
}

type programGainCacheEntry struct {
	once sync.Once
	gain float32
}

const (
	programCalibrationFrames    = sampleRate
	programNormalizationRMS     = 0.08
	programNormalizationPeak    = 0.35
	programNormalizationMinGain = 0.25
	programNormalizationMaxGain = 4.0
)

var programCalibrationKeys = [...]int32{48, 60, 72}

// configuredSoundFontFile returns the selected SoundFont's base name. Older
// settings files do not have a selection, so they continue to use the bundled
// soundfont.sf2. Do not allow a hand-edited setting to escape the audio folder.
func configuredSoundFontFile() string {
	name := gs.SoundFontFile
	if name == "" {
		return soundFontFile
	}
	if filepath.Base(name) != name || !strings.EqualFold(filepath.Ext(name), ".sf2") {
		return soundFontFile
	}
	return name
}

func loadSoundFont(path string) (*meltysynth.SoundFont, error) {
	sfData, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return meltysynth.NewSoundFont(bytes.NewReader(sfData))
}

func loadFallbackSoundFont(selectedName string, selected *meltysynth.SoundFont) *meltysynth.SoundFont {
	if strings.EqualFold(selectedName, soundFontFile) {
		return selected
	}
	fallback, err := loadSoundFont(defaultSoundFontPath())
	if err != nil {
		log.Printf("default soundfont unavailable for missing-instrument fallback: %v", err)
		return nil
	}
	return fallback
}

func soundFontHasProgram(font *meltysynth.SoundFont, program int) bool {
	if font == nil {
		return false
	}
	for _, preset := range font.Presets {
		if preset != nil && preset.BankNumber == 0 && preset.PatchNumber == int32(program) {
			return true
		}
	}
	return false
}

// musicSoundFontForProgram keeps partial custom SoundFonts useful. MeltySynth
// otherwise substitutes the first preset in a font when a requested program
// is absent, which can turn every missing Clan Lord instrument into an
// unrelated sound.
func musicSoundFontForProgram(selected, fallback *meltysynth.SoundFont, program int) *meltysynth.SoundFont {
	if selected == nil {
		if soundFontHasProgram(fallback, program) {
			return fallback
		}
		return nil
	}
	// Unit tests use an empty SoundFont with an injected synthesizer. A parsed
	// production SoundFont always contains at least one preset.
	if len(selected.Presets) == 0 || soundFontHasProgram(selected, program) {
		return selected
	}
	if soundFontHasProgram(fallback, program) {
		return fallback
	}
	return nil
}

func reportMissingSoundFontProgram(generation uint64, program int) string {
	message := fmt.Sprintf("Music SoundFont %q is missing Bank 0 preset %d, and %q cannot provide a fallback.", configuredSoundFontFile(), program, soundFontFile)
	key := programGainKey{generation: generation, program: program}
	missingProgramMu.Lock()
	_, reported := missingProgramReported[key]
	if !reported {
		missingProgramReported[key] = struct{}{}
	}
	missingProgramMu.Unlock()
	if !reported {
		consoleMessage(message)
		log.Print(message)
	}
	return message
}

func newSynthSettings() *meltysynth.SynthesizerSettings {
	settings := meltysynth.NewSynthesizerSettings(sampleRate)
	// Disable the built-in reverb/chorus effect to match the desired dry output.
	settings.EnableReverbAndChorus = false
	// Align meltysynth internal block size with our render loop to reduce
	// chances of effect buffers overrunning on odd boundaries.
	settings.BlockSize = block
	return settings
}

// listSoundFonts returns the valid SF2 files in the configured Assets & Audio
// folder. Parsing each candidate keeps malformed files out of the selector.
func listSoundFonts() ([]string, error) {
	entries, err := os.ReadDir(soundFontsDirPath())
	if err != nil {
		return nil, err
	}
	fonts := make([]string, 0)
	for _, entry := range entries {
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), ".sf2") {
			continue
		}
		if _, err := loadSoundFont(filepath.Join(soundFontsDirPath(), entry.Name())); err == nil {
			fonts = append(fonts, entry.Name())
		}
	}
	sort.Strings(fonts)
	return fonts, nil
}

// selectSoundFont validates and applies a SoundFont to newly started and
// currently playing bard music.
func selectSoundFont(name string) error {
	if filepath.Base(name) != name || !strings.EqualFold(filepath.Ext(name), ".sf2") {
		return fmt.Errorf("invalid soundfont filename %q", name)
	}
	font, err := loadSoundFont(filepath.Join(soundFontsDirPath(), name))
	if err != nil {
		return fmt.Errorf("load soundfont %q: %w", name, err)
	}
	fallback := loadFallbackSoundFont(name, font)
	synthCacheMu.Lock()
	sfntCached = font
	sfntFallback = fallback
	synthSettings = newSynthSettings()
	synthGeneration++
	synthCacheMu.Unlock()
	SettingsLock.Lock()
	gs.SoundFontFile = name
	SettingsLock.Unlock()
	settingsDirty = true
	restartMusicWithSelectedSoundFont()
	return nil
}

// restartMusicWithSelectedSoundFont continues each active bard track at its
// current playback position using the newly selected SoundFont. The replacement
// streams are prepared in background goroutines so selecting a large font does
// not stall the UI.
func restartMusicWithSelectedSoundFont() {
	restartActiveMusic("restart music with selected soundfont")
}

// restartMusicWithCurrentSettings continues active bard tracks from their
// current positions so changes that affect rendering are audible immediately.
func restartMusicWithCurrentSettings() {
	restartActiveMusic("restart music with updated settings")
}

func restartActiveMusic(errorContext string) {
	type restart struct {
		ctx   *audio.Context
		parts []musicPart
		whos  []int
		frame int
	}
	var restarts []restart
	musicPlayersMu.Lock()
	for player, track := range musicPlayers {
		if player == nil || track.ctx == nil {
			continue
		}
		whos := make([]int, 0, len(track.whos))
		for who := range track.whos {
			whos = append(whos, who)
		}
		parts := make([]musicPart, len(track.parts))
		for i, part := range track.parts {
			parts[i] = musicPart{program: part.program, notes: append([]Note(nil), part.notes...)}
		}
		frame := musicTrackPlaybackFrame(track, player.Position())
		restarts = append(restarts, restart{ctx: track.ctx, parts: parts, whos: whos, frame: frame})
		_ = track.stream.Close()
	}
	musicPlayersMu.Unlock()

	settings := currentMusicPlaybackSettings()
	for _, restart := range restarts {
		restart := restart
		go func() {
			if err := playMusicGroupWithSettingsAtFrame(restart.ctx, restart.parts, restart.whos, nil, nil, settings, restart.frame); err != nil {
				log.Printf("%s: %v", errorContext, err)
			}
		}()
	}
}

func musicTrackPlaybackFrame(track musicTrack, position time.Duration) int {
	position = max(position, time.Duration(0))
	return max(track.startFrame+int(position.Seconds()*sampleRate), 0)
}

// newSynthesizer constructs a meltysynth synthesizer. Tests may override this to
// inject a mock implementation.
var newSynthesizer = func(sf *meltysynth.SoundFont, settings *meltysynth.SynthesizerSettings) (synthesizer, error) {
	return meltysynth.NewSynthesizer(sf, settings)
}

func stopAllMusic() {
	musicPlayersMu.Lock()
	for _, track := range musicPlayers {
		_ = track.stream.Close()
	}
	musicPlayersMu.Unlock()
}

func stopMusicFor(who int) {
	musicPlayersMu.Lock()
	for _, track := range musicPlayers {
		if _, ok := track.whos[who]; !ok {
			continue
		}
		_ = track.stream.Close()
	}
	musicPlayersMu.Unlock()
}

func pauseAllMusic() {
	musicPlayersMu.Lock()
	for player, track := range musicPlayers {
		track.stream.setPaused(true)
		player.Pause()
	}
	musicPlayersMu.Unlock()
}

// resumeAllMusic returns whether an existing stream was available to resume.
// A paused movie seek has no stream, so its caller can rebuild from the index.
func resumeAllMusic() bool {
	musicPlayersMu.Lock()
	resumed := false
	for player, track := range musicPlayers {
		track.stream.setPaused(false)
		resumed = track.stream.playIfOpen(player) || resumed
	}
	musicPlayersMu.Unlock()
	return resumed
}

func setupSynth() {
	selectedName := configuredSoundFontFile()
	sfPath := filepath.Join(soundFontsDirPath(), selectedName)
	sfnt, err := loadSoundFont(sfPath)
	if err != nil {
		log.Printf("soundfont missing: %v", err)
		return
	}
	fallback := loadFallbackSoundFont(selectedName, sfnt)
	synthCacheMu.Lock()
	sfntCached = sfnt
	sfntFallback = fallback
	synthSettings = newSynthSettings()
	synthGeneration++
	synthCacheMu.Unlock()
}

// programNormalizationGain converts measured full-velocity reference notes
// into a conservative per-program gain. RMS evens out perceived loudness while
// the peak limit leaves room for chords and multiple simultaneous bards.
func programNormalizationGain(rms, peak float64) float32 {
	if rms <= 0 || peak <= 0 || math.IsNaN(rms) || math.IsNaN(peak) {
		return 1
	}
	gain := math.Min(programNormalizationRMS/rms, programNormalizationPeak/peak)
	gain = math.Max(programNormalizationMinGain, math.Min(programNormalizationMaxGain, gain))
	return float32(gain)
}

func measureProgramGain(font *meltysynth.SoundFont, program int) (gain float32) {
	gain = 1
	defer func() {
		if recovered := recover(); recovered != nil {
			gain = 1
		}
	}()
	var sumSquares float64
	var peak float64
	var samples int
	for _, note := range programCalibrationKeys {
		// Give each pitch a fresh synth so the tail of the previous sample does
		// not inflate the next pitch's measurement.
		syn, err := meltysynth.NewSynthesizer(font, newSynthSettings())
		if err != nil {
			return 1
		}
		syn.ProcessMidiMessage(0, 0xC0, int32(program), 0)
		syn.NoteOn(0, note, 127)
		var noteSquares float64
		var notePeak float64
		noteSamples := 0
		for rendered := 0; rendered < programCalibrationFrames; rendered += block {
			left := make([]float32, block)
			right := make([]float32, block)
			if err := safeRender(syn, left, right); err != nil {
				return 1
			}
			keep := min(block, programCalibrationFrames-rendered)
			for i := 0; i < keep; i++ {
				for _, value := range [...]float32{left[i], right[i]} {
					magnitude := math.Abs(float64(value))
					if magnitude > notePeak {
						notePeak = magnitude
					}
					noteSquares += magnitude * magnitude
					noteSamples++
				}
			}
		}
		// Some specialty presets intentionally cover only part of the keyboard.
		// Do not let a silent probe make an otherwise audible preset too loud.
		if notePeak == 0 {
			continue
		}
		sumSquares += noteSquares
		samples += noteSamples
		peak = max(peak, notePeak)
	}
	if samples == 0 {
		return 1
	}
	return programNormalizationGain(math.Sqrt(sumSquares/float64(samples)), peak)
}

var measureProgramGainForCache = measureProgramGain

func soundFontProgramGain(font *meltysynth.SoundFont, generation uint64, program int) float32 {
	key := programGainKey{generation: generation, program: program}
	programGainMu.Lock()
	entry := programGainCache[key]
	if entry == nil {
		entry = &programGainCacheEntry{}
		programGainCache[key] = entry
	}
	programGainMu.Unlock()
	entry.once.Do(func() { entry.gain = measureProgramGainForCache(font, program) })
	return entry.gain
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
	gain         float32
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
	synthCacheMu.RLock()
	selected, fallback, settings, generation := sfntCached, sfntFallback, synthSettings, synthGeneration
	synthCacheMu.RUnlock()
	font := musicSoundFontForProgram(selected, fallback, program)
	if selected != nil && font == nil {
		return nil, errors.New(reportMissingSoundFontProgram(generation, program))
	}
	if font == nil || settings == nil {
		return nil, errors.New("synth not initialized")
	}

	const ch = 0
	// Build a fresh synth per song to avoid concurrent use of internal state.
	syn, err := newSynthesizer(font, settings)
	if err != nil {
		return nil, err
	}
	syn.ProcessMidiMessage(ch, 0xC0, int32(program), 0)

	var events []songEvent
	for _, n := range notes {
		durSamples := int((n.Duration.Nanoseconds()*int64(sampleRate) + int64(time.Second/2)) / int64(time.Second))
		if durSamples <= 0 {
			continue
		}
		startSamples := int((n.Start.Nanoseconds()*int64(sampleRate) + int64(time.Second/2)) / int64(time.Second))
		ev := songEvent{key: n.Key, vel: n.Velocity, start: startSamples, end: startSamples + durSamples}
		events = append(events, ev)
	}
	startsByKey := make(map[int][]int)
	for i, ev := range events {
		startsByKey[ev.key] = append(startsByKey[ev.key], i)
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
	}

	// Rendering advances the synth in fixed blocks. Round note-offs toward the
	// next block so a note is never released early merely because its end lands
	// between two render boundaries.
	maxEnd := 0
	for _, idxs := range startsByKey {
		for j, idx := range idxs {
			originalEnd := events[idx].end
			if remainder := events[idx].end % block; remainder != 0 {
				events[idx].end += block - remainder
			}
			// Preserve an existing same-key retrigger in the containing block.
			// Rounding its preceding note past this start would suppress the
			// retrigger entirely because the synth still considers the key active.
			if j+1 < len(idxs) {
				nextStart := events[idxs[j+1]].start
				if originalEnd <= nextStart && events[idx].end > nextStart {
					events[idx].end = nextStart
				}
			}
			if events[idx].end > maxEnd {
				maxEnd = events[idx].end
			}
		}
	}

	return &songRenderer{
		syn:          syn,
		gain:         soundFontProgramGain(font, generation, program),
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
		if r.gain != 0 && r.gain != 1 {
			for i := range left {
				left[i] *= r.gain
				right[i] *= r.gain
			}
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
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("synth panic: %v", recovered)
		}
	}()
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

func (r *musicReverb) Process(left, right []float32, amount float64) {
	amount = clampMusicEnhancementAmount(amount)
	r.left.process(left, amount)
	r.right.process(right, amount)
}

func (r *musicReverbChannel) process(samples []float32, amount float64) {
	if len(r.combs) == 0 {
		return
	}
	const damping, baseWetMix, wetLowpass = 0.35, 0.34, 0.25
	wetMix := baseWetMix * amount
	dryMix := 1 - wetMix
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
func applyMusicReverb(left, right []float32, rate int) { newMusicReverb(rate).Process(left, right, 1) }

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
	musicRenderSeconds       = 1
	musicPlayerBuffer        = 2 * time.Second
	musicPlaybackPoll        = 50 * time.Millisecond
	musicPlaybackResumeEvery = 250 * time.Millisecond
	musicPlaybackStall       = 10 * time.Second
	musicPlaybackEndSlop     = 2 * musicPlaybackPoll
	// Keep intermediate stream chunks aligned to the synthesizer's render
	// block. Rendering a partial block advances meltysynth through the entire
	// block, so discarding its unused tail would create a seam before the next
	// chunk.
	musicChunkFrames = (musicRenderSeconds * sampleRate / block) * block
)

// musicStream exposes rendered PCM to Ebiten as it becomes available. The
// configured number of block-aligned chunks are produced before playback
// starts; after that the renderer keeps the queue full.
type musicStream struct {
	chunks       chan []byte
	done         chan struct{}
	ready        chan struct{}
	exhausted    chan struct{}
	producerDone chan struct{}
	totalFrames  int
	once         sync.Once
	readyOnce    sync.Once
	exhaustOnce  sync.Once
	data         []byte
	err          error
	mu           sync.Mutex
	lifecycleMu  sync.Mutex
	closed       bool
	paused       bool
	bufferChunks int
}

func newMusicStream(program int, notes []Note) (*musicStream, error) {
	return newMixedMusicStream([]musicPart{{program: program, notes: notes}})
}

type musicPart struct {
	program int
	notes   []Note
}

// scaleMusicParts changes note timing without resampling the synthesized PCM,
// preserving instrument pitch when movie playback UPS changes.
func scaleMusicParts(parts []musicPart, rate float64) []musicPart {
	if rate <= 0 {
		rate = 1
	}
	scaled := make([]musicPart, len(parts))
	for i, part := range parts {
		scaled[i].program = part.program
		scaled[i].notes = make([]Note, len(part.notes))
		for j, note := range part.notes {
			note.Start = time.Duration(float64(note.Start) / rate)
			note.Duration = time.Duration(float64(note.Duration) / rate)
			scaled[i].notes[j] = note
		}
	}
	return scaled
}

type musicPlaybackSettings struct {
	enabled           bool
	volume            float64
	enhancement       bool
	enhancementAmount float64
	bufferSeconds     int
}

func currentMusicPlaybackSettings() musicPlaybackSettings {
	enabled := !gs.Mute && !focusMuted && gs.Music && gs.MasterVolume > 0 && gs.MusicVolume > 0
	return musicPlaybackSettings{
		enabled:           enabled,
		volume:            effectiveAudioVolume(gs.MasterVolume * gs.MusicVolume),
		enhancement:       gs.MusicEnhancement,
		enhancementAmount: gs.MusicEnhancementAmount,
		bufferSeconds:     clampMusicBufferSeconds(gs.MusicBufferSeconds),
	}
}

// newMixedMusicStream renders all parts into one PCM stream. This avoids
// relying on simultaneous audio-backend players for /with bard groups.
func newMixedMusicStream(parts []musicPart) (*musicStream, error) {
	settings := currentMusicPlaybackSettings()
	return newMixedMusicStreamWithSettingsAtFrame(parts, settings, 0)
}

func newMixedMusicStreamWithSettings(parts []musicPart, settings musicPlaybackSettings) (*musicStream, error) {
	return newMixedMusicStreamWithSettingsAtFrame(parts, settings, 0)
}

func newMixedMusicStreamWithSettingsAtFrame(parts []musicPart, settings musicPlaybackSettings, startFrame int) (*musicStream, error) {
	if len(parts) == 0 {
		return nil, errors.New("empty music group")
	}
	bufferSeconds := settings.bufferSeconds
	if bufferSeconds == 0 {
		bufferSeconds = gsdef.MusicBufferSeconds
	}
	bufferSeconds = clampMusicBufferSeconds(bufferSeconds)
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
	startFrame = min(max(startFrame, 0), maxFrames)
	for remaining := startFrame; remaining > 0; {
		frames := min(block, remaining)
		for _, renderer := range renderers {
			if renderer.remaining() == 0 {
				continue
			}
			if _, _, err := renderer.render(min(frames, renderer.remaining())); err != nil {
				return nil, fmt.Errorf("seek music: %w", err)
			}
		}
		remaining -= frames
	}
	s := &musicStream{
		chunks:       make(chan []byte, bufferSeconds),
		done:         make(chan struct{}),
		ready:        make(chan struct{}),
		exhausted:    make(chan struct{}),
		producerDone: make(chan struct{}),
		totalFrames:  maxFrames - startFrame,
		bufferChunks: bufferSeconds,
	}
	go s.produceMixed(renderers, settings.enhancement, settings.enhancementAmount)
	<-s.ready // render the configured buffer before the caller starts the player
	if err := s.renderError(); err != nil {
		_ = s.Close()
		s.waitForProducer()
		return nil, fmt.Errorf("pre-render music: %w", err)
	}
	return s, nil
}

func (s *musicStream) produceMixed(renderers []*songRenderer, enhancement bool, enhancementAmount float64) {
	defer close(s.producerDone)
	defer close(s.chunks)
	defer s.signalReady()
	chunkFrames := musicChunkFrames
	buffered := 0
	var dump []byte
	var reverb *musicReverb
	for pos := 0; pos < s.totalFrames; {
		frames := min(chunkFrames, s.totalFrames-pos)
		left, right := make([]float32, frames), make([]float32, frames)
		for _, renderer := range renderers {
			if renderer.remaining() == 0 {
				continue
			}
			partLeft, partRight, err := renderer.render(min(frames, renderer.remaining()))
			if err != nil {
				s.setRenderError(err)
				return
			}
			for i := range partLeft {
				left[i] += partLeft[i]
				right[i] += partRight[i]
			}
		}
		if enhancement {
			if reverb == nil {
				reverb = newMusicReverb(sampleRate)
			}
			reverb.Process(left, right, enhancementAmount)
		}
		pos += frames
		pcm := mixPCMChunk(left, right, pos == s.totalFrames)
		if dumpMusic {
			dump = append(dump, pcm...)
		}
		select {
		case s.chunks <- pcm:
			buffered++
			if buffered == s.bufferChunks {
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

func (s *musicStream) signalExhausted() {
	if s.exhausted != nil {
		s.exhaustOnce.Do(func() { close(s.exhausted) })
	}
}

func (s *musicStream) isExhausted() bool {
	if s.exhausted == nil {
		return false
	}
	select {
	case <-s.exhausted:
		return true
	default:
		return false
	}
}

func (s *musicStream) setRenderError(err error) {
	s.mu.Lock()
	s.err = err
	s.mu.Unlock()
}

func (s *musicStream) renderError() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.err
}

// Read fills the player's request across as many rendered chunks as needed.
// The producer blocks when its five-chunk queue is full and resumes as soon as
// this consumes data, keeping both the render queue and player buffer topped up.
func (s *musicStream) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}

	n := 0
	for n < len(p) {
		if len(s.data) > 0 {
			copied := copy(p[n:], s.data)
			s.data = s.data[copied:]
			n += copied
			continue
		}

		select {
		case data, ok := <-s.chunks:
			if !ok {
				s.signalExhausted()
				// Keep renderer failures out of the shared audio backend. The
				// playback owner observes renderError and reports it separately.
				return n, io.EOF
			}
			s.data = data
		case <-s.done:
			return n, io.EOF
		}
	}
	return n, nil
}

func (s *musicStream) Close() error {
	s.lifecycleMu.Lock()
	s.closed = true
	s.once.Do(func() { close(s.done) })
	s.lifecycleMu.Unlock()
	return nil
}

func (s *musicStream) setPaused(paused bool) {
	s.lifecycleMu.Lock()
	s.paused = paused
	s.lifecycleMu.Unlock()
}

func (s *musicStream) isPaused() bool {
	s.lifecycleMu.Lock()
	defer s.lifecycleMu.Unlock()
	return s.paused
}

func (s *musicStream) waitForProducer() {
	if s.producerDone != nil {
		<-s.producerDone
	}
}

func (s *musicStream) playIfOpen(player musicPlaybackPlayer) bool {
	s.lifecycleMu.Lock()
	open := !s.closed
	s.lifecycleMu.Unlock()
	if !open {
		return false
	}
	// The playback owner is the only goroutine that closes player. A stop may
	// close the stream after the check, but Play then observes clean EOF rather
	// than racing a concurrent Player.Close call.
	player.Play()
	return true
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

// Play starts an independently buffered music stream. Rendering is done in
// one-second increments, allowing several bards to play together.
func Play(ctx *audio.Context, program int, notes []Note) error {
	return playMusic(ctx, program, notes, 0, nil, nil)
}

// prepared is called after the configured initial buffer is rendered. start
// gates playback so a /with group can begin its independently rendered tracks
// at the same instant.
func playMusic(ctx *audio.Context, program int, notes []Note, who int, prepared func(), start <-chan struct{}) error {
	return playMusicGroup(ctx, []musicPart{{program: program, notes: notes}}, []int{who}, prepared, start)
}

type musicPlaybackPlayer interface {
	Play()
	IsPlaying() bool
	Position() time.Duration
}

type musicPlaybackMonitor struct {
	duration     time.Duration
	lastPosition time.Duration
	lastProgress time.Time
	lastResume   time.Time
	seenPlaying  bool
}

func newMusicPlaybackMonitor(now time.Time, duration time.Duration) *musicPlaybackMonitor {
	return &musicPlaybackMonitor{duration: duration, lastProgress: now}
}

// update decides whether playback is complete, needs a resume attempt, or has
// remained stopped long enough to report an error. Wall-clock song duration is
// deliberately not a completion signal: device buffers and audio suspension
// can leave valid PCM unplayed after that much real time has elapsed.
func (monitor *musicPlaybackMonitor) update(now time.Time, playing bool, position time.Duration, exhausted bool, renderErr error) (complete, resume bool, err error) {
	if position < 0 {
		position = 0
	}
	if position > monitor.lastPosition {
		monitor.lastPosition = position
		monitor.lastProgress = now
	}
	if playing {
		if !monitor.seenPlaying {
			monitor.lastProgress = now
		}
		monitor.seenPlaying = true
		return false, false, nil
	}
	if exhausted && renderErr != nil {
		return false, false, fmt.Errorf("music rendering stopped: %w", renderErr)
	}
	if exhausted && position+musicPlaybackEndSlop >= monitor.duration {
		return true, false, nil
	}
	if now.Sub(monitor.lastProgress) >= musicPlaybackStall {
		return false, false, fmt.Errorf("music player stopped at %s of %s", position.Round(time.Millisecond), monitor.duration.Round(time.Millisecond))
	}
	if monitor.lastResume.IsZero() || now.Sub(monitor.lastResume) >= musicPlaybackResumeEvery {
		monitor.lastResume = now
		return false, true, nil
	}
	return false, false, nil
}

func waitForMusicPlayback(player musicPlaybackPlayer, stream *musicStream, duration time.Duration) error {
	monitor := newMusicPlaybackMonitor(time.Now(), duration)
	ticker := time.NewTicker(musicPlaybackPoll)
	defer ticker.Stop()
	recoveryLogged := false

	for {
		select {
		case <-stream.done:
			return nil
		case now := <-ticker.C:
			if stream.isPaused() {
				monitor.lastProgress = now
				monitor.lastResume = time.Time{}
				continue
			}
			complete, resume, err := monitor.update(now, player.IsPlaying(), player.Position(), stream.isExhausted(), stream.renderError())
			if err != nil {
				return err
			}
			if complete {
				return nil
			}
			if resume {
				if monitor.seenPlaying && !recoveryLogged {
					log.Printf("music player stopped before its buffered audio ended; attempting to resume")
					recoveryLogged = true
				}
				stream.playIfOpen(player)
			}
		}
	}
}

func playMusicGroup(ctx *audio.Context, parts []musicPart, whos []int, prepared func(), start <-chan struct{}) error {
	return playMusicGroupWithSettings(ctx, parts, whos, prepared, start, currentMusicPlaybackSettings())
}

func playMusicGroupWithSettings(ctx *audio.Context, parts []musicPart, whos []int, prepared func(), start <-chan struct{}, settings musicPlaybackSettings) error {
	return playMusicGroupWithSettingsAtFrame(ctx, parts, whos, prepared, start, settings, 0)
}

func playMusicGroupWithSettingsAtFrame(ctx *audio.Context, parts []musicPart, whos []int, prepared func(), start <-chan struct{}, settings musicPlaybackSettings, startFrame int) error {
	return playMusicGroupWithSettingsAtFrameIf(ctx, parts, whos, prepared, start, settings, startFrame, nil)
}

// playMusicGroupWithSettingsAtFrameIf discards a prepared stream when valid
// reports that the movie seek which requested it has been superseded.
func playMusicGroupWithSettingsAtFrameIf(ctx *audio.Context, parts []musicPart, whos []int, prepared func(), start <-chan struct{}, settings musicPlaybackSettings, startFrame int, valid func() bool) error {

	if ctx == nil {
		return errors.New("nil audio context")
	}

	if !settings.enabled {
		return errors.New("music muted")
	}

	stream, err := newMixedMusicStreamWithSettingsAtFrame(parts, settings, startFrame)
	if err != nil {
		if prepared != nil {
			prepared()
		}
		return err
	}
	if valid != nil && !valid() {
		_ = stream.Close()
		stream.waitForProducer()
		if prepared != nil {
			prepared()
		}
		return nil
	}
	player, err := ctx.NewPlayer(stream)
	if err != nil {
		_ = stream.Close()
		stream.waitForProducer()
		if prepared != nil {
			prepared()
		}
		return err
	}
	player.SetBufferSize(musicPlayerBuffer)

	player.SetVolume(settings.volume)

	musicPlayersMu.Lock()
	trackWhos := make(map[int]struct{}, len(whos))
	for _, who := range whos {
		trackWhos[who] = struct{}{}
	}
	musicPlayers[player] = musicTrack{stream: stream, whos: trackWhos, ctx: ctx, parts: parts, startFrame: startFrame}
	musicPlayersMu.Unlock()
	defer func() {
		_ = stream.Close()
		stream.waitForProducer()
		musicPlayersMu.Lock()
		delete(musicPlayers, player)
		musicPlayersMu.Unlock()
		_ = player.Close()
	}()

	if prepared != nil {
		prepared()
	}
	if valid != nil && !valid() {
		return nil
	}
	if start != nil {
		select {
		case <-start:
		case <-stream.done:
			return nil
		}
	}
	if movieMode && movieMusicPaused.Load() {
		stream.setPaused(true)
	} else {
		stream.playIfOpen(player)
	}

	playDuration := time.Duration(stream.totalFrames) * time.Second / sampleRate
	return waitForMusicPlayback(player, stream, playDuration)
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
