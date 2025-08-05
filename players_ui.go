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

	buf := fmt.Sprintf("Players Online: %v", len(ps))
	t, _ := eui.NewText(&eui.ItemData{Text: buf, FontSize: 10, Size: eui.Point{X: 100, Y: 24}})
	playersList.AddItem(t)
	for _, p := range ps {
		if p.Name == "" {
			continue
		}
		t, _ := eui.NewText(&eui.ItemData{Text: p.Name, FontSize: 10, Size: eui.Point{X: 100, Y: 24}})
		playersList.AddItem(t)
	}
}
