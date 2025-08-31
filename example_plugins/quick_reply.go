//go:build plugin

package main

import "gt"

// Quick Reply – reply to the last exile who "thinks to you".
//
// Notes for non‑technical players:
// - Use /r message to send: /thinkto <name> message
// - The plugin remembers only the most recent thinker.

// Plugin metadata
var PluginName = "Quick Reply"
var PluginAuthor = "Examples"
var PluginCategory = "Quality Of Life"

const PluginAPIVersion = 1

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
		gt.Console("No one to reply to yet.")
		return
	}
	gt.Run("/thinkto " + lastThinker + " " + args)
}
