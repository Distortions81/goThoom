//go:build !test

package main

import (
	"fmt"
	"sort"

	"github.com/Distortions81/EUI/eui"
)

var playersWin *eui.WindowData
var playersList *eui.ItemData
var playersDirty bool

func updatePlayersWindow() {
	if playersList == nil {
		return
	}
	ps := getPlayers()
	sort.Slice(ps, func(i, j int) bool { return ps[i].Name < ps[j].Name })
	for i := range playersList.Contents {
		playersList.Contents[i] = nil
	}
	playersList.Contents = playersList.Contents[:0]

	var exiles []Player
	for _, p := range ps {
		if p.Name == "" || p.IsNPC {
			continue
		}
		exiles = append(exiles, p)
	}

	buf := fmt.Sprintf("Players Online: %v", len(exiles))
	t, err := eui.NewText(&eui.ItemData{ItemType: eui.ITEM_TEXT, Text: buf, FontSize: 10, Size: eui.Point{X: 100, Y: 24}})
	if err != nil {
		logError("failed to create players online text: %v", err)
		return
	}
	playersList.AddItem(t)
	for _, p := range exiles {
		t, err = eui.NewText(&eui.ItemData{Text: p.Name, FontSize: 10, Size: eui.Point{X: 100, Y: 24}})
		if err != nil {
			logError("failed to create player name text: %v", err)
			continue
		}
		playersList.AddItem(t)
	}
	playersDirty = true
}
