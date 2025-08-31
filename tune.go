package main

import (
	"fmt"
	"log"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
)

const (
	defaultInstrument    = 0
	durationBlack        = 2.0
	durationWhite        = 4.0
	defaultChordDuration = 2.0
)

// instruments holds the instrument table extracted from the classic client.
// Each instrument defines the General MIDI program number, octave offset, and
// velocity scaling factors for chord and melody notes.
var instruments = []instrument{
    // program, octave, chord%, melody%, longChord, hasChords, hasMelody, polyphony
    {47, 1, 100, 100, false, true,  true,  6},  // 0 Lucky Lyra
    {73, 1, 100, 100, false, false, true,  0},  // 1 Bone Flute (melody only)
    {46, 0, 100, 100, false, true,  true,  6},  // 2 Starbuck Harp
    {106, 0, 100, 100, false, true,  true,  6},  // 3 Torjo
    {13, 0, 100, 100, false, true,  true,  6},  // 4 Xylo
    {25, 0, 100, 100, false, true,  true,  6},  // 5 Gitor
    {76, 1, 100, 100, false, false, true,  0},  // 6 Reed Flute (melody only)
    {17, -1, 100, 100, true,  true,  true,  6},  // 7 Temple Organ (longChord)
    {94, -1, 100, 100, true,  true,  true,  6},  // 8 Conch (longChord)
    {79, 1, 100, 100, false, false, true,  0},  // 9 Ocarina (melody only)
    {19, 1, 100, 100, true,  true,  true,  6},  // 10 Centaur Organ (longChord)
    {11, 0, 100, 100, false, true,  true,  6},  // 11 Vibra
    {59, -1, 100, 100, false, false, true,  0},  // 12 Tuborn (melody only)
    {109, 0, 100, 100, true,  true,  true,  6},  // 13 Bagpipe (longChord)
    {117, -1, 100, 100, false, false, true,  0},  // 14 Orga Drum (melody only; G/B only)
    {115, 0, 100, 100, false, true,  true,  6},  // 15 Casserole
    {41, 1, 100, 100, false, true,  true,  6},  // 16 Violène
    {78, 1, 100, 100, false, false, true,  0},  // 17 Pine Flute (melody only)
    {22, -1, 100, 100, true,  true,  true,  6},  // 18 Groanbox (longChord)
    {108, -1, 100, 100, false, true,  true,  6},  // 19 Gho-To
    {44, -2, 100, 100, false, true,  true,  6},  // 20 Mammoth Violène
    {33, -2, 100, 100, false, false, true,  0},  // 21 Gutbucket Bass (melody only)
    {76, 0, 100, 100, false, false, true,  0},  // 22 Glass Jug (melody only)
    {17, -1, 100, 100, true,  true,  true,  6},  // 23 Vibra Sustained (Temple Organ substitute) (longChord)
    {19, -1, 100, 100, true,  true,  true,  6},  // 24 Church Organ (strong sustain) (longChord)
    {48, 0, 100, 100, false, true,  true,  6},  // 25 String Ensemble 1 (soft sustain)
    {49, 0, 100, 100, false, true,  true,  6},  // 26 String Ensemble 2 (brighter sustain)
    {52, 0, 100, 100, false, true,  true,  6},  // 27 Choir Aahs (vocal sustain)
    {89, 0, 100, 100, true,  true,  true,  6},  // 28 Warm Pad (synth pad sustain) (allow long)
}

// instrument describes a playable instrument mapping Clan Lord's instrument
// index to a General MIDI program number, octave offset, and velocity scaling
// factors for chords and melodies.
type instrument struct {
    program   int
    octave    int
    chord     int  // chord velocity factor (0-100)
    melody    int  // melody velocity factor (0-100)
    longChord bool // supports long-chord sustain ('$')
    hasChords bool // instrument can play chords
    hasMelody bool // instrument can play melody
    polyphony int  // maximum simultaneous chord notes (classic default 6)
}

// queue sequentializes tune playback so overlapping /play commands do not
// render concurrently. Each tune section is played to completion before the
// next begins.
type tuneJob struct {
	program int
	notes   []Note
	who     int
}

var (
    tuneOnce   sync.Once
    tuneQueue  chan tuneJob
    currentMu  sync.Mutex
    currentWho int
    // musicTimingScale retained for legacy tests; runtime uses classic path.
    musicTimingScale = 0.6
    // Timestamp of most recent stop; used to impose a small start delay
    // to avoid late-arriving stops killing freshly started playback.
    lastMusicStop   time.Time
    lastMusicStopMu sync.Mutex
)

func disableMusic() {
	gs.Music = false
	settingsDirty = true
	stopAllMusic()
	clearTuneQueue()
	updateSoundVolume()
	if musicMixCB != nil {
		musicMixCB.Checked = false
	}
	if musicMixSlider != nil {
		musicMixSlider.Disabled = true
	}
}

func startTuneWorker() {
    tuneQueue = make(chan tuneJob, 128)
    go func() {
        for job := range tuneQueue {
            if audioContext == nil {
                disableMusic()
                continue
            }
            currentMu.Lock()
            currentWho = job.who
            currentMu.Unlock()
            // Impose a small minimum delay after the last stop to avoid a
            // late stop request immediately killing a newly started player.
            const minStartDelay = 80 * time.Millisecond
            lastMusicStopMu.Lock()
            since := time.Since(lastMusicStop)
            lastMusicStopMu.Unlock()
            if since < minStartDelay {
                time.Sleep(minStartDelay - since)
            }
            if err := Play(audioContext, job.program, job.notes); err != nil {
                log.Printf("play tune worker: %v", err)
                if musicDebug {
                    consoleMessage("play tune: " + err.Error())
                    chatMessage("play tune: " + err.Error())
                }
                disableMusic()
            }
            currentMu.Lock()
            currentWho = 0
            currentMu.Unlock()
        }
    }()
}

// noteEvent represents a parsed tune event. A single event may contain multiple
// simultaneous notes (a chord). Durations are stored in quarter-beats and
// converted to milliseconds later once tempo and loop processing is applied.
type noteEvent struct {
	keys   []int
	beats  float64
	volume int
	// nogap indicates this note should not shorten its duration to leave
	// a separation gap before the next event (used for simple tie handling).
	nogap bool
	// longChord marks a chord that should sustain (no time consumed at the
	// event) until the next chord or end-of-song.
	longChord bool
}

// tempoEvent notes a tempo change occurring before the event at the given index.
type tempoEvent struct {
	index int
	tempo int
}

// loopMarker describes a looped sequence of events.
type loopMarker struct {
	start  int // index of the first event in the loop
	end    int // index after the last event in the loop
	repeat int // total number of times to play the loop
	// endings map from iteration index (1-based) to event index to jump to
	// when the loop end is reached on that iteration. If not present, def
	// is used when >= 0.
	endings map[int]int
	def     int // default ending start event index, or -1 if none
}

// parsedTune aggregates events with optional loop and tempo metadata.
type parsedTune struct {
	events []noteEvent
	tempos []tempoEvent
	loops  []loopMarker
	tempo  int // initial tempo in BPM
}

// playClanLordTune decodes a Clan Lord music string and plays it using the
// music package. The tune may optionally begin with an instrument index.
// For example: "3 cde" plays on instrument #3. It returns any playback error.
func playClanLordTune(tune string) error {
    if audioContext == nil {
        disableMusic()
        return fmt.Errorf("audio disabled")
    }
	if blockMusic {
		return fmt.Errorf("music blocked")
	}
	if gs.Mute || !gs.Music || gs.MasterVolume <= 0 || gs.MusicVolume <= 0 {
		return fmt.Errorf("music muted")
	}

    // Determine instrument prefix ("<inst> <notes>")
    inst := defaultInstrument
    fields := strings.Fields(tune)
    if len(fields) > 1 {
        if n, err := strconv.Atoi(fields[0]); err == nil && n >= 0 && n < len(instruments) {
            inst = n
            tune = strings.Join(fields[1:], " ")
        }
    }

    // Use classic parser/timing exclusively for playback parity
    ns := classicNotesFromTune(tune, instruments[inst], 120, 100)
    if len(ns) == 0 {
        return fmt.Errorf("empty tune")
    }
    prog := instruments[inst].program

	// Enqueue for sequential playback and return immediately.
	tuneOnce.Do(startTuneWorker)
	select {
    case tuneQueue <- tuneJob{program: prog, notes: ns}:
    default:
        // If the queue is full, drop the oldest by draining one then enqueue.
        // This prevents unbounded growth during bursts.
        select {
        case <-tuneQueue:
        default:
        }
        tuneQueue <- tuneJob{program: prog, notes: ns}
    }
    return nil
}

// eventsToNotes converts parsed note events into synth notes with explicit start
// times. All notes in the same event (a chord) share the same start time. The
// provided instrument's chord or melody velocity factors are applied depending
// on the event type.
func eventsToNotes(pt parsedTune, inst instrument, velocity int) []Note {
	var notes []Note
	tempo := pt.tempo
	tempoIdx := 0
	startMS := 0
	// Track an active long-chord: indices of notes to be extended until the
	// next chord (or end-of-song).
	var activeLong []int
	// Tie-merge state for single-note melodies
	lastMelIdx := -1
	lastMelKey := -1
	lastMelEndMS := 0
	prevNogap := false

	// Build map of loop starts for quick lookup
	loopMap := make(map[int][]loopMarker)
	for _, lp := range pt.loops {
		loopMap[lp.start] = append(loopMap[lp.start], lp)
	}
	type loopState struct {
		start     int
		end       int
		remaining int
		index     int // 1-based iteration index
		phase     int // 0: main body, 1: in ending segment
		endings   map[int]int
		def       int
		// set of all ending start indices for quick skipping during main body
		endingStarts map[int]struct{}
	}
	var stack []loopState
	activeLoops := make(map[int]int)

	i := 0
	for i < len(pt.events) {
		// Pre-handle loop end before processing event at this index.
		for len(stack) > 0 && i == stack[len(stack)-1].end {
			top := &stack[len(stack)-1]
			if top.phase == 0 {
				// First reach of end: jump to selected ending start for this iteration if any.
				if pos, ok := top.endings[top.index]; ok {
					i = pos
					top.phase = 1
					break
				} else if top.def >= 0 {
					i = top.def
					top.phase = 1
					break
				}
				// No ending: finalize iteration immediately
			}
			// Finalize iteration after finishing ending segment or no ending
			top.phase = 0
			if top.remaining > 0 {
				top.remaining--
				top.index++
				i = top.start
				// reset tempo to state at loop start
				tempo = pt.tempo
				tempoIdx = 0
				for tempoIdx < len(pt.tempos) && pt.tempos[tempoIdx].index <= i {
					tempo = pt.tempos[tempoIdx].tempo
					tempoIdx++
				}
				break
			} else {
				delete(activeLoops, top.start)
				stack = stack[:len(stack)-1]
				// continue to check next stacked loop end, if any
			}
		}
		// apply tempo changes at this position
		for tempoIdx < len(pt.tempos) && pt.tempos[tempoIdx].index == i {
			tempo = pt.tempos[tempoIdx].tempo
			tempoIdx++
		}

		if lps, ok := loopMap[i]; ok {
			for _, lp := range lps {
				if activeLoops[lp.start] == 0 {
					es := make(map[int]struct{})
					for _, s := range lp.endings {
						es[s] = struct{}{}
					}
					if lp.def >= 0 {
						es[lp.def] = struct{}{}
					}
					stack = append(stack, loopState{start: lp.start, end: lp.end, remaining: lp.repeat - 1, index: 1, phase: 0, endings: lp.endings, def: lp.def, endingStarts: es})
					activeLoops[lp.start] = 1
				}
			}
		}

		// In the main body of a loop, skip over any alternate-ending segments.
		if len(stack) > 0 {
			top := &stack[len(stack)-1]
			if top.phase == 0 {
				if _, ok := top.endingStarts[i]; ok {
					i = top.end
					continue
				}
			}
		}

		ev := pt.events[i]

		// If we are about to start a new chord, finalize any active long chord
		// using the current startMS as the end time.
		if len(ev.keys) > 1 && len(activeLong) > 0 {
			for _, idx := range activeLong {
				// Extend to current start time (no gap)
				end := time.Duration(startMS) * time.Millisecond
				if end > notes[idx].Start {
					notes[idx].Duration = end - notes[idx].Start
				} else {
					notes[idx].Duration = 0
				}
			}
			activeLong = activeLong[:0]
		}
		durMS := int(math.Round((ev.beats / 4) * (60000.0 / float64(tempo)) * musicTimingScale))

		if len(ev.keys) == 0 {
			// rest: advance timeline, reset tie context
			prevNogap = false
			startMS += durMS
		} else {
			gapMS := int(math.Round(1500.0 / float64(tempo)))
			if ev.nogap || (ev.longChord && inst.longChord) {
				gapMS = 0
			}
			noteMS := durMS - gapMS
			// If this is the final event and the prior event was a rest,
			// keep the full event duration for the last note to align the
			// overall timeline with Sum(durMS).
			if i == len(pt.events)-1 && !(ev.longChord && inst.longChord) {
				if i > 0 && len(pt.events[i-1].keys) == 0 {
					noteMS = durMS
				}
			}
			if noteMS < 0 {
				noteMS = 0
			}

			v := velocity
			if len(ev.keys) > 1 {
				v = v * inst.chord / 100
			} else {
				v = v * inst.melody / 100
			}
			v = int(float64(v)*math.Sqrt(float64(ev.volume)/10.0) + 0.5)
			if v < 1 {
				v = 1
			} else if v > 127 {
				v = 127
			}
			if len(ev.keys) == 1 && !(ev.longChord && inst.longChord) {
				// Single-note: allow tie merge with immediately previous melody note
				k := ev.keys[0]
				key := k + inst.octave*12
				if prevNogap && lastMelIdx >= 0 && lastMelKey == key && lastMelEndMS == startMS {
					// Extend previous note by the full event duration (no gap)
					notes[lastMelIdx].Duration += time.Duration(durMS) * time.Millisecond
					lastMelEndMS += durMS
				} else {
					notes = append(notes, Note{
						Key:      key,
						Velocity: v,
						Start:    time.Duration(startMS) * time.Millisecond,
						Duration: time.Duration(noteMS) * time.Millisecond,
					})
					lastMelIdx = len(notes) - 1
					lastMelKey = key
					lastMelEndMS = startMS + durMS
				}
			} else {
				// Chords or long-chords or multi-note events
				for _, k := range ev.keys {
					key := k + inst.octave*12
					notes = append(notes, Note{
						Key:      key,
						Velocity: v,
						Start:    time.Duration(startMS) * time.Millisecond,
						Duration: time.Duration(noteMS) * time.Millisecond,
					})
					if ev.longChord && inst.longChord {
						activeLong = append(activeLong, len(notes)-1)
					}
				}
				// Reset tie-merge context when not a single-note melody
				lastMelIdx = -1
				lastMelKey = -1
				lastMelEndMS = 0
			}
			// Long-chord consumes no time here; otherwise, advance.
			if !(ev.longChord && inst.longChord) {
				startMS += durMS
			}
			prevNogap = ev.nogap
		}
		i++
	}
	// Finalize any remaining long-chord notes at song end.
	if len(activeLong) > 0 {
		for _, idx := range activeLong {
			end := time.Duration(startMS) * time.Millisecond
			if end > notes[idx].Start {
				notes[idx].Duration = end - notes[idx].Start
			} else {
				notes[idx].Duration = 0
			}
		}
	}
	return notes
}

// parseClanLordTune converts Clan Lord music notation into parsed events at
// the default tempo of 120 BPM.
func parseClanLordTune(s string) parsedTune {
	return parseClanLordTuneWithTempo(s, 120)
}

// parseClanLordTuneWithTempo converts Clan Lord music notation into parsed
// events using the provided tempo in BPM. It also records loop markers,
// tempo changes and volume modifiers.
func parseClanLordTuneWithTempo(s string, tempo int) parsedTune {
	if tempo <= 0 {
		tempo = 120
	}
	pt := parsedTune{tempo: tempo}
	octave := 4
	volume := 10
	i := 0
	type loopBuild struct {
		start   int
		endings map[int]int
		def     int
	}
	var loopStarts []loopBuild
	for i < len(s) {
		c := s[i]
		switch c {
		case ' ', '\n', '\r', '\t':
			i++
		case '<': // comment
			for i < len(s) && s[i] != '>' {
				i++
			}
			if i < len(s) {
				i++
			}
		case '+', '-', '=', '/', '\\':
			handleOctave(&octave, c)
			i++
		case 'p': // rest
			i++

			// By default, rests use the same base length as a lowercase note
			// (durationBlack). This matches classic timing and avoids overly short
			// default rests.
			beats := durationBlack

			if i < len(s) && s[i] >= '1' && s[i] <= '9' {
				beats = float64(s[i] - '0')
				i++
			}
			pt.events = append(pt.events, noteEvent{beats: beats, volume: volume})

		case '[': // chord
			i++
			var keys []int
			for i < len(s) && s[i] != ']' {
				if handleOctave(&octave, s[i]) {
					i++
					continue
				}
				if isNoteLetter(s[i]) {
					k, _, _ := parseNoteCL(s, &i, &octave)
					if k >= 0 {
						keys = append(keys, k)
					}
					continue
				}
				i++
			}
			if i < len(s) && s[i] == ']' {
				i++
			}
			beats := defaultChordDuration
			if i < len(s) && s[i] >= '1' && s[i] <= '9' {
				beats = float64(s[i] - '0')
				i++
			}
			// Optional long-chord marker '$' after duration
			sustain := false
			if i < len(s) && s[i] == '$' {
				sustain = true
				i++
			}
			if len(keys) > 0 {
				// Always keep the parsed beats; longChord handling is gated per
				// instrument later. If unsupported, it plays as a normal chord.
				pt.events = append(pt.events, noteEvent{keys: keys, beats: beats, volume: volume, longChord: sustain})
			}
		case '(':
			i++
			loopStarts = append(loopStarts, loopBuild{start: len(pt.events), endings: make(map[int]int), def: -1})
		case ')':
			i++
			count := 1
			if i < len(s) && s[i] >= '1' && s[i] <= '9' {
				count = int(s[i] - '0')
				i++
			}
			if len(loopStarts) > 0 {
				lb := loopStarts[len(loopStarts)-1]
				loopStarts = loopStarts[:len(loopStarts)-1]
				pt.loops = append(pt.loops, loopMarker{start: lb.start, end: len(pt.events), repeat: count, endings: lb.endings, def: lb.def})
			}
		case '|', '!':
			// Alternate ending markers are only meaningful within a loop.
			if len(loopStarts) == 0 {
				i++
				// If '|' has a digit, skip it to avoid reprocessing.
				if s[i-1] == '|' && i < len(s) && s[i] >= '1' && s[i] <= '9' {
					i++
				}
				break
			}
			if s[i] == '!' {
				// default ending: current event index
				i++
				lb := &loopStarts[len(loopStarts)-1]
				lb.def = len(pt.events)
			} else { // '|'
				i++
				if i < len(s) && s[i] >= '1' && s[i] <= '9' {
					idx := int(s[i] - '0')
					i++
					lb := &loopStarts[len(loopStarts)-1]
					lb.endings[idx] = len(pt.events)
				}
			}
		case '@':
			i++
			sign := byte(0)
			if i < len(s) && (s[i] == '+' || s[i] == '-' || s[i] == '=') {
				sign = s[i]
				i++
			}
			val := 0
			for i < len(s) && s[i] >= '0' && s[i] <= '9' {
				val = val*10 + int(s[i]-'0')
				i++
			}
			// Default '@' (no value) resets to classic default 120 BPM.
			newTempo := 120
			switch sign {
			case '+':
				newTempo = tempo + val
			case '-':
				newTempo = tempo - val
			default:
				if val != 0 {
					newTempo = val
				}
			}
			if newTempo < 1 {
				newTempo = 1
			}
			tempo = newTempo
			pt.tempos = append(pt.tempos, tempoEvent{index: len(pt.events), tempo: tempo})
		case '%', '{', '}':
			cmd := c
			i++
			val := 0
			if i < len(s) && s[i] >= '1' && s[i] <= '9' {
				val = int(s[i] - '0')
				i++
			}
			switch cmd {
			case '%':
				if val == 0 {
					volume = 10
				} else {
					volume = val
				}
			case '{':
				if val == 0 {
					val = 1
				}
				volume -= val
			case '}':
				if val == 0 {
					val = 1
				}
				volume += val
			}
			if volume < 1 {
				volume = 1
			}
			if volume > 10 {
				volume = 10
			}
		default:
			if isNoteLetter(c) {
				k, beats, tie := parseNoteCL(s, &i, &octave)
				if k >= 0 {
					pt.events = append(pt.events, noteEvent{keys: []int{k}, beats: beats, volume: volume, nogap: tie})
				}
			} else {
				i++
			}
		}
	}
	return pt
}

func handleOctave(oct *int, c byte) bool {
	switch c {
	case '+':
		*oct = *oct + 1
		return true
	case '-':
		*oct = *oct - 1
		return true
	case '=':
		*oct = 4
		return true
	case '/':
		*oct = 5
		return true
	case '\\':
		*oct = 3
		return true
	}
	return false
}

// parseNoteCL parses a single note and returns its MIDI key and beat length.
func parseNoteCL(s string, i *int, octave *int) (int, float64, bool) {
	c := s[*i]
	isUpper := unicode.IsUpper(rune(c))
	base := noteOffset(unicode.ToLower(rune(c)))
	if base < 0 {
		(*i)++
		return -1, 0, false
	}
	(*i)++
	pitch := base + ((*octave)+1)*12
	beats := durationBlack
	if isUpper {
		beats = durationWhite
	}
	tied := false
	for *i < len(s) {
		ch := s[*i]
		switch {
		case ch == '#':
			pitch++
			(*i)++
		case ch == '.':
			pitch--
			(*i)++
		case ch >= '1' && ch <= '9':
			beats = float64(ch - '0')
			(*i)++
		case ch == '_':
			// Mark this note as tied to the following one. We don't merge
			// durations (full legato) here, but we do suppress the inter-note
			// gap so there is no audible rest between them.
			tied = true
			(*i)++
		default:
			return pitch, beats, tied
		}
	}
	return pitch, beats, tied
}

func noteOffset(r rune) int {
	switch r {
	case 'c':
		return 0
	case 'd':
		return 2
	case 'e':
		return 4
	case 'f':
		return 5
	case 'g':
		return 7
	case 'a':
		return 9
	case 'b':
		return 11
	}
	return -1
}

// Extended support for /music commands
type MusicParams struct {
	Inst   int
	Notes  string
	Tempo  int // BPM 60..180
	VolPct int // 0..100
	Part   bool
	Stop   bool
	Who    int
	With   []int
	Me     bool
}

// Internal state for assembling multipart songs.
type pendingSong struct {
	inst    int
	tempo   int
	volPct  int
	notes   []string
	withIDs []int
}

var (
	pendingMu   sync.Mutex
	pendingByID = make(map[int]*pendingSong)
)

// handleMusicParams translates parsed music params into queued playback. It
// supports /stop, /part accumulation and tempo/volume/instrument parameters.
func handleMusicParams(mp MusicParams) {
	if mp.Stop {
		// Scoped stop: if who provided, clear that pending and stop if playing.
		if mp.Who != 0 {
			pendingMu.Lock()
			delete(pendingByID, mp.Who)
			pendingMu.Unlock()
			currentMu.Lock()
			cw := currentWho
			currentMu.Unlock()
			if cw == mp.Who {
				stopAllMusic()
				clearTuneQueue()
			}
		} else {
			// Global stop
			pendingMu.Lock()
			pendingByID = make(map[int]*pendingSong)
			pendingMu.Unlock()
			stopAllMusic()
			clearTuneQueue()
		}
		return
	}
	if blockMusic {
		return
	}
	// Ignore play requests while muted, matching classic behavior when sound
	// is off. Still handled /stop above regardless of mute state.
	if gs.Mute || !gs.Music || gs.MasterVolume <= 0 || gs.MusicVolume <= 0 {
		return
	}
	// Validate basics
	if mp.Inst < 0 || mp.Inst >= len(instruments) {
		mp.Inst = defaultInstrument
	}
	if mp.Tempo <= 0 {
		mp.Tempo = 120
	}
	if mp.VolPct <= 0 {
		mp.VolPct = 100
	}
	id := mp.Who // 0 is the system queue

	// Accumulate multipart songs when /part is present.
	if mp.Part {
		pendingMu.Lock()
		ps := pendingByID[id]
		if ps == nil {
			ps = &pendingSong{inst: mp.Inst, tempo: mp.Tempo, volPct: mp.VolPct}
			pendingByID[id] = ps
		} else {
			if mp.Inst != 0 {
				ps.inst = mp.Inst
			}
			if mp.Tempo != 0 {
				ps.tempo = mp.Tempo
			}
			if mp.VolPct != 0 {
				ps.volPct = mp.VolPct
			}
		}
		if n := strings.TrimSpace(mp.Notes); n != "" {
			ps.notes = append(ps.notes, n)
		}
		if len(mp.With) > 0 {
			ps.withIDs = append([]int(nil), mp.With...)
		}
		pendingMu.Unlock()
		return
	}

	// Finalize: merge any pending parts, then queue a single tune.
	inst := mp.Inst
	tempo := mp.Tempo
	vol := mp.VolPct
	notes := strings.TrimSpace(mp.Notes)
	pendingMu.Lock()
	hadPending := false
	if ps := pendingByID[id]; ps != nil {
		if notes != "" {
			ps.notes = append(ps.notes, notes)
		}
		notes = strings.Join(ps.notes, " ")
		if ps.inst != 0 {
			inst = ps.inst
		}
		if ps.tempo != 0 {
			tempo = ps.tempo
		}
		if ps.volPct != 0 {
			vol = ps.volPct
		}
		if len(mp.With) == 0 && len(ps.withIDs) > 0 {
			mp.With = append([]int(nil), ps.withIDs...)
		}
		delete(pendingByID, id)
		hadPending = true
	}
	// If sync requested via /with, require that all referenced IDs also have
	// pending content; otherwise, store this song and return until ready.
	if len(mp.With) > 0 {
		// Save current as pending with its group
		p := &pendingSong{inst: inst, tempo: tempo, volPct: vol, notes: []string{notes}, withIDs: append([]int(nil), mp.With...)}
		pendingByID[id] = p
		// Check readiness of group (including self)
		all := append([]int{id}, mp.With...)
		ready := true
		for _, w := range all {
			if _, ok := pendingByID[w]; !ok {
				ready = false
				break
			}
		}
		if !ready {
			pendingMu.Unlock()
			return
		}
		// All parts present: build jobs in sorted order
		// Deduplicate and sort IDs
		idmap := map[int]struct{}{}
		for _, w := range all {
			idmap[w] = struct{}{}
		}
		ids := make([]int, 0, len(idmap))
		for w := range idmap {
			ids = append(ids, w)
		}
		// simple insertion sort
		for i := 1; i < len(ids); i++ {
			j := i
			for j > 0 && ids[j-1] > ids[j] {
				ids[j-1], ids[j] = ids[j], ids[j-1]
				j--
			}
		}
		jobs := make([]tuneJob, 0, len(ids))
		for _, w := range ids {
			ps := pendingByID[w]
			nstr := strings.Join(ps.notes, " ")
			jobs = append(jobs, makeTuneJob(w, ps.inst, ps.tempo, ps.volPct, nstr))
			delete(pendingByID, w)
		}
		pendingMu.Unlock()
		// Enqueue jobs sequentially
		// Clear any queued previous jobs so the synchronized set starts cleanly.
		clearTuneQueue()
		for _, job := range jobs {
			enqueueTune(job)
		}
		return
	}
	pendingMu.Unlock()
	if notes == "" {
		return
	}

	// If we just finalized pending parts for this id, clear any queued
	// previous jobs so the freshly assembled song starts cleanly. Avoid
	// clearing the queue for simple one-shot plays to reduce chances of
	// racing with other enqueued tunes.
	if hadPending {
		clearTuneQueue()
	}
	job := makeTuneJob(id, inst, tempo, vol, notes)
	enqueueTune(job)
    if musicDebug {
        // Classic-only debug: compute notes via classic path and dump.
        ns := classicNotesFromTune(notes, instruments[inst], tempo, 100)
        var end time.Duration
        for _, n := range ns {
            if e := n.Start + n.Duration; e > end {
                end = e
            }
        }
        log.Printf("[musicDebug] notes who=%d inst=%d tempo=%d count=%d end=%dms", id, inst, tempo, len(ns), end.Milliseconds())
        for i, n := range ns {
            log.Printf("[musicDebug] %02d key=%3d start=%6dms dur=%6dms", i, n.Key, n.Start.Milliseconds(), n.Duration.Milliseconds())
        }
    }
}

func makeTuneJob(who, inst, tempo, vol int, notes string) tuneJob {
	instData := instruments[inst]
	prog := instData.program
	// Scale 0..100 to 1..127 velocity.
	vel := vol
	if vel <= 0 {
		vel = 100
	}
	if vel > 100 {
		vel = 100
	}
	vel = int(float64(vel)*1.27 + 0.5)
	if vel < 1 {
		vel = 1
	} else if vel > 127 {
		vel = 127
	}
    notesOut := classicNotesFromTune(notes, instData, tempo, vel)
	return tuneJob{program: prog, notes: notesOut, who: who}
}

func enqueueTune(job tuneJob) {
	tuneOnce.Do(startTuneWorker)
	select {
	case tuneQueue <- job:
	default:
		select {
		case <-tuneQueue:
		default:
		}
		tuneQueue <- job
	}
}

// clearTuneQueue drains any queued tunes so newly queued items can take effect
// immediately after mute/unmute or a stop request.
func clearTuneQueue() {
	if tuneQueue == nil {
		return
	}
	for {
		select {
		case <-tuneQueue:
			// drained one
		default:
			return
		}
	}
}
