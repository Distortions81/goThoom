package main

import (
	"testing"
	"time"

	"gothoom/eui"
)

func TestPlayerClanAutomaticGroup(t *testing.T) {
	original := gs
	gs = gsdef
	gs.GroupClanMembers = true
	t.Cleanup(func() { gs = original })

	clanmate := Player{Name: "Clanmate", SameClan: true}
	if got := playerDisplayGroup(clanmate, time.Now()); got != "clan" {
		t.Fatalf("clanmate group = %q, want clan", got)
	}
	if got := playerDisplayGroupTitle("clan"); got != "Clan" {
		t.Fatalf("clan group title = %q, want Clan", got)
	}

	gs.PlayerGroups.add("Friends")
	gs.PlayerGroups.assign("clanmate", "Friends")
	if got := playerDisplayGroup(clanmate, time.Now()); got != "custom:Friends" {
		t.Fatalf("custom group should take precedence, got %q", got)
	}
}

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

func TestPlayerShareIndicatorIncludesRightMargin(t *testing.T) {
	if got, want := playerShareIndicatorReservation(12), float32(20); got != want {
		t.Fatalf("share indicator reservation = %v, want %v", got, want)
	}
	if got := playerShareIndicatorReservation(0); got != 0 {
		t.Fatalf("empty share indicator reservation = %v, want 0", got)
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
