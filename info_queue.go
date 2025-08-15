package main

import (
	"strings"
	"time"
)

// A tiny, throttled queue of /be-info lookups for players missing details.
var (
	infoQueue    = map[string]struct{}{}
	lastInfoSent time.Time
	infoCooldown = 500 * time.Millisecond
)

// queueInfoRequest enqueues a be-info for name when details are incomplete.
func queueInfoRequest(name string) {
	name = strings.TrimSpace(name)
	if name == "" {
		return
	}
	playersMu.RLock()
	p, ok := players[name]
	playersMu.RUnlock()
	if ok {
		if p.Class != "" && p.Gender != "" && p.Race != "" && p.Clan != "" {
			return // no need
		}
	}
	infoQueue[name] = struct{}{}
}

// maybeEnqueueInfo sets pendingCommand to "/be-info <name>" when throttled and
// a name is queued. Returns true if it queued a command.
func maybeEnqueueInfo() bool {
	if pendingCommand != "" {
		return false
	}
	if time.Since(lastInfoSent) < infoCooldown {
		return false
	}
	for name := range infoQueue {
		pendingCommand = "/be-info " + name
		delete(infoQueue, name)
		lastInfoSent = time.Now()
		return true
	}
	return false
}
