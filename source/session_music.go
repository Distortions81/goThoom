package main

import "sync"

// musicSourcePolicy owns the one live session allowed to feed the shared
// synthesizer. Parsed multipart state remains with every session even while it
// is not the selected source.
type musicSourcePolicy struct {
	mu     sync.RWMutex
	source SessionID
}

var (
	appMusicSource           = musicSourcePolicy{source: primarySessionID}
	stopMusicSourcePlayback  = stopAllMusic
	queueMusicSourceUIUpdate = func() {}
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

// selectMusicSource stops the current app-owned playback. It deliberately
// leaves every session's parsed timeline alone: only a future tune from the
// newly selected source may start playback.
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
	stopMusicSourcePlayback()
	appMusicSource.mu.Unlock()
	queueMusicSourceUIUpdate()
	return true
}
