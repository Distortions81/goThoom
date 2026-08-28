package main

import (
	_ "embed"
	"encoding/binary"
	"slices"
	"sync"
	"time"

	"github.com/hajimehoshi/ebiten/v2/audio"
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

const notificationSoundQueueSize = 16

type notificationSoundRequest struct {
	program    int
	key        int
	keys       []int
	context    *audio.Context
	generation uint64
	volume     float64
}

var (
	beepMu    sync.Mutex
	beepCache = make(map[beepSpec][]byte)

	notificationSoundWorkerOnce sync.Once
	notificationSoundJobs       chan notificationSoundRequest
)

// focusMuted gates audio when window is unfocused and user enabled it.
var focusMuted bool

// playBeep queues a short note using the given program and key. Notes that are
// not embedded are rendered and cached by the notification sound worker.
func playBeep(program, key int) {
	if seekingMov || gs.Mute || focusMuted || !gs.GameSound {
		return
	}
	queueNotificationSound(notificationSoundRequest{program: program, key: key})
}

func startNotificationSoundWorker() {
	notificationSoundWorkerOnce.Do(func() {
		notificationSoundJobs = make(chan notificationSoundRequest, notificationSoundQueueSize)
		go func() {
			for request := range notificationSoundJobs {
				processNotificationSound(request)
			}
		}()
	})
}

func queueNotificationSound(request notificationSoundRequest) {
	soundMu.Lock()
	request.context = audioContext
	request.generation = soundPlaybackGeneration
	soundMu.Unlock()
	if request.context == nil {
		return
	}
	request.volume = effectiveAudioVolume(gs.MasterVolume * gs.NotificationVolume)
	request.keys = slices.Clone(request.keys)
	startNotificationSoundWorker()
	if !tryQueueNotificationSound(notificationSoundJobs, request) {
		logDebug("dropping notification sound: background queue is full")
	}
}

func tryQueueNotificationSound(queue chan<- notificationSoundRequest, request notificationSoundRequest) bool {
	select {
	case queue <- request:
		return true
	default:
		return false
	}
}

func processNotificationSound(request notificationSoundRequest) {
	if !notificationSoundRequestCurrent(request) {
		return
	}

	var pcm []byte
	if len(request.keys) == 0 {
		spec := beepSpec{program: request.program, key: request.key}
		beepMu.Lock()
		pcm = beepCache[spec]
		beepMu.Unlock()
		if len(pcm) == 0 {
			pcm = embeddedBeepPCM(request.program, request.key)
		}
		if len(pcm) == 0 {
			notes := []Note{{Key: request.key, Velocity: 120, Start: 0, Duration: 200 * time.Millisecond}}
			left, right, err := renderSong(request.program, notes)
			if err != nil {
				return
			}
			pcm = mixPCM(left, right)
			beepMu.Lock()
			beepCache[spec] = pcm
			beepMu.Unlock()
		}
	} else {
		pcm = embeddedHarpPCM(request.keys)
		if len(pcm) == 0 {
			notes := make([]Note, len(request.keys))
			dur := 150 * time.Millisecond
			for i, key := range request.keys {
				notes[i] = Note{Key: key, Velocity: 120, Start: time.Duration(i) * dur, Duration: dur}
			}
			left, right, err := renderSong(46, notes)
			if err != nil {
				return
			}
			pcm = mixPCM(left, right)
		}
	}
	if len(pcm) == 0 || !notificationSoundRequestCurrent(request) {
		return
	}
	p := request.context.NewPlayerFromBytes(pcm)
	p.SetVolume(request.volume)

	soundMu.Lock()
	if request.context != audioContext || request.generation != soundPlaybackGeneration {
		soundMu.Unlock()
		_ = p.Close()
		return
	}
	pruneStoppedSoundPlayersLocked()
	if maxSounds > 0 && len(soundPlayers) >= maxSounds {
		soundMu.Unlock()
		logDebug("notification sound skipped: too many sound players")
		_ = p.Close()
		return
	}
	soundPlayers[p] = struct{}{}
	reservedSoundPlayers[p] = struct{}{}
	notifPlayersMu.Lock()
	notifPlayers[p] = struct{}{}
	notifPlayersMu.Unlock()
	p.Play()
	delete(reservedSoundPlayers, p)
	soundMu.Unlock()
}

func notificationSoundRequestCurrent(request notificationSoundRequest) bool {
	soundMu.Lock()
	current := request.context == audioContext && request.generation == soundPlaybackGeneration
	soundMu.Unlock()
	return current
}

// playHarpNotes renders and plays a short harp sequence using the provided
// MIDI key values. Notes are spaced evenly.
func playHarpNotes(keys ...int) {
	if len(keys) == 0 || seekingMov || gs.Mute || focusMuted || !gs.GameSound {
		return
	}
	queueNotificationSound(notificationSoundRequest{keys: keys})
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
