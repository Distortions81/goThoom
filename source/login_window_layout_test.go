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
	originalDeleteBtn := deleteCharBtn
	originalConnectBtn := loginConnectButton
	originalServerDropdown := loginServerDropdown
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
		deleteCharBtn = originalDeleteBtn
		loginConnectButton = originalConnectBtn
		loginServerDropdown = originalServerDropdown
		eui.SetScreenSize(originalWidth, originalHeight)
	})

	makeLoginWindow()
	if len(loginWin.Contents) != 1 {
		t.Fatalf("login root items = %d, want 1 flow", len(loginWin.Contents))
	}
	items := loginWin.Contents[0].Contents
	if len(items) != 11 {
		t.Fatalf("login flow items = %d, want controls, Character list label, and spacers", len(items))
	}
	utilities := items[0].Contents
	if len(utilities) != 2 || utilities[0].Text != "Play movie file [clMov]" || utilities[1].Text != "Quit" {
		t.Fatal("login utility row should contain movie playback and Quit")
	}
	if items[2].Text != "Edit Characters:" || items[5].Text != "Character list:" || items[6] != charactersList {
		t.Fatal("login controls are not ordered utilities, character actions, character list, Connect")
	}
	connectRow := items[8].Contents
	if len(connectRow) != 2 || connectRow[0].Text != "Connect" || connectRow[1] != loginServerDropdown {
		t.Fatal("Connect row should contain Connect and the server selector")
	}
	if !utilities[1].Outlined || utilities[1].OutlineColor != eui.ColorRed || !connectRow[0].Outlined || connectRow[0].OutlineColor != eui.ColorGreen {
		t.Fatal("Quit and Connect do not have red and green frames")
	}
	actions := items[3].Contents
	if len(actions) != 3 || actions[0].Text != "Add" || actions[1].Text != "Edit" || actions[2].Text != "Delete" {
		t.Fatalf("character action row = %#v, want Add, Edit, Delete", actions)
	}
	versionRow := items[10].Contents
	if len(versionRow) != 3 || versionRow[1].Text != "Changelog" || versionRow[2].Text != "About" {
		t.Fatalf("version row does not end with Changelog and About")
	}
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
	originalDeleteBtn := deleteCharBtn
	originalConnectBtn := loginConnectButton
	originalServerDropdown := loginServerDropdown
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
		deleteCharBtn = originalDeleteBtn
		loginConnectButton = originalConnectBtn
		loginServerDropdown = originalServerDropdown
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

	if got, want := len(charactersList.Contents), len(characters)+1; got != want {
		t.Fatalf("hidden login character rows = %d, want %d", got, want)
	}
	if !loginConnectButton.Disabled {
		t.Fatal("Connect must be disabled until a character is selected")
	}
}

func TestLoginCharacterChoicesIncludeFreeDemo(t *testing.T) {
	originalCharacters := characters
	characters = []Character{{Name: "Alice", Profession: "Fighter", PictID: 123}}
	t.Cleanup(func() { characters = originalCharacters })

	choices := loginCharacterChoices()
	if len(choices) != 2 {
		t.Fatalf("login choices = %d, want 2", len(choices))
	}
	if choices[0].character.Name != "Alice" {
		t.Fatalf("first login choice = %+v, want saved character Alice", choices[0])
	}
	demo := choices[len(choices)-1]
	if !demo.demo || demo.selection != freeDemoSelection || demo.character.Name != freeDemoCharacterName {
		t.Fatalf("last login choice = %+v, want Demo Character", demo)
	}
	if demo.character.Profession != "" || demo.character.PictID != defaultMobilePictID(genderUnknown) {
		t.Fatalf("demo metadata = %+v, want default portrait and no profession", demo.character)
	}
	if len(demo.character.Colors) != len(newbieBrownColors) {
		t.Fatalf("demo clothing colors = %d, want %d", len(demo.character.Colors), len(newbieBrownColors))
	}
	for i := range newbieBrownColors {
		if demo.character.Colors[i] != newbieBrownColors[i] {
			t.Fatalf("demo clothing color %d = %d, want %d", i, demo.character.Colors[i], newbieBrownColors[i])
		}
	}
}

func TestEditCharacterOptionsFollowSelectedCharacter(t *testing.T) {
	initFont()
	originalWindow := loginWin
	originalList := charactersList
	originalEditBtn := editCharBtn
	originalDeleteBtn := deleteCharBtn
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
	deleteCharBtn = nil
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
		deleteCharBtn = originalDeleteBtn
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
	if editCharBtn == nil || editCharBtn.Disabled || deleteCharBtn == nil || deleteCharBtn.Disabled {
		t.Fatal("Edit and Delete action buttons were not visible and enabled")
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

	name = freeDemoSelection
	updateCharacterButtons()
	if !editCharBtn.Disabled || !deleteCharBtn.Disabled {
		t.Fatal("Edit and Delete must be disabled for Demo Character")
	}
	if _, ok := selectedCharacter(name); ok {
		t.Fatal("Demo Character was treated as an editable saved character")
	}
	if got := len(charactersList.Contents[len(charactersList.Contents)-1].Contents); got != 3 {
		t.Fatalf("Demo Character row controls = %d, want profession, portrait, and selector only", got)
	}
	name = "Alice"
	updateCharacterButtons()
	if editCharBtn.Disabled || deleteCharBtn.Disabled {
		t.Fatal("Edit and Delete did not re-enable for a saved character")
	}
}
