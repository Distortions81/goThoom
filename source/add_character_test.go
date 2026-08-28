package main

import "testing"

func TestAddedCharacterWithoutPasswordPromptsAtLogin(t *testing.T) {
	originalCharacters := characters
	characters = []Character{{Name: "Hero", DontRemember: true}}
	discardStagedPassword()
	t.Cleanup(func() {
		characters = originalCharacters
		discardStagedPassword()
	})

	if got := stageAddedCharacterPassword("Hero", "", true); got != "" {
		t.Fatalf("empty optional password produced hash %q", got)
	}
	if _, staged := stagedPasswordHash("Hero"); staged {
		t.Fatal("empty optional password staged a credential instead of prompting at login")
	}
}

func TestAddedCharacterPasswordIsStagedWhenProvided(t *testing.T) {
	originalCharacters := characters
	characters = []Character{{Name: "Hero", DontRemember: true}}
	discardStagedPassword()
	t.Cleanup(func() {
		characters = originalCharacters
		discardStagedPassword()
	})

	want := hashPassword("secret")
	if got := stageAddedCharacterPassword("Hero", "secret", true); got != want {
		t.Fatalf("provided password hash = %q, want %q", got, want)
	}
	if got, staged := stagedPasswordHash("Hero"); !staged || got != want {
		t.Fatalf("provided password was not staged: hash=%q staged=%v", got, staged)
	}
}
