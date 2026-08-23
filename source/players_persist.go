package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type persistPlayers struct {
	Players    []persistPlayer `json:"players"`
	LabelNames []string        `json:"label_names,omitempty"`
}

type persistPlayer struct {
	Name        string `json:"name"`
	Gender      string `json:"gender"`
	Class       string `json:"class"`
	Clan        string `json:"clan"`
	PictID      uint16 `json:"pict"`
	Dead        bool   `json:"dead"`
	GMLevel     int    `json:"gm,omitempty"`
	Friend      bool   `json:"friend,omitempty"`
	GlobalLabel int    `json:"friend_label,omitempty"`
	Blocked     bool   `json:"blocked,omitempty"`
	Ignored     bool   `json:"ignored,omitempty"`
	Bard        bool   `json:"bard,omitempty"`
	SameClan    bool   `json:"same_clan,omitempty"`
	BeWho       bool   `json:"bewho,omitempty"`
	Seen        bool   `json:"seen,omitempty"`
	ColorsHex   string `json:"colors,omitempty"` // hex of [count][colors...]
	FellWhere   string `json:"fell_where,omitempty"`
	FellTime    int64  `json:"fell_time,omitempty"`
	KillerName  string `json:"killer,omitempty"`
	LastSeen    int64  `json:"last_seen,omitempty"`
}

const PlayersFile = "GT_Players.json"

var (
	lastPlayersSave     = lastSettingsSave
	playersPersistDirty bool
)

func loadPlayersPersist() {
	path := filepath.Join(dataDirPath, PlayersFile)
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var pp persistPlayers
	if err := json.Unmarshal(data, &pp); err != nil {
		return
	}
	if len(pp.LabelNames) > 0 {
		labelNames = append(labelNames[:0], pp.LabelNames...)
		for len(labelNames) < len(labelColors) {
			labelNames = append(labelNames, "")
		}
	}
	if len(pp.Players) == 0 {
		return
	}
	for _, p := range pp.Players {
		if p.Name == "" || strings.HasPrefix(p.Name, agratisPrefix) {
			continue
		}
		last := time.Unix(p.LastSeen, 0)
		pr := getPlayer(p.Name)
		playersMu.Lock()
		pr.Gender = p.Gender
		pr.Class = p.Class
		pr.clan = p.Clan
		pr.PictID = p.PictID
		// Decode ColorsHex: optional hex of [count][bytes]
		if p.ColorsHex != "" {
			if b, ok := decodeHex(p.ColorsHex); ok && len(b) > 0 {
				count := int(b[0])
				if count > 0 && 1+count <= len(b) {
					pr.Colors = append([]byte(nil), b[1:1+count]...)
				} else {
					// if malformed, fall back to raw tail
					pr.Colors = append([]byte(nil), b...)
				}
			}
		}
		pr.Dead = p.Dead
		pr.gmLevel = p.GMLevel
		pr.GlobalLabel = p.GlobalLabel
		applyPlayerLabel(pr)
		pr.Bard = p.Bard
		pr.SameClan = p.SameClan
		// Presence is session state. Saved records start offline until the
		// server confirms them in the next /be-who enumeration.
		pr.beWho = false
		pr.Offline = true
		pr.Seen = p.Seen
		pr.FellWhere = p.FellWhere
		if p.FellTime != 0 {
			pr.FellTime = time.Unix(p.FellTime, 0)
		} else {
			pr.FellTime = time.Time{}
		}
		pr.KillerName = p.KillerName
		if pr.LastSeen.IsZero() && p.LastSeen > 0 {
			pr.LastSeen = last
		}
		playersMu.Unlock()
	}
	playersDirty = true
}

func savePlayersPersist() {
	if isWASM {
		// Skip persistence in WASM.
		return
	}
	// Ensure data directory exists
	_ = os.MkdirAll(dataDirPath, 0o755)
	playersMu.RLock()
	list := make([]persistPlayer, 0, len(players))
	names := make([]string, 0, len(players))
	for name, p := range players {
		if p == nil || p.IsNPC || name == "" || strings.HasPrefix(name, agratisPrefix) {
			continue
		}
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		p := players[name]
		if p == nil || strings.HasPrefix(name, agratisPrefix) {
			continue
		}
		// Build colors payload as [count][colors...]
		var hex string
		if len(p.Colors) > 0 {
			buf := make([]byte, 1+len(p.Colors))
			if len(p.Colors) > 255 {
				buf[0] = 255
				copy(buf[1:], p.Colors[:255])
			} else {
				buf[0] = byte(len(p.Colors))
				copy(buf[1:], p.Colors)
			}
			hex = encodeHex(buf)
		}
		var ft int64
		if !p.FellTime.IsZero() {
			ft = p.FellTime.Unix()
		}
		var lastSeen int64
		if !p.LastSeen.IsZero() {
			lastSeen = p.LastSeen.Unix()
		}
		friend := p.GlobalLabel > 0 && p.GlobalLabel < 6
		blocked := p.GlobalLabel == 6
		ignored := p.GlobalLabel == 7
		list = append(list, persistPlayer{
			Name:        p.Name,
			Gender:      p.Gender,
			Class:       p.Class,
			Clan:        p.clan,
			PictID:      p.PictID,
			Dead:        p.Dead,
			GMLevel:     p.gmLevel,
			Friend:      friend,
			GlobalLabel: p.GlobalLabel,
			Blocked:     blocked,
			Ignored:     ignored,
			Bard:        p.Bard,
			SameClan:    p.SameClan,
			BeWho:       p.beWho,
			Seen:        p.Seen,
			ColorsHex:   hex,
			FellWhere:   p.FellWhere,
			FellTime:    ft,
			KillerName:  p.KillerName,
			LastSeen:    lastSeen,
		})
	}
	playersMu.RUnlock()

	pp := persistPlayers{Players: list, LabelNames: labelNames}
	data, err := json.MarshalIndent(pp, "", "  ")
	if err != nil {
		return
	}
	path := filepath.Join(dataDirPath, PlayersFile)
	_ = os.WriteFile(path, data, 0644)
}

// Minimal hex helpers (lowercase, no 0x prefix) to avoid extra deps.
func encodeHex(b []byte) string {
	const hexdigits = "0123456789abcdef"
	out := make([]byte, len(b)*2)
	for i, v := range b {
		out[2*i] = hexdigits[v>>4]
		out[2*i+1] = hexdigits[v&0x0f]
	}
	return string(out)
}

func decodeHex(s string) ([]byte, bool) {
	if len(s)%2 != 0 {
		return nil, false
	}
	out := make([]byte, len(s)/2)
	for i := 0; i < len(out); i++ {
		a := fromHex(s[2*i])
		b := fromHex(s[2*i+1])
		if a < 0 || b < 0 {
			return nil, false
		}
		out[i] = byte(a<<4 | b)
	}
	return out, true
}

func fromHex(c byte) int {
	switch {
	case '0' <= c && c <= '9':
		return int(c - '0')
	case 'a' <= c && c <= 'f':
		return int(c - 'a' + 10)
	case 'A' <= c && c <= 'F':
		return int(c - 'A' + 10)
	default:
		return -1
	}
}
