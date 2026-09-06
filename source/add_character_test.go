package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"gothoom/eui"
)

func TestAddCharacterRejectsExistingName(t *testing.T) {
	initFont()
	originalCharacters, originalDir := characters, dataDirPath
	originalProfiles := characterProfiles
	originalWindow := addCharWin
	originalNameInput, originalPassInput := addCharNameInput, addCharPassInput
	originalWarn, originalProfileCB := addCharPassWarn, addCharProfileCB
	originalName, originalPass, originalPrev := addCharName, addCharPass, addCharPassPrev
	originalRemember, originalProfile := addCharRemember, addCharProfile
	originalStaged := stagedPassword
	originalWindows := append([]*eui.WindowData(nil), eui.Windows()...)
	t.Cleanup(func() {
		for _, win := range append([]*eui.WindowData(nil), eui.Windows()...) {
			found := false
			for _, original := range originalWindows {
				found = found || win == original
			}
			if !found {
				win.RemoveWindow()
			}
		}
		characters, dataDirPath = originalCharacters, originalDir
		characterProfiles = originalProfiles
		addCharWin = originalWindow
		addCharNameInput, addCharPassInput = originalNameInput, originalPassInput
		addCharPassWarn, addCharProfileCB = originalWarn, originalProfileCB
		addCharName, addCharPass, addCharPassPrev = originalName, originalPass, originalPrev
		addCharRemember, addCharProfile = originalRemember, originalProfile
		stagedPassword = originalStaged
	})
	dataDirPath = t.TempDir()
	characters = []Character{{Name: "Hero", passHash: hashPassword("saved"), PictID: 123, Profession: "fighter"}}
	saveCharacters()
	wantCharacter := characters[0]
	file := filepath.Join(dataDirPath, charsFilePath)
	wantData, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	characterProfiles = characterProfilesDocument{
		Version: characterProfilesVersion,
		Enabled: map[string]bool{"hero": true},
	}
	wantHash := stagePasswordUpdate("Hero", "pending", true)
	addCharWin = nil
	addCharPass, addCharRemember, addCharProfile = "replacement", false, false
	makeAddCharacterWindow()
	addCharWin.MarkOpen()
	wantName, wantPass, wantPassHash := name, pass, passHash

	for _, input := range []string{"Hero", "hErO", "  HERO  "} {
		t.Run(input, func(t *testing.T) {
			addCharName = input
			addCharWin.DefaultButton.Handler.Emit(eui.UIEvent{Type: eui.EventClick})
			if len(characters) != 1 || !reflect.DeepEqual(characters[0], wantCharacter) {
				t.Fatal("duplicate changed the saved character")
			}
			data, err := os.ReadFile(file)
			if err != nil || string(data) != string(wantData) {
				t.Fatalf("duplicate changed the characters file: %v", err)
			}
			if hash, remember, ok := stagedPasswordSettings("Hero"); !ok || hash != wantHash || !remember {
				t.Fatal("duplicate replaced the staged password")
			}
			if !characterProfileEnabled("Hero") {
				t.Fatal("duplicate disabled the character's settings profile")
			}
			if name != wantName || pass != wantPass || passHash != wantPassHash {
				t.Fatal("duplicate changed the login selection or credentials")
			}
			if !addCharWin.IsOpen() || addCharName != input || addCharPass != "replacement" {
				t.Fatal("duplicate closed or cleared the Add Character form")
			}
			var popup *eui.WindowData
			for _, win := range eui.Windows() {
				if win.Title == "Error" && win.IsOpen() {
					popup = win
				}
			}
			if popup == nil || !strings.Contains(popup.Contents[0].Contents[0].Text, "character already exists") {
				t.Fatal("duplicate did not show an explanatory error")
			}
			popup.Close()
		})
	}
}

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
