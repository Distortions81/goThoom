//go:build script

package main

import (
	"time"

	"gt2"
)

const scriptAuthor = "Examples"
const scriptID = "dance"
const scriptCategory = "Fun"
const scriptAPIVersion = 2
const scriptName = "Dance Macros"

// How to use:
//   - Type /dance or press Shift+D and your exile will run a short
//     sequence of /pose actions for fun screenshots.
//   - Safe to spam; it just sends a few /pose commands with short pauses.
func Init() {
	// Allow typing /dance to trigger it.
	gt2.Command("dance", danceCmd)
	// Press Shift+D to start dancing (simpler key binding).
	gt2.Bind("Shift-D", danceHotkey)
}

func danceCmd(args string)       { runDance() }
func danceHotkey(gt2.InputEvent) { runDance() }

func runDance() {
	// A tiny routine of poses played in sequence.
	poses := []string{"celebrate", "leanleft", "leanright", "celebrate"}
	for _, p := range poses {
		gt2.Send("/pose " + p)
		gt2.Wait(250 * time.Millisecond)
	}
}
