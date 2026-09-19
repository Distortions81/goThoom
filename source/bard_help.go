package main

import (
	_ "embed"
	"strings"

	"gothoom/eui"
)

//go:embed data/bard_help.txt
var bardHelpText string

var bardHelpWin *eui.WindowData

func showBardHelp() {
	if bardHelpWin != nil {
		bardHelpWin.MarkOpen()
		bardHelpWin.BringForward()
		return
	}
	win := eui.NewWindow()
	bardHelpWin = win
	win.Title = "Bard Tools Help"
	win.Closable, win.Movable, win.Resizable, win.NoScroll = true, true, true, true
	win.Size = eui.Point{X: 620, Y: 560}
	win.SetZone(eui.HZoneCenter, eui.VZoneMiddleTop)
	root, body := eui.NewColumn(), eui.NewColumn()
	body.Fixed, body.Scrollable = true, true
	topic, events := eui.NewDropdown()
	topic.Label = "Topic"
	topic.Selected = 0
	topic.Size = eui.Point{X: 560, Y: 28}
	var pages []string
	for _, section := range strings.Split(strings.TrimSpace(bardHelpText), "\n# ") {
		title, content, _ := strings.Cut(strings.TrimPrefix(section, "# "), "\n")
		topic.Options = append(topic.Options, title)
		pages = append(pages, strings.TrimSpace(content))
	}
	root.AddItem(topic)
	root.AddItem(body)
	closeButton := eui.NewActionButton("Close", win.Close)
	closeButton.Size.X = 1
	root.AddItem(closeButton)
	win.AddItem(root)
	win.OnResize = func() {
		eui.LayoutWindowBody(win, root, body)
		topic.Size.X = body.Size.X
		for _, paragraph := range body.Contents {
			paragraph.Size.X = max(1, body.Size.X-12)
		}
		eui.LayoutWindowBody(win, root, body)
		win.Refresh()
	}
	showTopic := func() {
		body.SetItems(nil)
		for _, paragraph := range strings.Split(pages[topic.Selected], "\n\n") {
			body.AddItem(eui.NewWrappedLabel(paragraph, 560))
		}
		body.Scroll = eui.Point{}
		win.OnResize()
	}
	events.Handle = func(event eui.UIEvent) {
		if event.Type == eui.EventDropdownSelected {
			showTopic()
		}
	}
	win.OnClose = func() {
		win.RemoveWindow()
		bardHelpWin = nil
	}
	showTopic()
	win.MarkOpen()
}
