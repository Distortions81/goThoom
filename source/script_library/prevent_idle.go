//go:build script

package main

import (
	"gt2"
	"math/rand"
	"time"
)

// script metadata
const scriptName = "Prevent Idle"
const scriptID = "prevent-idle"
const scriptAuthor = "Examples"
const scriptCategory = "Quality Of Life"
const scriptAPIVersion = 2

const maxKeepAlive = 6 // 5 * 6 = 30min

var (
	keepAliveCount = 0
	lastKeepalive  time.Time
)

func Init() {
	// Seed RNG once for random command selection.
	rand.Seed(time.Now().UnixNano())
	gt2.OnServerMessage(gt2.ServerMessageFilter{Contains: "You have been idle for too long."}, onIdleWarning)
	lastKeepalive = time.Now()
}

func onIdleWarning(gt2.ServerMessage) {
	if time.Since(lastKeepalive) < time.Minute*4 {
		//Too soon, something is wrong
		return
	}
	if time.Since(lastKeepalive) > time.Minute*15 {
		//Its been long enough... User is not AFK reset the count
		keepAliveCount = 0
	}
	if keepAliveCount > maxKeepAlive {
		//Don't prevent disconnect forever
		return
	}
	randomIdleCommand()
	lastKeepalive = time.Now()
	keepAliveCount++
}

func randomIdleCommand() {
	n := len(idleCommands)
	if n == 0 {
		return
	}
	rn := rand.Intn(n)
	gt2.Send(idleCommands[rn])
}

var idleCommands = []string{
	"/money",
	"/options ?",
	"/help",
	"/examine",
	"/karma",
	"/news",
	"/share",
	"/who",
	"/use ?",
	"/sky",
	"/pose lie",
	"/info",
}
