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
	serverSlot int
	character  string
	hash       string
	remember   bool
}

// Accepted/rejected logins can finish on different network goroutines. Keep
// updates to the shared saved-character list and file serialized.
var sessionCredentialCommitMu sync.Mutex

func hashPassword(password string) string {
	digest := md5.Sum([]byte(password))
	return hex.EncodeToString(digest[:])
}

func stagePasswordUpdate(character, password string, remember bool) string {
	return stageSessionPasswordUpdateForServerSlot(primarySession, selectedServerSlot(), character, password, remember)
}

func stageSessionPasswordUpdate(session *Session, character, password string, remember bool) string {
	return stageSessionPasswordUpdateForServerSlot(session, sessionCharacterServerSlot(session), character, password, remember)
}

func stageSessionPasswordUpdateForServerSlot(session *Session, serverSlot int, character, password string, remember bool) string {
	if session == nil {
		return ""
	}
	return session.login.stagePassword(serverSlot, character, password, remember)
}

func stagedPasswordHash(character string) (string, bool) {
	return primarySession.login.stagedPasswordHash(selectedServerSlot(), character)
}

func stagedPasswordSettings(character string) (hash string, remember bool, ok bool) {
	return primarySession.login.stagedPasswordSettings(selectedServerSlot(), character)
}

func updateStagedPasswordRemember(character string, remember bool) (hash string, ok bool) {
	return primarySession.login.updateStagedPasswordRemember(selectedServerSlot(), character, remember)
}

func discardStagedPasswordFor(character string) {
	primarySession.login.discardStagedPasswordFor(selectedServerSlot(), character)
}

func sessionCharacterServerSlot(session *Session) int {
	if session == nil || (session == primarySession && serverAddressOverride != "") {
		return selectedServerSlot()
	}
	if server := session.login.requestSnapshot().host; server != "" {
		if slot := serverSlotForAddress(server); slot > 0 {
			return slot
		}
	}
	return selectedServerSlot()
}

// applyCharacterCredentialEdit updates the credential selected in the Edit
// Character window. Replacement passwords remain staged until the server
// accepts them, so a typo cannot overwrite a known-good saved password.
func applyCharacterCredentialEdit(character, password string, remember bool) (string, error) {
	return applyCharacterCredentialEditForSession(primarySession, character, password, remember)
}

func applyCharacterCredentialEditForSession(session *Session, character, password string, remember bool) (string, error) {
	return applyCharacterCredentialEditForServerSlotAndSession(session, selectedServerSlot(), character, password, remember)
}

func applyCharacterCredentialEditForServerSlotAndSession(session *Session, serverSlot int, character, password string, remember bool) (string, error) {
	if session == nil {
		return "", errors.New("login session is unavailable")
	}
	character = strings.TrimSpace(character)
	characterIndex := -1
	for i := range characters {
		if savedCharacterServerSlot(characters[i]) == serverSlot && strings.EqualFold(characters[i].Name, character) {
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
			setCharacterPassHashForServerSlot(serverSlot, character, "", false)
		}
		return stageSessionPasswordUpdateForServerSlot(session, serverSlot, character, password, remember), nil
	}

	if hash, staged := session.login.updateStagedPasswordRemember(serverSlot, character, remember); staged {
		if !remember {
			setCharacterPassHashForServerSlot(serverSlot, character, "", false)
		}
		return hash, nil
	}

	if remember {
		if characters[characterIndex].DontRemember || characters[characterIndex].passHash == "" {
			return "", errors.New("enter a new password before enabling Save Password")
		}
		return characters[characterIndex].passHash, nil
	}

	session.login.discardStagedPasswordFor(serverSlot, character)
	setCharacterPassHashForServerSlot(serverSlot, character, "", false)
	return "", nil
}

func takeStagedPassword(character string) (stagedPasswordUpdate, bool) {
	return primarySession.login.takeStagedPassword(selectedServerSlot(), character)
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
	serverSlot := sessionCharacterServerSlot(session)
	update, ok := session.login.takeStagedPassword(serverSlot, character)
	sessionCredentialCommitMu.Lock()
	if ok {
		if update.remember {
			setCharacterPassHashForServerSlot(serverSlot, character, update.hash, true)
		} else {
			setCharacterPassHashForServerSlot(serverSlot, character, "", false)
		}
	} else {
		session.login.discardStagedPassword()
	}
	sessionCredentialCommitMu.Unlock()
	session.login.clearCredentials()
}

func forgetSavedPassword(character string) {
	forgetSavedPasswordForSession(primarySession, character)
}

func forgetSavedPasswordForSession(session *Session, character string) {
	forgetSavedPasswordForServerSlotAndSession(session, selectedServerSlot(), character)
}

func forgetSavedPasswordForServerSlotAndSession(session *Session, serverSlot int, character string) {
	setCharacterPassHashForServerSlot(serverSlot, character, "", false)
	if session != nil {
		session.login.updateStagedPasswordRemember(serverSlot, character, false)
	}
	if session == primarySession && strings.EqualFold(name, character) {
		passHash = ""
	}
}

func setEditCharacterRemember(remember bool) {
	setEditCharacterRememberForSession(primarySession, remember)
}

func setEditCharacterRememberForSession(session *Session, remember bool) {
	editCharRemember = remember
	if !remember {
		forgetSavedPasswordForServerSlotAndSession(session, serverSlotForLoginTarget(characterEditorTarget), editCharName)
	}
}

func setPasswordPromptRemember(remember bool) {
	setPasswordPromptRememberForSession(primarySession, name, remember)
}

func setPasswordPromptRememberForSession(session *Session, character string, remember bool) {
	setPasswordPromptRememberForServerSlotAndSession(session, selectedServerSlot(), character, remember)
}

func setPasswordPromptRememberForServerSlotAndSession(session *Session, serverSlot int, character string, remember bool) {
	passRemember = remember
	if !remember {
		forgetSavedPasswordForServerSlotAndSession(session, serverSlot, character)
	}
}

func passwordRememberPreference(character string) bool {
	return passwordRememberPreferenceForServerSlot(selectedServerSlot(), character)
}

func passwordRememberPreferenceForServerSlot(serverSlot int, character string) bool {
	if _, remember, ok := primarySession.login.stagedPasswordSettings(serverSlot, character); ok {
		return remember
	}
	if saved, ok := characterForServerSlot(serverSlot, character); ok {
		return !saved.DontRemember
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
	serverSlot := sessionCharacterServerSlot(session)
	_, staged := session.login.takeStagedPassword(serverSlot, character)
	session.login.clearCredentials()
	if !staged {
		sessionCredentialCommitMu.Lock()
		setCharacterPassHashForServerSlot(serverSlot, character, "", false)
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
