package main

const (
	maxMessages = 1000
	sndTink     = 58 // notification sound
)

var consoleLog = messageLog{max: maxMessages}

func consoleMessage(msg string) {
	if msg == "" {
		return
	}
	if wasmPrivacyActive() {
		return
	}
	legacyMacroSetTextLog(msg)
	if msg == "You have been idle for too long." {
		showNotification(msg)
		playSound([]uint16{sndTink})
	}
	consoleLog.Add(msg)
	appendConsoleLog(msg)

	updateConsoleWindow()

	runConsoleTriggers(msg)
}

func getConsoleMessages() []string {
	format := gs.TimestampFormat
	if format == "" {
		format = "3:04PM"
	}
	return consoleLog.Entries(format, gs.ConsoleTimestamps)
}
