package main

import (
	"strings"

	"gothoom/eui"
)

const scriptSettingsContentWidth float32 = 540

func openscriptConfigWindow(owner string) {
	openScriptConfigWindowForSession(nil, owner)
}

func openScriptConfigWindowForSession(session *Session, owner string) {
	running, _ := session.scriptRuntimeSnapshot(owner)
	if !running {
		return
	}
	if scriptConfigWin != nil {
		scriptConfigWin.Close()
	}
	entries := session.scriptConfigEntriesSnapshot(owner)
	win := eui.NewWindow()
	sessionID := SessionID(0)
	if session != nil {
		sessionID = session.ID()
	}
	scriptConfigWin, scriptConfigOwner, scriptConfigSession = win, owner, sessionID
	win.Title = "Settings: " + scriptDisplayName(owner) + " — " + scriptManagerCharacter(session)
	win.ShowTooltipIndicators = true
	win.Size = eui.Point{X: 600, Y: 500}
	win.Closable, win.Movable = true, true
	win.Resizable = false
	win.SetZone(eui.HZoneCenterLeft, eui.VZoneMiddleTop)
	var recordingInputs []*eui.ItemData
	win.OnClose = func() {
		for _, input := range recordingInputs {
			if recordTarget == input {
				recording, recordTarget, recordedCombo = false, nil, ""
			}
		}
		win.RemoveWindow()
		if scriptConfigWin == win {
			scriptConfigWin, scriptConfigOwner, scriptConfigSession = nil, "", 0
		}
	}
	outer := eui.NewColumn()
	preferences := eui.NewColumn()
	preferences.Name = "Preferences"
	if len(entries) == 0 {
		preferences.AddItem(scriptSettingsText("This script has no preferences."))
	} else {
		preferences.AddItem(scriptSettingsText("Preferences save as you change them. Each field shows which characters it affects."))
		addScriptPreferenceControlsForSession(preferences, session, owner, entries)
	}
	bindings := eui.NewColumn()
	bindings.Name = "Key bindings"
	bindings.AddItem(scriptSettingsText("Key bindings apply to all characters using this script. Apply saves each edit; Reset restores the script's default."))
	keys := scriptHotkeysForSession(session, owner)
	if len(keys) == 0 {
		bindings.AddItem(scriptSettingsText("This script has no key bindings."))
	}
	for _, hk := range keys {
		original := scriptHotkeyDefault(hk)
		input := addScriptControlEditor(bindings, "Default: "+original, hk.Combo, original, true, func(value string) error {
			return setScriptBindingForSession(session, owner, original, value)
		})
		recordingInputs = append(recordingInputs, input)
	}
	commands := eui.NewColumn()
	commands.Name = "Commands"
	commands.AddItem(scriptSettingsText("Local command names apply to all characters using this script. Arguments and actions stay the same. Apply saves each edit; Reset restores the default name."))
	registered := scriptCommandSettingsForSession(session, owner)
	if len(registered) == 0 {
		commands.AddItem(scriptSettingsText("This script has no local commands."))
	}
	for _, entry := range registered {
		addScriptControlEditor(commands, "Default: /"+entry.Default, entry.Value, entry.Default, false, func(value string) error {
			return setScriptCommandNameForSession(session, owner, entry.Default, value)
		})
	}
	outer.Tabs = []*eui.ItemData{preferences, bindings, commands}
	win.AddItem(outer)
	win.AddWindow(false)
	win.MarkOpen()
}

func scriptSettingsText(text string) *eui.ItemData {
	return eui.NewWrappedLabel(text, scriptSettingsContentWidth)
}

func addScriptControlEditor(parent *eui.ItemData, label, value, defaultValue string, binding bool, apply func(string) error) *eui.ItemData {
	section := eui.NewColumn(scriptSettingsText(label))
	input, _ := eui.NewInput()
	input.Text = value
	input.Size = eui.Point{X: 230, Y: 28}
	status := scriptSettingsText("")
	row := eui.NewRow(input)
	button := func(label string, action func()) {
		item := eui.NewActionButton(label, action)
		item.Size.X = 84
		row.AddItem(item)
	}
	if binding {
		button("Record", func() { startHotkeyRecording(input) })
	}
	save := func(value string) {
		if recording && recordTarget == input {
			status.SetWrappedText("Finish recording before applying this binding.")
		} else if err := apply(value); err != nil {
			status.SetWrappedText(err.Error())
		} else {
			input.Text = strings.TrimSpace(value)
			if !binding {
				input.Text = normalizeScriptCommand(input.Text)
			}
			input.Dirty = true
			status.SetWrappedText("Saved.")
			refreshscriptDetails()
		}
		status.Dirty = true
		if scriptConfigWin != nil {
			scriptConfigWin.Refresh()
		}
	}
	button("Apply", func() { save(input.Text) })
	button("Reset", func() { save(defaultValue) })
	section.AddItem(row)
	section.AddItem(status)
	parent.AddItem(section)
	return input
}
