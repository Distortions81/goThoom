package main

import (
	"fmt"
	"strings"
)

func sessionPanelTitle(session *Session, title string) string {
	if session == nil {
		return title
	}
	character := strings.TrimSpace(session.characterName())
	if session == primarySession && character == "" {
		character = strings.TrimSpace(playerName)
	}
	if character == "" {
		return title
	}
	return fmt.Sprintf("%s - %s", character, title)
}
