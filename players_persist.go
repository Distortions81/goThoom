package main

import (
	"encoding/xml"
	"os"
	"path/filepath"
	"sort"
)

type persistPlayers struct {
	XMLName xml.Name        `xml:"players"`
	Players []persistPlayer `xml:"player"`
}

type persistPlayer struct {
	Name       string `xml:"name,attr"`
	Gender     string `xml:"gender,attr"`
	Class      string `xml:"class,attr"`
	Clan       string `xml:"clan,attr"`
	PictID     uint16 `xml:"pict,attr"`
	Dead       bool   `xml:"dead,attr"`
	GMLevel    int    `xml:"gm,attr,omitempty"`
	ColorsHex  string `xml:"colors,attr,omitempty"` // hex of [count][colors...]
	FellWhere  string `xml:"fell_where,omitempty"`
	KillerName string `xml:"killer,omitempty"`
}

const PlayersFile = "GT_Players.xml"

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
	if err := xml.Unmarshal(data, &pp); err != nil {
		return
	}
	if len(pp.Players) == 0 {
		return
	}
	for _, p := range pp.Players {
		if p.Name == "" {
			continue
		}
		pr := getPlayer(p.Name)
		playersMu.Lock()
		pr.Gender = p.Gender
		pr.Class = p.Class
		pr.Clan = p.Clan
		pr.PictID = p.PictID
		// Decode ColorsHex: optional hex of [count][bytes]
		if p.ColorsHex != "" {
			if b, ok := decodeHex(p.ColorsHex); ok && len(b) > 0 {
				count := int(b[0])
				if count > 0 && 1+count <= len(b) {
					pr.Colors = append(pr.Colors[:0], b[1:1+count]...)
				} else {
					// if malformed, fall back to raw tail
					pr.Colors = append(pr.Colors[:0], b...)
				}
			}
		}
		pr.Dead = p.Dead
		pr.GMLevel = p.GMLevel
		pr.FellWhere = p.FellWhere
		pr.KillerName = p.KillerName
		playersMu.Unlock()
	}
	playersDirty = true
}

func savePlayersPersist() {
	// Ensure data directory exists
	_ = os.MkdirAll(dataDirPath, 0o755)
	playersMu.RLock()
	list := make([]persistPlayer, 0, len(players))
	names := make([]string, 0, len(players))
	for name, p := range players {
		if p == nil || p.IsNPC || name == "" {
			continue
		}
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		p := players[name]
		if p == nil {
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
		list = append(list, persistPlayer{
			Name:       p.Name,
			Gender:     p.Gender,
			Class:      p.Class,
			Clan:       p.Clan,
			PictID:     p.PictID,
			Dead:       p.Dead,
			GMLevel:    p.GMLevel,
			ColorsHex:  hex,
			FellWhere:  p.FellWhere,
			KillerName: p.KillerName,
		})
	}
	playersMu.RUnlock()

	pp := persistPlayers{Players: list}
	data, err := xml.MarshalIndent(pp, "", "  ")
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
