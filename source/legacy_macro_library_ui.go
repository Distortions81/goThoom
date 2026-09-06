package main

import (
	"fmt"
	"sort"
	"strings"

	"gothoom/eui"

	text "github.com/hajimehoshi/ebiten/v2/text/v2"
	open "github.com/skratchdot/open-golang/open"
)

var (
	legacyMacroLibraryWin        *eui.WindowData
	legacyMacroLibraryRoot       *eui.ItemData
	legacyMacroLibraryList       *eui.ItemData
	legacyMacroLibraryButtons    *eui.ItemData
	legacyMacroLibraryErrors     *eui.ItemData
	legacyMacroLibraryContinuous *eui.ItemData
)

const (
	legacyMacroNameWidth       = 360
	legacyMacroGlobalWidth     = 110
	legacyMacroPlayerWidth     = 150
	legacyMacroListWidth       = 720
	legacyMacroPaneHeight      = 420
	legacyMacroRowHeight       = 28
	legacyMacroContinuousLabel = "Allow continuous macros"
)

func makeLegacyMacroLibraryWindow() {
	if legacyMacroLibraryWin != nil {
		return
	}
	legacyMacroLibraryWin = eui.NewWindow()
	legacyMacroLibraryWin.Title = "Legacy Macros"
	legacyMacroLibraryWin.Closable = true
	legacyMacroLibraryWin.Movable = true
	legacyMacroLibraryWin.Resizable = true
	legacyMacroLibraryWin.NoScroll = true
	legacyMacroLibraryWin.Size = eui.Point{X: legacyMacroListWidth, Y: 560}
	legacyMacroLibraryWin.SetZone(eui.HZoneCenterLeft, eui.VZoneMiddleTop)

	legacyMacroLibraryRoot = &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL, Fixed: true}
	legacyMacroLibraryWin.AddItem(legacyMacroLibraryRoot)

	listHeader := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_HORIZONTAL, Fixed: true}
	infoHeader, _ := eui.NewText()
	infoHeader.Size = eui.Point{X: 68, Y: 24}
	listHeader.AddItem(infoHeader)
	nameHeader, _ := eui.NewText()
	nameHeader.Text = "Macro"
	nameHeader.FontSize = 12
	nameHeader.Size = eui.Point{X: legacyMacroNameWidth, Y: 24}
	listHeader.AddItem(nameHeader)
	globalHeader, _ := eui.NewText()
	globalHeader.Text = "Global"
	globalHeader.FontSize = 12
	globalHeader.Size = eui.Point{X: legacyMacroGlobalWidth, Y: 24}
	listHeader.AddItem(globalHeader)
	playerHeader, _ := eui.NewText()
	playerHeader.Text = "Player"
	playerHeader.FontSize = 12
	playerHeader.Size = eui.Point{X: legacyMacroPlayerWidth, Y: 24}
	listHeader.AddItem(playerHeader)
	legacyMacroLibraryRoot.AddItem(listHeader)
	legacyMacroLibraryList = &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL, Scrollable: true, Fixed: true}
	legacyMacroLibraryList.Size = eui.Point{X: legacyMacroListWidth, Y: legacyMacroPaneHeight}
	legacyMacroLibraryRoot.AddItem(legacyMacroLibraryList)

	legacyMacroLibraryButtons = &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_HORIZONTAL, Fixed: true}
	legacyMacroLibraryRoot.AddItem(legacyMacroLibraryButtons)

	refreshButton, refreshEvents := eui.NewButton()
	refreshButton.Text = "Refresh"
	setMaterialButtonIcon(refreshButton, "refresh")
	refreshButton.Size = eui.Point{X: 80, Y: 24}
	refreshButton.Disabled = isWASM
	refreshButton.SetTooltip("Rescan the macro library.")
	if isWASM {
		refreshButton.SetTooltip("Embedded macro list is fixed.")
	}
	refreshEvents.Handle = func(event eui.UIEvent) {
		if event.Type == eui.EventClick {
			refreshLegacyMacroLibraryWindow()
		}
	}
	legacyMacroLibraryButtons.AddItem(refreshButton)

	openButton, openEvents := eui.NewButton()
	openButton.Text = "Open Folder"
	setMaterialButtonIcon(openButton, "folder_open")
	openButton.Size = eui.Point{X: 112, Y: 24}
	openButton.Disabled = isWASM
	if isWASM {
		openButton.SetTooltip("Embedded library is read-only.")
	}
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
	setMaterialButtonIcon(reloadButton, "restart_alt")
	reloadButton.Size = eui.Point{X: 112, Y: 24}
	reloadButton.SetTooltip("Reload enabled macros.")
	reloadEvents.Handle = func(event eui.UIEvent) {
		if event.Type == eui.EventClick {
			legacyMacroLibraryReload()
		}
	}
	legacyMacroLibraryButtons.AddItem(reloadButton)

	errorsButton, errorsEvents := eui.NewButton()
	errorsButton.Text = "No Errors"
	setMaterialButtonIcon(errorsButton, "check_circle")
	errorsButton.Size = eui.Point{X: 96, Y: 24}
	errorsButton.SetTooltip("Show macro errors.")
	errorsEvents.Handle = func(event eui.UIEvent) {
		if event.Type == eui.EventClick {
			legacyMacroLibraryShowDiagnostics()
		}
	}
	legacyMacroLibraryErrors = errorsButton
	legacyMacroLibraryButtons.AddItem(errorsButton)

	continuousCheckbox, continuousEvents := eui.NewCheckbox()
	continuousCheckbox.Text = legacyMacroContinuousLabel
	continuousCheckbox.Size = eui.Point{X: 200, Y: 24}
	continuousCheckbox.Checked = gs.LegacyMacroContinuous
	continuousCheckbox.SetTooltip("Allow continuous macro loops.")
	continuousEvents.Handle = func(event eui.UIEvent) {
		if event.Type == eui.EventCheckboxChanged {
			legacyMacroLibrarySetAllowContinuous(event.Checked)
		}
	}
	legacyMacroLibraryContinuous = continuousCheckbox
	legacyMacroLibraryRoot.AddItem(continuousCheckbox)

	legacyMacroLibraryWin.OnResize = refreshLegacyMacroLibraryWindow
	legacyMacroLibraryWin.AddWindow(false)
	refreshLegacyMacroLibraryWindow()
}

func refreshLegacyMacroLibraryWindow() {
	if legacyMacroLibraryList == nil || legacyMacroLibraryWin == nil {
		return
	}
	savedScroll := legacyMacroLibraryList.Scroll
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
	legacyMacroLibraryRefreshErrorsButton()
	if legacyMacroLibraryContinuous != nil {
		legacyMacroLibraryContinuous.Checked = gs.LegacyMacroContinuous
	}

	nameSize := eui.Point{X: legacyMacroNameWidth, Y: legacyMacroRowHeight}
	if len(entries) == 0 {
		empty, _ := eui.NewText()
		empty.Text = "No .mac or .txt files found in Macros/Library."
		empty.FontSize = 12
		empty.Size = nameSize
		legacyMacroLibraryList.AddItem(empty)
	}

	for index, entry := range entries {
		entry := entry
		row := eui.NewRow()
		if index%2 != 0 {
			row.Filled = true
			row.Color = legacyMacroLibraryStripeColor()
		}
		infoButton, infoEvents := eui.NewButton()
		infoButton.Text = "i"
		setMaterialIconOnly(infoButton, "info", "i")
		infoButton.Size = eui.Point{X: 24, Y: 24}
		infoButton.SetTooltip("Show macro details, commands, and keybindings.")
		infoEvents.Handle = func(event eui.UIEvent) {
			if event.Type == eui.EventClick {
				legacyMacroLibraryShowInfo(entry)
			}
		}
		row.AddItem(infoButton)

		editButton, editEvents := eui.NewButton()
		editButton.Text = "Edit"
		setMaterialButtonIcon(editButton, "edit")
		editButton.Size = eui.Point{X: 44, Y: 24}
		editButton.Disabled = isWASM
		editButton.SetTooltip("Open this macro file.")
		if isWASM {
			editButton.SetTooltip("Embedded library is read-only.")
		}
		editEvents.Handle = func(event eui.UIEvent) {
			if event.Type == eui.EventClick && !editButton.Disabled {
				if err := open.Run(entry.Path); err != nil {
					legacyMacroLibraryReport(fmt.Sprintf("edit %s: %v", entry.Name, err))
				}
			}
		}
		row.AddItem(editButton)

		details, _ := eui.NewText()
		details.Text = legacyMacroLibraryRowLabel(entry.Name, entry.Description, legacyMacroNameWidth)
		details.FontSize = 12
		details.Size = nameSize
		row.AddItem(details)

		globalCheckbox, globalEvents := eui.NewCheckbox()
		globalCheckbox.Text = "On"
		globalCheckbox.Checked = globalEnabled[legacyMacroLibraryIDKey(entry.ID)]
		globalCheckbox.Size = eui.Point{X: legacyMacroGlobalWidth, Y: 24}
		globalCheckbox.Disabled = isWASM
		globalCheckbox.SetTooltip("Enable for every character.")
		globalEvents.Handle = func(event eui.UIEvent) {
			if event.Type == eui.EventCheckboxChanged {
				legacyMacroLibrarySetEnabled(entry, legacyMacroLibraryGlobal, "", event.Checked, globalCheckbox)
			}
		}
		row.AddItem(globalCheckbox)

		playerCheckbox, playerEvents := eui.NewCheckbox()
		playerCheckbox.Text = "On"
		playerCheckbox.Checked = playerEnabled[legacyMacroLibraryIDKey(entry.ID)]
		playerCheckbox.Size = eui.Point{X: legacyMacroPlayerWidth, Y: 24}
		playerCheckbox.Disabled = character == "" || isWASM
		if character == "" {
			playerCheckbox.SetTooltip("Select a character first.")
		} else {
			playerCheckbox.SetTooltip("Enable for " + character + ".")
		}
		playerEvents.Handle = func(event eui.UIEvent) {
			if event.Type == eui.EventCheckboxChanged && character != "" {
				legacyMacroLibrarySetEnabled(entry, legacyMacroLibraryPlayer, character, event.Checked, playerCheckbox)
			}
		}
		row.AddItem(playerCheckbox)
		legacyMacroLibraryList.AddItem(row)
	}
	// AddItem clamps an incomplete rebuilt flow to the top. Put the prior
	// offset back once all macro rows exist; Refresh handles any real shrink.
	legacyMacroLibraryList.Scroll = savedScroll

	if legacyMacroLibraryWin != nil {
		legacyMacroLibraryWin.Refresh()
	}
}

func legacyMacroLibraryLayout() {
	scale := eui.UIScale()
	if legacyMacroLibraryWin.NoScale {
		scale = 1
	}
	clientW := legacyMacroLibraryWin.GetSize().X/scale - 2*(legacyMacroLibraryWin.Padding+legacyMacroLibraryWin.BorderPad)
	clientH := (legacyMacroLibraryWin.GetSize().Y-legacyMacroLibraryWin.GetTitleSize())/scale - 2*(legacyMacroLibraryWin.Padding+legacyMacroLibraryWin.BorderPad)
	clientW = max(0, clientW)
	clientH = max(0, clientH)

	legacyMacroLibraryRoot.Size = eui.Point{X: clientW, Y: clientH}
	legacyMacroLibraryButtons.Size = eui.Point{X: clientW, Y: 24}
	legacyMacroLibraryContinuous.Size = eui.Point{X: clientW, Y: 24}
	legacyMacroLibraryList.Size = eui.Point{X: clientW, Y: max(float32(24), clientH-24-24-24)}
}

func legacyMacroLibraryCurrentCharacter() string {
	return strings.TrimSpace(utfFold(effectiveCharacterName()))
}

func legacyMacroLibrarySetEnabled(entry legacyMacroLibraryEntry, scope legacyMacroLibraryScope, character string, enabled bool, checkbox *eui.ItemData) {
	result, err := setLegacyMacroLibraryEntryEnabled(entry.ID, scope, character, enabled)
	if err != nil {
		if checkbox != nil {
			checkbox.Checked = !enabled
			checkbox.Dirty = true
		}
		legacyMacroLibraryReport("update " + entry.Name + ": " + err.Error())
		legacyMacroLibraryRefreshStatus()
		return
	}
	if !result.Changed {
		if enabled {
			legacyMacroLibraryReport(entry.Name + " is already enabled")
		} else {
			legacyMacroLibraryReport(entry.Name + " is already disabled")
		}
		legacyMacroLibraryRefreshStatus()
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
	legacyMacroLibraryRefreshStatus()
}

func legacyMacroLibraryReload() {
	if err := loadLegacyMacrosForCharacter(legacyMacroLibraryCurrentCharacter()); err != nil {
		legacyMacroLibraryReport("reload macros: " + err.Error())
	} else {
		legacyMacroLibraryReport("macros reloaded")
	}
	refreshLegacyMacroLibraryWindow()
}

func legacyMacroLibrarySetAllowContinuous(enabled bool) {
	SettingsLock.Lock()
	gs.LegacyMacroContinuous = enabled
	settingsDirty = true
	SettingsLock.Unlock()

	message := "continuous macros disabled; 10,000-instruction runaway-loop protection enabled"
	if enabled {
		message = "continuous macros enabled; classic time-slicing active"
	}
	if err := loadLegacyMacrosForCharacter(legacyMacroLibraryCurrentCharacter()); err != nil {
		message += "; reload failed: " + err.Error()
	} else {
		message += "; macros reloaded"
	}
	legacyMacroLibraryReport(message)
	legacyMacroLibraryRefreshStatus()
}

func legacyMacroLibraryDiagnostics() []legacyMacroDiagnostic {
	program := legacyMacroProgramSnapshot()
	diagnostics := append([]legacyMacroDiagnostic(nil), program.Diagnostics...)
	if runtime := legacyMacroRuntimeSnapshot(); runtime != nil {
		diagnostics = append(diagnostics, runtime.diagnosticsSnapshot()...)
	}
	return diagnostics
}

func legacyMacroLibraryRefreshErrorsButton() {
	if legacyMacroLibraryErrors == nil {
		return
	}
	count := len(legacyMacroLibraryDiagnostics())
	text := "No Errors"
	if count == 0 {
		text = "No Errors"
		setMaterialButtonIcon(legacyMacroLibraryErrors, "check_circle")
	} else {
		text = fmt.Sprintf("Errors (%d)", count)
		setMaterialButtonIcon(legacyMacroLibraryErrors, "error")
	}
	legacyMacroLibraryErrors.UpdateText(text)
	legacyMacroLibraryErrors.Disabled = false
}

func legacyMacroLibraryRefreshStatus() {
	legacyMacroLibraryRefreshErrorsButton()
	if legacyMacroLibraryWin != nil {
		legacyMacroLibraryWin.Refresh()
	}
}

func legacyMacroLibraryShowDiagnostics() {
	diagnostics := legacyMacroLibraryDiagnostics()
	if len(diagnostics) == 0 {
		legacyMacroLibraryReport("no errors")
		return
	}
	lines := make([]string, 0, len(diagnostics))
	for _, diagnostic := range diagnostics {
		lines = append(lines, diagnostic.Error())
	}
	eui.ShowPopup("Legacy Macro Errors", strings.Join(lines, "\n"), []eui.PopupButton{{Text: "Close"}})
}

func legacyMacroLibraryStripeColor() eui.Color {
	if legacyMacroLibraryWin != nil && legacyMacroLibraryWin.Theme != nil {
		return legacyMacroLibraryWin.Theme.Window.TitleBGColor
	}
	return eui.ColorDarkGray
}

func legacyMacroLibraryShowInfo(entry legacyMacroLibraryEntry) {
	info, err := collectLegacyMacroLibraryInfo(entry)
	if err != nil {
		legacyMacroLibraryReport("read " + entry.Name + ": " + err.Error())
		return
	}
	eui.ShowPopup("Macro Info: "+entry.Name, "", []eui.PopupButton{{Text: "Close"}}, legacyMacroLibraryInfoColumns(info))
}

type legacyMacroLibraryInfo struct {
	Metadata    []string
	Commands    []string
	Keybindings []string
}

func collectLegacyMacroLibraryInfo(entry legacyMacroLibraryEntry) (legacyMacroLibraryInfo, error) {
	var source legacyMacroSource
	if isWASM && entry.EmbeddedPath != "" {
		text, err := legacyMacroLibraryFS.ReadFile(entry.EmbeddedPath)
		if err != nil {
			return legacyMacroLibraryInfo{}, err
		}
		source = legacyMacroSource{Path: entry.EmbeddedPath, Text: decodeLegacyMacroSourceText(text)}
	} else {
		var exists bool
		var err error
		source, exists, err = readLegacyMacroSource(entry.Path)
		if err != nil {
			return legacyMacroLibraryInfo{}, err
		}
		if !exists {
			return legacyMacroLibraryInfo{}, fmt.Errorf("macro file no longer exists")
		}
	}
	source.Name = entry.ID
	program := parseLegacyMacroSources([]legacyMacroSource{source})
	info := legacyMacroLibraryInfo{
		Metadata:    make([]string, 0, 7),
		Commands:    make([]string, 0),
		Keybindings: make([]string, 0),
	}
	var activeBindings []legacyMacroDeclaration
	for _, declaration := range program.Macros {
		switch declaration.Kind {
		case legacyMacroExpression:
			info.Commands = append(info.Commands, declaration.Trigger)
		case legacyMacroReplacement:
			info.Commands = append(info.Commands, declaration.Trigger+" (replacement)")
		case legacyMacroKey, legacyMacroClick, legacyMacroWheel:
			if legacyMacroBindingCanFire(declaration, activeBindings) {
				activeBindings = append(activeBindings, declaration)
				info.Keybindings = append(info.Keybindings, legacyMacroLibraryKeybindingName(declaration))
			}
		}
	}
	sort.Strings(info.Commands)
	sort.Strings(info.Keybindings)
	if entry.Description != "" {
		info.Metadata = append(info.Metadata, "Description: "+entry.Description)
	}
	if entry.Version != "" {
		info.Metadata = append(info.Metadata, "Version: "+entry.Version)
	}
	if entry.Tags != "" {
		info.Metadata = append(info.Metadata, "Tags: "+entry.Tags)
	}
	if entry.Author != "" {
		info.Metadata = append(info.Metadata, "Author: "+entry.Author)
	}
	if entry.License != "" {
		info.Metadata = append(info.Metadata, "License: "+entry.License)
	}
	if entry.Website != "" {
		info.Metadata = append(info.Metadata, "Website: "+entry.Website)
	}
	if entry.Update != "" {
		info.Metadata = append(info.Metadata, "Update: "+entry.Update)
	}
	return info, nil
}

func legacyMacroLibraryInfoText(entry legacyMacroLibraryEntry) (string, error) {
	info, err := collectLegacyMacroLibraryInfo(entry)
	if err != nil {
		return "", err
	}
	sections := make([]string, 0, 3)
	if len(info.Metadata) > 0 {
		sections = append(sections, strings.Join(info.Metadata, "\n"))
	}
	if len(info.Commands) > 0 {
		sections = append(sections, "Commands:\n"+strings.Join(info.Commands, "\n"))
	}
	if len(info.Keybindings) > 0 {
		sections = append(sections, "Keybindings:\n"+strings.Join(info.Keybindings, "\n"))
	}
	if len(sections) == 0 {
		return "This macro has no command or keybinding triggers.", nil
	}
	return strings.Join(sections, "\n\n"), nil
}

func legacyMacroLibraryInfoColumns(info legacyMacroLibraryInfo) *eui.ItemData {
	const columnWidth = 230
	const columnHeight = 260
	columns := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_HORIZONTAL, Fixed: true}
	columns.Size = eui.Point{X: columnWidth * 3, Y: columnHeight}
	columns.AddItem(legacyMacroLibraryInfoColumn("About", info.Metadata, columnWidth, columnHeight))
	columns.AddItem(legacyMacroLibraryInfoColumn("Commands", info.Commands, columnWidth, columnHeight))
	columns.AddItem(legacyMacroLibraryInfoColumn("Keybindings", info.Keybindings, columnWidth, columnHeight))
	return columns
}

func legacyMacroLibraryInfoColumn(title string, lines []string, width, height float32) *eui.ItemData {
	column := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL, Scrollable: true, Fixed: true}
	column.Size = eui.Point{X: width, Y: height}
	heading, _ := eui.NewText()
	heading.Text = title
	heading.FontSize = 12
	heading.Size = eui.Point{X: width - eui.ScrollbarWidth(), Y: 24}
	column.AddItem(heading)
	if len(lines) == 0 {
		lines = []string{"None"}
	}
	lines = legacyMacroLibraryWrapInfoLines(lines, width-eui.ScrollbarWidth())
	content, _ := eui.NewText()
	content.Text = strings.Join(lines, "\n")
	content.FontSize = 12
	content.Size = eui.Point{X: width - eui.ScrollbarWidth(), Y: float32(len(lines)) * 18}
	column.AddItem(content)
	return column
}

func legacyMacroLibraryWrapInfoLines(lines []string, width float32) []string {
	face := legacyMacroLibraryRowFace()
	wrapped := make([]string, 0, len(lines))
	for _, line := range lines {
		_, chunks := eui.WrapText(line, face, float64(width*eui.UIScale()))
		wrapped = append(wrapped, chunks...)
	}
	return wrapped
}

func legacyMacroLibraryRowLabel(name, description string, width float32) string {
	description = strings.TrimSpace(description)
	if description == "" {
		return name
	}
	face := legacyMacroLibraryRowFace()
	maxWidth := float64(width * eui.UIScale())
	prefix := name + " — "
	if eui.MeasureTextWidth(prefix+"...", face) > maxWidth {
		return name
	}
	if eui.MeasureTextWidth(prefix+description, face) <= maxWidth {
		return prefix + description
	}
	var shortened strings.Builder
	for _, char := range description {
		candidate := shortened.String() + string(char)
		if eui.MeasureTextWidth(prefix+candidate+"...", face) > maxWidth {
			break
		}
		shortened.WriteRune(char)
	}
	if shortened.Len() == 0 {
		return name
	}
	return prefix + shortened.String() + "..."
}

func legacyMacroLibraryRowFace() text.Face {
	size := float64(12*eui.UIScale() + 2)
	if source := eui.FontSource(); source != nil {
		return &text.GoTextFace{Source: source, Size: size}
	}
	return &text.GoTextFace{Size: size}
}

func legacyMacroLibraryKeybindingName(declaration legacyMacroDeclaration) string {
	parts := make([]string, 0, 3)
	if declaration.Key.Modifiers&legacyMacroModCommand != 0 {
		parts = append(parts, "Command")
	}
	if declaration.Key.Modifiers&legacyMacroModControl != 0 {
		parts = append(parts, "Control")
	}
	if declaration.Key.Modifiers&legacyMacroModNumpad != 0 {
		parts = append(parts, "Numpad")
	}
	if declaration.Key.Modifiers&legacyMacroModOption != 0 {
		parts = append(parts, "Option")
	}
	if declaration.Key.Modifiers&legacyMacroModShift != 0 {
		parts = append(parts, "Shift")
	}

	name := declaration.Key.Name
	switch declaration.Kind {
	case legacyMacroClick:
		switch declaration.Key.Button {
		case 1:
			name = "Click"
		case 2:
			name = "Right Click"
		case 3:
			name = "Middle Click"
		default:
			name = fmt.Sprintf("Click %d", declaration.Key.Button)
		}
	case legacyMacroWheel:
		switch name {
		case "wheelup":
			name = "Wheel Up"
		case "wheeldown":
			name = "Wheel Down"
		case "wheelleft":
			name = "Wheel Left"
		case "wheelright":
			name = "Wheel Right"
		}
	}
	if len(name) == 1 || strings.HasPrefix(name, "f") {
		name = strings.ToUpper(name)
	}
	parts = append(parts, name)
	return strings.Join(parts, "-")
}

func legacyMacroLibraryReport(message string) {
	consoleMessage("legacy macros: " + message)
	showNotification(message)
}
