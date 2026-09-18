package main

import (
	"strings"

	"gothoom/eui"
)

func newStatusBar(width float32) (*eui.ItemData, *eui.ItemData) {
	label := eui.NewWrappedLabel("", width-16)
	label.FontSize = 10
	label.Size.Y = 30
	label.Position = eui.Point{X: 8, Y: 3}
	label.ConstrainToSize, label.ForceTextColor = true, true
	label.TextColor = eui.ColorWhite
	frame := eui.NewColumn(label)
	frame.Size = eui.Point{X: width, Y: 36}
	frame.Fixed, frame.ConstrainToSize, frame.Filled = true, true, true
	frame.Position.Y = 4
	return frame, label
}

func setStatusBar(frame, label *eui.ItemData, message string, good, problem bool) {
	frame.Color = eui.ColorVeryDarkGray
	prefix := "Status: "
	if problem {
		frame.Color = eui.ColorDarkRed
		prefix += "Problem — "
	} else if good {
		frame.Color = eui.NewColor(24, 100, 55, 255)
		prefix += "OK — "
	}
	message = prefix + strings.Join(strings.Fields(message), " ")
	label.Size.X = frame.Size.X - 16
	label.SetWrappedText(message)
	label.SetTooltip(message)
}
