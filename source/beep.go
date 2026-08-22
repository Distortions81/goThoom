package main

import (
	"sync"
	"time"
)

type beepSpec struct {
	program int
	key     int
}

var (
    beepMu    sync.Mutex
	beepCache = make(map[beepSpec][]byte)
)

// focusMuted gates audio when window is unfocused and user enabled it.
var focusMuted bool

// playBeep renders and plays a short note using the given program and key.
// The note is cached after the first render.
func playBeep(program, key int) {
    if gs.Mute || focusMuted || !gs.GameSound || audioContext == nil {
		return
	}

	spec := beepSpec{program: program, key: key}
	beepMu.Lock()
	pcm, ok := beepCache[spec]
	beepMu.Unlock()
	if !ok {
		notes := []Note{{Key: key, Velocity: 120, Start: 0, Duration: 200 * time.Millisecond}}
		left, right, err := renderSong(program, notes)
		if err != nil {
			return
		}
		pcm = mixPCM(left, right)
		beepMu.Lock()
		beepCache[spec] = pcm
		beepMu.Unlock()
	}

	p := audioContext.NewPlayerFromBytes(pcm)
	vol := gs.MasterVolume * gs.NotificationVolume
    if gs.Mute || focusMuted {
		vol = 0
	}
	p.SetVolume(vol)

	soundMu.Lock()
	for sp := range soundPlayers {
		if !sp.IsPlaying() {
			sp.Close()
			delete(soundPlayers, sp)
		}
	}
	if maxSounds > 0 && len(soundPlayers) >= maxSounds {
		soundMu.Unlock()
		logDebug("playBeep too many sound players (%d)", len(soundPlayers))
		p.Close()
		return
	}
	soundPlayers[p] = struct{}{}
	soundMu.Unlock()

	notifPlayersMu.Lock()
	notifPlayers[p] = struct{}{}
	notifPlayersMu.Unlock()

	p.Play()
}

// playHarpNotes renders and plays a short harp sequence using the provided
// MIDI key values. Notes are spaced evenly.
func playHarpNotes(keys ...int) {
    if gs.Mute || focusMuted || !gs.GameSound || audioContext == nil {
		return
	}
	if len(keys) == 0 {
		return
	}

	notes := make([]Note, len(keys))
	dur := 150 * time.Millisecond
	for i, k := range keys {
		notes[i] = Note{Key: k, Velocity: 120, Start: time.Duration(i) * dur, Duration: dur}
	}
	left, right, err := renderSong(46, notes)
	if err != nil {
		return
	}
	pcm := mixPCM(left, right)
	p := audioContext.NewPlayerFromBytes(pcm)
	vol := gs.MasterVolume * gs.NotificationVolume
    if gs.Mute || focusMuted {
		vol = 0
	}
	p.SetVolume(vol)

	soundMu.Lock()
	for sp := range soundPlayers {
		if !sp.IsPlaying() {
			sp.Close()
			delete(soundPlayers, sp)
		}
	}
	if maxSounds > 0 && len(soundPlayers) >= maxSounds {
		soundMu.Unlock()
		logDebug("playHarpNotes too many sound players (%d)", len(soundPlayers))
		p.Close()
		return
	}
	soundPlayers[p] = struct{}{}
	soundMu.Unlock()

	notifPlayersMu.Lock()
	notifPlayers[p] = struct{}{}
	notifPlayersMu.Unlock()

	p.Play()
}
