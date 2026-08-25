//go:build script

package main

import "gt2"

// script metadata
const scriptName = "Kudzu Helper"
const scriptID = "kudzu"
const scriptAuthor = "Examples"
const scriptCategory = "Tools"
const scriptAPIVersion = 2

// Init sets up a few helper commands for planting and moving kudzu seeds.
func Init() {
	gt2.Command("zu", zuCmd)
	gt2.Command("zuget", zuGetCmd)
	gt2.Command("zustore", zuStoreCmd)
	gt2.Command("zutrans", zuTransCmd)
	// Press Shift+K to plant a seed.
	gt2.Bind("Shift-K", zuHotkey)
}

func zuCmd(args string) {
	// Quickly plant a seed at your feet.
	gt2.Send("/plant kudzu")
}

func zuGetCmd(args string) {
	// Move a seed from the ground into your bag.
	gt2.Send("/useitem bag of kudzu seedlings /add")
}

func zuStoreCmd(args string) {
	// Take a seed out of your bag.
	gt2.Send("/useitem bag of kudzu seedlings /remove")
}

func zuTransCmd(args string) {
	// Give seeds to another exile if a name is provided.
	if args != "" {
		gt2.Send("/transfer " + args)
	}
}

func zuHotkey(gt2.InputEvent) { zuCmd("") }
