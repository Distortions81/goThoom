package main

import (
	"bytes"
	"strings"
	"sync"
	"time"
)

// Player holds minimal information extracted from BEP messages and descriptors.
type Player struct {
	Name        string
	Race        string
	Gender      string
	Class       string
	clan        string
	PictID      uint16
	Colors      []byte
	IsNPC       bool // entry represents an NPC
	Sharee      bool // we are sharing to this player
	Sharing     bool // player is sharing to us
	gmLevel     int  // parsed from be-who; not rendered
	Friend      bool // marked as friend
	FriendLabel int  // effective label/color (0-7)
	LocalLabel  int  // character-specific label
	GlobalLabel int  // global label
	Blocked     bool // true if player is blocked
	Ignored     bool // true if player is fully ignored
	Dead        bool // parsed from obit messages (future)
	FellWhere   string
	FellTime    time.Time
	KillerName  string
	Bard        bool // true if player is in the Bards' Guild
	SameClan    bool // true if player is in our clan
	beWho       bool // true if player has been enumerated via /be-who
	Seen        bool // true if player has been observed
	// Presence tracking
	LastSeen time.Time // last time we observed any activity/info for this player
	Offline  bool      // explicitly observed as offline/logged off
}

type playerHandler struct {
	owner string
	fn    func(Player)
}

var (
	players              = make(map[string]*Player)
	playersMu            sync.RWMutex
	playerHandlers       []func(Player)
	playerHandlersMu     sync.RWMutex
	pluginPlayerHandlers []playerHandler
)

func getPlayer(name string) *Player {
	playersMu.RLock()
	p, ok := players[name]
	playersMu.RUnlock()
	if ok {
		return p
	}
	playersMu.Lock()
	defer playersMu.Unlock()
	if p, ok = players[name]; ok {
		return p
	}
	p = &Player{Name: name}
	players[name] = p
	playersDirty = true
	return p
}

func updatePlayerAppearance(name string, pictID uint16, colors []byte, isNPC bool) {
	if isNPC {
		return
	}
	playersMu.Lock()
	p, ok := players[name]
	if !ok {
		p = &Player{Name: name}
		players[name] = p
	}
	p.PictID = pictID
	if len(colors) > 0 {
		p.Colors = append(p.Colors[:0], colors...)
	}
	p.IsNPC = false
	// Seeing a player on screen implies they are present now.
	p.LastSeen = time.Now()
	p.Offline = false
	seenChanged := !p.Seen
	p.Seen = true
	prevSC := p.SameClan
	if me, ok := players[playerName]; ok {
		p.SameClan = sameRealClan(me.clan, p.clan)
	}
	playerCopy := *p
	playersMu.Unlock()
	playersDirty = true
	if seenChanged || prevSC != p.SameClan {
		playersPersistDirty = true
	}
	if prevSC != p.SameClan {
		killNameTagCacheFor(name)
	}
	notifyPlayerHandlers(playerCopy)

	if playerName != "" && strings.EqualFold(name, playerName) {
		changed := false
		for i := range characters {
			if strings.EqualFold(characters[i].Name, name) {
				if characters[i].PictID != pictID {
					characters[i].PictID = pictID
					changed = true
				}
				if len(colors) > 0 && !bytes.Equal(characters[i].Colors, colors) {
					characters[i].Colors = append(characters[i].Colors[:0], colors...)
					changed = true
				}
				if changed {
					saveCharacters()
				}
				break
			}
		}
	}
}

func getPlayers() []Player {
	playersMu.RLock()
	defer playersMu.RUnlock()
	out := make([]Player, 0, len(players))
	for _, p := range players {
		out = append(out, *p)
	}
	return out
}

func notifyPlayerHandlers(p Player) {
	playerHandlersMu.RLock()
	handlers := make([]func(Player), 0, len(playerHandlers)+len(pluginPlayerHandlers))
	handlers = append(handlers, playerHandlers...)
	for _, h := range pluginPlayerHandlers {
		handlers = append(handlers, h.fn)
	}
	playerHandlersMu.RUnlock()
	for _, fn := range handlers {
		go fn(p)
	}
}

func clearBeWhoFlags() {
	playersMu.Lock()
	for _, p := range players {
		p.beWho = false
	}
	playersMu.Unlock()
}
