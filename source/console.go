package main

import scriptapi "gt2"

const (
	maxMessages = 1000
	sndTink     = 58 // notification sound
)

var consoleLog = messageLog{max: maxMessages}

func consoleMessage(msg string) {
	consoleMessageTyped(msg, messageTextTypeSystem)
}

func consoleMessageTyped(msg, messageType string) {
	if msg == "" {
		return
	}
	if wasmPrivacyActive() {
		return
	}
	legacyMacroSetDisplayedTextLog(msg, gs.ConsoleTimestamps)
	if msg == "You have been idle for too long." {
		showNotification(msg)
		playSound([]uint16{sndTink})
	}
	consoleLog.AddTyped(msg, messageType)
	appendConsoleLog(msg)

	queueConsoleWindowUpdate()
}

func serverConsoleMessage(msg string) {
	serverConsoleMessageTyped(msg, messageTextTypeSystem)
}

func serverConsoleMessageTyped(msg, messageType string) {
	consoleMessageTyped(msg, messageType)
	runServerMessageHandlers(scriptapi.ServerMessage{Message: msg, Type: messageType})
}

func getConsoleMessages() []string {
	format := gs.TimestampFormat
	if format == "" {
		format = "3:04PM"
	}
	return consoleLog.Entries(format, gs.ConsoleTimestamps)
}

func getConsoleMessageEntries() ([]string, []string) {
	format := gs.TimestampFormat
	if format == "" {
		format = "3:04PM"
	}
	return consoleLog.EntriesWithTypes(format, gs.ConsoleTimestamps)
}
