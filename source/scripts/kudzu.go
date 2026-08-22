//go:build script

package main

import "gt"

// script metadata
const scriptName = "Kudzu Helper"
const scriptAuthor = "Examples"
const scriptCategory = "Tools"
const scriptAPIVersion = 1

// Init sets up a few helper commands for planting and moving kudzu seeds.
func Init() {
	gt.RegisterCommand("zu", zuCmd)
	gt.RegisterCommand("zuget", zuGetCmd)
	gt.RegisterCommand("zustore", zuStoreCmd)
	gt.RegisterCommand("zutrans", zuTransCmd)
	// Press Shift+K to plant a seed.
	gt.Key("Shift-K", zuHotkey)
}

func zuCmd(args string) {
	// Quickly plant a seed at your feet.
	gt.Run("/plant kudzu")
}

func zuGetCmd(args string) {
	// Move a seed from the ground into your bag.
	gt.Run("/useitem bag of kudzu seedlings /add")
}

func zuStoreCmd(args string) {
	// Take a seed out of your bag.
	gt.Run("/useitem bag of kudzu seedlings /remove")
}

func zuTransCmd(args string) {
	// Give seeds to another exile if a name is provided.
	if args != "" {
		gt.Run("/transfer " + args)
	}
}

func zuHotkey() { zuCmd("") }
