//go:build script

package main

import (
	"gt"
	"math/rand"
	"time"
)

// script metadata
const scriptName = "Prevent Idle"
const scriptAuthor = "Examples"
const scriptCategory = "Quality Of Life"
const scriptAPIVersion = 1

const maxKeepAlive = 6 // 5 * 6 = 30min

var (
	keepAliveCount = 0
	lastKeepalive  time.Time
)

func Init() {
	// Seed RNG once for random command selection.
	rand.Seed(time.Now().UnixNano())
	gt.ConsoleMsg("You have been idle for too long.", onIdleWarning)
	lastKeepalive = time.Now()
}

func onIdleWarning(msg string) {
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
	gt.Run(idleCommands[rn])
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
