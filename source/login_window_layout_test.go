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
	originalProfileCB := loginProfileCB
	originalWidth, originalHeight := eui.ScreenSize()
	loginWin = nil
	eui.SetScreenSize(1200, 800)
	t.Cleanup(func() {
		if loginWin != nil {
			loginWin.RemoveWindow()
		}
		loginWin = originalWindow
		charactersList = originalList
		loginProfileCB = originalProfileCB
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
	originalProfileCB := loginProfileCB
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
		loginProfileCB = originalProfileCB
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

func TestLoginProfileToggleFollowsSelectedCharacter(t *testing.T) {
	initFont()
	originalWindow := loginWin
	originalList := charactersList
	originalProfileCB := loginProfileCB
	originalCharacters := characters
	originalProfiles := characterProfiles
	originalName := name
	originalLastCharacter := gs.LastCharacter
	loginWin = nil
	charactersList = nil
	loginProfileCB = nil
	characters = []Character{{Name: "Alice"}, {Name: "Bob"}}
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
		loginProfileCB = originalProfileCB
		characters = originalCharacters
		characterProfiles = originalProfiles
		name = originalName
		gs.LastCharacter = originalLastCharacter
	})

	makeLoginWindow()
	updateCharacterButtons()
	if loginProfileCB == nil || loginProfileCB.Disabled || !loginProfileCB.Checked {
		t.Fatal("enabled Alice profile was not shown on the Login window")
	}

	name = "Bob"
	updateCharacterButtons()
	if loginProfileCB.Disabled || loginProfileCB.Checked {
		t.Fatal("Bob should use global settings by default")
	}
}
