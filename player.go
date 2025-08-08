package main

import "sync"

// Player holds minimal information extracted from BEP messages and descriptors.
type Player struct {
	Name    string
	Race    string
	Gender  string
	Class   string
	Clan    string
	PictID  uint16
	Colors  []byte
	Sharee  bool // player is sharing to us
	Sharing bool // we are sharing to player
}

var (
	players   = make(map[string]*Player)
	playersMu sync.RWMutex
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
	updatePlayersWindow()
	return p
}

func updatePlayerAppearance(name string, pictID uint16, colors []byte) {
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
	playersMu.Unlock()
	updatePlayersWindow()
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
