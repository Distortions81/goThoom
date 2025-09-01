//go:build plugin

package main

import (
	"time"

	"gt"
)

const PluginAuthor = "Examples"
const PluginCategory = "Fun"
const PluginAPIVersion = 1
const PluginName = "Dance Macros"

// How to use:
//   - Type /dance or press Shift+D and your exile will run a short
//     sequence of /pose actions for fun screenshots.
//   - Safe to spam; it just sends a few /pose commands with short pauses.
func Init() {
	// Allow typing /dance to trigger it.
	gt.RegisterCommand("dance", danceCmd)
	// Press Shift+D to start dancing (simpler key binding).
	gt.Key("Shift-D", runDance)
}

func danceCmd(args string) { runDance() }

func runDance() {
	// A tiny routine of poses played in sequence.
	poses := []string{"celebrate", "leanleft", "leanright", "celebrate"}
	for _, p := range poses {
		gt.Run("/pose " + p)
		time.Sleep(250 * time.Millisecond)
	}
}
