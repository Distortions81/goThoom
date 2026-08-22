package main

import (
	"fmt"
	"strings"

	"gothoom/eui"

	open "github.com/skratchdot/open-golang/open"
)

var (
	legacyMacroLibraryWin     *eui.WindowData
	legacyMacroLibraryRoot    *eui.ItemData
	legacyMacroLibraryList    *eui.ItemData
	legacyMacroLibraryButtons *eui.ItemData
)

const (
	legacyMacroLibraryButtonsHeight = 24
	legacyMacroLibraryBottomGap     = 8
)

func makeLegacyMacroLibraryWindow() {
	if legacyMacroLibraryWin != nil {
		return
	}
	legacyMacroLibraryWin = eui.NewWindow()
	legacyMacroLibraryWin.Title = "Legacy Macros"
	legacyMacroLibraryWin.Size = eui.Point{X: 720, Y: 560}
	legacyMacroLibraryWin.Closable = true
	legacyMacroLibraryWin.Movable = true
	legacyMacroLibraryWin.Resizable = true
	legacyMacroLibraryWin.NoScroll = true
	legacyMacroLibraryWin.SetZone(eui.HZoneCenter, eui.VZoneMiddleTop)

	legacyMacroLibraryRoot = &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL, Fixed: true}
	legacyMacroLibraryWin.AddItem(legacyMacroLibraryRoot)

	legacyMacroLibraryList = &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL, Scrollable: true, Fixed: true}
	legacyMacroLibraryRoot.AddItem(legacyMacroLibraryList)

	legacyMacroLibraryButtons = &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_HORIZONTAL, Fixed: true}
	openButton, openEvents := eui.NewButton()
	openButton.Text = "Open Macro Folder"
	openButton.Size = eui.Point{X: 160, Y: legacyMacroLibraryButtonsHeight}
	openEvents.Handle = func(event eui.UIEvent) {
		if event.Type == eui.EventClick {
			if err := open.Run(legacyMacroLibraryPath()); err != nil {
				legacyMacroLibraryReport(fmt.Sprintf("open macro library: %v", err))
			}
		}
	}
	legacyMacroLibraryButtons.AddItem(openButton)
	reloadButton, reloadEvents := eui.NewButton()
	reloadButton.Text = "Reload Macros"
	reloadButton.Size = eui.Point{X: 120, Y: legacyMacroLibraryButtonsHeight}
	reloadButton.SetTooltip("Reload the selected macro files now. This restarts their active macro program.")
	reloadEvents.Handle = func(event eui.UIEvent) {
		if event.Type == eui.EventClick {
			legacyMacroLibraryReload()
		}
	}
	legacyMacroLibraryButtons.AddItem(reloadButton)
	legacyMacroLibraryRoot.AddItem(legacyMacroLibraryButtons)

	legacyMacroLibraryWin.OnResize = func() {
		refreshLegacyMacroLibraryWindow()
	}
	legacyMacroLibraryWin.AddWindow(false)
	refreshLegacyMacroLibraryWindow()
}

func refreshLegacyMacroLibraryWindow() {
	if legacyMacroLibraryList == nil {
		return
	}
	legacyMacroLibraryLayout()
	legacyMacroLibraryList.Contents = legacyMacroLibraryList.Contents[:0]

	character := legacyMacroLibraryCurrentCharacter()
	entries, err := legacyMacroLibraryEntries()
	if err != nil {
		legacyMacroLibraryReport("read macro library: " + err.Error())
		entries = nil
	}
	globalEnabled, err := legacyMacroLibraryEnabledIDs(legacyMacroLibraryGlobal, "")
	if err != nil {
		legacyMacroLibraryReport("read global macro selection: " + err.Error())
		globalEnabled = map[string]bool{}
	}
	playerEnabled := map[string]bool{}
	if character != "" {
		playerEnabled, err = legacyMacroLibraryEnabledIDs(legacyMacroLibraryPlayer, character)
		if err != nil {
			legacyMacroLibraryReport("read player macro selection: " + err.Error())
			playerEnabled = map[string]bool{}
		}
	}

	header := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_HORIZONTAL, Fixed: true}
	name, _ := eui.NewText()
	name.Text = "Macro"
	name.FontSize = 12
	name.Size = eui.Point{X: 390, Y: 24}
	header.AddItem(name)
	global, _ := eui.NewText()
	global.Text = "Global"
	global.FontSize = 12
	global.Size = eui.Point{X: 110, Y: 24}
	header.AddItem(global)
	player, _ := eui.NewText()
	if character == "" {
		player.Text = "Player"
	} else {
		player.Text = character
	}
	player.FontSize = 12
	player.Size = eui.Point{X: 150, Y: 24}
	header.AddItem(player)
	legacyMacroLibraryList.AddItem(header)
	if len(entries) == 0 {
		empty, _ := eui.NewText()
		empty.Text = "No .mac files found in Macros/Library."
		empty.FontSize = 12
		empty.Size = eui.Point{X: 650, Y: 24}
		legacyMacroLibraryList.AddItem(empty)
	}

	for index, entry := range entries {
		entry := entry
		row := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_HORIZONTAL, Fixed: true}
		row.Size = eui.Point{X: legacyMacroLibraryList.Size.X, Y: 28}
		if index%2 != 0 {
			row.Filled = true
			row.Color = legacyMacroLibraryStripeColor()
		}
		details, _ := eui.NewText()
		details.Text = entry.Name
		details.FontSize = 12
		details.Size = eui.Point{X: 390, Y: 28}
		row.AddItem(details)

		globalCheckbox, globalEvents := eui.NewCheckbox()
		globalCheckbox.Text = "On"
		globalCheckbox.Checked = globalEnabled[legacyMacroLibraryIDKey(entry.ID)]
		globalCheckbox.Size = eui.Point{X: 110, Y: 24}
		globalCheckbox.Disabled = isWASM
		globalCheckbox.SetTooltip("Enable this source for every character through Macros/Library/enabled.json. The active macro program reloads immediately; Default is not modified.")
		globalEvents.Handle = func(event eui.UIEvent) {
			if event.Type == eui.EventCheckboxChanged {
				legacyMacroLibrarySetEnabled(entry, legacyMacroLibraryGlobal, "", event.Checked)
			}
		}
		row.AddItem(globalCheckbox)

		playerCheckbox, playerEvents := eui.NewCheckbox()
		playerCheckbox.Text = "On"
		playerCheckbox.Checked = playerEnabled[legacyMacroLibraryIDKey(entry.ID)]
		playerCheckbox.Size = eui.Point{X: 150, Y: 24}
		playerCheckbox.Disabled = character == "" || isWASM
		if character == "" {
			playerCheckbox.SetTooltip("Select a character first.")
		} else {
			playerCheckbox.SetTooltip("Enable this source only for " + character + " through Macros/Library/enabled.json. The active macro program reloads immediately.")
		}
		playerEvents.Handle = func(event eui.UIEvent) {
			if event.Type == eui.EventCheckboxChanged && character != "" {
				legacyMacroLibrarySetEnabled(entry, legacyMacroLibraryPlayer, character, event.Checked)
			}
		}
		row.AddItem(playerCheckbox)
		legacyMacroLibraryList.AddItem(row)
	}

	if legacyMacroLibraryWin != nil {
		legacyMacroLibraryWin.Refresh()
	}
}

// legacyMacroLibraryLayout gives the fixed root and scrollable list explicit
// sizes. Without this, the root can shrink to its content and clip macro rows.
func legacyMacroLibraryLayout() {
	if legacyMacroLibraryWin == nil || legacyMacroLibraryRoot == nil || legacyMacroLibraryList == nil {
		return
	}
	clientW := legacyMacroLibraryWin.GetSize().X
	clientH := legacyMacroLibraryWin.GetSize().Y - legacyMacroLibraryWin.GetTitleSize()
	scale := eui.UIScale()
	if legacyMacroLibraryWin.NoScale {
		scale = 1
	}
	padding := (legacyMacroLibraryWin.Padding + legacyMacroLibraryWin.BorderPad) * scale
	clientW -= 2 * padding
	clientH -= 2 * padding
	if clientW < 0 {
		clientW = 0
	}
	if clientH < 0 {
		clientH = 0
	}
	legacyMacroLibraryRoot.Size = eui.Point{X: clientW, Y: clientH}
	if legacyMacroLibraryButtons != nil {
		legacyMacroLibraryButtons.Size = eui.Point{X: clientW, Y: legacyMacroLibraryButtonsHeight}
	}
	listHeight := clientH - legacyMacroLibraryButtonsHeight - legacyMacroLibraryBottomGap
	if listHeight < 24 {
		listHeight = 24
	}
	legacyMacroLibraryList.Size = eui.Point{X: clientW, Y: listHeight}
}

func legacyMacroLibraryCurrentCharacter() string {
	return strings.TrimSpace(utfFold(effectiveCharacterName()))
}

func legacyMacroLibrarySetEnabled(entry legacyMacroLibraryEntry, scope legacyMacroLibraryScope, character string, enabled bool) {
	result, err := setLegacyMacroLibraryEntryEnabled(entry.ID, scope, character, enabled)
	if err != nil {
		legacyMacroLibraryReport("update " + entry.Name + ": " + err.Error())
		refreshLegacyMacroLibraryWindow()
		return
	}
	if !result.Changed {
		if enabled {
			legacyMacroLibraryReport(entry.Name + " is already enabled")
		} else {
			legacyMacroLibraryReport(entry.Name + " is already disabled")
		}
		refreshLegacyMacroLibraryWindow()
		return
	}
	message := entry.Name + " disabled; source kept at " + result.SourcePath
	if enabled {
		message = entry.Name + " enabled; selection: " + result.SelectionPath
	}
	if err := loadLegacyMacrosForCharacter(legacyMacroLibraryCurrentCharacter()); err != nil {
		message += "; reload failed: " + err.Error()
	} else {
		message += "; macros reloaded"
	}
	legacyMacroLibraryReport(message)
	refreshLegacyMacroLibraryWindow()
}

func legacyMacroLibraryReload() {
	if err := loadLegacyMacrosForCharacter(legacyMacroLibraryCurrentCharacter()); err != nil {
		legacyMacroLibraryReport("reload macros: " + err.Error())
	} else {
		legacyMacroLibraryReport("macros reloaded")
	}
	refreshLegacyMacroLibraryWindow()
}

func legacyMacroLibraryStripeColor() eui.Color {
	if legacyMacroLibraryWin != nil && legacyMacroLibraryWin.Theme != nil {
		return legacyMacroLibraryWin.Theme.Window.TitleBGColor
	}
	return eui.ColorDarkGray
}

func legacyMacroLibraryReport(message string) {
	consoleMessage("legacy macros: " + message)
	showNotification(message)
}
