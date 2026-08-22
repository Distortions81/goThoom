package main

import (
	"testing"

	"gothoom/eui"
)

func TestResetSavedWindowSettings(t *testing.T) {
	original := gs
	defer func() { gs = original }()

	gs.GameWindow = WindowState{Open: false, Position: WindowPoint{X: 5, Y: 6}, Size: WindowPoint{X: 7, Y: 8}}
	gs.InventoryWindow = WindowState{Open: false}
	gs.PlayersWindow = WindowState{Open: false}
	gs.MessagesWindow = WindowState{Open: false}
	gs.ChatWindow = WindowState{Open: false}
	gs.WindowZones = map[string]eui.WindowZoneState{"Settings": {Zoned: true}}

	resetSavedWindowSettings()

	if gs.GameWindow != gsdef.GameWindow || gs.InventoryWindow != gsdef.InventoryWindow ||
		gs.PlayersWindow != gsdef.PlayersWindow || gs.MessagesWindow != gsdef.MessagesWindow ||
		gs.ChatWindow != gsdef.ChatWindow {
		t.Fatal("window states were not restored to defaults")
	}
	if len(gs.WindowZones) != 0 {
		t.Fatalf("window zones = %#v, want empty", gs.WindowZones)
	}
}
