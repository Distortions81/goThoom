//go:build !test

package main

import (
	"fmt"
	"sort"

	"github.com/Distortions81/EUI/eui"
)

var playersWin *eui.WindowData
var playersList *eui.ItemData

func updatePlayersWindow() {
	if playersList == nil {
		return
	}
	ps := getPlayers()
	sort.Slice(ps, func(i, j int) bool { return ps[i].Name < ps[j].Name })
	playersList.Contents = playersList.Contents[:0]

	var exiles, npcs []Player
	for _, p := range ps {
		if p.Name == "" {
			continue
		}
		if p.IsNPC {
			npcs = append(npcs, p)
		} else {
			exiles = append(exiles, p)
		}
	}

	buf := fmt.Sprintf("Players Online: %v", len(exiles))
	t, _ := eui.NewText(&eui.ItemData{ItemType: eui.ITEM_TEXT, Text: buf, FontSize: 10, Size: eui.Point{X: 100, Y: 24}})
	playersList.AddItem(t)
	for _, p := range exiles {
		t, _ := eui.NewText(&eui.ItemData{Text: p.Name, FontSize: 10, Size: eui.Point{X: 100, Y: 24}})
		playersList.AddItem(t)
	}

	if len(npcs) > 0 {
		buf = fmt.Sprintf("NPCs: %v", len(npcs))
		t, _ = eui.NewText(&eui.ItemData{ItemType: eui.ITEM_TEXT, Text: buf, FontSize: 10, Size: eui.Point{X: 100, Y: 24}})
		playersList.AddItem(t)
		for _, p := range npcs {
			t, _ := eui.NewText(&eui.ItemData{Text: p.Name, FontSize: 10, Size: eui.Point{X: 100, Y: 24}})
			playersList.AddItem(t)
		}
	}
	if playersWin != nil {
		playersWin.Refresh()
	}
}
