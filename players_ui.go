//go:build !test

package main

import (
	"fmt"
	"sort"

	"github.com/Distortions81/EUI/eui"
)

// playersDropdown holds the bottom-right dropdown listing nearby players.
var playersDropdown *eui.ItemData

// updatePlayersDropdown refreshes the dropdown options with the current
// player list. Each entry includes the player's profession when available.
func updatePlayersDropdown() {
	if playersDropdown == nil {
		return
	}
	ps := getPlayers()
	sort.Slice(ps, func(i, j int) bool { return ps[i].Name < ps[j].Name })
	playersDropdown.Options = playersDropdown.Options[:0]
	for _, p := range ps {
		if p.Name == "" {
			continue
		}
		label := p.Name
		if p.Class != "" {
			label = fmt.Sprintf("%s (%s)", p.Name, p.Class)
		}
		playersDropdown.Options = append(playersDropdown.Options, label)
	}
	playersDropdown.Selected = -1
	playersDropdown.Text = fmt.Sprintf("Players (%d)", len(playersDropdown.Options))
}
