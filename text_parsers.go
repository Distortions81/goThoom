package main

import (
	"bytes"
	"strings"
)

// parseWhoText parses a plain-text /who line with embedded BEPP player tags.
// Returns true if handled and should be suppressed from console.
func parseWhoText(raw []byte, s string) bool {
	if strings.HasPrefix(s, "You are the only one in the lands.") {
		// Nothing to add
		return true
	}
	if !strings.HasPrefix(s, "In the world are ") {
		return false
	}
	// Find first -pn tag segment and extract all names.
	// The format is: In the world are …: -pn <name> -pn , realname , <gm> \t ...
	off := bytes.Index(raw, []byte{0xC2, 'p', 'n'})
	if off < 0 {
		return true // handled, but no names
	}
	names := parseNames(raw[off:])
	if len(names) == 0 {
		return true
	}
	for _, name := range names {
		getPlayer(name)
	}
	playersDirty = true
	return true
}

// parseShareText parses plain share/unshare lines with embedded -pn tags.
// Returns true if the line was recognized and handled.
func parseShareText(raw []byte, s string) bool {
	switch {
	case strings.HasPrefix(s, "You are not sharing experiences with anyone."):
		// Clear sharees
		playersMu.Lock()
		for _, p := range players {
			p.Sharee = false
		}
		playersMu.Unlock()
		playersDirty = true
		return true
	case strings.HasPrefix(s, "You are no longer sharing experiences with "):
		// a single sharee removed
		// name will be in -pn tags
		off := bytes.Index(raw, []byte{0xC2, 'p', 'n'})
		if off >= 0 {
			for _, name := range parseNames(raw[off:]) {
				playersMu.Lock()
				if p, ok := players[name]; ok {
					p.Sharee = false
				}
				playersMu.Unlock()
			}
			playersDirty = true
		}
		return true
	case strings.HasPrefix(s, "You are sharing experiences with ") || strings.HasPrefix(s, "You begin sharing your experiences with "):
		// Self -> sharees
		off := bytes.Index(raw, []byte{0xC2, 'p', 'n'})
		if off >= 0 {
			for _, name := range parseNames(raw[off:]) {
				playersMu.Lock()
				p := getPlayer(name)
				p.Sharee = true
				playersMu.Unlock()
			}
			playersDirty = true
		}
		return true
	case strings.HasPrefix(s, "Currently sharing their experiences with you"):
		// Upstream sharers
		off := bytes.Index(raw, []byte{0xC2, 'p', 'n'})
		if off >= 0 {
			for _, name := range parseNames(raw[off:]) {
				playersMu.Lock()
				p := getPlayer(name)
				p.Sharing = true
				playersMu.Unlock()
			}
			playersDirty = true
		}
		return true
	}
	return false
}

// parseFallenText detects fallen/no-longer-fallen messages and updates state.
// Returns true if handled.
func parseFallenText(raw []byte, s string) bool {
	// Fallen: "<pn name> has fallen" (with optional -mn and -lo tags)
	if strings.Contains(s, " has fallen") {
		// Extract main player name
		name := firstTagContent(raw, 'p', 'n')
		if name == "" {
			return true
		}
		killer := firstTagContent(raw, 'm', 'n')
		where := firstTagContent(raw, 'l', 'o')
		playersMu.Lock()
		p := getPlayer(name)
		p.Dead = true
		p.KillerName = killer
		p.FellWhere = where
		playersMu.Unlock()
		playersDirty = true
		return true
	}
	// No longer fallen: "<pn name> is no longer fallen"
	if strings.Contains(s, " is no longer fallen") {
		name := firstTagContent(raw, 'p', 'n')
		if name == "" {
			return true
		}
		playersMu.Lock()
		if p, ok := players[name]; ok {
			p.Dead = false
			p.FellWhere = ""
			p.KillerName = ""
		}
		playersMu.Unlock()
		playersDirty = true
		return true
	}
	return false
}

// firstTagContent extracts the first bracketed content for a given 2-letter BEPP tag.
func firstTagContent(b []byte, a, b2 byte) string {
	i := bytes.Index(b, []byte{0xC2, a, b2})
	if i < 0 {
		return ""
	}
	rest := b[i+3:]
	j := bytes.Index(rest, []byte{0xC2, a, b2})
	if j < 0 {
		return ""
	}
	return strings.TrimSpace(decodeMacRoman(rest[:j]))
}
