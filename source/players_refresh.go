package main

import (
	"time"
)

// players maintenance state machine handling /be-who, /be-share and /be-info.

// internal phases
const (
	phaseWho = iota
	phaseShare
	phaseInfo
)

var (
	playersPhase   = phaseWho
	playersLastCmd time.Time
	whoRequested   bool
)

// requestPlayersData progresses the maintenance state machine.
func requestPlayersData() {
	now := time.Now()

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
