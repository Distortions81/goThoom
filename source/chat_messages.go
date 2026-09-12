package main

import (
	"strings"
	"sync"
)

const (
	maxChatMessages = 1000
)

var (
	chatLog             = messageLog{max: maxChatMessages}
	chatTTSDisabledOnce sync.Once
)

func chatMessage(msg string) {
	chatMessageTyped(msg, messageTextTypeSystem)
}

// displayChatMessageTyped routes chat to the configured display while always
// delivering it to script OnChat handlers. Console routing is a presentation
// choice and must not change the script event stream.
func displayChatMessageTyped(msg, messageType string) {
	if gs.MessagesToConsole {
		serverConsoleMessageTyped(msg, messageType)
		if gs.ChatTTS {
			handleChatTTS(msg, messageType, chatSpeaker(msg))
		}
		dispatchScriptChat(msg)
		return
	}
	chatMessageTyped(msg, messageType)
}

func chatMessageTyped(msg, messageType string) {
	if msg == "" {
		return
	}
	if wasmPrivacyActive() {
		return
	}
	legacyMacroSetDisplayedTextLog(msg, gs.ChatTimestamps)

	speaker := chatSpeaker(msg)
	if speaker != "" {
		playersMu.RLock()
		p, ok := players[speaker]
		blocked := ok && (p.Blocked || p.Ignored)
		playersMu.RUnlock()
		if blocked {
			return
		}
	}

	tagged := chatHasPlayerTag(msg)

	chatLog.AddTyped(msg, messageType)
	appendChatLog(msg)

	queueChatWindowUpdate()

	if tagged && !isSelfChatMessage(msg) {
		playMentionSound()
		// Notify on mentions only when unfocused (respect setting)
		if gs.NotifyWhenBackground && !windowIsFocused() {
			notifyDesktop("Mention", msg)
		}
	}

	handleChatTTS(msg, messageType, speaker)

	dispatchScriptChat(msg)
}

func handleChatTTS(msg, messageType, speaker string) {
	if msg == "" || wasmPrivacyActive() {
		return
	}
	if !gs.ChatTTS {
		chatTTSDisabledOnce.Do(func() {
			consoleMessage("Chat TTS is disabled. Enable it in settings to hear messages.")
		})
		return
	}
	if blockTTS || !chatTTSMessageSelected(msg, messageType) {
		return
	}
	if speaker != "" {
		playersMu.RLock()
		player := players[speaker]
		blocked := player != nil && (player.Blocked || player.Ignored)
		playersMu.RUnlock()
		if blocked || isTTSBlocked(speaker) {
			return
		}
	}
	speakChatMessage(msg)
}

func chatTTSMessageSelected(msg, messageType string) bool {
	return chatTTSMessageTypeEnabled(messageType) && (gs.ChatTTSSelf || !isSelfChatMessage(msg))
}

func chatTTSMessageTypeEnabled(messageType string) bool {
	switch messageType {
	case messageTextTypeSay:
		return gs.ChatTTSSay
	case messageTextTypeWhisper:
		return gs.ChatTTSWhisper
	case messageTextTypeYell:
		return gs.ChatTTSYell
	case messageTextTypeThink:
		return gs.ChatTTSThink
	case messageTextTypeAction:
		return gs.ChatTTSAction
	case messageTextTypePonder:
		return gs.ChatTTSPonder
	case messageTextTypeMonster:
		return gs.ChatTTSMonster
	default:
		// Preserve TTS for direct client chat messages that predate typed
		// server bubbles and do not have one of the configurable categories.
		return true
	}
}

func getChatMessages() []string {
	format := gs.TimestampFormat
	if format == "" {
		format = "3:04PM"
	}
	return chatLog.Entries(format, gs.ChatTimestamps)
}

func getChatMessageEntries() ([]string, []string) {
	format := gs.TimestampFormat
	if format == "" {
		format = "3:04PM"
	}
	return chatLog.EntriesWithTypes(format, gs.ChatTimestamps)
}

func isSelfChatMessage(msg string) bool {
	if playerName == "" {
		return false
	}
	m := strings.ToLower(strings.TrimSpace(msg))
	name := strings.ToLower(playerName)

	// Emotes like "(Hero waves)"
	if strings.HasPrefix(m, "("+name+" ") {
		return true
	}
	// Spoken lines like "Hero says, ..." or "Hero yells, ..."
	if strings.HasPrefix(m, name+" ") {
		rest := strings.TrimSpace(m[len(name):])
		if strings.HasPrefix(rest, "says,") ||
			strings.HasPrefix(rest, "yells,") ||
			strings.HasPrefix(rest, "whispers,") ||
			strings.HasPrefix(rest, "exclaims,") ||
			strings.HasPrefix(rest, "asks,") ||
			strings.HasPrefix(rest, "thinks,") ||
			strings.HasPrefix(rest, "ponders,") {
			return true
		}
	}
	return false
}

// chatSpeaker extracts the leading player name from a chat message, folded to
// canonical form. It returns an empty string if no name could be parsed.
func chatSpeaker(msg string) string {
	m := strings.TrimSpace(msg)
	if m == "" {
		return ""
	}
	if strings.HasPrefix(m, "(") {
		if end := strings.IndexByte(m, ')'); end > 1 {
			return utfFold(strings.TrimSpace(m[1:end]))
		}
		m = strings.TrimPrefix(m, "(")
	}
	lower := strings.ToLower(m)
	for _, sep := range []string{" says", " yells", " whispers", " asks", " exclaims", " thinks to you"} {
		if idx := strings.Index(lower, sep); idx > 0 {
			return utfFold(strings.TrimSpace(m[:idx]))
		}
	}
	if i := strings.IndexByte(m, ' '); i > 0 {
		return utfFold(m[:i])
	}
	return ""
}

func chatHasPlayerTag(msg string) bool {
	if playerName == "" {
		return false
	}
	lower := strings.ToLower(msg)
	name := strings.ToLower("@" + playerName)
	if strings.Contains(lower, name) {
		return true
	}
	var prof string
	playersMu.RLock()
	if p, ok := players[playerName]; ok {
		prof = p.Class
	}
	playersMu.RUnlock()
	if prof != "" && strings.Contains(lower, "@"+strings.ToLower(prof)) {
		return true
	}
	return false
}
