//go:build script

package main

import (
	"time"

	"gt"
)

// "Yes Boats" – automatically whisper "yes" when a ferryman offers a ride.
//
// Notes for non‑technical players:
// - This runs when an NPC says something like "My fine boats" and mentions
//   your character by name, to avoid false triggers.
// - It replies only once every few seconds so it won’t spam.

const scriptName = "Yes Boats"
const scriptAuthor = "Example"
const scriptCategory = "Quality Of Life"
const scriptAPIVersion = 1

var (
	lastYes     time.Time         // last time we replied
	yesCooldown = 8 * time.Second // throttle window
)

func Init() {
	// React when an NPC message contains this phrase (case‑insensitive).
	gt.NPCChat("my fine boats", sendYes)
}

func sendYes(msg string) {
	// Only respond when our name appears in the message, because we will overhear other people buying boats
	me := gt.PlayerName()
	if me == "" || !gt.Includes(gt.Lower(msg), gt.Lower(me)) {
		return
	}
	now := time.Now()
	if !lastYes.IsZero() && now.Sub(lastYes) < yesCooldown {
		// Too soon since last reply: skip to avoid spam.
		return
	}
	lastYes = now
	gt.Run("/whisper yes")
}
