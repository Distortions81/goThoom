//go:build !test

package main

import (
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
	for _, p := range ps {
		t, _ := eui.NewText(&eui.ItemData{Text: p.Name})
		playersList.AddItem(t)
	}
}
