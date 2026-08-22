//go:build script

package main

import "gt"

// Quick Reply – reply to the last exile who "thinks to you".
//
// Notes for non‑technical players:
// - Use /r message to send: /thinkto <name> message
// - The script remembers only the most recent thinker.

// script metadata
var scriptName = "Quick Reply"
var scriptAuthor = "Examples"
var scriptCategory = "Quality Of Life"

const scriptAPIVersion = 1

var lastThinker string // remembers who last thought to us

// Init watches chat for "thinks to you" messages and adds /r.
func Init() {
	gt.Chat("thinks to you", quickReplyWatch)
	gt.RegisterCommand("r", quickReplyCmd)
}

func quickReplyWatch(msg string) {
	lower := gt.Lower(msg)
	if gt.Includes(lower, "thinks to you") {
		words := gt.Words(msg)
		if len(words) > 0 {
			lastThinker = words[0]
		}
	}
}

func quickReplyCmd(args string) {
	if lastThinker == "" {
		gt.Print("No one to reply to yet.")
		return
	}
	gt.Run("/thinkto " + lastThinker + " " + args)
}
