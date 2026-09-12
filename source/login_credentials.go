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

// Accepted/rejected logins can finish on different network goroutines. Keep
// updates to the shared saved-character list and file serialized.
var sessionCredentialCommitMu sync.Mutex

func hashPassword(password string) string {
	digest := md5.Sum([]byte(password))
	return hex.EncodeToString(digest[:])
}

func stagePasswordUpdate(character, password string, remember bool) string {
	return stageSessionPasswordUpdate(primarySession, character, password, remember)
}

func stageSessionPasswordUpdate(session *Session, character, password string, remember bool) string {
	if session == nil {
		return ""
	}
	return session.login.stagePassword(character, password, remember)
}

func stagedPasswordHash(character string) (string, bool) {
	return primarySession.login.stagedPasswordHash(character)
}

func stagedPasswordSettings(character string) (hash string, remember bool, ok bool) {
	return primarySession.login.stagedPasswordSettings(character)
}

func updateStagedPasswordRemember(character string, remember bool) (hash string, ok bool) {
	return primarySession.login.updateStagedPasswordRemember(character, remember)
}

func discardStagedPasswordFor(character string) {
	primarySession.login.discardStagedPasswordFor(character)
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
	return primarySession.login.takeStagedPassword(character)
}

func discardStagedPassword() {
	primarySession.login.discardStagedPassword()
}

func commitStagedPassword(character string) {
	commitSessionStagedPassword(primarySession, character)
	pass = ""
	passHash = ""
}

func commitSessionStagedPassword(session *Session, character string) {
	if session == nil {
		return
	}
	update, ok := session.login.takeStagedPassword(character)
	sessionCredentialCommitMu.Lock()
	if ok {
		if update.remember {
			setCharacterPassHash(character, update.hash, true)
		} else {
			setCharacterPassHash(character, "", false)
		}
	} else {
		session.login.discardStagedPassword()
	}
	sessionCredentialCommitMu.Unlock()
	session.login.clearCredentials()
}

func forgetSavedPassword(character string) {
	setCharacterPassHash(character, "", false)
	updateStagedPasswordRemember(character, false)
	if strings.EqualFold(name, character) {
		passHash = ""
	}
}

func setEditCharacterRemember(remember bool) {
	editCharRemember = remember
	if !remember {
		forgetSavedPassword(editCharName)
	}
}

func setPasswordPromptRemember(remember bool) {
	passRemember = remember
	if !remember {
		forgetSavedPassword(name)
	}
}

func passwordRememberPreference(character string) bool {
	if _, remember, ok := stagedPasswordSettings(character); ok {
		return remember
	}
	for _, saved := range characters {
		if strings.EqualFold(saved.Name, character) {
			return !saved.DontRemember
		}
	}
	return true
}

func rejectPassword(character string) {
	rejectSessionPassword(primarySession, character)
	pass = ""
	passHash = ""
}

func rejectSessionPassword(session *Session, character string) {
	if session == nil {
		return
	}
	_, staged := session.login.takeStagedPassword(character)
	session.login.clearCredentials()
	if !staged {
		sessionCredentialCommitMu.Lock()
		setCharacterPassHash(character, "", false)
		sessionCredentialCommitMu.Unlock()
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
