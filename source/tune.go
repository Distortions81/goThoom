package main

import (
	"fmt"
	"log"
	"slices"
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
	duration time.Duration
	program  int
	notes    []Note
	who      int
	debug    bool
	parseErr *tuneParseError
}

var (
	sessionMusicIndicators       [maxSessions]bool
	lastSessionMusicIndicatorRun time.Time
)

// processMusicRequests keeps the session tabs in sync with time-based music
// playback. Playback failures and song stops never alter the Music preference.
func processMusicRequests() {
	now := time.Now()
	if !lastSessionMusicIndicatorRun.IsZero() && now.Sub(lastSessionMusicIndicatorRun) < 100*time.Millisecond {
		return
	}
	lastSessionMusicIndicatorRun = now
	refreshSessionMusicIndicators(now)
}

func refreshSessionMusicIndicators(now time.Time) {
	if appSessions == nil {
		return
	}
	next := [maxSessions]bool{}
	for _, session := range appSessions.snapshot() {
		if session == nil || session.music == nil {
			continue
		}
		slot, ok := session.ID().Slot()
		if ok {
			next[slot] = len(session.music.activeTracks(now)) > 0
		}
	}
	if next != sessionMusicIndicators {
		sessionMusicIndicators = next
		refreshSessionTabs()
	}
}

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
	Inst      int
	Notes     string
	Tempo     int // BPM 60..180
	VolPct    int // 0..100
	Part      bool
	Stop      bool
	Who       int
	With      []int
	Me        bool
	debug     bool
	volumeSet bool // Distinguishes an explicit /vol0 from omitted Go parameters.
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

type sessionMusicTrack struct {
	started time.Time
	jobs    []tuneJob
}

const musicPartTimeout = 20 * time.Second

type sessionMusicState struct {
	mu          sync.Mutex
	bardWatch   *bardMusicWatch
	pendingByID map[int]*pendingSong
	active      []sessionMusicTrack
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
	s.active = nil
	s.bardWatch = nil
	s.mu.Unlock()
}

func (s *sessionMusicState) startTracks(jobs []tuneJob, now time.Time) {
	if s == nil || len(jobs) == 0 {
		return
	}
	whos := make(map[int]struct{}, len(jobs))
	for _, job := range jobs {
		whos[job.who] = struct{}{}
	}
	s.mu.Lock()
	kept := s.active[:0]
	for _, track := range s.active {
		for who := range whos {
			track.jobs = musicJobsWithoutPerformer(track.jobs, who)
		}
		if len(track.jobs) > 0 {
			kept = append(kept, track)
		}
	}
	s.active = append(kept, sessionMusicTrack{started: now, jobs: append([]tuneJob(nil), jobs...)})
	s.mu.Unlock()
}

func (s *sessionMusicState) stopTracks(who int) {
	if s == nil {
		return
	}
	s.mu.Lock()
	if who == 0 {
		s.active = nil
		s.mu.Unlock()
		return
	}
	kept := s.active[:0]
	for _, track := range s.active {
		track.jobs = musicJobsWithoutPerformer(track.jobs, who)
		if len(track.jobs) > 0 {
			kept = append(kept, track)
		}
	}
	s.active = kept
	s.mu.Unlock()
}

func musicJobsWithoutPerformer(jobs []tuneJob, who int) []tuneJob {
	kept := make([]tuneJob, 0, len(jobs))
	for _, job := range jobs {
		if job.who != who {
			kept = append(kept, job)
		}
	}
	return kept
}

func (s *sessionMusicState) activeTracks(now time.Time) []sessionMusicTrack {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	kept := s.active[:0]
	result := make([]sessionMusicTrack, 0, len(s.active))
	for _, track := range s.active {
		elapsed := now.Sub(track.started)
		if elapsed < 0 || !movieMusicJobsActiveAt(track.jobs, elapsed) {
			continue
		}
		kept = append(kept, track)
		result = append(result, sessionMusicTrack{started: track.started, jobs: append([]tuneJob(nil), track.jobs...)})
	}
	s.active = kept
	return result
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
	watching := false
	if !blockMusic && movieMusicIndexCapture == nil {
		watching = state.observeBardCommand(mp)
	}
	// The classic client runs its idle purge before every music command.
	state.mu.Lock()
	for who, song := range state.pendingByID {
		if !song.touched.IsZero() && now.Sub(song.touched) > musicPartTimeout {
			delete(state.pendingByID, who)
		}
	}
	state.mu.Unlock()

	if mp.Stop {
		state.stopTracks(mp.Who)
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
			state.mu.Lock()
			state.pendingByID = make(map[int]*pendingSong)
			state.mu.Unlock()
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
	// A watched Bard performance still assembles server events while muted so
	// its status remains accurate. enqueueSessionTunes keeps the audio silent.
	if !watching && movieMusicIndexCapture == nil && (gs.Mute || focusMuted || !gs.Music || gs.MasterVolume <= 0 || gs.MusicVolume <= 0) {
		return
	}
	// Validate basics
	if mp.Inst < 0 || mp.Inst >= len(instruments) {
		mp.Inst = defaultInstrument
	}
	mp.Tempo = classicCommandTempo(mp.Tempo)
	if mp.VolPct < 0 || mp.VolPct > 100 || mp.VolPct == 0 && !mp.volumeSet {
		mp.VolPct = 100
	}
	id := mp.Who // 0 is the system queue
	state.mu.Lock()
	ps := state.pendingByID[id]
	// Classic retains the first part's settings for a performer. An instrument
	// change begins a new song instead of applying that instrument to old parts.
	if ps == nil || ps.inst != mp.Inst {
		ps = &pendingSong{inst: mp.Inst, tempo: mp.Tempo, volPct: mp.VolPct}
		state.pendingByID[id] = ps
	}
	if notes := strings.TrimSpace(mp.Notes); notes != "" {
		ps.notes = append(ps.notes, notes)
	}
	ps.withIDs = append(ps.withIDs, mp.With...)
	ps.touched, ps.ready = now, !mp.Part
	if mp.Part {
		state.mu.Unlock()
		return
	}
	ids := pendingMusicGroup(state.pendingByID, id)
	for _, who := range ids {
		if song := state.pendingByID[who]; song == nil || !song.ready {
			state.mu.Unlock()
			return
		}
	}
	jobs := make([]tuneJob, 0, len(ids))
	for _, who := range ids {
		song := state.pendingByID[who]
		notes := strings.Join(song.notes, " ")
		if notes != "" {
			jobs = append(jobs, makeTuneJob(who, song.inst, song.tempo, song.volPct, notes, mp.debug))
		}
		delete(state.pendingByID, who)
	}
	state.mu.Unlock()
	enqueueSessionTunes(session, jobs)
	if mp.debug {
		for _, job := range jobs {
			var end time.Duration
			for _, n := range job.notes {
				if e := n.Start + n.Duration; e > end {
					end = e
				}
			}
			log.Printf("[musicDebug] notes who=%d program=%d count=%d end=%dms", job.who, job.program, len(job.notes), end.Milliseconds())
			for i, n := range job.notes {
				log.Printf("[musicDebug] %02d key=%3d start=%6dms dur=%6dms", i, n.Key, n.Start.Milliseconds(), n.Duration.Milliseconds())
			}
		}
	}
}

// Include both forward and reverse dependencies: a bard can finish without
// repeating /with when another pending song is already waiting for that bard.
func pendingMusicGroup(pending map[int]*pendingSong, id int) []int {
	group := map[int]bool{id: true}
	if id != 0 {
		for changed := true; changed; {
			changed = false
			for who, song := range pending {
				if who == 0 {
					continue
				}
				for _, partner := range song.withIDs {
					if partner <= 0 || !group[who] && !group[partner] {
						continue
					}
					if !group[who] || !group[partner] {
						changed = true
					}
					group[who], group[partner] = true, true
				}
			}
		}
	}
	ids := make([]int, 0, len(group))
	for who := range group {
		ids = append(ids, who)
	}
	slices.Sort(ids)
	return ids
}

func classicCommandTempo(tempo int) int {
	if tempo < 60 || tempo > 180 {
		return 120
	}
	return tempo
}

func makeTuneJob(who, inst, tempo, vol int, notes string, debug bool) tuneJob {
	instData := instruments[inst]
	prog := instData.program
	// Classic's default velocity is 100, and /vol is a percentage of it.
	if vol < 0 || vol > 100 {
		vol = 100
	}
	notesOut, duration, parseErr := parseClassicTuneTimeline(notes, instData, classicCommandTempo(tempo), vol)
	return tuneJob{program: prog, notes: notesOut, duration: duration, who: who, debug: debug, parseErr: parseErr}
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
	for _, job := range jobs {
		if job.parseErr != nil {
			reportTuneParseError(job.who, job.parseErr)
			return
		}
	}
	if session != nil && session.music != nil {
		session.music.observeBardStart(jobs, musicCommandNow())
		if gs.Mute || focusMuted || !gs.Music || gs.MasterVolume <= 0 || gs.MusicVolume <= 0 {
			return
		}
		session.music.startTracks(jobs, musicCommandNow())
	}
	routeSessionMusic(session, func() {
		for _, job := range jobs {
			stopMusicFor(job.who)
		}
		soundMu.Lock()
		context := audioContext
		soundMu.Unlock()
		settings := currentMusicPlaybackSettings()
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
		reservation := reserveMusicPlayback(whos)
		go func() {
			if context == nil {
				log.Printf("play tune: audio disabled")
				return
			}
			if err := playReservedMusicGroup(context, parts, whos, nil, nil, settings, 0, nil, reservation); err != nil {
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
