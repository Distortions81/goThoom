//go:build script

package main

import (
	"strings"
	"time"

	"gt2"
)

// script metadata
const scriptName = "Bard Macros"
const scriptID = "bard"
const scriptAuthor = "Examples"
const scriptCategory = "Profession"
const scriptAPIVersion = 2

// Init sets up our commands and hotkeys.
func Init() {
	// /playsong <instrument> <notes>
	gt2.Command("playsong", playSongCmd)

	// A handy hotkey that plays a simple tune directly.
	gt2.Bind("Shift-B", playSongHotkey)
}

func playSongCmd(args string) {
	// Split the arguments into words.
	parts := strings.Fields(args)
	if len(parts) < 2 {
		// Need an instrument and at least one note.
		return
	}
	inst := parts[0]
	notes := strings.Join(parts[1:], " ")

	// Each wait reacts to an inventory update; there is no polling loop.
	gt2.WithEquipment("instrument case", func() {
		gt2.Send("/useitem instrument case /remove " + inst)
		if !gt2.WaitForInventory(inst, true, 2*time.Second) {
			gt2.Print("The instrument case did not produce " + inst + ".")
			return
		}
		gt2.WithEquipment(inst, func() {
			gt2.Send("/useitem " + inst + " " + notes)
		})
		gt2.Send("/useitem instrument case /add " + inst)
		if !gt2.WaitForInventory(inst, false, 2*time.Second) {
			gt2.Print(inst + " did not return to the instrument case.")
		}
	})
}

func playSongHotkey(gt2.InputEvent) {
	// Example: play a short riff on pine_flute.
	playSongCmd("pine_flute cfedcgdec")
}
