package main

import "testing"

// helper to wrap message into BEPP prefix without pn tag
func fallenLine(prefix, msg string) []byte {
	b := []byte{0xC2, prefix[0], prefix[1]}
	b = append(b, []byte(msg)...)
	b = append(b, 0) // simulate NUL terminator
	return b
}

func TestDecodeFallenWithoutTag(t *testing.T) {
	primarySession.players.reset()
	raw := fallenLine("hf", "Bob has fallen")
	if got := decodeBEPP(raw); got != "Bob has fallen" {
		t.Fatalf("decodeBEPP returned %q", got)
	}
	player, ok := primarySession.players.player("Bob")
	dead := ok && player.Dead
	if !dead {
		t.Errorf("player not marked dead")
	}
}

func TestDecodeUnfallenWithoutTag(t *testing.T) {
	primarySession.players.reset()
	primarySession.players.players["Bob"] = &Player{Name: "Bob", Dead: true}
	raw := fallenLine("nf", "Bob is no longer fallen")
	if got := decodeBEPP(raw); got != "Bob is no longer fallen" {
		t.Fatalf("decodeBEPP returned %q", got)
	}
	player, _ := primarySession.players.player("Bob")
	dead := player.Dead
	if dead {
		t.Errorf("player still marked dead")
	}
}

func TestDecodeSelfFallen(t *testing.T) {
	primarySession.players.reset()
	primarySession.setCharacterName("Hero")
	primarySession.players.players["Hero"] = &Player{Name: "Hero"}
	raw := fallenLine("hf", "You have fallen")
	if got := decodeBEPP(raw); got != "You have fallen" {
		t.Fatalf("decodeBEPP returned %q", got)
	}
	player, _ := primarySession.players.player("Hero")
	dead := player.Dead
	if !dead {
		t.Errorf("player not marked dead")
	}
}

func TestDecodeSelfUnfallen(t *testing.T) {
	primarySession.players.reset()
	primarySession.setCharacterName("Hero")
	primarySession.players.players["Hero"] = &Player{Name: "Hero", Dead: true}
	raw := fallenLine("nf", "You are no longer fallen")
	if got := decodeBEPP(raw); got != "You are no longer fallen" {
		t.Fatalf("decodeBEPP returned %q", got)
	}
	player, _ := primarySession.players.player("Hero")
	dead := player.Dead
	if dead {
		t.Errorf("player still marked dead")
	}
}
