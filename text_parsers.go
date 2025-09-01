package main

import (
	"bytes"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"
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
		p := getPlayer(name)
		playersMu.Lock()
		prevSC := p.SameClan
		prevSeen := p.Seen
		if me, ok := players[playerName]; ok {
			p.SameClan = sameRealClan(me.clan, p.clan)
		}
		p.LastSeen = time.Now()
		p.Offline = false
		p.Seen = true
		playerCopy := *p
		playersMu.Unlock()
		if prevSC != p.SameClan {
			killNameTagCacheFor(name)
		}
		if !prevSeen {
			playersPersistDirty = true
		}
		notifyPlayerHandlers(playerCopy)
	}
	playersDirty = true
	return true
}

// parseShareText parses plain share/unshare lines with embedded -pn tags.
// Returns true if the line was recognized and handled.
func parseShareText(raw []byte, s string) bool {
	switch {
	case strings.HasPrefix(s, "You are not sharing experiences with anyone.") ||
		strings.HasPrefix(s, "You are no longer sharing experiences with anyone."):
		// Clear sharees
		playersMu.Lock()
		changed := make([]Player, 0, len(players))
		for _, p := range players {
			if p.Sharee {
				p.Sharee = false
				changed = append(changed, *p)
			}
		}
		playersMu.Unlock()
		for _, pl := range changed {
			killNameTagCacheFor(pl.Name)
			notifyPlayerHandlers(pl)
		}
		playersDirty = true
		return true
	case strings.HasPrefix(s, "You are no longer sharing experiences with "):
		// a single sharee removed
		// name will be in -pn tags
		off := bytes.Index(raw, []byte{0xC2, 'p', 'n'})
		if off >= 0 {
			names := parseNames(raw[off:])
			playersMu.Lock()
			changed := make([]Player, 0, len(names))
			for _, name := range names {
				if p, ok := players[name]; ok {
					changedPlayer := false
					if p.Sharee {
						p.Sharee = false
						changedPlayer = true
					}
					if me, ok := players[playerName]; ok {
						sc := sameRealClan(me.clan, p.clan)
						if p.SameClan != sc {
							p.SameClan = sc
							changedPlayer = true
						}
					}
					if !p.Seen {
						p.Seen = true
						playersPersistDirty = true
					}
					if changedPlayer {
						changed = append(changed, *p)
					}
				}
			}
			playersMu.Unlock()
			for _, pl := range changed {
				killNameTagCacheFor(pl.Name)
				notifyPlayerHandlers(pl)
			}
			playersDirty = true
		}
		return true
	case strings.HasPrefix(s, "You are sharing experiences with ") || strings.HasPrefix(s, "You begin sharing your experiences with "):
		// Self -> sharees
		// Clear any existing sharees first
		playersMu.Lock()
		cleared := make([]Player, 0, len(players))
		for _, p := range players {
			if p.Sharee {
				p.Sharee = false
				cleared = append(cleared, *p)
			}
		}
		playersMu.Unlock()
		for _, pl := range cleared {
			killNameTagCacheFor(pl.Name)
			notifyPlayerHandlers(pl)
		}
		off := bytes.Index(raw, []byte{0xC2, 'p', 'n'})
		if off >= 0 {
			names := parseNames(raw[off:])
			added := make([]Player, 0, len(names))
			for _, name := range names {
				p := getPlayer(name)
				if !p.Sharee {
					p.Sharee = true
					added = append(added, *p)
				}
				if me, ok := players[playerName]; ok {
					sc := sameRealClan(me.clan, p.clan)
					if p.SameClan != sc {
						p.SameClan = sc
						added = append(added, *p)
					}
				}
				if !p.Seen {
					p.Seen = true
					playersPersistDirty = true
				}
			}
			for _, pl := range added {
				killNameTagCacheFor(pl.Name)
				notifyPlayerHandlers(pl)
			}
			playersDirty = true
		}
		return true
	case playerName != "" && (strings.HasPrefix(s, playerName+" is sharing experiences with ") || strings.HasPrefix(s, playerName+" begins sharing experiences with ")):
		// Hero (you) sharing others in third-person form
		playersMu.Lock()
		cleared := make([]Player, 0, len(players))
		for _, p := range players {
			if p.Sharee {
				p.Sharee = false
				cleared = append(cleared, *p)
			}
		}
		playersMu.Unlock()
		for _, pl := range cleared {
			killNameTagCacheFor(pl.Name)
			notifyPlayerHandlers(pl)
		}
		phrase := playerName + " is sharing experiences with "
		rest := strings.TrimPrefix(s, phrase)
		if rest == s {
			phrase = playerName + " begins sharing experiences with "
			rest = strings.TrimPrefix(s, phrase)
		}
		rest = strings.TrimSuffix(rest, ".")
		rest = strings.ReplaceAll(rest, " and ", ",")
		parts := strings.Split(rest, ",")
		playersMu.Lock()
		added := make([]Player, 0, len(parts))
		for _, name := range parts {
			name = strings.TrimSpace(name)
			if name == "" || strings.EqualFold(name, playerName) {
				continue
			}
			p, ok := players[name]
			if !ok {
				p = &Player{Name: name}
				players[name] = p
			}
			changed := false
			if !p.Sharee {
				p.Sharee = true
				changed = true
			}
			if me, ok := players[playerName]; ok {
				sc := sameRealClan(me.clan, p.clan)
				if p.SameClan != sc {
					p.SameClan = sc
					changed = true
				}
			}
			if !p.Seen {
				p.Seen = true
				playersPersistDirty = true
			}
			if changed {
				added = append(added, *p)
			}
		}
		playersMu.Unlock()
		for _, pl := range added {
			killNameTagCacheFor(pl.Name)
			notifyPlayerHandlers(pl)
		}
		playersDirty = true
		return true
	case playerName != "" && strings.HasPrefix(s, playerName+" is no longer sharing experiences with "):
		// Hero (you) unsharing others in third-person form
		phrase := playerName + " is no longer sharing experiences with "
		rest := strings.TrimPrefix(s, phrase)
		rest = strings.TrimSuffix(rest, ".")
		rest = strings.ReplaceAll(rest, " and ", ",")
		parts := strings.Split(rest, ",")
		playersMu.Lock()
		changed := make([]Player, 0, len(parts))
		for _, name := range parts {
			name = strings.TrimSpace(name)
			if name == "" || strings.EqualFold(name, playerName) {
				continue
			}
			if p, ok := players[name]; ok {
				changedPlayer := false
				if p.Sharee {
					p.Sharee = false
					changedPlayer = true
				}
				if me, ok := players[playerName]; ok {
					sc := sameRealClan(me.clan, p.clan)
					if p.SameClan != sc {
						p.SameClan = sc
						changedPlayer = true
					}
				}
				if !p.Seen {
					p.Seen = true
					playersPersistDirty = true
				}
				if changedPlayer {
					changed = append(changed, *p)
				}
			}
		}
		playersMu.Unlock()
		for _, pl := range changed {
			killNameTagCacheFor(pl.Name)
			notifyPlayerHandlers(pl)
		}
		playersDirty = true
		return true
	case strings.HasSuffix(s, " is sharing experiences with you."):
		name := utfFold(firstTagContent(raw, 'p', 'n'))
		if name != "" {
			p := getPlayer(name)
			playersMu.Lock()
			changed := !p.Sharing
			p.Sharing = true
			playerCopy := *p
			playersMu.Unlock()
			if changed {
				killNameTagCacheFor(name)
				notifyPlayerHandlers(playerCopy)
			}
			playersDirty = true
			showNotification(name + " is sharing with you")
		}
		return true
	case strings.Contains(s, " is no longer sharing experiences with you"):
		name := utfFold(firstTagContent(raw, 'p', 'n'))
		if name != "" {
			playersMu.Lock()
			changed := false
			var playerCopy Player
			if p, ok := players[name]; ok {
				if p.Sharing {
					p.Sharing = false
					changed = true
					playerCopy = *p
				}
			}
			playersMu.Unlock()
			if changed {
				killNameTagCacheFor(name)
				notifyPlayerHandlers(playerCopy)
			}
			playersDirty = true
		}
		return true
	case strings.HasPrefix(s, "Currently sharing their experiences with you"):
		// Upstream sharers
		off := bytes.Index(raw, []byte{0xC2, 'p', 'n'})
		if off >= 0 {
			names := parseNames(raw[off:])
			playersMu.Lock()
			changed := make([]Player, 0, len(names))
			for _, name := range names {
				p, ok := players[name]
				if !ok {
					p = &Player{Name: name}
					players[name] = p
				}
				changedPlayer := false
				if !p.Sharing {
					p.Sharing = true
					changedPlayer = true
				}
				if me, ok := players[playerName]; ok {
					sc := me.clan != "" && p.clan != "" && strings.EqualFold(p.clan, me.clan)
					if p.SameClan != sc {
						p.SameClan = sc
						changedPlayer = true
					}
				}
				if !p.Seen {
					p.Seen = true
					playersPersistDirty = true
				}
				if changedPlayer {
					changed = append(changed, *p)
				}
			}
			playersMu.Unlock()
			for _, pl := range changed {
				killNameTagCacheFor(pl.Name)
				notifyPlayerHandlers(pl)
			}
			playersDirty = true
		}
		return true
	}
	return false
}

// parseFallenText detects fallen/not-fallen messages and updates state.
// Returns true if handled.
func parseFallenText(raw []byte, s string) bool {
	if playerName != "" {
		if strings.HasPrefix(s, "You have fallen") {
			playersMu.Lock()
			if p, ok := players[playerName]; ok {
				p.Dead = true
				p.KillerName = ""
				p.FellWhere = ""
				p.FellTime = time.Now()
				playerCopy := *p
				playersMu.Unlock()
				playersDirty = true
				notifyPlayerHandlers(playerCopy)
			} else {
				playersMu.Unlock()
				playersDirty = true
			}
			if gs.NotifyFallen {
				showNotification(playerName + " has fallen")
			}
			return true
		}
		if strings.HasPrefix(s, "You are no longer fallen") {
			playersMu.Lock()
			if p, ok := players[playerName]; ok {
				p.Dead = false
				p.FellWhere = ""
				p.KillerName = ""
				p.FellTime = time.Time{}
				playerCopy := *p
				playersMu.Unlock()
				playersDirty = true
				notifyPlayerHandlers(playerCopy)
			} else {
				playersMu.Unlock()
				playersDirty = true
			}
			if gs.NotifyNotFallen {
				showNotification(playerName + " is no longer fallen")
			}
			return true
		}
	}
	// Fallen: "<pn name> has fallen" (with optional -mn and -lo tags)
	if strings.Contains(s, " has fallen") {
		// Extract main player name
		name := utfFold(firstTagContent(raw, 'p', 'n'))
		if name == "" {
			if idx := strings.Index(s, " has fallen"); idx >= 0 {
				name = utfFold(strings.TrimSpace(s[:idx]))
			}
		}
		if name == "" {
			return false
		}
		killer := utfFold(firstTagContent(raw, 'm', 'n'))
		where := firstTagContent(raw, 'l', 'o')
		p := getPlayer(name)
		playersMu.Lock()
		p.Dead = true
		p.KillerName = killer
		p.FellWhere = where
		p.FellTime = time.Now()
		playerCopy := *p
		playersMu.Unlock()
		playersDirty = true
		notifyPlayerHandlers(playerCopy)
		if gs.NotifyFallen {
			showNotification(name + " has fallen")
		}
		return true
	}
	// Not fallen: "<pn name> is no longer fallen"
	if strings.Contains(s, " is no longer fallen") {
		name := utfFold(firstTagContent(raw, 'p', 'n'))
		if name == "" {
			if idx := strings.Index(s, " is no longer fallen"); idx >= 0 {
				name = utfFold(strings.TrimSpace(s[:idx]))
			}
		}
		if name == "" {
			return false
		}
		playersMu.Lock()
		if p, ok := players[name]; ok {
			p.Dead = false
			p.FellWhere = ""
			p.KillerName = ""
			p.FellTime = time.Time{}
			playerCopy := *p
			playersMu.Unlock()
			playersDirty = true
			notifyPlayerHandlers(playerCopy)
		} else {
			playersMu.Unlock()
			playersDirty = true
		}
		if gs.NotifyNotFallen {
			showNotification(name + " is no longer fallen")
		}
		return true
	}
	return false
}

// parsePresenceText detects login/logoff/plain presence changes. Returns true if handled.
func parsePresenceText(raw []byte, s string) bool {
	// Attempt to detect common phrases. Names are provided in -pn tags.
	// We treat any recognized login as Online and any recognized logout as Offline.
	lower := strings.ToLower(s)
	name := utfFold(firstTagContent(raw, 'p', 'n'))
	if name == "" {
		return false
	}
	labelStr := firstTagContent(raw, 'p', 'l')
	label := -1
	if labelStr != "" {
		if v, err := strconv.Atoi(labelStr); err == nil {
			label = v
		}
	}
	// Login-like phrases
	if strings.Contains(lower, "has logged on") || strings.Contains(lower, "has entered the lands") || strings.Contains(lower, "has joined the world") || strings.Contains(lower, "has arrived") {
		var friend bool
		var playerCopy Player
		var labelChanged bool
		changed := false
		playersMu.Lock()
		if p, ok := players[name]; ok {
			p.LastSeen = time.Now()
			p.Offline = false
			if label >= 0 {
				if p.GlobalLabel != label {
					p.GlobalLabel = label
					applyPlayerLabel(p)
					labelChanged = true
				}
			}
			friend = p.Friend
			playerCopy = *p
			changed = true
		}
		playersMu.Unlock()
		if labelChanged {
			killNameTagCacheFor(name)
			playersPersistDirty = true
		}
		playersDirty = true
		if changed {
			notifyPlayerHandlers(playerCopy)
		}
		if friend && gs.NotifyFriendOnline {
			showNotification(name + " is online")
		}
		return true
	}
	// Logout-like phrases
	if strings.Contains(lower, "has logged off") || strings.Contains(lower, "has left the lands") || strings.Contains(lower, "has left the world") || strings.Contains(lower, "has departed") || strings.Contains(lower, "has signed off") {
		playersMu.Lock()
		if p, ok := players[name]; ok {
			p.Offline = true
			if label >= 0 && p.GlobalLabel != label {
				p.GlobalLabel = label
				applyPlayerLabel(p)
				playersPersistDirty = true
				killNameTagCacheFor(name)
			}
			playerCopy := *p
			playersMu.Unlock()
			playersDirty = true
			notifyPlayerHandlers(playerCopy)
		} else {
			playersMu.Unlock()
			playersDirty = true
		}
		return true
	}
	return false
}

// parseBardText detects bard guild messages and updates bard status.
// It also handles bard tune messages. Returns true if the message was fully
// handled and should not be displayed.
func parseBardText(raw []byte, s string) bool {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "* ") {
		s = strings.TrimSpace(s[2:])
	}
	if strings.HasPrefix(s, "¥ ") {
		s = strings.TrimSpace(s[2:])
	}

	// Prefer explicit BEPP tags for music, but also allow permissive detection
	// since some servers/messages may omit the tag yet include a "/music/..."
	// payload or a leading "play" form.
	hasMu := bytes.Contains(raw, []byte{0xC2, 'm', 'u'}) || bytes.Contains(raw, []byte{0xC2, 'b', 'a'})
	if hasMu || strings.Contains(s, "/music/") || strings.HasPrefix(s, "play ") || strings.HasPrefix(s, "play/") {
		if parseMusicCommand(s, raw) {
			return true
		}
	}

	phrases := []struct {
		suffix string
		bard   bool
	}{
		{" is a Bard Crafter", true},
		{" is a Bard Master", true},
		{" is a Bard Trustee", true},
		{" is a Bard Quester", true},
		{" is a Bard Guest", true},
		{" is a Bard", true},
		{" is not in the Bards' Guild", false},
		{" is not a Bard", false},
	}
	for _, ph := range phrases {
		if strings.HasSuffix(s, ph.suffix) {
			name := strings.TrimSpace(strings.TrimSuffix(s, ph.suffix))
			if name == "" {
				return false
			}
			p := getPlayer(name)
			playersMu.Lock()
			p.Bard = ph.bard
			p.LastSeen = time.Now()
			p.Offline = false
			playerCopy := *p
			playersMu.Unlock()
			playersDirty = true
			playersPersistDirty = true
			notifyPlayerHandlers(playerCopy)
			return false
		}
	}
	return false
}

// parseMusicCommand handles bard /music or /play commands and plays the tune.
// It supports both the simple "/play <inst> <notes>" form and the slash-delimited
// backend messages like "/music/.../play/inst<inst>/notes<notes>". If the
// stripped text is empty, it falls back to decoding the raw BEPP payload.
func parseMusicCommand(s string, raw []byte) bool {
	orig := s
	debug := func(msg string) {
		if musicDebug {
			consoleMessage(msg)
			chatMessage(msg)
			log.Print(msg)
		}
	}
	if s == "" && len(raw) > 0 {
		s = strings.TrimSpace(decodeMacRoman(raw))
		orig = s
	}
	if s == "" {
		if orig != "" {
			debug(orig)
		}
		return false
	}
	s = strings.TrimPrefix(s, "/music/")

	// Recognize and act on /stop (or /S) even if combined with play.
	stop := false
	if strings.Contains(s, "/stop") || strings.Contains(s, "/S") {
		stop = true
	}

	// Look for an explicit play command: "/play", "/P" or a leading
	// "play " (plain text). Avoid misclassifying arbitrary 'P...' lines.
	simplePlay := false
	if idx := strings.Index(s, "/play"); idx >= 0 {
		s = s[idx+len("/play"):]
	} else if idx := strings.Index(s, "/P"); idx >= 0 {
		s = s[idx+len("/P"):]
	} else if strings.HasPrefix(s, "play/") {
		s = s[len("play"):]
		simplePlay = true
	} else if strings.HasPrefix(s, "play ") {
		s = s[len("play "):]
		simplePlay = true
	} else {
		debug(orig)
		return false
	}

	s = strings.TrimSpace(s)

	// Parse parameters
	inst := defaultInstrument
	tempo := 120
	vol := 100
	who := 0
	me := false
	part := false
	withIDs := []int{}

	getInt := func(after string) (int, bool) {
		if idx := strings.Index(s, after); idx >= 0 {
			v := s[idx+len(after):]
			if len(v) > 0 && v[0] == '/' {
				v = v[1:]
			}
			if j := strings.IndexByte(v, '/'); j >= 0 {
				v = v[:j]
			}
			if n, err := strconv.Atoi(v); err == nil {
				return n, true
			}
		}
		return 0, false
	}

	if n, ok := getInt("/inst"); ok {
		inst = n
	} else if n, ok := getInt("/I"); ok {
		inst = n
	}
	if n, ok := getInt("/tempo"); ok {
		tempo = n
	} else if n, ok := getInt("/T"); ok {
		tempo = n
	}
	if n, ok := getInt("/vol"); ok {
		vol = n
	} else if n, ok := getInt("/V"); ok {
		vol = n
	}
	if n, ok := getInt("/who"); ok {
		who = n
	} else if n, ok := getInt("/W"); ok {
		who = n
	}
	if strings.Contains(s, "/me") || strings.Contains(s, "/E") {
		me = true
	}
	if strings.Contains(s, "/part") || strings.Contains(s, "/M") {
		part = true
	}
	// Extract all /with or /H occurrences
	scanWith := func(tag string) {
		pos := 0
		for {
			idx := strings.Index(s[pos:], tag)
			if idx < 0 {
				break
			}
			idx += pos + len(tag)
			start := idx
			v := s[idx:]
			if len(v) > 0 && v[0] == '/' {
				v = v[1:]
				idx++
			}
			if j := strings.IndexByte(v, '/'); j >= 0 {
				v = v[:j]
			}
			if n, err := strconv.Atoi(v); err == nil {
				withIDs = append(withIDs, n)
			}
			pos = idx + len(v)
			if pos <= start {
				pos = start + 1
			}
			if pos >= len(s) {
				break
			}
		}
	}
	scanWith("/with")
	scanWith("/H")

	notes := ""
	if idx := strings.Index(s, "/notes"); idx >= 0 {
		notes = s[idx+len("/notes"):]
	} else if idx := strings.Index(s, "/N"); idx >= 0 {
		notes = s[idx+len("/N"):]
	} else if simplePlay {
		// Accept the simple "play <inst> <notes>" form only when starting with
		// "play ". Require an instrument integer before notes to avoid false
		// positives on regular sentences.
		f := strings.Fields(s)
		if len(f) >= 2 {
			if n, err := strconv.Atoi(f[0]); err == nil {
				inst = n
				notes = strings.Join(f[1:], " ")
			}
		}
	}
	notes = strings.Trim(notes, "/")

	if stop {
		handleMusicParams(MusicParams{Stop: true})
		// Continue to parse play if present; otherwise return.
		if strings.TrimSpace(notes) == "" && !part {
			return true
		}
	}

	// If we still have no notes, do not treat this line as music.
	if strings.TrimSpace(notes) == "" && !part {
		debug(orig)
		return false
	}

	if musicDebug {
		msg := fmt.Sprintf("/play %d %s", inst, notes)
		debug(msg)
	}
	go func() {
		handleMusicParams(MusicParams{Inst: inst, Notes: notes, Tempo: tempo, VolPct: vol, Part: part, Who: who, With: withIDs, Me: me})
	}()
	return true
}

// parseInterruptCommand handles server-issued interrupt commands that should
// immediately stop any in-flight music playback and clear pending tunes. The
// classic client uses a special "/m_interrupt" directive for this purpose.
// Returns true if handled and output should be suppressed.
func parseInterruptCommand(s string) bool {
	ss := strings.TrimSpace(s)
	if ss == "" {
		return false
	}
	// Normalize common leading markers that may prefix system lines.
	if strings.HasPrefix(ss, "* ") {
		ss = strings.TrimSpace(ss[2:])
	}
	if strings.HasPrefix(ss, "¥ ") {
		ss = strings.TrimSpace(ss[2:])
	}
	// Only act on a standalone interrupt directive, not arbitrary substrings.
	if ss == "/m_interrupt" || strings.HasPrefix(ss, "/m_interrupt ") {
		stopAllMusic()
		clearTuneQueue()
		return true
	}
	return false
}

// truncate helps keep debug output short
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
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
