//go:build plugin

package main

import "gt"

// Plugin metadata
const PluginName = "Bard Macros"
const PluginAuthor = "Examples"
const PluginCategory = "Profession"
const PluginAPIVersion = 1

// Init sets up our commands and hotkeys.
func Init() {
    // /playsong <instrument> <notes>
    gt.RegisterCommand("playsong", playSongCmd)

    // A handy hotkey that plays a simple tune directly.
    gt.AddHotkeyFn("Shift-B", playSongHotkey)
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
    gt.RunCommand("/equip instrument case")
    gt.RunCommand("/useitem instrument case /remove " + inst)
    gt.RunCommand("/equip " + inst)
    gt.RunCommand("/useitem " + inst + " " + notes)
    gt.RunCommand("/useitem instrument case /add " + inst)
}

func playSongHotkey(e gt.HotkeyEvent) {
    // Example: play a short riff on pine_flute.
    playSongCmd("pine_flute cfedcgdec")
}
