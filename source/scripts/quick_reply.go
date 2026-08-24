//go:build script

package main

import "gt2"

// Quick Reply – reply to the last exile who "thinks to you".
//
// Notes for non‑technical players:
// - Use /r message to send: /thinkto <name> message
// - The script remembers only the most recent thinker.

// script metadata
var scriptName = "Quick Reply"

const scriptID = "quick-reply"

var scriptAuthor = "Examples"
var scriptCategory = "Quality Of Life"

const scriptAPIVersion = 2

var lastThinker string // remembers who last thought to us

// Init watches chat for "thinks to you" messages and adds /r.
func Init() {
	gt2.OnChat(gt2.ChatFilter{Type: gt2.ChatTypeThinkTo}, quickReplyWatch)
	gt2.Command("r", quickReplyCmd)
}

func quickReplyWatch(event gt2.ChatEvent) {
	if event.Speaker != "" {
		lastThinker = event.Speaker
	}
}

func quickReplyCmd(args string) {
	if lastThinker == "" {
		gt2.Print("No one to reply to yet.")
		return
	}
	gt2.Send("/thinkto " + lastThinker + " " + args)
}
