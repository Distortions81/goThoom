package main

import (
	"time"
)

// players maintenance state machine handling /be-who, /be-share and /be-info.
// It also tracks LastSeen times to refresh the UI when players age out.

// internal phases
const (
	phaseWho = iota
	phaseShare
	phaseInfo
)

var (
	playersPhase   = phaseWho
	playersLastCmd time.Time
	playersOffline = map[string]bool{}
	whoRequested   bool
)

const playersOfflineThreshold = 5 * time.Minute

// requestPlayersData progresses the maintenance state machine and
// updates playersDirty when LastSeen crosses the offline threshold.
func requestPlayersData() {
	// Cache time once to avoid repeated runtimeNow calls.
	now := time.Now()

	// track offline transitions
	changed := false
	playersMu.RLock()
	for name, p := range players {
		// Use the cached now to avoid per-iteration time.Now()
		offline := now.Sub(p.LastSeen) > playersOfflineThreshold
		if prev, ok := playersOffline[name]; !ok || prev != offline {
			playersOffline[name] = offline
			changed = true
		}
	}
	playersMu.RUnlock()
	if changed {
		playersDirty = true
	}

	if !commandQueueIsIdle() {
		return
	}
	if now.Sub(playersLastCmd) < time.Second {
		return
	}

	switch playersPhase {
	case phaseWho:
		if whoActive {
			if maybeEnqueueWho() {
				playersLastCmd = now
			}
			return
		}
		if !whoRequested {
			if !enqueueCommandIfIdle("/be-who") {
				return
			}
			whoLastRequest = now
			playersLastCmd = now
			whoRequested = true
			return
		}
		// who scan finished
		playersPhase = phaseShare
		whoRequested = false
	case phaseShare:
		if !enqueueCommandIfIdle("/be-share") {
			return
		}
		playersLastCmd = now
		playersPhase = phaseInfo
	case phaseInfo:
		if maybeEnqueueInfo() {
			playersLastCmd = now
		} else if !whoActive {
			playersPhase = phaseWho
		}
	}
}
