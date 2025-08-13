//go:build !test

package main

import "gothoom/eui"

var consoleWin *eui.WindowData
var messagesFlow *eui.ItemData
var inputFlow *eui.ItemData

func updateConsoleWindow() {
	inputMsg := "[Press Enter To Type]"
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
