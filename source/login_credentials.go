package main

import (
	"crypto/md5"
	"encoding/hex"
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
