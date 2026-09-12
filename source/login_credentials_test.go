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

func TestCharacterCredentialEditStagesReplacementUntilLoginSucceeds(t *testing.T) {
	dir := t.TempDir()
	origDir := dataDirPath
	origCharacters := characters
	dataDirPath = dir
	const oldHash = "0123456789abcdef0123456789abcdef"
	characters = []Character{{Name: "Hero", passHash: oldHash}}
	discardStagedPassword()
	t.Cleanup(func() {
		dataDirPath = origDir
		characters = origCharacters
		discardStagedPassword()
	})

	hash, err := applyCharacterCredentialEdit("Hero", "replacement", true)
	if err != nil {
		t.Fatalf("edit credential: %v", err)
	}
	if hash != hashPassword("replacement") {
		t.Fatalf("staged hash = %q, want replacement hash", hash)
	}
	if characters[0].passHash != oldHash {
		t.Fatalf("replacement overwrote known-good password before login: %q", characters[0].passHash)
	}
	if _, remember, ok := stagedPasswordSettings("Hero"); !ok || !remember {
		t.Fatal("replacement password was not staged with Save Password enabled")
	}
}

func TestCharacterCredentialEditCanForgetSavedPassword(t *testing.T) {
	dir := t.TempDir()
	origDir := dataDirPath
	origCharacters := characters
	dataDirPath = dir
	characters = []Character{{Name: "Hero", passHash: "0123456789abcdef0123456789abcdef"}}
	discardStagedPassword()
	t.Cleanup(func() {
		dataDirPath = origDir
		characters = origCharacters
		discardStagedPassword()
	})

	hash, err := applyCharacterCredentialEdit("Hero", "", false)
	if err != nil {
		t.Fatalf("forget credential: %v", err)
	}
	if hash != "" || characters[0].passHash != "" || !characters[0].DontRemember {
		t.Fatalf("saved password was not forgotten: hash=%q character=%+v", hash, characters[0])
	}

	characters = nil
	loadCharacters()
	if len(characters) != 1 || characters[0].passHash != "" || !characters[0].DontRemember {
		t.Fatalf("forgotten password returned after reload: %+v", characters)
	}
}

func TestCharacterCredentialEditRequiresPasswordToEnableSaving(t *testing.T) {
	origCharacters := characters
	characters = []Character{{Name: "Hero", DontRemember: true}}
	discardStagedPassword()
	t.Cleanup(func() {
		characters = origCharacters
		discardStagedPassword()
	})

	if _, err := applyCharacterCredentialEdit("Hero", "", true); err == nil {
		t.Fatal("enabled Save Password without a password")
	}
}

func TestPasswordRetryPreservesSavingChoice(t *testing.T) {
	initFont()
	oldLogin, oldPassWin := loginWin, passWin
	oldInput, oldMessage, oldWarn, oldCheckbox := passInput, passMessage, passWarn, passRememberCB
	oldName, oldPass, oldHash, oldPrev := name, pass, passHash, passPrev
	oldRemember, oldCharacters, oldDir := passRemember, characters, dataDirPath
	loginWin = eui.NewWindow()
	loginWin.AddWindow(false)
	passWin = nil
	name = "Hero"
	dataDirPath = t.TempDir()
	t.Cleanup(func() {
		loginWin.RemoveWindow()
		if passWin != nil {
			passWin.RemoveWindow()
		}
		loginWin, passWin = oldLogin, oldPassWin
		passInput, passMessage, passWarn, passRememberCB = oldInput, oldMessage, oldWarn, oldCheckbox
		name, pass, passHash, passPrev = oldName, oldPass, oldHash, oldPrev
		passRemember, characters, dataDirPath = oldRemember, oldCharacters, oldDir
		discardStagedPassword()
	})
	for _, remember := range []bool{true, false} {
		for _, result := range []int16{-30998, -30987} {
			characters = []Character{{Name: name, DontRemember: !remember}}
			stagePasswordUpdate(name, "wrong", remember)
			choice := passwordRememberPreference(name)
			rejectPassword(name)
			showLoginFailure(&loginResultError{result: result}, false, choice)
			if !passWin.IsOpen() || loginWin.IsOpen() {
				t.Fatal("password rejection did not open the retry prompt")
			}
			if passMessage.Text != "Incorrect password. Please try again." || passWin.Title != "Password for Hero" {
				t.Fatal("retry prompt does not explain the rejected password and character")
			}
			if passRemember != remember || passRememberCB.Checked != remember {
				t.Fatalf("retry changed password saving choice %v", remember)
			}
			if pass != "" || passInput.SecretText != "" || !passInput.HideText {
				t.Fatal("retry input must be empty and masked")
			}
			stagePasswordUpdate(name, "correct", passRemember)
			commitStagedPassword(name)
			characters = nil
			loadCharacters()
			if len(characters) != 1 || characters[0].DontRemember == remember {
				t.Fatal("successful retry did not preserve saving choice")
			}
			want := ""
			if remember {
				want = hashPassword("correct")
			}
			if characters[0].passHash != want {
				t.Fatal("replacement password persistence does not match saving choice")
			}
		}
	}
	showPasswordPrompt(false, false, nil)
	if passMessage.Text != "Enter your password to connect." || passRememberCB.Checked {
		t.Fatal("ordinary prompt retained retry state")
	}
}

func TestPasswordPromptDisablingSavingForgetsImmediately(t *testing.T) {
	oldDir, oldCharacters := dataDirPath, characters
	oldName, oldRemember, oldHash := name, passRemember, passHash
	dataDirPath = t.TempDir()
	name = "Hero"
	characters = []Character{{Name: name, passHash: hashPassword("saved")}}
	t.Cleanup(func() {
		dataDirPath, characters = oldDir, oldCharacters
		name, passRemember, passHash = oldName, oldRemember, oldHash
		discardStagedPassword()
	})
	stagePasswordUpdate(name, "replacement", true)
	setPasswordPromptRemember(false)
	if passRemember {
		t.Fatal("saving remains enabled")
	}
	if _, remember, ok := stagedPasswordSettings(name); !ok || remember {
		t.Fatal("staged password would restore saving")
	}
	characters = nil
	loadCharacters()
	if len(characters) != 1 || characters[0].passHash != "" || characters[0].Key != "" || !characters[0].DontRemember {
		t.Fatal("disabling saving did not immediately remove persisted credentials")
	}
}

func TestEditCharacterDisablingSavingForgetsImmediately(t *testing.T) {
	oldDir, oldCharacters := dataDirPath, characters
	oldName, oldHash := name, passHash
	oldEditName, oldRemember := editCharName, editCharRemember
	dataDirPath = t.TempDir()
	name, editCharName = "Hero", "Hero"
	passHash = hashPassword("saved")
	characters = []Character{{Name: name, passHash: passHash}}
	t.Cleanup(func() {
		dataDirPath, characters = oldDir, oldCharacters
		name, passHash = oldName, oldHash
		editCharName, editCharRemember = oldEditName, oldRemember
		discardStagedPassword()
	})
	setEditCharacterRemember(false)
	if editCharRemember || passHash != "" {
		t.Fatal("edit retained saved credential for next login")
	}
	characters = nil
	loadCharacters()
	if len(characters) != 1 || characters[0].passHash != "" || characters[0].Key != "" || !characters[0].DontRemember {
		t.Fatal("edit did not immediately remove persisted credentials")
	}
}
