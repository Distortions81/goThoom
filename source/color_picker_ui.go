package main

import "gothoom/eui"

var colorPickerWin *eui.WindowData

func newColorSwatch(title string, initial eui.Color, apply func(eui.Color)) *eui.ItemData {
	return eui.NewColorSwatch(title, initial, openColorPicker, apply)
}

// The client keeps a single picker open across all settings windows.
func openColorPicker(title string, initial eui.Color, apply func(eui.Color)) {
	if colorPickerWin != nil {
		colorPickerWin.Close()
	}
	win := eui.ShowColorPicker(title, initial, apply)
	colorPickerWin = win
	win.OnClose = func() {
		win.RemoveWindow()
		if colorPickerWin == win {
			colorPickerWin = nil
		}
	}
}
