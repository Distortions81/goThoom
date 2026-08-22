package main

import (
	"encoding/binary"
	"strings"
)

type hookMsg struct {
	name string
	tag  string
	desc string
}

var hookTypes = []hookMsg{
	{"bard", "ba", "bard message"},
	{"backend", "be", "back-end command"},
	{"clan", "cn", "clan name"},
	{"config", "cf", "config"},
	{"nodisplay", "dd", "do not display"},
	{"demo", "de", "demo notice"},
	{"depart", "dp", "depart"},
	{"download", "dl", "download"},
	{"error", "er", "error message"},
	{"gm", "gm", "game master"},
	{"fallen", "hf", "has fallen"},
	{"notfallen", "nf", "no longer fallen"},
	{"info", "in", "info"},
	{"inventory", "iv", "inventory"},
	{"karma", "ka", "karma"},
	{"karmarecv", "kr", "karma received"},
	{"logoff", "lf", "log off"},
	{"logon", "lg", "log on"},
	{"location", "lo", "location"},
	{"multi", "ml", "multilingual"},
	{"monster", "mn", "monster name"},
	{"music", "mu", "music"},
	{"news", "nw", "news"},
	{"player", "pn", "player name"},
	{"share", "sh", "share"},
	{"unshare", "su", "unshare"},
	{"textlog", "tl", "text log only"},
	{"think", "th", "think"},
	{"mono", "tt", "monospaced style"},
	{"who", "wh", "who list"},
	{"youkilled", "yk", "you killed"},
}

var hookLookup = func() map[string]string {
	m := make(map[string]string, len(hookTypes)*2)
	for _, h := range hookTypes {
		m[h.name] = h.tag
		m[h.tag] = h.tag
	}
	return m
}()

func testHooksHelp() {
	consoleMessage("usage: /testhooks <message> [name] <data>")
	consoleMessage("messages:")
	for _, h := range hookTypes {
		consoleMessage("  " + h.name + " - " + h.desc)
	}
}

// testScriptHooks emits sample chat and console messages to exercise script trigger hooks.
// With arguments, it crafts a server message with the given type, optional name,
// and data. Use /testhooks help to list message names.
func testScriptHooks(args string) {
	if args == "" {
		// NPC chat with a blank name
		chatMessage(" says, testing NPC chat")
		// System console message
		consoleMessage("System: testing message")
		// Simulated share message using info text format
		msg := append([]byte("You are sharing experiences with "), pnTag("Tester")...)
		msg = append(msg, '.')
		handleInfoText(append(bepp("sh", msg), '\r'))
		consoleMessage("run /testhooks help for message types")
		return
	}
	if strings.ToLower(args) == "help" {
		testHooksHelp()
		return
	}
	parts := strings.SplitN(args, " ", 3)
	if len(parts) < 2 {
		testHooksHelp()
		return
	}
	key := strings.ToLower(parts[0])
	typ, ok := hookLookup[key]
	if !ok {
		consoleMessage("unknown message type: " + key)
		testHooksHelp()
		return
	}
	var name, data string
	if len(parts) == 2 {
		data = parts[1]
	} else {
		name = parts[1]
		data = parts[2]
	}
	payload := encodeMacRoman(data)
	if name != "" {
		payload = append(append(pnTag(name), ' '), payload...)
	}
	msg := bepp(typ, payload)
	m := make([]byte, 16+len(msg))
	binary.BigEndian.PutUint16(m[0:2], 0)
	copy(m[16:], msg)
	processServerMessage(m)
}
