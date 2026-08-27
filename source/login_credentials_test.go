package main

import (
	"testing"

	"gothoom/eui"
)

func TestClearPasswordInputClearsHiddenState(t *testing.T) {
	value := "old-password"
	input, _ := eui.NewInput()
	input.TextPtr = &value
	input.HideText = true
	input.Text = "************"
	input.SecretText = value
	input.CursorPos = 5
	input.SelectStart = 1
	input.SelectEnd = 4

	clearPasswordInput(input, &value)

	if value != "" || input.Text != "" || input.SecretText != "" {
		t.Fatalf("password input not cleared: value=%q text=%q secret=%q", value, input.Text, input.SecretText)
	}
	if input.CursorPos != 0 || input.SelectStart != 0 || input.SelectEnd != 0 {
		t.Fatalf("password input cursor state not cleared: cursor=%d selection=%d:%d", input.CursorPos, input.SelectStart, input.SelectEnd)
	}
}

func TestStagedPasswordCommitsOnlyAfterSuccess(t *testing.T) {
	dir := t.TempDir()
	origDir := dataDirPath
	origCharacters := characters
	origPass := pass
	origPassHash := passHash
	dataDirPath = dir
	characters = []Character{{
		Name:     "Hero",
		passHash: "0123456789abcdef0123456789abcdef",
	}}
	discardStagedPassword()
	t.Cleanup(func() {
		dataDirPath = origDir
		characters = origCharacters
		pass = origPass
		passHash = origPassHash
		discardStagedPassword()
	})

	wantHash := hashPassword("new-password")
	passHash = stagePasswordUpdate("Hero", "new-password", true)
	if characters[0].passHash == wantHash {
		t.Fatal("staged password changed the character before authentication")
	}

	commitStagedPassword("Hero")
	if characters[0].passHash != wantHash || characters[0].DontRemember {
		t.Fatalf("committed password state = hash %q, dontRemember %v", characters[0].passHash, characters[0].DontRemember)
	}
	if pass != "" || passHash != "" {
		t.Fatalf("session credentials retained after authentication: pass=%q hash=%q", pass, passHash)
	}
}

func TestRejectedStagedPasswordPreservesPreviouslySavedPassword(t *testing.T) {
	dir := t.TempDir()
	origDir := dataDirPath
	origCharacters := characters
	origPass := pass
	origPassHash := passHash
	dataDirPath = dir
	const oldHash = "0123456789abcdef0123456789abcdef"
	characters = []Character{{Name: "Hero", passHash: oldHash}}
	discardStagedPassword()
	t.Cleanup(func() {
		dataDirPath = origDir
		characters = origCharacters
		pass = origPass
		passHash = origPassHash
		discardStagedPassword()
	})

	stagePasswordUpdate("Hero", "mistyped-password", true)
	rejectPassword("Hero")
	if characters[0].passHash != oldHash {
		t.Fatalf("rejected staged password replaced saved hash with %q", characters[0].passHash)
	}
}

func TestBothPasswordErrorsForgetRejectedSavedPassword(t *testing.T) {
	for _, result := range []int16{-30998, -30987} {
		t.Run((&loginResultError{result: result}).Error(), func(t *testing.T) {
			dir := t.TempDir()
			origDir := dataDirPath
			origCharacters := characters
			origPass := pass
			origPassHash := passHash
			dataDirPath = dir
			characters = []Character{{Name: "Hero", passHash: "0123456789abcdef0123456789abcdef"}}
			discardStagedPassword()
			t.Cleanup(func() {
				dataDirPath = origDir
				characters = origCharacters
				pass = origPass
				passHash = origPassHash
				discardStagedPassword()
			})

			if !isBadPasswordResult(result) {
				t.Fatalf("password result %d was not recognized", result)
			}
			rejectPassword("Hero")
			if characters[0].passHash != "" || !characters[0].DontRemember {
				t.Fatalf("rejected saved password retained: %+v", characters[0])
			}

			characters = nil
			loadCharacters()
			if len(characters) != 1 {
				t.Fatalf("character removed with rejected password; loaded %d", len(characters))
			}
			if characters[0].passHash != "" || !characters[0].DontRemember {
				t.Fatalf("rejected password returned after reload: %+v", characters[0])
			}
		})
	}
}

func TestRememberOffPersistsCharacterAfterSuccessfulLogin(t *testing.T) {
	dir := t.TempDir()
	origDir := dataDirPath
	origCharacters := characters
	origPass := pass
	origPassHash := passHash
	dataDirPath = dir
	characters = []Character{{Name: "Hero", DontRemember: true}}
	discardStagedPassword()
	t.Cleanup(func() {
		dataDirPath = origDir
		characters = origCharacters
		pass = origPass
		passHash = origPassHash
		discardStagedPassword()
	})

	stagePasswordUpdate("Hero", "session-password", false)
	commitStagedPassword("Hero")
	characters = nil
	loadCharacters()
	if len(characters) != 1 {
		t.Fatalf("character removed when password was not remembered; loaded %d", len(characters))
	}
	if characters[0].passHash != "" || !characters[0].DontRemember {
		t.Fatalf("password was remembered unexpectedly: %+v", characters[0])
	}
}
