package main

import (
	"bytes"
	"strings"
)

// parseBackend handles back-end BEP commands following the "be" prefix.
func parseBackend(data []byte) {
	if len(data) < 3 || data[0] != 0xC2 {
		return
	}
	cmd := string(data[1:3])
	payload := data[3:]
	switch cmd {
	case "in":
		parseBackendInfo(payload)
	case "sh":
		parseBackendShare(payload)
	case "wh":
		parseBackendWho(payload)
	}
}

// parseBackendInfo parses "be-in" messages containing player info.
func parseBackendInfo(data []byte) {
	if len(data) < 3 || data[0] != 0xC2 || data[1] != 'p' || data[2] != 'n' {
		return
	}
	rest := data[3:]
	end := bytes.Index(rest, []byte{0xC2, 'p', 'n'})
	if end < 0 {
		return
	}
	name := strings.TrimSpace(decodeMacRoman(rest[:end]))
	rest = rest[end+3:]
	fields := bytes.Split(rest, []byte{'\t'})
	if len(fields) < 3 {
		return
	}
	race := strings.TrimSpace(decodeMacRoman(fields[0]))
	gender := strings.TrimSpace(decodeMacRoman(fields[1]))
	class := strings.TrimSpace(decodeMacRoman(fields[2]))
	clan := ""
	if len(fields) > 3 {
		clan = strings.TrimSpace(decodeMacRoman(fields[3]))
	}
	playersMu.Lock()
	p, ok := players[name]
	if !ok {
		p = &Player{Name: name}
		players[name] = p
	}
	p.Race = race
	p.Gender = gender
	p.Class = class
	p.Clan = clan
	playersMu.Unlock()
	playersDirty = true
}

// parseBackendShare parses "be-sh" messages describing sharing relationships.
func parseBackendShare(data []byte) {
	playersMu.Lock()
	for _, p := range players {
		p.Sharee = false
		p.Sharing = false
	}
	playersMu.Unlock()
	parts := bytes.SplitN(data, []byte{'\t'}, 2)
	shareePart := parts[0]
	var sharerPart []byte
	if len(parts) > 1 {
		sharerPart = parts[1]
	}
	for _, name := range parseNames(shareePart) {
		playersMu.Lock()
		p, ok := players[name]
		if !ok {
			p = &Player{Name: name}
			players[name] = p
		}
		p.Sharee = true
		playersMu.Unlock()
	}
	for _, name := range parseNames(sharerPart) {
		playersMu.Lock()
		p, ok := players[name]
		if !ok {
			p = &Player{Name: name}
			players[name] = p
		}
		p.Sharing = true
		playersMu.Unlock()
	}
	playersDirty = true
}

// parseBackendWho parses "be-wh" messages listing players.
func parseBackendWho(data []byte) {
	for len(data) > 0 {
		if len(data) < 3 || data[0] != 0xC2 || data[1] != 'p' || data[2] != 'n' {
			return
		}
		data = data[3:]
		end := bytes.Index(data, []byte{0xC2, 'p', 'n'})
		if end < 0 {
			return
		}
		name := strings.TrimSpace(decodeMacRoman(data[:end]))
		getPlayer(name)
		data = data[end+3:]
		idx := bytes.IndexByte(data, '\t')
		if idx < 0 {
			return
		}
		data = data[idx+1:]
	}
	playersDirty = true
}

// parseNames extracts a slice of names from a sequence of "-pn name -pn" entries.
func parseNames(data []byte) []string {
	var names []string
	for len(data) >= 3 {
		if data[0] != 0xC2 || data[1] != 'p' || data[2] != 'n' {
			break
		}
		data = data[3:]
		end := bytes.Index(data, []byte{0xC2, 'p', 'n'})
		if end < 0 {
			break
		}
		name := strings.TrimSpace(decodeMacRoman(data[:end]))
		names = append(names, name)
		data = data[end+3:]
		if len(data) > 0 && data[0] == ',' {
			data = data[1:]
		}
	}
	return names
}
