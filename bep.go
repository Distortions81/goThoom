package main

import (
	"bytes"
	"strconv"
	"strings"
	"time"
)

// parseBackend handles back-end BEP commands following the "be" prefix.
func parseBackend(data []byte) {
	// Expect a BEPP tag for the backend subcommand (e.g., -wh, -in, -sh)
	// immediately following the initial -be.
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
	// Skip any leading tabs before the race field (stock client does this).
	for len(rest) > 0 && rest[0] == '\t' {
		rest = rest[1:]
	}
	fields := bytes.Split(rest, []byte{'\t'})
	// Some servers/messages include a leading empty field; drop it.
	if len(fields) > 0 && len(fields[0]) == 0 {
		fields = fields[1:]
	}
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
	p.clan = clan
	p.LastSeen = time.Now()
	p.Offline = false
	p.Seen = true
	changedNames := make([]string, 0, 1)
	if playerName != "" {
		if name == playerName {
			myClan := clan
			for _, pl := range players {
				sc := sameRealClan(pl.clan, myClan)
				if pl.SameClan != sc {
					pl.SameClan = sc
					changedNames = append(changedNames, pl.Name)
				}
			}
		} else if me, ok := players[playerName]; ok {
			sc := sameRealClan(me.clan, p.clan)
			if p.SameClan != sc {
				p.SameClan = sc
				changedNames = append(changedNames, p.Name)
			}
		}
	}
	playerCopy := *p
	playersMu.Unlock()
	playersDirty = true
	playersPersistDirty = true
	notifyPlayerHandlers(playerCopy)
	for _, nm := range changedNames {
		killNameTagCacheFor(nm)
	}

	if playerName != "" && strings.EqualFold(name, playerName) {
		for i := range characters {
			if strings.EqualFold(characters[i].Name, name) {
				if characters[i].Profession != class {
					characters[i].Profession = class
					saveCharacters()
				}
				break
			}
		}
	}
}

// parseBackendShare parses "be-sh" messages describing sharing relationships.
func parseBackendShare(data []byte) {
	playersMu.Lock()
	cleared := make([]Player, 0, len(players))
	for _, p := range players {
		if p.Sharee || p.Sharing {
			p.Sharee = false
			p.Sharing = false
			cleared = append(cleared, *p)
		}
	}
	playersMu.Unlock()
	for _, pl := range cleared {
		killNameTagCacheFor(pl.Name)
		notifyPlayerHandlers(pl)
	}
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
		changed := !p.Sharee
		seenChanged := !p.Seen
		if me, ok := players[playerName]; ok {
			sc := sameRealClan(me.clan, p.clan)
			if p.SameClan != sc {
				p.SameClan = sc
				changed = true
			}
		}
		p.Seen = true
		p.Sharee = true
		p.LastSeen = time.Now()
		playerCopy := *p
		playersMu.Unlock()
		if changed {
			killNameTagCacheFor(name)
		}
		if seenChanged {
			playersPersistDirty = true
		}
		notifyPlayerHandlers(playerCopy)
	}
	for _, name := range parseNames(sharerPart) {
		playersMu.Lock()
		p, ok := players[name]
		if !ok {
			p = &Player{Name: name}
			players[name] = p
		}
		changed := !p.Sharing
		seenChanged := !p.Seen
		if me, ok := players[playerName]; ok {
			sc := sameRealClan(me.clan, p.clan)
			if p.SameClan != sc {
				p.SameClan = sc
				changed = true
			}
		}
		p.Seen = true
		p.Sharing = true
		p.LastSeen = time.Now()
		playerCopy := *p
		playersMu.Unlock()
		if changed {
			killNameTagCacheFor(name)
		}
		if seenChanged {
			playersPersistDirty = true
		}
		notifyPlayerHandlers(playerCopy)
	}
	playersDirty = true
}

// parseBackendWho parses "be-wh" messages listing players.
func parseBackendWho(data []byte) {
	if !whoActive {
		clearBeWhoFlags()
	}
	batchCount := 0
	newCount := 0
	for len(data) > 0 {
		if len(data) < 3 || data[0] != 0xC2 || data[1] != 'p' || data[2] != 'n' {
			break
		}
		data = data[3:]
		end := bytes.Index(data, []byte{0xC2, 'p', 'n'})
		if end < 0 {
			break
		}
		name := strings.TrimSpace(decodeMacRoman(data[:end]))
		// After name, expect: ',' <real-name> ',' <gmlevel> '\t'
		seg := data[end+3:]
		tab := bytes.IndexByte(seg, '\t')
		if tab < 0 {
			break
		}
		meta := seg[:tab] // leading comma-realname-comma-gm
		gm := 0
		if c1 := bytes.IndexByte(meta, ','); c1 >= 0 {
			if c2 := bytes.IndexByte(meta[c1+1:], ','); c2 >= 0 {
				gmStr := strings.TrimSpace(decodeMacRoman(meta[c1+1+c2+1:]))
				if gmv, err := strconv.Atoi(gmStr); err == nil {
					gm = gmv
				}
			}
		}
		// Advance to after tab for next entry
		data = seg[tab+1:]

		// Update player record and enqueue info request if needed.
		playersMu.Lock()
		p, ok := players[name]
		if ok && p.beWho {
			playersMu.Unlock()
			break
		}
		if !ok {
			p = &Player{Name: name}
			players[name] = p
			newCount++
		}
		if gm >= 0 {
			p.gmLevel = gm
		}
		p.LastSeen = time.Now()
		p.Offline = false
		bwChanged := !p.beWho
		seenChanged := !p.Seen
		scChanged := false
		if me, ok := players[playerName]; ok {
			sc := sameRealClan(me.clan, p.clan)
			if p.SameClan != sc {
				p.SameClan = sc
				scChanged = true
			}
		}
		p.beWho = true
		p.Seen = true
		playerCopy := *p
		playersMu.Unlock()
		notifyPlayerHandlers(playerCopy)
		if scChanged {
			killNameTagCacheFor(name)
		}
		if bwChanged || seenChanged {
			playersPersistDirty = true
		}
		queueInfoRequest(name)
		batchCount++
	}
	if batchCount > 0 {
		playersDirty = true
	}
	if newCount > 0 {
		playersPersistDirty = true
	}
	// Consider requesting another who batch if this looks like a partial page
	considerNextWhoBatch(batchCount)
}

// parseNames extracts a slice of names from a sequence of "-pn name -pn" entries.
// Between segments commas, spaces and the word "and" (case-insensitive) are ignored.
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
		name := utfFold(strings.TrimSpace(decodeMacRoman(data[:end])))
		names = append(names, name)
		data = data[end+3:]
		for {
			data = bytes.TrimLeft(data, " ")
			switch {
			case len(data) >= 3 && bytes.EqualFold(data[:3], []byte("and")):
				data = data[3:]
			case len(data) > 0 && data[0] == ',':
				data = data[1:]
			default:
				goto next
			}
		}
	next:
	}
	return names
}
