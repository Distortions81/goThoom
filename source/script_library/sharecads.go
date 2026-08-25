//go:build script

package main

import (
	"gt2"
	"strings"
	"time"
)

// script metadata
const scriptName = "Sharecads"
const scriptID = "sharecads"
const scriptAuthor = "Examples"
const scriptCategory = "Quality Of Life"
const scriptAPIVersion = 2

var (
	scOn    bool
	scShare = map[string]time.Time{}
)

// Init toggles the feature with /shcads or Shift+S.
func Init() {
	gt2.Command("shcads", scToggleCmd)
	gt2.OnChat(gt2.ChatFilter{Contains: "You sense healing energy from "}, handleSharecads)
	gt2.Bind("Shift-S", scToggleHotkey)
}

func scToggleCmd(args string) {
	scOn = !scOn
	if scOn {
		gt2.Print("* Sharecads enabled")
	} else {
		gt2.Print("* Sharecads disabled")
	}
}

func scToggleHotkey(gt2.InputEvent) { scToggleCmd("") }

// handleSharecads watches for healing energy messages and shares back once.
func handleSharecads(event gt2.ChatEvent) {
	if !scOn {
		return
	}
	const prefix = "You sense healing energy from "
	msg := event.Raw
	if !strings.HasPrefix(msg, prefix) {
		return
	}
	name := strings.TrimSuffix(strings.TrimPrefix(msg, prefix), ".")
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
	gt2.Send("/share " + name)
}
