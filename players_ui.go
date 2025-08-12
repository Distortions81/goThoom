//go:build !test

package main

import (
	"fmt"
	"sort"

	"go_client/eui"
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
	t, _ := eui.NewText()
	t.Text = buf
	t.FontSize = 10
	t.Size = eui.Point{X: 100, Y: 24}
	playersList.AddItem(t)
	for _, p := range exiles {
		t, _ = eui.NewText()
		t.Text = p.Name
		t.FontSize = 10
		t.Size = eui.Point{X: 100, Y: 24}
		playersList.AddItem(t)
	}
	playersDirty = true
}
