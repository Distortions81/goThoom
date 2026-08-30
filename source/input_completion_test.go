package main

import "testing"

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
