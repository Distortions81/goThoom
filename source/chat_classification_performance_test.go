package main

import (
	"fmt"
	"testing"
)

func TestScriptChatClassificationPreservesPlayerKinds(t *testing.T) {
	oldPlayers, oldName := players, playerName
	oldDescriptors := primarySession.draw.current.descriptors
	t.Cleanup(func() {
		players, playerName = oldPlayers, oldName
		primarySession.draw.current.descriptors = oldDescriptors
	})
	players = map[string]*Player{
		"Alice":   {Name: "Alice"},
		"Bart":    {Name: "Bart", IsNPC: true},
		"Foreign": {Name: "Foreign"},
		"mIxEd":   {Name: "mIxEd"},
	}
	playerName = "Hero"
	s := mustNewSession(2)
	s.setCharacterName("Hero")
	s.players.players = map[string]*Player{
		"Alice":   {Name: "Alice"},
		"Bart":    {Name: "Bart"},
		"Camille": {Name: "Camille", IsNPC: true},
		"mIxEd":   {Name: "mIxEd"},
	}
	descriptors := map[uint8]frameDescriptor{1: {Index: 1, Name: "Descriptor NPC", Type: kDescNPC}}
	primarySession.draw.current.descriptors = descriptors
	s.draw.current.descriptors = descriptors
	for _, test := range []struct {
		name    string
		session *Session
		speaker string
		kind    int
	}{
		{"global player", nil, "Alice", ChatPlayer | ChatOther},
		{"global npc", nil, "Bart", ChatNPC | ChatOther},
		{"global case folding", nil, "Mixed", ChatPlayer | ChatOther},
		{"global unknown", nil, "Rat", ChatCreature | ChatOther},
		{"global self", nil, "Hero", ChatPlayer | ChatSelf},
		{"global descriptor npc", nil, "Descriptor NPC", ChatNPC | ChatOther},
		{"session player", s, "Alice", ChatPlayer | ChatOther},
		{"session overrides profile kind", s, "Bart", ChatPlayer | ChatOther},
		{"session npc", s, "Camille", ChatNPC | ChatOther},
		{"session case folding", s, "Mixed", ChatPlayer | ChatOther},
		{"another session's profile", s, "Foreign", ChatCreature | ChatOther},
		{"session unknown", s, "Rat", ChatCreature | ChatOther},
		{"session self", s, "Hero", ChatPlayer | ChatSelf},
		{"session descriptor npc", s, "Descriptor NPC", ChatNPC | ChatOther},
	} {
		t.Run(test.name, func(t *testing.T) {
			event := classifyScriptChatForSession(test.session, test.speaker+" says, \"Hello.\"")
			if event.Kinds != ChatAny|test.kind {
				t.Fatalf("chat kinds = %d, want %d", event.Kinds, ChatAny|test.kind)
			}
		})
	}
}

func BenchmarkClassifyScriptChat(b *testing.B) {
	oldPlayers := players
	b.Cleanup(func() { players = oldPlayers })
	players = make(map[string]*Player, 1000)
	s := mustNewSession(2)
	for i := range 1000 {
		name := fmt.Sprintf("Player%04d", i)
		players[name] = &Player{Name: name, Colors: []byte{1, 2, 3, 4}}
		s.players.players[name] = &Player{Name: name, Colors: []byte{1, 2, 3, 4}}
	}
	for _, scope := range []struct {
		name    string
		session *Session
	}{{"Global", nil}, {"Session", s}} {
		b.Run(scope.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				event := classifyScriptChatForSession(scope.session, "Player0500 says, \"Hello.\"")
				if event.Kinds&ChatPlayer == 0 {
					b.Fatal("known speaker was not classified as a player")
				}
			}
		})
	}
}
