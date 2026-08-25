package main

import (
	"path/filepath"
	"strings"

	"gothoom/eui"

	open "github.com/skratchdot/open-golang/open"
)

var (
	scriptLibraryWin  *eui.WindowData
	scriptLibraryList *eui.ItemData
)

func openScriptLibraryWindow() {
	if scriptLibraryWin != nil {
		refreshScriptLibraryWindow()
		scriptLibraryWin.MarkOpen()
		return
	}
	scriptLibraryWin = eui.NewWindow()
	scriptLibraryWin.Title = "Example Scripts"
	scriptLibraryWin.Size = eui.Point{X: 680, Y: 480}
	scriptLibraryWin.Closable = true
	scriptLibraryWin.Movable = true
	scriptLibraryWin.Resizable = true
	scriptLibraryWin.SetZone(eui.HZoneCenter, eui.VZoneMiddleTop)
	scriptLibraryWin.OnClose = func() {
		scriptLibraryWin = nil
		scriptLibraryList = nil
	}
	root := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL, Fixed: true}
	scriptLibraryWin.AddItem(root)
	intro, _ := eui.NewText()
	intro.Text = "Examples stay read-only here until you install one. Existing files are never replaced."
	intro.FontSize = 12
	intro.Size = eui.Point{X: 640, Y: 28}
	root.AddItem(intro)
	scriptLibraryList = &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL, Scrollable: true, Fixed: true}
	scriptLibraryList.Size = eui.Point{X: 650, Y: 400}
	root.AddItem(scriptLibraryList)
	scriptLibraryWin.AddWindow(false)
	refreshScriptLibraryWindow()
}

func refreshScriptLibraryWindow() {
	if scriptLibraryList == nil {
		return
	}
	scriptLibraryList.Contents = scriptLibraryList.Contents[:0]
	entries, err := scriptLibraryEntries()
	if err != nil {
		scriptLibraryText("Could not read examples: "+err.Error(), 640)
		return
	}
	for _, entry := range entries {
		entry := entry
		row := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_HORIZONTAL, Fixed: true}
		row.Size = eui.Point{X: 640, Y: 42}
		path := filepath.Join(userScriptsDir(), entry.Filename)
		installed := scriptRootFileExists(userScriptsDir(), entry.Filename)

		install, installEvents := eui.NewButton()
		install.Text = "Install"
		install.Size = eui.Point{X: 64, Y: 24}
		install.Disabled = installed || isWASM
		installEvents.Handle = func(event eui.UIEvent) {
			if event.Type != eui.EventClick || install.Disabled {
				return
			}
			installedPath, err := installBundledScript(userScriptsDir(), entry.Filename)
			if err != nil {
				consoleMessage("[script] install example: " + err.Error())
				return
			}
			consoleMessage("[script] installed example: " + entry.Name)
			rescanscripts()
			selectedscript = entry.ID
			refreshScriptLibraryWindow()
			if err := open.Run(installedPath); err != nil {
				consoleMessage("[script] open example: " + err.Error())
			}
		}
		row.AddItem(install)

		edit, editEvents := eui.NewButton()
		edit.Text = "Edit"
		edit.Size = eui.Point{X: 48, Y: 24}
		edit.Disabled = !installed || isWASM
		editEvents.Handle = func(event eui.UIEvent) {
			if event.Type == eui.EventClick && !edit.Disabled {
				if err := open.Run(path); err != nil {
					consoleMessage("[script] open example: " + err.Error())
				}
			}
		}
		row.AddItem(edit)

		details, _ := eui.NewText()
		details.Text = entry.Name
		if entry.Description != "" {
			details.Text += " — " + entry.Description
		} else if entry.Author != "" {
			details.Text += " — by " + entry.Author
		}
		if installed {
			details.Text += " [Installed]"
		}
		details.FontSize = 12
		details.Size = eui.Point{X: 510, Y: 38}
		row.AddItem(details)
		scriptLibraryList.AddItem(row)
	}
	if scriptLibraryWin != nil {
		scriptLibraryWin.Refresh()
	}
}

func scriptLibraryText(message string, width float32) {
	text, _ := eui.NewText()
	text.Text = strings.TrimSpace(message)
	text.FontSize = 12
	text.Size = eui.Point{X: width, Y: 24}
	scriptLibraryList.AddItem(text)
}
