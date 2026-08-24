package main

import (
	"testing"

	"gothoom/eui"
)

func TestDefaultWindowLayoutMatchesReferenceDesktop(t *testing.T) {
	if gsdef.WindowWidth != 2409 || gsdef.WindowHeight != 1404 {
		t.Fatalf("default application size = %dx%d", gsdef.WindowWidth, gsdef.WindowHeight)
	}
	tests := []struct {
		name string
		got  WindowState
		want WindowState
	}{
		{name: "game", got: gsdef.GameWindow, want: WindowState{Open: true, Position: WindowPoint{X: 468}, Size: WindowPoint{X: 936, Y: 948}}},
		{name: "inventory", got: gsdef.InventoryWindow, want: WindowState{Open: true, Position: WindowPoint{Y: 87}, Size: WindowPoint{X: 438, Y: 444}}},
		{name: "players", got: gsdef.PlayersWindow, want: WindowState{Open: true, Position: WindowPoint{X: 1436}, Size: WindowPoint{X: 484, Y: 526}}},
		{name: "messages", got: gsdef.MessagesWindow, want: WindowState{Open: true, Position: WindowPoint{X: 1, Y: 534}, Size: WindowPoint{X: 438, Y: 417}}},
		{name: "chat", got: gsdef.ChatWindow, want: WindowState{Open: true, Position: WindowPoint{X: 1429, Y: 529}, Size: WindowPoint{X: 489, Y: 420}}},
		{name: "movie", got: gsdef.MovieWindow, want: WindowState{Position: WindowPoint{X: 350, Y: 117}, Size: WindowPoint{X: 1076, Y: 96}}},
	}
	for _, test := range tests {
		if test.got != test.want {
			t.Errorf("%s default = %+v, want %+v", test.name, test.got, test.want)
		}
	}
}

func TestResetSavedWindowSettings(t *testing.T) {
	original := gs
	defer func() { gs = original }()

	gs.GameWindow = WindowState{Open: false, Position: WindowPoint{X: 5, Y: 6}, Size: WindowPoint{X: 7, Y: 8}}
	gs.InventoryWindow = WindowState{Open: false}
	gs.PlayersWindow = WindowState{Open: false}
	gs.MessagesWindow = WindowState{Open: false}
	gs.ChatWindow = WindowState{Open: false}
	gs.MovieWindow = WindowState{Open: true, Position: WindowPoint{X: 9, Y: 10}, Size: WindowPoint{X: 11, Y: 12}}
	gs.WindowZones = map[string]eui.WindowZoneState{"Settings": {Zoned: true}}

	resetSavedWindowSettings()

	if gs.GameWindow != gsdef.GameWindow || gs.InventoryWindow != gsdef.InventoryWindow ||
		gs.PlayersWindow != gsdef.PlayersWindow || gs.MessagesWindow != gsdef.MessagesWindow ||
		gs.ChatWindow != gsdef.ChatWindow || gs.MovieWindow != gsdef.MovieWindow {
		t.Fatal("window states were not restored to defaults")
	}
	if len(gs.WindowZones) != 0 {
		t.Fatalf("window zones = %#v, want empty", gs.WindowZones)
	}
}
