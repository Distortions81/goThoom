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
	primarySession.players.requestData(primarySession.commands)
}
