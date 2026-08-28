package main

import (
	"crypto/md5"
	"encoding/hex"
	"errors"
	"strings"
	"sync"

	"gothoom/eui"
)

type stagedPasswordUpdate struct {
	character string
	hash      string
	remember  bool
}

var (
	stagedPasswordMu sync.Mutex
	stagedPassword   *stagedPasswordUpdate
)

func hashPassword(password string) string {
	digest := md5.Sum([]byte(password))
	return hex.EncodeToString(digest[:])
}

func stagePasswordUpdate(character, password string, remember bool) string {
	hash := hashPassword(password)
	stagedPasswordMu.Lock()
	stagedPassword = &stagedPasswordUpdate{
		character: character,
		hash:      hash,
		remember:  remember,
	}
	stagedPasswordMu.Unlock()
	return hash
}

func stagedPasswordHash(character string) (string, bool) {
	stagedPasswordMu.Lock()
	defer stagedPasswordMu.Unlock()
	if stagedPassword == nil || !strings.EqualFold(stagedPassword.character, character) {
		return "", false
	}
	return stagedPassword.hash, true
}

func stagedPasswordSettings(character string) (hash string, remember bool, ok bool) {
	stagedPasswordMu.Lock()
	defer stagedPasswordMu.Unlock()
	if stagedPassword == nil || !strings.EqualFold(stagedPassword.character, character) {
		return "", false, false
	}
	return stagedPassword.hash, stagedPassword.remember, true
}

func updateStagedPasswordRemember(character string, remember bool) (hash string, ok bool) {
	stagedPasswordMu.Lock()
	defer stagedPasswordMu.Unlock()
	if stagedPassword == nil || !strings.EqualFold(stagedPassword.character, character) {
		return "", false
	}
	stagedPassword.remember = remember
	return stagedPassword.hash, true
}

func discardStagedPasswordFor(character string) {
	stagedPasswordMu.Lock()
	if stagedPassword != nil && strings.EqualFold(stagedPassword.character, character) {
		stagedPassword = nil
	}
	stagedPasswordMu.Unlock()
}

// applyCharacterCredentialEdit updates the credential selected in the Edit
// Character window. Replacement passwords remain staged until the server
// accepts them, so a typo cannot overwrite a known-good saved password.
func applyCharacterCredentialEdit(character, password string, remember bool) (string, error) {
	character = strings.TrimSpace(character)
	characterIndex := -1
	for i := range characters {
		if strings.EqualFold(characters[i].Name, character) {
			characterIndex = i
			character = characters[i].Name
			break
		}
	}
	if characterIndex < 0 {
		return "", errors.New("character is no longer available")
	}

	if password != "" {
		if !remember {
			// Turning saving off is immediate even though the replacement
			// password remains available for this session's next login.
			setCharacterPassHash(character, "", false)
		}
		return stagePasswordUpdate(character, password, remember), nil
	}

	if hash, staged := updateStagedPasswordRemember(character, remember); staged {
		if !remember {
			setCharacterPassHash(character, "", false)
		}
		return hash, nil
	}

	if remember {
		if characters[characterIndex].DontRemember || characters[characterIndex].passHash == "" {
			return "", errors.New("enter a new password before enabling Save Password")
		}
		return characters[characterIndex].passHash, nil
	}

	discardStagedPasswordFor(character)
	setCharacterPassHash(character, "", false)
	return "", nil
}

func takeStagedPassword(character string) (stagedPasswordUpdate, bool) {
	stagedPasswordMu.Lock()
	defer stagedPasswordMu.Unlock()
	if stagedPassword == nil || !strings.EqualFold(stagedPassword.character, character) {
		return stagedPasswordUpdate{}, false
	}
	update := *stagedPassword
	stagedPassword = nil
	return update, true
}

func discardStagedPassword() {
	stagedPasswordMu.Lock()
	stagedPassword = nil
	stagedPasswordMu.Unlock()
}

func commitStagedPassword(character string) {
	update, ok := takeStagedPassword(character)
	if ok {
		if update.remember {
			setCharacterPassHash(character, update.hash, true)
		} else {
			setCharacterPassHash(character, "", false)
		}
	} else {
		discardStagedPassword()
	}
	pass = ""
	passHash = ""
}

func rejectPassword(character string) {
	_, staged := takeStagedPassword(character)
	pass = ""
	passHash = ""
	if !staged {
		setCharacterPassHash(character, "", false)
	}
}

func isBadPasswordResult(result int16) bool {
	return result == -30998 || result == -30987
}

// clearPasswordInput resets both the masked display and the separate secret
// value retained by eui. Clearing only Text leaves SecretText available to be
// inserted into the next password entered when a dialog is reused.
func clearPasswordInput(input *eui.ItemData, value *string) {
	if value != nil {
		*value = ""
	}
	if input == nil {
		return
	}
	eui.ClearFocus(input)
	if input.TextPtr != nil {
		*input.TextPtr = ""
	}
	input.Text = ""
	input.SecretText = ""
	input.CursorPos = 0
	input.SelectStart = 0
	input.SelectEnd = 0
	input.Dirty = true
}
