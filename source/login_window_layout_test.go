package main

import (
	"math"
	"testing"

	"gothoom/eui"
)

func TestLoginWindowStartsCentered(t *testing.T) {
	initFont()
	originalWindow := loginWin
	originalList := charactersList
	originalEditBtn := editCharBtn
	originalWidth, originalHeight := eui.ScreenSize()
	loginWin = nil
	eui.SetScreenSize(1200, 800)
	t.Cleanup(func() {
		if loginWin != nil {
			loginWin.RemoveWindow()
		}
		loginWin = originalWindow
		charactersList = originalList
		editCharBtn = originalEditBtn
		eui.SetScreenSize(originalWidth, originalHeight)
	})

	makeLoginWindow()
	pos := loginWin.GetPos()
	size := loginWin.GetSize()
	centerX := pos.X + size.X/2
	centerY := pos.Y + size.Y/2
	if math.Abs(float64(centerX-600)) > 0.5 || math.Abs(float64(centerY-400)) > 0.5 {
		t.Fatalf("login center = (%.1f, %.1f), want (600, 400)", centerX, centerY)
	}
}

func TestLoginWindowPopulatesCharactersWhileHiddenBySetupWizard(t *testing.T) {
	initFont()
	originalWindow := loginWin
	originalList := charactersList
	originalEditBtn := editCharBtn
	originalCharacters := characters
	originalName := name
	originalPassHash := passHash
	originalPass := pass
	originalLastCharacter := gs.LastCharacter
	loginWin = nil
	charactersList = nil
	characters = []Character{{Name: "Alice"}, {Name: "Bob"}}
	name = ""
	passHash = ""
	pass = ""
	gs.LastCharacter = ""
	t.Cleanup(func() {
		if loginWin != nil {
			loginWin.RemoveWindow()
		}
		loginWin = originalWindow
		charactersList = originalList
		editCharBtn = originalEditBtn
		characters = originalCharacters
		name = originalName
		passHash = originalPassHash
		pass = originalPass
		gs.LastCharacter = originalLastCharacter
	})

	makeLoginWindow()
	if loginWin.IsOpen() {
		t.Fatal("login window unexpectedly open before wizard completion")
	}
	updateCharacterButtons()

	if got := len(charactersList.Contents); got != len(characters) {
		t.Fatalf("hidden login character rows = %d, want %d", got, len(characters))
	}
}

func TestEditCharacterOptionsFollowSelectedCharacter(t *testing.T) {
	initFont()
	originalWindow := loginWin
	originalList := charactersList
	originalEditBtn := editCharBtn
	originalCharacters := characters
	originalProfiles := characterProfiles
	originalName := name
	originalLastCharacter := gs.LastCharacter
	originalEditName := editCharName
	originalEditRemember := editCharRemember
	originalEditProfile := editCharProfile
	loginWin = nil
	charactersList = nil
	editCharBtn = nil
	characters = []Character{
		{Name: "Alice", passHash: "0123456789abcdef0123456789abcdef"},
		{Name: "Bob", DontRemember: true},
	}
	characterProfiles = characterProfilesDocument{
		Version: characterProfilesVersion,
		Enabled: map[string]bool{"alice": true},
	}
	name = "Alice"
	gs.LastCharacter = "Alice"
	t.Cleanup(func() {
		if loginWin != nil {
			loginWin.RemoveWindow()
		}
		loginWin = originalWindow
		charactersList = originalList
		editCharBtn = originalEditBtn
		characters = originalCharacters
		characterProfiles = originalProfiles
		name = originalName
		gs.LastCharacter = originalLastCharacter
		editCharName = originalEditName
		editCharRemember = originalEditRemember
		editCharProfile = originalEditProfile
	})

	makeLoginWindow()
	updateCharacterButtons()
	if editCharBtn == nil || editCharBtn.Disabled {
		t.Fatal("Edit Character was not enabled for Alice")
	}
	if err := prepareEditCharacter("Alice"); err != nil {
		t.Fatalf("prepare Alice: %v", err)
	}
	if !editCharRemember || !editCharProfile {
		t.Fatal("Alice's saved password and profile choices were not shown")
	}

	name = "Bob"
	updateCharacterButtons()
	if err := prepareEditCharacter("Bob"); err != nil {
		t.Fatalf("prepare Bob: %v", err)
	}
	if editCharRemember || editCharProfile {
		t.Fatal("Bob should default to no saved password and global settings")
	}
}
