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
	{47, 1, 100, 100, false, true, true, 6},    // 0 Lucky Lyra
	{73, 1, 100, 100, false, false, true, 0},   // 1 Bone Flute (melody only)
	{46, 0, 100, 100, false, true, true, 6},    // 2 Starbuck Harp
	{106, 0, 100, 100, false, true, true, 6},   // 3 Torjo
	{13, 0, 100, 100, false, true, true, 6},    // 4 Xylo
	{25, 0, 100, 100, false, true, true, 6},    // 5 Gitor
	{76, 1, 100, 100, false, false, true, 0},   // 6 Reed Flute (melody only)
	{17, -1, 100, 100, true, true, true, 6},    // 7 Temple Organ (longChord)
	{94, -1, 100, 100, true, true, true, 6},    // 8 Conch (longChord)
	{79, 1, 100, 100, false, false, true, 0},   // 9 Ocarina (melody only)
	{19, 1, 100, 100, true, true, true, 6},     // 10 Centaur Organ (longChord)
	{11, 0, 100, 100, false, true, true, 6},    // 11 Vibra
	{59, -1, 100, 100, false, false, true, 0},  // 12 Tuborn (melody only)
	{109, 0, 100, 100, true, true, true, 6},    // 13 Bagpipe (longChord)
	{117, -1, 100, 100, false, false, true, 0}, // 14 Orga Drum (melody only; G/B only)
	{115, 0, 100, 100, false, true, true, 6},   // 15 Casserole
	{41, 1, 100, 100, false, true, true, 6},    // 16 Violène
	{78, 1, 100, 100, false, false, true, 0},   // 17 Pine Flute (melody only)
	{22, -1, 100, 100, true, true, true, 6},    // 18 Groanbox (longChord)
	{108, -1, 100, 100, false, true, true, 6},  // 19 Gho-To
	{44, -2, 100, 100, false, true, true, 6},   // 20 Mammoth Violène
	{33, -2, 100, 100, false, false, true, 0},  // 21 Gutbucket Bass (melody only)
	{76, 0, 100, 100, false, false, true, 0},   // 22 Glass Jug (melody only)
	{17, -1, 100, 100, true, true, true, 6},    // 23 Vibra Sustained (Temple Organ substitute) (longChord)
	{19, -1, 100, 100, true, true, true, 6},    // 24 Church Organ (strong sustain) (longChord)
	{48, 0, 100, 100, false, true, true, 6},    // 25 String Ensemble 1 (soft sustain)
	{49, 0, 100, 100, false, true, true, 6},    // 26 String Ensemble 2 (brighter sustain)
	{52, 0, 100, 100, false, true, true, 6},    // 27 Choir Aahs (vocal sustain)
	{89, 0, 100, 100, true, true, true, 6},     // 28 Warm Pad (synth pad sustain) (allow long)
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

type tuneJob struct {
	program int
	notes   []Note
	who     int
	debug   bool
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
	ns := classicNotesFromTune(tune, instruments[inst], 120, 100)
	if len(ns) == 0 {
		return fmt.Errorf("empty tune")
	}
	prog := instruments[inst].program

	enqueueTune(tuneJob{program: prog, notes: ns})
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
			stopMusicFor(mp.Who)
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
	if gs.Mute || focusMuted || !gs.Music || gs.MasterVolume <= 0 || gs.MusicVolume <= 0 {
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
			jobs = append(jobs, makeTuneJob(w, ps.inst, ps.tempo, ps.volPct, nstr, mp.debug))
			delete(pendingByID, w)
		}
		pendingMu.Unlock()
		// Enqueue jobs sequentially
		// Clear any queued previous jobs so the synchronized set starts cleanly.
		clearTuneQueue()
		enqueueTunes(jobs)
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
	job := makeTuneJob(id, inst, tempo, vol, notes, mp.debug)
	enqueueTune(job)
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
	notesOut := classicNotesFromTune(notes, instData, tempo, vel)
	return tuneJob{program: prog, notes: notesOut, who: who, debug: debug}
}

func enqueueTune(job tuneJob) {
	enqueueTunes([]tuneJob{job})
}

// enqueueTunes starts each track on its own renderer. A group waits until every
// track has rendered its initial five seconds, then releases all players
// together so /with bards remain synchronized.
func enqueueTunes(jobs []tuneJob) {
	if len(jobs) == 0 {
		return
	}
	start := make(chan struct{})
	var prepared sync.WaitGroup
	prepared.Add(len(jobs))
	for _, job := range jobs {
		job := job
		go func() {
			if audioContext == nil {
				prepared.Done()
				log.Printf("play tune: audio disabled")
				return
			}
			if err := playMusic(audioContext, job.program, job.notes, job.who, prepared.Done, start); err != nil {
				log.Printf("play tune: %v", err)
				if job.debug {
					consoleMessage("play tune: " + err.Error())
					chatMessage("play tune: " + err.Error())
				}
			}
		}()
	}
	go func() {
		prepared.Wait()
		close(start)
	}()
}

// clearTuneQueue remains for message compatibility. Tunes start independently,
// so there is no serial queue to drain.
func clearTuneQueue() {
}
