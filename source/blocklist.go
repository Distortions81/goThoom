package main

import "strings"

func init() {
	scriptRegisterCommand("client", "block", handleBlockCommand)
	scriptRegisterCommand("client", "ignore", handleIgnoreCommand)
	scriptRegisterCommand("client", "forget", handleForgetCommand)
}

func handleBlockCommand(args string) {
	name := utfFold(strings.TrimSpace(args))
	if name == "" {
		return
	}
	p := getPlayer(name)
	playersMu.Lock()
	wasBlocked := p.GlobalLabel == 6
	if wasBlocked {
		p.GlobalLabel = 0
	} else {
		p.GlobalLabel = 6
	}
	applyPlayerLabel(p)
	playerCopy := *p
	playersMu.Unlock()
	playersDirty = true
	playersPersistDirty = true
	killNameTagCacheFor(p.Name)
	notifyPlayerHandlers(playerCopy)
	msg := "Blocking " + p.Name + "."
	if wasBlocked {
		msg = "No longer blocking " + p.Name + "."
	}
	consoleMessage(msg)
}

func handleIgnoreCommand(args string) {
	name := utfFold(strings.TrimSpace(args))
	if name == "" {
		return
	}
	p := getPlayer(name)
	playersMu.Lock()
	wasIgnored := p.GlobalLabel == 7
	if wasIgnored {
		p.GlobalLabel = 0
	} else {
		p.GlobalLabel = 7
	}
	applyPlayerLabel(p)
	playerCopy := *p
	playersMu.Unlock()
	playersDirty = true
	playersPersistDirty = true
	killNameTagCacheFor(p.Name)
	notifyPlayerHandlers(playerCopy)
	msg := "Ignoring " + p.Name + "."
	if wasIgnored {
		msg = "No longer ignoring " + p.Name + "."
	}
	consoleMessage(msg)
}

func handleForgetCommand(args string) {
	name := utfFold(strings.TrimSpace(args))
	if name == "" {
		return
	}
	p := getPlayer(name)
	playersMu.Lock()
	wasBlocked := p.GlobalLabel == 6
	wasIgnored := p.GlobalLabel == 7
	wasFriend := p.GlobalLabel > 0 && p.GlobalLabel < 6
	p.GlobalLabel = 0
	p.LocalLabel = 0
	applyPlayerLabel(p)
	playerCopy := *p
	playersMu.Unlock()
	playersDirty = true
	playersPersistDirty = true
	killNameTagCacheFor(p.Name)
	notifyPlayerHandlers(playerCopy)
	msg := "Forgot " + p.Name + "."
	switch {
	case wasIgnored:
		msg = "No longer ignoring " + p.Name + "."
	case wasBlocked:
		msg = "No longer blocking " + p.Name + "."
	case wasFriend:
		msg = "Removing label from " + p.Name + "."
	}
	consoleMessage(msg)
}
