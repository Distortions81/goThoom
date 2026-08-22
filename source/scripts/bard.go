//go:build script

package main

import "gt"

// script metadata
const scriptName = "Bard Macros"
const scriptAuthor = "Examples"
const scriptCategory = "Profession"
const scriptAPIVersion = 1

// Init sets up our commands and hotkeys.
func Init() {
	// /playsong <instrument> <notes>
	gt.RegisterCommand("playsong", playSongCmd)

	// A handy hotkey that plays a simple tune directly.
	gt.Key("Shift-B", playSongHotkey)
}

func playSongCmd(args string) {
	// Split the arguments into words.
	parts := gt.Words(args)
	if len(parts) < 2 {
		// Need an instrument and at least one note.
		return
	}
	inst := parts[0]
	notes := gt.Join(parts[1:], " ")

	// Pull the instrument from our case, play the notes,
	// then put it back where we found it.
	gt.Run("/equip instrument case")
	gt.Run("/useitem instrument case /remove " + inst)
	gt.Run("/equip " + inst)
	gt.Run("/useitem " + inst + " " + notes)
	gt.Run("/useitem instrument case /add " + inst)
}

func playSongHotkey() {
	// Example: play a short riff on pine_flute.
	playSongCmd("pine_flute cfedcgdec")
}
