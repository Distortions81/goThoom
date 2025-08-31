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
// - Type /dance or press Shift+D and your exile will run a short
//   sequence of /pose actions for fun screenshots.
// - Safe to spam; it just sends a few /pose commands with short pauses.
func Init() {
    // Allow typing /dance to trigger it.
    gt.RegisterCommand("dance", danceCmd)
    // Press Shift+D to start dancing (function hotkey; no slash command).
    gt.AddHotkeyFn("Shift-D", danceHotkey)
}

func danceCmd(args string) { runDance() }
func danceHotkey(e gt.HotkeyEvent) { runDance() }

func runDance() {
    // A tiny routine of poses played in sequence.
    poses := []string{"celebrate", "leanleft", "leanright", "celebrate"}
    for _, p := range poses {
        gt.RunCommand("/pose " + p)
        time.Sleep(250 * time.Millisecond)
    }
}
