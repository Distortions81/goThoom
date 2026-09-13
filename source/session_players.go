package main

import (
	"bytes"
	"strconv"
	"strings"
	"sync"
	"time"
)

// sessionPlayerState is the decoder-owned player directory for one session.
// The existing primary Players UI remains as an adapter until that window can
// bind directly to the selected session.
type sessionPlayerState struct {
	mu             sync.RWMutex
	players        map[string]*Player
	infoQueue      map[string]struct{}
	lastInfoSent   time.Time
	whoActive      bool
	whoScanStarted time.Time
	whoLastRequest time.Time
}

func (s *sessionPlayerState) maybeEnqueueInfo(commands *commandState) bool {
	if s == nil || commands == nil || !commands.idle() {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if time.Since(s.lastInfoSent) < infoCooldown {
		return false
	}
	for name := range s.infoQueue {
		if !commands.enqueueIfIdle("/be-info " + name) {
			return false
		}
		delete(s.infoQueue, name)
		s.lastInfoSent = time.Now()
		return true
	}
	return false
}

func (s *sessionPlayerState) maybeEnqueueWho(commands *commandState) bool {
	if s == nil || commands == nil || !commands.idle() {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.whoActive || time.Since(s.whoLastRequest) < whoCooldown {
		return false
	}
	if !commands.enqueueIfIdle("/be-who") {
		return false
	}
	s.whoLastRequest = time.Now()
	return true
}

func newSessionPlayerState() *sessionPlayerState {
	return &sessionPlayerState{players: make(map[string]*Player), infoQueue: make(map[string]struct{})}
}

func (s *sessionPlayerState) queueInfoRequest(name string) {
	name = strings.TrimSpace(name)
	if s == nil || name == "" {
		return
	}
	s.mu.Lock()
	player := s.players[name]
	if player == nil || player.Class == "" || player.Gender == "" || player.Race == "" || player.clan == "" {
		s.infoQueue[name] = struct{}{}
	}
	s.mu.Unlock()
}

func (s *sessionPlayerState) parseBackendInfo(data []byte) {
	if s == nil || len(data) < 3 || data[0] != 0xC2 || data[1] != 'p' || data[2] != 'n' {
		return
	}
	rest := data[3:]
	end := bytes.Index(rest, []byte{0xC2, 'p', 'n'})
	if end < 0 {
		return
	}
	name := strings.TrimSpace(decodeServerText(rest[:end]))
	rest = bytes.TrimLeft(data[3+end+3:], "\t")
	fields := bytes.Split(rest, []byte{'\t'})
	if len(fields) > 0 && len(fields[0]) == 0 {
		fields = fields[1:]
	}
	if len(fields) < 3 {
		return
	}
	s.mu.Lock()
	player := s.players[name]
	if player == nil {
		player = &Player{Name: name, Offline: true}
		s.players[name] = player
	}
	player.Race = strings.TrimSpace(decodeServerText(fields[0]))
	player.Gender = strings.TrimSpace(decodeServerText(fields[1]))
	player.Class = strings.TrimSpace(decodeServerText(fields[2]))
	if len(fields) > 3 {
		player.clan = strings.TrimSpace(decodeServerText(fields[3]))
	}
	player.LastSeen, player.Seen = time.Now(), true
	delete(s.infoQueue, name)
	s.mu.Unlock()
	playersDirty = true
}

func (s *sessionPlayerState) parseBackendShare(data []byte, self string) {
	if s == nil {
		return
	}
	parts := bytes.SplitN(data, []byte{'\t'}, 2)
	var sharees, sharers []string
	sharees = parseNames(parts[0])
	if len(parts) > 1 {
		sharers = parseNames(parts[1])
	}
	s.mu.Lock()
	for _, player := range s.players {
		player.Sharee, player.Sharing = false, false
	}
	mark := func(names []string, outbound bool) {
		for _, name := range names {
			if strings.EqualFold(name, self) {
				continue
			}
			player := s.players[name]
			if player == nil {
				player = &Player{Name: name}
				s.players[name] = player
			}
			if outbound {
				player.Sharee = true
			} else {
				player.Sharing = true
			}
			player.Seen, player.Offline, player.LastSeen = true, false, time.Now()
		}
	}
	mark(sharees, true)
	mark(sharers, false)
	s.mu.Unlock()
	playersDirty = true
}

func (s *sessionPlayerState) parseBackendWho(data []byte) {
	if s == nil {
		return
	}
	now := time.Now()
	s.mu.Lock()
	if !s.whoActive {
		s.whoActive = true
		s.whoScanStarted = now
		for _, player := range s.players {
			player.beWho = false
		}
	}
	batchCount := 0
	duplicate := false
	infoNames := make([]string, 0, 20)
	for len(data) >= 3 && data[0] == 0xC2 && data[1] == 'p' && data[2] == 'n' {
		data = data[3:]
		end := bytes.Index(data, []byte{0xC2, 'p', 'n'})
		if end < 0 {
			break
		}
		name := strings.TrimSpace(decodeServerText(data[:end]))
		segment := data[end+3:]
		tab := bytes.IndexByte(segment, '\t')
		if tab < 0 {
			break
		}
		gm := 0
		meta := segment[:tab]
		if comma := bytes.LastIndexByte(meta, ','); comma >= 0 {
			gm, _ = strconv.Atoi(strings.TrimSpace(decodeServerText(meta[comma+1:])))
		}
		player := s.players[name]
		if player != nil && player.beWho {
			duplicate = true
			break
		}
		if player == nil {
			player = &Player{Name: name}
			s.players[name] = player
		}
		player.gmLevel, player.beWho = gm, true
		player.Seen, player.Offline, player.LastSeen = true, false, now
		if player.Class == "" || player.Gender == "" || player.Race == "" || player.clan == "" {
			infoNames = append(infoNames, name)
		}
		batchCount++
		data = segment[tab+1:]
	}
	if duplicate || batchCount < 20 {
		for _, player := range s.players {
			if player.beWho || player.Offline || player.LastSeen.After(s.whoScanStarted) {
				continue
			}
			player.Offline = true
		}
		s.whoActive = false
		s.whoScanStarted = time.Time{}
	}
	for _, name := range infoNames {
		s.infoQueue[name] = struct{}{}
	}
	s.mu.Unlock()
	playersDirty = true
}

func (s *sessionPlayerState) parsePresenceText(raw []byte, text string) bool {
	name := utfFold(firstTagContent(raw, 'p', 'n'))
	if s == nil || name == "" {
		return false
	}
	lower := strings.ToLower(text)
	online := strings.Contains(lower, "has logged on") || strings.Contains(lower, "has entered the lands") || strings.Contains(lower, "has joined the world") || strings.Contains(lower, "has arrived")
	offline := strings.Contains(lower, "has logged off") || strings.Contains(lower, "has left the lands") || strings.Contains(lower, "has left the world") || strings.Contains(lower, "has departed") || strings.Contains(lower, "has signed off")
	if !online && !offline {
		return false
	}
	label := -1
	if value := firstTagContent(raw, 'p', 'l'); value != "" {
		label, _ = strconv.Atoi(value)
	}
	s.mu.Lock()
	player := s.players[name]
	if player == nil {
		player = &Player{Name: name}
		s.players[name] = player
	}
	player.Offline = offline
	if online {
		player.Seen, player.LastSeen = true, time.Now()
	}
	if label >= 0 {
		player.GlobalLabel = label
		applyPlayerLabel(player)
	}
	s.mu.Unlock()
	playersDirty = true
	return true
}

func (s *sessionPlayerState) parseFallenText(raw []byte, text, self string) bool {
	if s == nil {
		return false
	}
	name := utfFold(firstTagContent(raw, 'p', 'n'))
	fallen := strings.Contains(text, " has fallen") || strings.HasPrefix(text, "You have fallen")
	recovered := strings.Contains(text, " is no longer fallen") || strings.HasPrefix(text, "You are no longer fallen")
	if name == "" && (strings.HasPrefix(text, "You ")) {
		name = self
	}
	if name == "" {
		for _, marker := range []string{" has fallen", " is no longer fallen"} {
			if index := strings.Index(text, marker); index >= 0 {
				name = utfFold(strings.TrimSpace(text[:index]))
				break
			}
		}
	}
	if name == "" || !fallen && !recovered {
		return false
	}
	s.mu.Lock()
	player := s.players[name]
	if player == nil {
		player = &Player{Name: name}
		s.players[name] = player
	}
	player.Dead = fallen
	if fallen {
		player.KillerName = utfFold(firstTagContent(raw, 'm', 'n'))
		player.FellWhere = firstTagContent(raw, 'l', 'o')
		player.FellTime = time.Now()
	} else {
		player.KillerName, player.FellWhere, player.FellTime = "", "", time.Time{}
	}
	s.mu.Unlock()
	playersDirty = true
	return true
}

func (s *sessionPlayerState) parseShareText(raw []byte, text, self string) bool {
	if s == nil {
		return false
	}
	lower := strings.ToLower(text)
	s.mu.Lock()
	defer s.mu.Unlock()
	clearSharees := func() {
		for _, player := range s.players {
			player.Sharee = false
		}
	}
	markSharees := func(names []string, sharing bool) {
		for _, name := range names {
			if name == "" || strings.EqualFold(name, self) {
				continue
			}
			player := s.players[name]
			if player == nil && !sharing {
				continue
			}
			if player == nil {
				player = &Player{Name: name}
				s.players[name] = player
			}
			player.Sharee = sharing
			player.Seen = true
		}
	}
	markSharers := func(names []string, sharing bool) {
		for _, name := range names {
			if name == "" || strings.EqualFold(name, self) {
				continue
			}
			player := s.players[name]
			if player == nil && !sharing {
				continue
			}
			if player == nil {
				player = &Player{Name: name}
				s.players[name] = player
			}
			player.Sharing = sharing
			player.Seen = true
			if sharing {
				player.Offline = false
				player.LastSeen = time.Now()
			}
		}
	}
	taggedNames := func() []string {
		if offset := bytes.Index(raw, []byte{0xC2, 'p', 'n'}); offset >= 0 {
			return parseNames(raw[offset:])
		}
		return nil
	}
	textNames := func(prefix string) []string {
		rest := strings.TrimSuffix(strings.TrimPrefix(text, prefix), ".")
		rest = strings.ReplaceAll(rest, " and ", ",")
		parts := strings.Split(rest, ",")
		names := make([]string, 0, len(parts))
		for _, name := range parts {
			if name = utfFold(strings.TrimSpace(name)); name != "" {
				names = append(names, name)
			}
		}
		return names
	}

	switch {
	case strings.HasPrefix(text, "You are not sharing experiences with anyone") || strings.HasPrefix(text, "You are no longer sharing experiences with anyone"):
		clearSharees()
		playersDirty = true
		return true
	case strings.HasPrefix(text, "You are no longer sharing experiences with "):
		markSharees(taggedNames(), false)
		playersDirty = true
		return true
	case strings.HasPrefix(text, "You are sharing experiences with ") || strings.HasPrefix(text, "You begin sharing your experiences with "):
		clearSharees()
		markSharees(taggedNames(), true)
		playersDirty = true
		return true
	case self != "" && (strings.HasPrefix(text, self+" is sharing experiences with ") || strings.HasPrefix(text, self+" begins sharing experiences with ")):
		clearSharees()
		prefix := self + " is sharing experiences with "
		if !strings.HasPrefix(text, prefix) {
			prefix = self + " begins sharing experiences with "
		}
		markSharees(textNames(prefix), true)
		playersDirty = true
		return true
	case self != "" && strings.HasPrefix(text, self+" is no longer sharing experiences with "):
		markSharees(textNames(self+" is no longer sharing experiences with "), false)
		playersDirty = true
		return true
	case strings.HasSuffix(lower, " is sharing experiences with you."):
		if name := utfFold(firstTagContent(raw, 'p', 'n')); name != "" {
			markSharers([]string{name}, true)
		}
		playersDirty = true
		return true
	case strings.Contains(lower, " is no longer sharing experiences with you"):
		if name := utfFold(firstTagContent(raw, 'p', 'n')); name != "" {
			markSharers([]string{name}, false)
		}
		playersDirty = true
		return true
	case strings.HasPrefix(text, "Currently sharing their experiences with you"):
		markSharers(taggedNames(), true)
		playersDirty = true
		return true
	}
	return false
}

func (s *sessionPlayerState) parseBardText(text string) bool {
	if s == nil {
		return false
	}
	text = strings.TrimSpace(text)
	if strings.HasPrefix(text, "* ") || strings.HasPrefix(text, "¥ ") {
		text = strings.TrimSpace(text[2:])
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
	for _, phrase := range phrases {
		if !strings.HasSuffix(text, phrase.suffix) {
			continue
		}
		name := utfFold(strings.TrimSpace(strings.TrimSuffix(text, phrase.suffix)))
		if name == "" {
			return false
		}
		s.mu.Lock()
		player := s.players[name]
		if player == nil {
			player = &Player{Name: name}
			s.players[name] = player
		}
		player.Bard = phrase.bard
		player.Seen, player.Offline, player.LastSeen = true, false, time.Now()
		s.mu.Unlock()
		playersDirty = true
		return true
	}
	return false
}

func (s *sessionPlayerState) observeAppearance(name string, pictID uint16, colors []byte, isNPC bool) {
	if s == nil || name == "" || isNPC {
		return
	}
	s.mu.Lock()
	p := s.players[name]
	if p == nil {
		p = &Player{Name: name}
		s.players[name] = p
	}
	p.PictID = pictID
	if !bytes.Equal(p.Colors, colors) {
		p.Colors = append([]byte(nil), colors...)
	}
	p.LastSeen = time.Now()
	p.Offline = false
	s.mu.Unlock()
	playersDirty = true
}

func (s *sessionPlayerState) markOnScreen(mobiles []frameMobile, descriptors map[uint8]frameDescriptor, now time.Time, selfName string) {
	if s == nil {
		return
	}
	s.mu.Lock()
	for _, mobile := range mobiles {
		d, ok := descriptors[mobile.Index]
		if !ok || d.Type == kDescNPC || d.Name == "" || mobile.Persist {
			continue
		}
		p := s.players[d.Name]
		if p == nil {
			p = &Player{Name: d.Name}
			s.players[d.Name] = p
		}
		self := selfName != "" && strings.EqualFold(d.Name, selfName)
		if len(d.Colors) != 0 {
			p.Sharing = !self && mobile.Colors&styleBold != 0
			p.SameClan = mobile.Colors&styleItalic != 0
		}
		if self {
			p.Sharing = false
			p.Sharee = false
		}
		p.LastSeen = now
		p.Offline = false
		if mobileActuallyVisible(mobile, d) {
			p.LastOnScreen = now
		}
	}
	s.mu.Unlock()
	playersDirty = true
}

func (s *sessionPlayerState) player(name string) (Player, bool) {
	if s == nil {
		return Player{}, false
	}
	s.mu.RLock()
	p, ok := s.players[name]
	if !ok {
		s.mu.RUnlock()
		return Player{}, false
	}
	out := *p
	out.Colors = append([]byte(nil), p.Colors...)
	s.mu.RUnlock()
	return out, true
}

func (s *sessionPlayerState) snapshot() []Player {
	if s == nil {
		return nil
	}
	s.mu.RLock()
	out := make([]Player, 0, len(s.players))
	for _, p := range s.players {
		copy := *p
		copy.Colors = append([]byte(nil), p.Colors...)
		out = append(out, copy)
	}
	s.mu.RUnlock()
	return out
}

// playersSnapshotForSession returns the selected session's live player state
// with app-wide player metadata and character-local labels applied. The
// primary session retains the established player map as its compatibility
// adapter while secondary sessions keep presence and sharing independent.
func playersSnapshotForSession(session *Session) []Player {
	if session == nil {
		return nil
	}
	if session == primarySession {
		return getPlayers()
	}

	profiles := getPlayers()
	profileByName := make(map[string]Player, len(profiles))
	for _, profile := range profiles {
		profileByName[strings.ToLower(profile.Name)] = profile
	}
	result := session.players.snapshot()
	for i := range result {
		key := strings.ToLower(result[i].Name)
		if profile, ok := profileByName[key]; ok {
			mergePlayerProfile(&result[i], profile)
		}
		applySessionPlayerLabel(&result[i], session.characterName())
	}
	return result
}

func mergePlayerProfile(player *Player, profile Player) {
	if player.Race == "" {
		player.Race = profile.Race
	}
	if player.Gender == "" {
		player.Gender = profile.Gender
	}
	if player.Class == "" {
		player.Class = profile.Class
	}
	if player.clan == "" {
		player.clan = profile.clan
	}
	if player.PictID == 0 {
		player.PictID = profile.PictID
	}
	if len(player.Colors) == 0 && len(profile.Colors) != 0 {
		player.Colors = append([]byte(nil), profile.Colors...)
	}
	if player.gmLevel == 0 {
		player.gmLevel = profile.gmLevel
	}
	player.GlobalLabel = profile.GlobalLabel
	player.Bard = profile.Bard
}

func applySessionPlayerLabel(player *Player, character string) {
	player.LocalLabel = characterPlayerLabel(character, player.Name)
	applyPlayerLabel(player)
}

func characterPlayerLabel(character, name string) int {
	for i := range characters {
		if !strings.EqualFold(characters[i].Name, character) {
			continue
		}
		for candidate, label := range characters[i].Labels {
			if strings.EqualFold(candidate, name) {
				return label
			}
		}
		break
	}
	return 0
}

func playerSnapshotForSession(session *Session, name string) (Player, bool) {
	for _, player := range playersSnapshotForSession(session) {
		if strings.EqualFold(player.Name, name) {
			return player, true
		}
	}
	return Player{}, false
}

func (s *sessionPlayerState) reset() {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.players = make(map[string]*Player)
	s.infoQueue = make(map[string]struct{})
	s.lastInfoSent = time.Time{}
	s.whoActive = false
	s.whoScanStarted = time.Time{}
	s.whoLastRequest = time.Time{}
	s.mu.Unlock()
	playersDirty = true
}
