package main

import (
	"log"
	"sync"
	"time"
)

// musicSourcePolicy owns the one live session allowed to feed the shared
// synthesizer. Parsed multipart state remains with every session even while it
// is not the selected source.
type musicSourcePolicy struct {
	mu         sync.RWMutex
	source     SessionID
	generation uint64
}

var (
	appMusicSource          = musicSourcePolicy{source: primarySessionID}
	stopMusicSourcePlayback = stopAllMusic
)

func musicSourceSessionID() SessionID {
	appMusicSource.mu.RLock()
	id := appMusicSource.source
	appMusicSource.mu.RUnlock()
	return id
}

func sessionIsMusicSource(session *Session) bool {
	return session != nil && session.ID() == musicSourceSessionID()
}

func routeSessionMusic(session *Session, play func()) bool {
	if session == nil || play == nil {
		return false
	}
	appMusicSource.mu.RLock()
	defer appMusicSource.mu.RUnlock()
	if session.ID() != appMusicSource.source {
		return false
	}
	play()
	return true
}

// selectMusicSource stops the current app-owned playback while leaving every
// session's parsed timeline intact. Active tracks in the newly selected
// session are rebuilt at their current wall-clock position.
func selectMusicSource(id SessionID) bool {
	if !id.Valid() {
		return false
	}
	appMusicSource.mu.Lock()
	if appMusicSource.source == id {
		appMusicSource.mu.Unlock()
		return true
	}
	appMusicSource.source = id
	appMusicSource.generation++
	generation := appMusicSource.generation
	stopMusicSourcePlayback()
	appMusicSource.mu.Unlock()
	if appSessions != nil {
		if session, open := appSessions.session(id); open {
			restoreSessionMusic(session, generation, time.Now())
		}
	}
	return true
}

// selectSessionAudioSource makes the selected tab the sole source of live
// audio. In-flight sound and speech work from the previous tab is invalidated
// before its music is replaced.
func selectSessionAudioSource(id SessionID) bool {
	if !id.Valid() {
		return false
	}
	if musicSourceSessionID() != id {
		stopAllSounds()
		stopAllTTS()
	}
	return selectMusicSource(id)
}

func invalidateSessionMusicRestore(session *Session) {
	if session == nil {
		return
	}
	appMusicSource.mu.Lock()
	if appMusicSource.source == session.ID() {
		appMusicSource.generation++
	}
	appMusicSource.mu.Unlock()
}

func restoreSessionMusic(session *Session, generation uint64, now time.Time) {
	if session == nil || session.music == nil {
		return
	}
	tracks := session.music.activeTracks(now)
	soundMu.Lock()
	context := audioContext
	soundMu.Unlock()
	settings := currentMusicPlaybackSettings()
	if context == nil || !settings.enabled {
		return
	}
	for _, track := range tracks {
		parts := make([]musicPart, 0, len(track.jobs))
		whos := make([]int, 0, len(track.jobs))
		for _, job := range track.jobs {
			parts = append(parts, musicPart{program: job.program, notes: job.notes})
			whos = append(whos, job.who)
		}
		startFrame := sessionMusicStartFrame(track.started, now)
		go func(parts []musicPart, whos []int, startFrame int) {
			valid := func() bool {
				appMusicSource.mu.RLock()
				current := appMusicSource.source == session.ID() && appMusicSource.generation == generation
				appMusicSource.mu.RUnlock()
				return current
			}
			if err := playMusicGroupWithSettingsAtFrameIf(context, parts, whos, nil, nil, settings, startFrame, valid); err != nil {
				log.Printf("resume session music: %v", err)
			}
		}(parts, whos, startFrame)
	}
}

func sessionMusicStartFrame(started, now time.Time) int {
	if now.Before(started) {
		return 0
	}
	return int(now.Sub(started).Seconds() * sampleRate)
}
