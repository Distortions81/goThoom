package main

import (
	"strings"

	"gothoom/eui"
)

const scriptSettingsContentWidth float32 = 540

func openscriptConfigWindow(owner string) {
	if scriptIsDisabled(owner) {
		return
	}
	if scriptConfigWin != nil {
		scriptConfigWin.Close()
	}
	scriptConfigMu.RLock()
	entries := append([]scriptConfigEntry(nil), scriptConfigEntries[owner]...)
	scriptConfigMu.RUnlock()
	win := eui.NewWindow()
	scriptConfigWin, scriptConfigOwner = win, owner
	win.Title = "Settings: " + scriptDisplayName(owner)
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
			scriptConfigWin, scriptConfigOwner = nil, ""
		}
	}
	outer := eui.NewColumn()
	preferences := eui.NewColumn()
	preferences.Name = "Preferences"
	if len(entries) == 0 {
		preferences.AddItem(scriptSettingsText("This script has no preferences."))
	} else {
		preferences.AddItem(scriptSettingsText("Preferences save as you change them. Each field shows which characters it affects."))
		addScriptPreferenceControls(preferences, owner, entries)
	}
	bindings := eui.NewColumn()
	bindings.Name = "Key bindings"
	bindings.AddItem(scriptSettingsText("Key bindings apply to all characters using this script. Apply saves each edit; Reset restores the script's default."))
	keys := scriptHotkeys(owner)
	if len(keys) == 0 {
		bindings.AddItem(scriptSettingsText("This script has no key bindings."))
	}
	for _, hk := range keys {
		original := scriptHotkeyDefault(hk)
		input := addScriptControlEditor(bindings, "Default: "+original, hk.Combo, original, true, func(value string) error {
			return setScriptBinding(owner, original, value)
		})
		recordingInputs = append(recordingInputs, input)
	}
	commands := eui.NewColumn()
	commands.Name = "Commands"
	commands.AddItem(scriptSettingsText("Local command names apply to all characters using this script. Arguments and actions stay the same. Apply saves each edit; Reset restores the default name."))
	registered := scriptCommandSettingsFor(owner)
	if len(registered) == 0 {
		commands.AddItem(scriptSettingsText("This script has no local commands."))
	}
	for _, entry := range registered {
		addScriptControlEditor(commands, "Default: /"+entry.Default, entry.Value, entry.Default, false, func(value string) error {
			return setScriptCommandName(owner, entry.Default, value)
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
