//go:build !test

package main

import "gothoom/eui"

var consoleWin *eui.WindowData
var messagesFlow *eui.ItemData
var inputFlow *eui.ItemData
var consolePrevCount int

func updateConsoleWindow() {
	inputMsg := "[Press Enter To Type]"
	if inputActive {
		inputMsg = string(inputText)
	}
	msgs := getConsoleMessages()
	updateTextWindow(consoleWin, messagesFlow, inputFlow, msgs, gs.ConsoleFontSize, inputMsg)
	if messagesFlow != nil && len(msgs) > consolePrevCount {
		// Scroll to bottom on new text; clamp occurs on Refresh.
		messagesFlow.Scroll.Y = 1e9
		if consoleWin != nil {
			consoleWin.Refresh()
		}
	}
	consolePrevCount = len(msgs)
}

func makeConsoleWindow() {
	if consoleWin != nil {
		return
	}
	consoleWin, messagesFlow, inputFlow = makeTextWindow("Console", eui.HZoneLeft, eui.VZoneBottom, true)
	// Rewrap and refresh on window resize
	consoleWin.OnResize = func() {
		updateConsoleWindow()
		if consoleWin != nil {
			consoleWin.Refresh()
		}
	}
	updateConsoleWindow()
}
