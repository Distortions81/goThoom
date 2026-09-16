package main

import (
	"fmt"
	"testing"
	"time"
)

func TestInputCompletionCommandsAndArguments(t *testing.T) {
	candidates := inputCompletionCandidates{
		commands: []string{"/equip", "/examine", "/setting", "/simulate", "say"},
		items:    []string{"Healing Potion", "Sunstone"},
		chat:     []string{"Agratis", "Healing Potion", "Sunstone"},
	}
	for _, test := range []struct {
		text string
		want string
	}{
		{text: "/equ", want: "ip"},
		{text: "/sim", want: "ulate"},
		{text: "sa", want: "y"},
		{text: "/equip sun", want: "stone"},
		{text: "/equip healing p", want: "otion"},
	} {
		if got := inputCompletionSuffix(test.text, len([]rune(test.text)), candidates); got != test.want {
			t.Errorf("completion for %q = %q, want %q", test.text, got, test.want)
		}
	}
}

func TestInputCompletionChatPlayersAndItems(t *testing.T) {
	candidates := inputCompletionCandidates{
		commands: []string{"/equip"},
		items:    []string{"Healing Potion", "Sunstone"},
		chat:     []string{"Agratis", "Healing Potion", "Sunstone"},
	}
	for _, test := range []struct {
		text string
		want string
	}{
		{text: "Hello Agr", want: "atis"},
		{text: "Use the sun", want: "stone"},
		{text: "I need a healing p", want: "otion"},
		{text: "No match", want: ""},
	} {
		if got := inputCompletionSuffix(test.text, len([]rune(test.text)), candidates); got != test.want {
			t.Errorf("completion for %q = %q, want %q", test.text, got, test.want)
		}
	}
}

func TestInputCompletionOnlyPredictsAtEnd(t *testing.T) {
	candidates := inputCompletionCandidates{commands: []string{"/think", "/thinkclan"}, chat: []string{"Agratis"}}
	if got := inputCompletionSuffix("Agr", 2, candidates); got != "" {
		t.Fatalf("mid-line completion = %q, want none", got)
	}
	if got := inputCompletionSuffix("/think", len("/think"), candidates); got != "" {
		t.Fatalf("exact command completion = %q, want none", got)
	}
}

func TestInputPredictionShowsCommandSyntaxWithoutMakingItCompletion(t *testing.T) {
	candidates := inputCompletionCandidates{commands: []string{"/give", "/money"}}
	for _, test := range []struct {
		text string
		want string
	}{
		{text: "/gi", want: "ve"},
		{text: "/give", want: " <person> <amount>"},
		{text: "/give ", want: "<person> <amount>"},
		{text: "/give Agratis", want: ""},
		{text: "/money", want: ""},
	} {
		cursor := len([]rune(test.text))
		if got := inputPredictionSuffix(test.text, cursor, candidates); got != test.want {
			t.Errorf("prediction for %q = %q, want %q", test.text, got, test.want)
		}
	}
	if got := inputCompletionSuffix("/give", len("/give"), candidates); got != "" {
		t.Fatalf("Tab completion would insert syntax %q", got)
	}
	if got := inputPredictionSuffix("/give", 3, candidates); got != "" {
		t.Fatalf("mid-line command prediction = %q, want none", got)
	}
}

func TestCompletionPreservesNormalizationAndUnicode(t *testing.T) {
	candidates := []string{" healing potion ", "Healing Potion", "Élodie", "élodie", "", "SUNSTONE", "Sunstone"}
	for _, test := range []struct{ text, want string }{
		{"I need healing p", "otion"}, {"Hello Élo", "die"},
		{"Hello Élodie", ""}, {"sun", "STONE"}, {"no match", ""},
	} {
		if got := completionAtWordBoundary(test.text, candidates); got != test.want {
			t.Errorf("completion for %q = %q, want %q", test.text, got, test.want)
		}
	}
}

func TestRecentPlayerCompletionCandidatesUseNewest32(t *testing.T) {
	session := mustNewSession(1)
	base := time.Unix(1_000, 0)
	for index := range 40 {
		name := fmt.Sprintf("Player %02d", index)
		session.players.players[name] = &Player{Name: name, LastSeen: base.Add(time.Duration(index) * time.Second)}
	}
	session.players.players["Recent NPC"] = &Player{Name: "Recent NPC", IsNPC: true, LastSeen: base.Add(time.Hour)}

	names := recentPlayerCompletionNamesForSession(session, inputCompletionCandidateLimit)
	if len(names) != inputCompletionCandidateLimit {
		t.Fatalf("recent player candidates = %d, want %d", len(names), inputCompletionCandidateLimit)
	}
	if names[0] != "Player 39" || names[len(names)-1] != "Player 08" {
		t.Fatalf("recent player range = %q..%q, want Player 39..Player 08", names[0], names[len(names)-1])
	}
	for _, name := range names {
		if name == "Recent NPC" {
			t.Fatal("NPC included in player completion candidates")
		}
	}
}

func TestInventoryCompletionCandidatesRemainUnlimited(t *testing.T) {
	state := newInventoryState()
	for index := range 40 {
		state.add(uint16(index+1), -1, fmt.Sprintf("Item %02d", index), false)
	}
	if names := state.completionNames(); len(names) != 40 {
		t.Fatalf("inventory completion candidates = %d, want all 40", len(names))
	}
}
