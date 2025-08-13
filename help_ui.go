//go:build !test

package main

import (
	_ "embed"
	"strings"

	"go_client/eui"
)

var helpWin *eui.WindowData
var helpFlow *eui.ItemData

//go:embed data/help.txt
var helpText string

func updateHelpWindow() {
	if helpFlow == nil || helpWin == nil {
		return
	}
	// Clear existing contents
	for i := range helpFlow.Contents {
		helpFlow.Contents[i] = nil
	}
	helpFlow.Contents = helpFlow.Contents[:0]

	maxWidth := int(helpWin.Size.X) - 20
	if maxWidth <= 0 {
		maxWidth = 300
	}

	for _, para := range strings.Split(helpText, "\n") {
		var lines []string
		if mainFont != nil {
			_, lines = wrapText(para, mainFont, float64(maxWidth))
		} else {
			lines = []string{para}
		}
		for _, line := range lines {
			t, _ := eui.NewText()
			t.Text = line
			t.Size = eui.Point{X: float32(maxWidth), Y: 24}
			t.FontSize = float32(gs.MainFontSize)
			helpFlow.AddItem(t)
		}
	}
}

func makeHelpWindow() {
	if helpWin != nil {
		return
	}
	helpWin = eui.NewWindow()
	helpWin.Title = "Help"
	helpWin.Size = eui.Point{X: 410, Y: 450}
	helpWin.Closable = true
	helpWin.Resizable = true
	helpWin.Movable = true
	helpWin.SetZone(eui.HZoneCenterLeft, eui.VZoneMiddleTop)

	helpFlow = &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL, Scrollable: true}
	helpWin.AddItem(helpFlow)
	helpWin.AddWindow(false)
	updateHelpWindow()
}
