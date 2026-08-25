//go:build script

package main

import "gt2"

// Default Shortcuts – type short codes that expand into full commands.
//
// Notes for non‑technical players:
// - Type these at the very start of the chat bar. Example: pp hello → /ponder hello
// - If the full command ends with a space, you can continue typing arguments.
// - You can add or remove entries in the list below; each line is "short": "full".

const scriptAPIVersion = 2
const scriptID = "default-shortcuts"
const scriptName = "Default Shortcuts"
const scriptAuthor = "Distortions"
const scriptCategory = "Quality Of Life"

func Init() {
	// Keys are what you type; values are the full command to send.
	shortcuts := map[string]string{
		"??": "/help ",
		"aa": "/action ",
		"gg": "/give ",
		"ii": "/info ",
		"kk": "/karma ",
		"mm": "/money", // no args
		"nn": "/news",  // no args
		"pp": "/ponder ",
		"sh": "/share ",
		"sl": "/sleep", // no args
		"t":  "/think ",
		"tt": "/thinkto ",
		"th": "/thank ",
		"ui": "/useitem ",
		"uu": "/use ",
		"un": "/unshare ",
		"w":  "/who ",
		"wh": "/whisper ",
		"yy": "/yell ",
	}
	for short, full := range shortcuts {
		gt2.AddShortcut(short, full)
	}
}
