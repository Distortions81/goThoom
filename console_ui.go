//go:build !test

package main

import "go_client/eui"

var consoleWin *eui.WindowData
var messagesFlow *eui.ItemData
var inputFlow *eui.ItemData

func updateConsoleWindow() {
	inputMsg := "[Command Input Bar] (Press enter to switch to command mode)"
	if inputActive {
		inputMsg = string(inputText)
	}
	updateTextWindow(consoleWin, messagesFlow, inputFlow, getConsoleMessages(), gs.ConsoleFontSize, inputMsg)
}

func makeConsoleWindow() {
	if consoleWin != nil {
		return
	}
	consoleWin, messagesFlow, inputFlow = makeTextWindow("Console", eui.HZoneLeft, eui.VZoneBottom, true)
	updateConsoleWindow()
}
