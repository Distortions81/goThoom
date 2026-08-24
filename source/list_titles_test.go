package main

import (
	"testing"

	"gothoom/eui"
)

func TestListTitles(t *testing.T) {
	if got, want := playersWindowTitle(4, 2, 1), "Players   Online: 4   Shared: 2   Sharing: 1"; got != want {
		t.Fatalf("playersWindowTitle() = %q, want %q", got, want)
	}
	if got, want := inventoryWindowTitle(27), "Inventory   Slots: 27/32 (5 free)"; got != want {
		t.Fatalf("inventoryWindowTitle() = %q, want %q", got, want)
	}
	if got, want := inventoryWindowTitle(20), "Inventory   Slots: 20/32"; got != want {
		t.Fatalf("inventoryWindowTitle() = %q, want %q", got, want)
	}
	if got, want := inventoryWindowTitle(32), "Inventory   Slots: 32/32 (pack full)"; got != want {
		t.Fatalf("inventoryWindowTitle() = %q, want %q", got, want)
	}
}

func TestPlayerSharingIndicator(t *testing.T) {
	tests := []struct {
		player Player
		want   string
	}{
		{want: ""},
		{player: Player{Sharee: true}, want: "→"},
		{player: Player{Sharing: true}, want: "←"},
		{player: Player{Sharee: true, Sharing: true}, want: "↔"},
	}
	for _, test := range tests {
		if got := playerSharingIndicator(test.player); got != test.want {
			t.Errorf("playerSharingIndicator(%+v) = %q, want %q", test.player, got, test.want)
		}
	}
	if got, want := playerSharingTooltip(Player{Sharing: true}), "This player shares to you"; got != want {
		t.Errorf("playerSharingTooltip() = %q, want %q", got, want)
	}
}

func TestPlayerGroupEditButtonReservesScrollbarSpace(t *testing.T) {
	const width float32 = 240
	header := makePlayerGroupHeader("Hunting Party", 3, width, 20, 12, true)
	if len(header.Contents) != 2 {
		t.Fatalf("group header has %d children, want label and edit button", len(header.Contents))
	}
	wantWidth := width - eui.ScrollbarWidth()
	if header.Size.X != wantWidth {
		t.Fatalf("group header width = %v, want %v", header.Size.X, wantWidth)
	}
	if got := header.Contents[0].Size.X + header.Contents[1].Size.X; got > wantWidth {
		t.Fatalf("label and edit button width = %v, exceeds available %v", got, wantWidth)
	}
}
