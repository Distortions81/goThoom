package main

import (
	_ "embed"
	"strings"

	"gothoom/eui"
)

//go:embed data/about.txt
var aboutText string

var aboutWin *eui.WindowData
var aboutList *eui.ItemData
var aboutLines []string

func initAboutUI() {
	if aboutWin != nil {
		return
	}
	aboutWin, aboutList, _ = makeTextWindow("About", eui.HZoneCenter, eui.VZoneMiddleTop, false)
	aboutWin.AutoSize = true
	aboutLines = strings.Split(strings.ReplaceAll(aboutText, "\r\n", "\n"), "\n")
	aboutWin.OnResize = func() { updateTextWindow(aboutWin, aboutList, nil, aboutLines, 15, "", monoFaceSource) }
}

func openAboutWindow(anchor *eui.ItemData) {
	if aboutWin == nil {
		return
	}

	if aboutWin.Open {
		aboutWin.Close()
		return
	}

	updateTextWindow(aboutWin, aboutList, nil, aboutLines, 15, "", monoFaceSource)
	if anchor != nil {
		aboutWin.MarkOpenNear(anchor)
	} else {
		aboutWin.MarkOpen()
	}
	if aboutWin.OnResize != nil {
		aboutWin.OnResize()
	}
	aboutWin.Refresh()
}
