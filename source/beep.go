package main

import (
	_ "embed"
	"encoding/binary"
	"sync"
	"time"
)

//go:embed data/audio/notification_default.wav
var notificationDefaultWAV []byte

//go:embed data/audio/notification_mention.wav
var notificationMentionWAV []byte

//go:embed data/audio/notification_fallen.wav
var notificationFallenWAV []byte

//go:embed data/audio/notification_recovered.wav
var notificationRecoveredWAV []byte

//go:embed data/audio/notification_online.wav
var notificationOnlineWAV []byte

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
		pcm = embeddedBeepPCM(program, key)
		ok = len(pcm) > 0
	}
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
	p.SetVolume(effectiveAudioVolume(vol))

	soundMu.Lock()
	pruneStoppedSoundPlayersLocked()
	if maxSounds > 0 && len(soundPlayers) >= maxSounds {
		soundMu.Unlock()
		logDebug("playBeep too many sound players (%d)", len(soundPlayers))
		p.Close()
		return
	}
	soundPlayers[p] = struct{}{}
	reservedSoundPlayers[p] = struct{}{}
	soundMu.Unlock()

	notifPlayersMu.Lock()
	notifPlayers[p] = struct{}{}
	notifPlayersMu.Unlock()

	p.Play()
	soundMu.Lock()
	delete(reservedSoundPlayers, p)
	soundMu.Unlock()
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

	pcm := embeddedHarpPCM(keys)
	if len(pcm) == 0 {
		notes := make([]Note, len(keys))
		dur := 150 * time.Millisecond
		for i, k := range keys {
			notes[i] = Note{Key: k, Velocity: 120, Start: time.Duration(i) * dur, Duration: dur}
		}
		left, right, err := renderSong(46, notes)
		if err != nil {
			return
		}
		pcm = mixPCM(left, right)
	}
	p := audioContext.NewPlayerFromBytes(pcm)
	vol := gs.MasterVolume * gs.NotificationVolume
	if gs.Mute || focusMuted {
		vol = 0
	}
	p.SetVolume(effectiveAudioVolume(vol))

	soundMu.Lock()
	pruneStoppedSoundPlayersLocked()
	if maxSounds > 0 && len(soundPlayers) >= maxSounds {
		soundMu.Unlock()
		logDebug("playHarpNotes too many sound players (%d)", len(soundPlayers))
		p.Close()
		return
	}
	soundPlayers[p] = struct{}{}
	reservedSoundPlayers[p] = struct{}{}
	soundMu.Unlock()

	notifPlayersMu.Lock()
	notifPlayers[p] = struct{}{}
	notifPlayersMu.Unlock()

	p.Play()
	soundMu.Lock()
	delete(reservedSoundPlayers, p)
	soundMu.Unlock()
}

func embeddedBeepPCM(program, key int) []byte {
	switch {
	case program == 46 && key == 60:
		return wavPCMData(notificationDefaultWAV)
	case program == 0 && key == 84:
		return wavPCMData(notificationMentionWAV)
	default:
		return nil
	}
}

func embeddedHarpPCM(keys []int) []byte {
	switch {
	case len(keys) == 3 && keys[0] == 72 && keys[1] == 69 && keys[2] == 65:
		return wavPCMData(notificationFallenWAV)
	case len(keys) == 3 && keys[0] == 60 && keys[1] == 64 && keys[2] == 67:
		return wavPCMData(notificationRecoveredWAV)
	case len(keys) == 2 && keys[0] == 84 && keys[1] == 84:
		return wavPCMData(notificationOnlineWAV)
	default:
		return nil
	}
}

func wavPCMData(wav []byte) []byte {
	if len(wav) < 44 || string(wav[:4]) != "RIFF" || string(wav[8:12]) != "WAVE" || string(wav[36:40]) != "data" {
		return nil
	}
	size := int(binary.LittleEndian.Uint32(wav[40:44]))
	if size < 0 || size > len(wav)-44 {
		return nil
	}
	return wav[44 : 44+size]
}
