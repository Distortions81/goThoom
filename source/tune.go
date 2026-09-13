package main

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	defaultInstrument      = 0
	durationBlack          = 2.0
	durationWhite          = 4.0
	defaultChordDuration   = 2.0
	classicInstrumentCount = 23
	orgaDrumNoteClasses    = 1<<7 | 1<<11 // G and B in every allowed octave.
)

// instruments holds the instrument table extracted from the classic client.
// Each entry keeps the resource's one-based General MIDI number separate from
// the zero-based program sent to the synthesizer.
var instruments = []instrument{
	// resource program, octave, chord%, melody%, longChord, hasChords, hasMelody, polyphony, allowed note classes
	classicInstrument(47, 1, 100, 100, false, true, true, 6, 0),                      // 0 Lucky Lyra
	classicInstrument(73, 1, 100, 100, false, false, true, 0, 0),                     // 1 Bone Flute (melody only)
	classicInstrument(47, 0, 100, 100, false, true, true, 10, 0),                     // 2 Starbuck Harp
	classicInstrument(106, 0, 100, 100, false, true, true, 6, 0),                     // 3 Torjo
	classicInstrument(13, 0, 100, 100, false, true, true, 6, 0),                      // 4 Xylo
	classicInstrument(25, 0, 100, 100, false, true, true, 6, 0),                      // 5 Gitor
	classicInstrument(76, 1, 100, 100, false, false, true, 0, 0),                     // 6 Reed Flute (melody only)
	classicInstrument(17, -1, 100, 100, true, true, true, 10, 0),                     // 7 Temple Organ (longChord)
	classicInstrument(94, -1, 100, 100, true, true, true, 1, 0),                      // 8 Conch (longChord)
	classicInstrument(80, 1, 100, 100, false, false, true, 0, 0),                     // 9 Ocarina (melody only)
	classicInstrument(77, 1, 100, 100, true, true, true, 6, 0),                       // 10 Centaur Organ (Bottle Blow, matching classic)
	classicInstrument(12, 0, 100, 100, false, true, true, 6, 0),                      // 11 Vibra
	classicInstrument(59, -1, 100, 100, false, false, true, 0, 0),                    // 12 Tuborn (melody only)
	classicInstrument(110, 0, 100, 100, true, true, true, 3, 0),                      // 13 Bagpipe (longChord)
	classicInstrument(117, -1, 100, 100, false, false, true, 0, orgaDrumNoteClasses), // 14 Orga Drum (melody only; G/B only)
	classicInstrument(115, 0, 100, 100, false, true, true, 4, 0),                     // 15 Casserole
	classicInstrument(41, 1, 100, 100, false, true, true, 2, 0),                      // 16 Violène
	classicInstrument(78, 1, 100, 100, false, false, true, 0, 0),                     // 17 Pine Flute (melody only)
	classicInstrument(22, -1, 100, 100, true, true, true, 6, 0),                      // 18 Groanbox (longChord)
	classicInstrument(108, -1, 100, 100, false, true, true, 3, 0),                    // 19 Gho-To
	classicInstrument(44, -2, 100, 100, false, true, true, 2, 0),                     // 20 Mammoth Violène
	classicInstrument(33, -2, 100, 100, false, false, true, 0, 0),                    // 21 Gutbucket Bass (melody only)
	classicInstrument(77, 0, 100, 100, false, false, true, 1, 0),                     // 22 Glass Jug (melody only)
}

// instrument describes a playable instrument mapping Clan Lord's instrument
// index to a General MIDI program number, octave offset, and velocity scaling
// factors for chords and melodies.
type instrument struct {
	classicProgram   int // one-based General MIDI number from the classic resource
	program          int // zero-based General MIDI program sent to the synthesizer
	octave           int
	chord            int    // chord velocity factor (0-100)
	melody           int    // melody velocity factor (0-100)
	longChord        bool   // supports long-chord sustain ('$')
	hasChords        bool   // instrument can play chords
	hasMelody        bool   // instrument can play melody
	polyphony        int    // resource polyphony; effective polyphony is zero without chords
	allowedNoteClass uint16 // MIDI pitch-class bitset; zero permits every class
}

func classicInstrument(program, octave, chord, melody int, longChord, hasChords, hasMelody bool, polyphony int, allowedNoteClass uint16) instrument {
	return instrument{
		classicProgram:   program,
		program:          program - 1,
		octave:           octave,
		chord:            chord,
		melody:           melody,
		longChord:        longChord,
		hasChords:        hasChords,
		hasMelody:        hasMelody,
		polyphony:        polyphony,
		allowedNoteClass: allowedNoteClass,
	}
}

type tuneJob struct {
	program  int
	notes    []Note
	who      int
	debug    bool
	parseErr *tuneParseError
}

// processMusicRequests remains a main-loop hook. Playback failures and song
// stops must never alter the user's Music mixer preference.
func processMusicRequests() {}

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

// playClanLordTune decodes a Clan Lord music string and plays it using the
// music package. The tune may optionally begin with an instrument index.
// For example: "3 cde" plays on instrument #3. It returns any playback error.
func playClanLordTune(tune string) error {
	return playSessionClanLordTune(primarySession, tune)
}

func playSessionClanLordTune(session *Session, tune string) error {
	if audioContext == nil {
		return fmt.Errorf("audio disabled")
	}
	if blockMusic {
		return fmt.Errorf("music blocked")
	}
	if gs.Mute || focusMuted || !gs.Music || gs.MasterVolume <= 0 || gs.MusicVolume <= 0 {
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
	ns, parseErr := parseClassicTune(tune, instruments[inst], 120, 100)
	if parseErr != nil {
		return parseErr
	}
	if len(ns) == 0 {
		return fmt.Errorf("empty tune")
	}
	prog := instruments[inst].program

	enqueueSessionTune(session, tuneJob{program: prog, notes: ns})
	return nil
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
	debug  bool
}

// Internal state for assembling multipart songs.
type pendingSong struct {
	inst    int
	tempo   int
	volPct  int
	notes   []string
	withIDs []int
	ready   bool
	touched time.Time
}

const musicPartTimeout = 20 * time.Second

type sessionMusicState struct {
	mu          sync.Mutex
	pendingByID map[int]*pendingSong
}

func newSessionMusicState() *sessionMusicState {
	return &sessionMusicState{pendingByID: make(map[int]*pendingSong)}
}

func (s *sessionMusicState) reset() {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.pendingByID = make(map[int]*pendingSong)
	s.mu.Unlock()
}

var (
	// musicCommandNow uses wall time during live play. Movie indexing replaces
	// it temporarily with the recording's fixed-UPS timeline so the classic
	// client's multipart timeout remains meaningful during a fast scan.
	musicCommandNow = time.Now

	// movieMusicIndexCapture is set only while a movie is scanned for music
	// starts. It receives fully assembled jobs, including /part and /with
	// groups, without starting audio during the scan.
	movieMusicIndexCapture func([]tuneJob)
	movieMusicIndexStop    func(int)
)

// handleMusicParams translates parsed music params into queued playback. It
// supports /stop, /part accumulation and tempo/volume/instrument parameters.
func handleMusicParams(mp MusicParams) {
	handleSessionMusicParams(primarySession, mp)
}

func handleSessionMusicParams(session *Session, mp MusicParams) {
	if session == nil || session.music == nil {
		return
	}
	state := session.music
	now := musicCommandNow()
	// The classic client runs its idle purge before every music command.
	state.mu.Lock()
	for who, song := range state.pendingByID {
		if !song.touched.IsZero() && now.Sub(song.touched) > musicPartTimeout {
			delete(state.pendingByID, who)
		}
	}
	state.mu.Unlock()

	if mp.Stop {
		// Scoped stop: if who provided, clear that pending and stop if playing.
		if mp.Who != 0 {
			state.mu.Lock()
			delete(state.pendingByID, mp.Who)
			state.mu.Unlock()
			if movieMusicIndexStop != nil {
				movieMusicIndexStop(mp.Who)
				return
			}
			routeSessionMusic(session, func() {
				stopMusicFor(mp.Who)
			})
		} else {
			// Global stop
			state.reset()
			if movieMusicIndexStop != nil {
				movieMusicIndexStop(0)
				return
			}
			routeSessionMusic(session, func() {
				stopAllMusic()
				clearTuneQueue()
			})
		}
		return
	}
	if blockMusic {
		return
	}
	// Ignore play requests while muted, matching classic behavior when sound
	// is off. Still handled /stop above regardless of mute state.
	if movieMusicIndexCapture == nil && (gs.Mute || focusMuted || !gs.Music || gs.MasterVolume <= 0 || gs.MusicVolume <= 0) {
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
		state.mu.Lock()
		ps := state.pendingByID[id]
		if ps == nil {
			ps = &pendingSong{inst: mp.Inst, tempo: mp.Tempo, volPct: mp.VolPct, touched: now}
			state.pendingByID[id] = ps
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
		ps.touched = now
		state.mu.Unlock()
		return
	}

	// Finalize: merge any pending parts, then queue a single tune.
	inst := mp.Inst
	tempo := mp.Tempo
	vol := mp.VolPct
	notes := strings.TrimSpace(mp.Notes)
	state.mu.Lock()
	hadPending := false
	if ps := state.pendingByID[id]; ps != nil {
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
		delete(state.pendingByID, id)
		hadPending = true
	}
	// If sync requested via /with, require that all referenced IDs also have
	// pending content; otherwise, store this song and return until ready.
	if len(mp.With) > 0 {
		// Save current as pending with its group
		p := &pendingSong{inst: inst, tempo: tempo, volPct: vol, notes: []string{notes}, withIDs: append([]int(nil), mp.With...), ready: true, touched: now}
		state.pendingByID[id] = p
		// Deduplicate and sort the requested group IDs.
		idmap := map[int]struct{}{}
		for _, w := range append([]int{id}, mp.With...) {
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
		for _, w := range ids {
			song := state.pendingByID[w]
			if song == nil || !song.ready {
				state.mu.Unlock()
				return
			}
		}
		// All parts present: build jobs in sorted order.
		jobs := make([]tuneJob, 0, len(ids))
		for _, w := range ids {
			ps := state.pendingByID[w]
			nstr := strings.Join(ps.notes, " ")
			jobs = append(jobs, makeTuneJob(w, ps.inst, ps.tempo, ps.volPct, nstr, mp.debug))
			delete(state.pendingByID, w)
		}
		state.mu.Unlock()
		// Enqueue jobs sequentially
		// Clear any queued previous jobs so the synchronized set starts cleanly.
		clearTuneQueue()
		enqueueSessionTunes(session, jobs)
		return
	}
	state.mu.Unlock()
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
	job := makeTuneJob(id, inst, tempo, vol, notes, mp.debug)
	enqueueSessionTune(session, job)
	if mp.debug {
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

func makeTuneJob(who, inst, tempo, vol int, notes string, debug bool) tuneJob {
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
	notesOut, parseErr := parseClassicTune(notes, instData, tempo, vel)
	return tuneJob{program: prog, notes: notesOut, who: who, debug: debug, parseErr: parseErr}
}

func enqueueTune(job tuneJob) {
	enqueueSessionTune(primarySession, job)
}

func enqueueSessionTune(session *Session, job tuneJob) {
	enqueueSessionTunes(session, []tuneJob{job})
}

// enqueueTunes renders a /with group into one buffered player so every bard is
// mixed and started together, even on audio backends that do not reliably
// start several new players at once.
func enqueueTunes(jobs []tuneJob) {
	enqueueSessionTunes(primarySession, jobs)
}

func enqueueSessionTunes(session *Session, jobs []tuneJob) {
	if len(jobs) == 0 {
		return
	}
	if movieMusicIndexCapture != nil {
		for _, job := range jobs {
			if job.parseErr != nil {
				reportTuneParseError(job.who, job.parseErr)
				return
			}
		}
		captured := append([]tuneJob(nil), jobs...)
		movieMusicIndexCapture(captured)
		return
	}
	routeSessionMusic(session, func() {
		for _, job := range jobs {
			if job.parseErr != nil {
				reportTuneParseError(job.who, job.parseErr)
				return
			}
		}
		soundMu.Lock()
		context := audioContext
		soundMu.Unlock()
		settings := currentMusicPlaybackSettings()
		go func() {
			if context == nil {
				log.Printf("play tune: audio disabled")
				return
			}
			parts := make([]musicPart, 0, len(jobs))
			whos := make([]int, 0, len(jobs))
			debug := false
			for _, job := range jobs {
				parts = append(parts, musicPart{program: job.program, notes: job.notes})
				whos = append(whos, job.who)
				debug = debug || job.debug
			}
			if movieMode {
				parts = scaleMusicParts(parts, currentMovieMusicTempoRate())
			}
			if err := playMusicGroupWithSettings(context, parts, whos, nil, nil, settings); err != nil {
				log.Printf("play tune: %v", err)
				if debug {
					consoleMessage("play tune: " + err.Error())
					chatMessage("play tune: " + err.Error())
				}
			}
		}()
	})
}

func reportTuneParseError(who int, parseErr *tuneParseError) {
	if parseErr == nil {
		return
	}
	message := "* " + parseErr.Error() + "."
	log.Printf("play tune for %d at byte %d: %s", who, parseErr.Position, parseErr.Error())
	consoleMessage(message)
	chatMessage(message)
}

// clearTuneQueue remains for message compatibility. Tunes start independently,
// so there is no serial queue to drain.
func clearTuneQueue() {
}
