package main

import (
	"bytes"
	"strings"
	"sync"
	"time"
)

const recentPlayerWindow = 2 * time.Minute

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
	// LastOnScreen tracks visible mobile descriptors and is not persisted.
	LastOnScreen time.Time
	Offline      bool // explicitly observed as offline/logged off
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
	scriptPlayerHandlers []playerHandler
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
	paletteChanged := !bytes.Equal(p.Colors, colors)
	appearanceChanged := p.PictID != pictID || paletteChanged
	p.PictID = pictID
	if paletteChanged {
		// Publish a new backing array so draw callers can safely retain the
		// previous palette after releasing playersMu. An empty palette is a
		// real update: it means the player removed their custom clothing.
		p.Colors = append([]byte(nil), colors...)
	}
	p.IsNPC = false
	// A descriptor confirms presence, but not that the mobile is actually in
	// the viewport. markPlayersOnScreen records LastOnScreen after positions
	// are available.
	now := time.Now()
	p.LastSeen = now
	p.Offline = false
	if p.Dead {
		p.Dead = false
		p.FellWhere = ""
		p.KillerName = ""
		p.FellTime = time.Time{}
	}
	seenChanged := !p.Seen
	p.Seen = true
	prevSC := p.SameClan
	if me, ok := players[playerName]; ok {
		p.SameClan = sameRealClan(me.clan, p.clan)
	}
	playerCopy := *p
	playerCopy.Colors = append([]byte(nil), p.Colors...)
	playersMu.Unlock()
	playersDirty = true
	if seenChanged || appearanceChanged || prevSC != playerCopy.SameClan {
		playersPersistDirty = true
	}
	if prevSC != playerCopy.SameClan {
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
				if !bytes.Equal(characters[i].Colors, colors) {
					characters[i].Colors = append([]byte(nil), colors...)
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

func mobileActuallyVisible(mobile frameMobile, desc frameDescriptor) bool {
	if mobile.Persist {
		return false
	}
	half := mobileSizeFunc(desc.PictID) / 2
	return int(mobile.H)+half >= -fieldCenterX && int(mobile.H)-half <= fieldCenterX &&
		int(mobile.V)+half >= -fieldCenterY && int(mobile.V)-half <= fieldCenterY
}

// markPlayersOnScreen refreshes recency only for mobiles whose sprites really
// overlap the viewport. Other descriptors may be present elsewhere on the
// snell and still count as online without entering the recent group.
func markPlayersOnScreen(mobiles []frameMobile, descriptors map[uint8]frameDescriptor, now time.Time) {
	changed := false
	playersMu.Lock()
	for _, mobile := range mobiles {
		d, ok := descriptors[mobile.Index]
		if !ok || d.Type == kDescNPC || d.Name == "" || !mobileActuallyVisible(mobile, d) {
			continue
		}
		p, ok := players[d.Name]
		if !ok {
			p = &Player{Name: d.Name}
			players[d.Name] = p
			changed = true
		}
		age := now.Sub(p.LastOnScreen)
		wasRecent := !p.Offline && !p.LastOnScreen.IsZero() && age >= 0 && age < recentPlayerWindow
		if !wasRecent || p.Offline {
			changed = true
		}
		p.LastSeen = now
		p.LastOnScreen = now
		p.Offline = false
	}
	playersMu.Unlock()
	if changed {
		playersDirty = true
	}
}

func getPlayers() []Player {
	playersMu.RLock()
	defer playersMu.RUnlock()
	out := make([]Player, 0, len(players))
	for _, p := range players {
		playerCopy := *p
		playerCopy.Colors = append([]byte(nil), p.Colors...)
		out = append(out, playerCopy)
	}
	return out
}

// playerColorsForDescriptor returns the effective immutable palette for a
// descriptor. Player appearance updates replace the Colors backing array, so
// the returned slice remains valid after playersMu is released.
func playerColorsForDescriptor(d frameDescriptor) []byte {
	colors := d.Colors
	playersMu.RLock()
	if p, ok := players[d.Name]; ok && len(p.Colors) > 0 {
		colors = p.Colors
	}
	playersMu.RUnlock()
	return colors
}

func notifyPlayerHandlers(p Player) {
	playerHandlersMu.RLock()
	base := append([]func(Player){}, playerHandlers...)
	plug := append([]playerHandler{}, scriptPlayerHandlers...)
	playerHandlersMu.RUnlock()
	for _, fn := range base {
		go fn(p)
	}
	for _, h := range plug {
		scriptLogEvent(h.owner, "PlayerHandler", p.Name)
		go h.fn(p)
	}
}

// beginBeWhoScan makes the server's new /be-who enumeration authoritative.
// Entries returned by the scan are marked online again by parseBackendWho.
func beginBeWhoScan() {
	playersMu.Lock()
	for _, p := range players {
		p.beWho = false
		p.Offline = true
	}
	playersMu.Unlock()
}
