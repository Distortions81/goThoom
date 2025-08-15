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
	Colors     []byte `xml:"colors,omitempty"`
	FellWhere  string `xml:"fell_where,omitempty"`
	KillerName string `xml:"killer,omitempty"`
}

const PlayersFile = "Players.xml"

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
	playersMu.Lock()
	for _, p := range pp.Players {
		pr := getPlayer(p.Name)
		pr.Gender = p.Gender
		pr.Class = p.Class
		pr.Clan = p.Clan
		pr.PictID = p.PictID
		if len(p.Colors) > 0 {
			pr.Colors = append(pr.Colors[:0], p.Colors...)
		}
		pr.Dead = p.Dead
		pr.GMLevel = p.GMLevel
		pr.FellWhere = p.FellWhere
		pr.KillerName = p.KillerName
	}
	playersMu.Unlock()
	playersDirty = true
}

func savePlayersPersist() {
	playersMu.RLock()
	list := make([]persistPlayer, 0, len(players))
	names := make([]string, 0, len(players))
	for name := range players {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		p := players[name]
		if p == nil {
			continue
		}
		list = append(list, persistPlayer{
			Name:       p.Name,
			Gender:     p.Gender,
			Class:      p.Class,
			Clan:       p.Clan,
			PictID:     p.PictID,
			Dead:       p.Dead,
			GMLevel:    p.GMLevel,
			Colors:     append([]byte(nil), p.Colors...),
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
