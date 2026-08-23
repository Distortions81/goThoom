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
		{name: "game", got: gsdef.GameWindow, want: WindowState{Open: true, Position: WindowPoint{X: 457.5}, Size: WindowPoint{X: 1398, Y: 1404}}},
		{name: "inventory", got: gsdef.InventoryWindow, want: WindowState{Open: true, Position: WindowPoint{Y: 87}, Size: WindowPoint{X: 455, Y: 858}}},
		{name: "players", got: gsdef.PlayersWindow, want: WindowState{Open: true, Position: WindowPoint{X: 1860}, Size: WindowPoint{X: 549, Y: 932}}},
		{name: "messages", got: gsdef.MessagesWindow, want: WindowState{Open: true, Position: WindowPoint{Y: 947}, Size: WindowPoint{X: 455, Y: 454}}},
		{name: "chat", got: gsdef.ChatWindow, want: WindowState{Open: true, Position: WindowPoint{X: 1861, Y: 936}, Size: WindowPoint{X: 546, Y: 467}}},
		{name: "movie", got: gsdef.MovieWindow, want: WindowState{Position: WindowPoint{X: 619}, Size: WindowPoint{X: 1076, Y: 96}}},
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
