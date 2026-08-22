package main

import (
	"strings"
	"sync"
)

var (
	ttsBlocklist   map[string]struct{}
	ttsBlocklistMu sync.RWMutex
)

func init() {
	ttsBlocklist = make(map[string]struct{})
	syncTTSBlocklist()
	scriptRegisterCommand("client", "notts", handleNoTTSCommand)
}

func syncTTSBlocklist() {
	ttsBlocklistMu.Lock()
	ttsBlocklist = make(map[string]struct{}, len(gs.ChatTTSBlocklist))
	for _, n := range gs.ChatTTSBlocklist {
		name := strings.ToLower(utfFold(strings.TrimSpace(n)))
		if name != "" {
			ttsBlocklist[name] = struct{}{}
		}
	}
	ttsBlocklistMu.Unlock()
}

func isTTSBlocked(name string) bool {
	name = strings.ToLower(utfFold(strings.TrimSpace(name)))
	ttsBlocklistMu.RLock()
	_, ok := ttsBlocklist[name]
	ttsBlocklistMu.RUnlock()
	return ok
}

func handleNoTTSCommand(args string) {
	fields := strings.Fields(args)
	if len(fields) == 1 && strings.ToLower(fields[0]) == "list" {
		ttsBlocklistMu.RLock()
		if len(gs.ChatTTSBlocklist) == 0 {
			consoleMessage("TTS blocklist is empty.")
		} else {
			consoleMessage("TTS blocklist: " + strings.Join(gs.ChatTTSBlocklist, ", "))
		}
		ttsBlocklistMu.RUnlock()
		return
	}
	if len(fields) != 2 {
		consoleMessage("Usage: /notts add|remove <name> or /notts list")
		return
	}
	action := strings.ToLower(fields[0])
	name := strings.ToLower(utfFold(fields[1]))
	if name == "" {
		return
	}

	ttsBlocklistMu.Lock()
	defer ttsBlocklistMu.Unlock()

	switch action {
	case "add":
		if _, exists := ttsBlocklist[name]; exists {
			consoleMessage(name + " is already in the TTS blocklist.")
			return
		}
		ttsBlocklist[name] = struct{}{}
		gs.ChatTTSBlocklist = append(gs.ChatTTSBlocklist, name)
		settingsDirty = true
		consoleMessage("Added " + name + " to the TTS blocklist.")
	case "remove":
		if _, exists := ttsBlocklist[name]; !exists {
			consoleMessage(name + " is not in the TTS blocklist.")
			return
		}
		delete(ttsBlocklist, name)
		for i, n := range gs.ChatTTSBlocklist {
			if strings.ToLower(utfFold(n)) == name {
				gs.ChatTTSBlocklist = append(gs.ChatTTSBlocklist[:i], gs.ChatTTSBlocklist[i+1:]...)
				break
			}
		}
		settingsDirty = true
		consoleMessage("Removed " + name + " from the TTS blocklist.")
	default:
		consoleMessage("Usage: /notts add|remove <name> or /notts list")
	}
}
