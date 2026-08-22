//go:build script

package main

import (
	"gt"
	"time"
)

// script metadata
const scriptName = "Sharecads"
const scriptAuthor = "Examples"
const scriptCategory = "Quality Of Life"
const scriptAPIVersion = 1

var (
	scOn    bool
	scShare = map[string]time.Time{}
)

// Init toggles the feature with /shcads or Shift+S.
func Init() {
	gt.RegisterCommand("shcads", scToggleCmd)
	gt.Chat("You sense healing energy from ", handleSharecads)
	gt.AddHotkeyFn("Shift-S", scToggleHotkey)
}

func scToggleCmd(args string) {
	scOn = !scOn
	if scOn {
		gt.Print("* Sharecads enabled")
	} else {
		gt.Print("* Sharecads disabled")
	}
}

func scToggleHotkey(e gt.HotkeyEvent) { scToggleCmd("") }

// handleSharecads watches for healing energy messages and shares back once.
func handleSharecads(msg string) {
	if !scOn {
		return
	}
	const prefix = "You sense healing energy from "
	if !gt.StartsWith(msg, prefix) {
		return
	}
	name := gt.TrimEnd(gt.TrimStart(msg, prefix), ".")
	now := time.Now()
	if t, ok := scShare[name]; ok && now.Sub(t) < 3*time.Second {
		return
	}
	if len(scShare) >= 5 {
		oldest := now
		oldestName := ""
		for n, ts := range scShare {
			if ts.Before(oldest) {
				oldest = ts
				oldestName = n
			}
		}
		if oldestName != "" {
			delete(scShare, oldestName)
		}
	}
	scShare[name] = now
	gt.Cmd("/share " + name)
}
