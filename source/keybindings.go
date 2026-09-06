package main

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"gothoom/eui"
)

type keybindingEntry struct {
	Binding string
	Owner   string
	Kind    string
	Line    int
}

var (
	keybindingsWin  *eui.WindowData
	keybindingsList *eui.ItemData
)

func makeKeybindingsWindow() {
	if keybindingsWin != nil {
		return
	}
	keybindingsWin = eui.NewWindow()
	keybindingsWin.Title = "Keybindings"
	keybindingsWin.Size = eui.Point{X: 500, Y: 400}
	keybindingsWin.Closable = true
	keybindingsWin.Movable = true
	keybindingsWin.Resizable = true
	keybindingsWin.NoScroll = true
	keybindingsWin.SetZone(eui.HZoneCenter, eui.VZoneMiddleTop)

	flow := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL, Fixed: true}
	keybindingsWin.AddItem(flow)
	buttons := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_HORIZONTAL, Fixed: true}
	buttons.Size = eui.Point{X: keybindingsWin.Size.X, Y: 28}
	testButton, testEvents := eui.NewButton()
	testButton.Text = "Test Keyboard/Mouse"
	setMaterialButtonIcon(testButton, "keyboard")
	testButton.Size = eui.Point{X: 160, Y: 24}
	testButton.SetTooltip("Show detected input.")
	testEvents.Handle = func(event eui.UIEvent) {
		if event.Type == eui.EventClick {
			openKeyboardTestWindow()
		}
	}
	buttons.AddItem(testButton)
	flow.AddItem(buttons)
	keybindingsList = &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL, Scrollable: true, Fixed: true}
	flow.AddItem(keybindingsList)

	keybindingsWin.OnResize = func() {
		refreshKeybindingsList()
		if keybindingsWin != nil {
			keybindingsWin.Refresh()
		}
	}
	keybindingsWin.AddWindow(false)
	refreshKeybindingsList()
}

func refreshKeybindingsList() {
	if keybindingsList == nil || keybindingsWin == nil {
		return
	}

	clientW := keybindingsWin.GetSize().X
	clientH := keybindingsWin.GetSize().Y - keybindingsWin.GetTitleSize()
	scale := eui.UIScale()
	if keybindingsWin.NoScale {
		scale = 1
	}
	pad := (keybindingsWin.Padding + keybindingsWin.BorderPad) * scale
	eui.SizeTextWindowList(keybindingsList, max(0, clientW-2*pad), max(0, clientH-2*pad))

	keybindingsList.Contents = keybindingsList.Contents[:0]
	entries := keybindingEntriesSnapshot()
	fontSize := float32(gs.ConsoleFontSize)
	if fontSize <= 0 {
		fontSize = 12
	}
	if len(entries) == 0 {
		empty, _ := eui.NewText()
		empty.Text = "No script or legacy macro keybindings are active."
		empty.FontSize = fontSize
		empty.Size = eui.Point{X: keybindingsList.Size.X, Y: 24}
		keybindingsList.AddItem(empty)
	} else {
		for i, entry := range entries {
			line := fmt.Sprintf("%s  —  %s (%s)", entry.Binding, entry.Owner, entry.Kind)
			if entry.Line > 0 {
				line += fmt.Sprintf(":%d", entry.Line)
			}
			row, _ := eui.NewText()
			row.Text = line
			row.FontSize = fontSize
			row.Size = eui.Point{X: keybindingsList.Size.X, Y: 24}
			restoreAlternateTextRow(row, i, false)
			keybindingsList.AddItem(row)
		}
	}
	keybindingsList.Dirty = true
	keybindingsWin.Refresh()
}

func keybindingEntriesSnapshot() []keybindingEntry {
	hotkeysMu.RLock()
	registered := append([]Hotkey(nil), hotkeys...)
	hotkeysMu.RUnlock()
	program := legacyMacroProgram{}
	if legacyMacroRuntimeSnapshot() != nil {
		program = legacyMacroProgramSnapshot()
	}
	return collectKeybindingEntries(registered, program, getscriptDisplayName, func(hotkey Hotkey) bool {
		if hotkey.Disabled || !scriptIsRunning(hotkey.Script) {
			return false
		}
		_, registered := scriptGetHotkeyFn(hotkey.Script, hotkey.Combo)
		return registered
	})
}

func collectKeybindingEntries(registered []Hotkey, program legacyMacroProgram, displayName func(string) string, scriptActive func(Hotkey) bool) []keybindingEntry {
	var entries []keybindingEntry
	var activeLegacyDeclarations []legacyMacroDeclaration
	for _, declaration := range program.Macros {
		switch declaration.Kind {
		case legacyMacroKey, legacyMacroClick, legacyMacroWheel:
		default:
			continue
		}
		binding := strings.TrimSpace(declaration.Trigger)
		if binding == "" {
			continue
		}
		if !legacyMacroBindingCanFire(declaration, activeLegacyDeclarations) {
			continue
		}
		activeLegacyDeclarations = append(activeLegacyDeclarations, declaration)
		owner := filepath.Base(declaration.Header.Path)
		if owner == "" || owner == "." {
			owner = "legacy macro"
		}
		entries = append(entries, keybindingEntry{
			Binding: binding,
			Owner:   owner,
			Kind:    "macro",
			Line:    declaration.Header.Line,
		})
	}

	var activeScriptBindings []string
	for _, hotkey := range registered {
		if hotkey.Script == "" || strings.TrimSpace(hotkey.Combo) == "" || scriptActive == nil || !scriptActive(hotkey) {
			continue
		}
		shadowed := legacyMacroShadowsScriptBinding(activeLegacyDeclarations, hotkey.Combo)
		for _, binding := range activeScriptBindings {
			if sameCombo(binding, hotkey.Combo) {
				shadowed = true
				break
			}
		}
		if shadowed {
			continue
		}
		activeScriptBindings = append(activeScriptBindings, hotkey.Combo)
		owner := hotkey.Script
		if displayName != nil {
			owner = displayName(hotkey.Script)
		}
		entries = append(entries, keybindingEntry{
			Binding: hotkey.Combo,
			Owner:   owner,
			Kind:    "script",
		})
	}

	sort.SliceStable(entries, func(i, j int) bool {
		left := strings.ToLower(entries[i].Binding)
		right := strings.ToLower(entries[j].Binding)
		if left != right {
			return left < right
		}
		if entries[i].Kind != entries[j].Kind {
			return entries[i].Kind < entries[j].Kind
		}
		if entries[i].Owner != entries[j].Owner {
			return entries[i].Owner < entries[j].Owner
		}
		return entries[i].Line < entries[j].Line
	})
	return entries
}

func legacyMacroBindingCanFire(declaration legacyMacroDeclaration, earlier []legacyMacroDeclaration) bool {
	if declaration.Key.Modifiers&legacyMacroModNumpad != 0 {
		switch declaration.Kind {
		case legacyMacroClick, legacyMacroWheel:
			return false
		case legacyMacroKey:
			name := strings.ToLower(declaration.Key.Name)
			if !(len(name) == 1 && name[0] >= '0' && name[0] <= '9') {
				switch name {
				case ".", "/", "minus", "enter", "=", "+", "*":
				default:
					return false
				}
			}
		}
	}

	switch declaration.Kind {
	case legacyMacroKey:
		name := strings.ToLower(declaration.Key.Name)
		if name == "undo" || (name == "enter" && declaration.Key.Modifiers&legacyMacroModNumpad == 0) {
			return false
		}
		for _, previous := range earlier {
			if previous.Kind == legacyMacroKey && legacyMacroKeyMatches(previous, declaration.Key.Name, declaration.Key.Modifiers) {
				return false
			}
		}
	case legacyMacroWheel:
		if declaration.Key.Modifiers&legacyMacroModShift != 0 &&
			(declaration.Key.Name == "wheelup" || declaration.Key.Name == "wheeldown") {
			return false
		}
		for _, previous := range earlier {
			if previous.Kind == legacyMacroWheel && legacyMacroKeyMatches(previous, declaration.Key.Name, declaration.Key.Modifiers) {
				return false
			}
		}
	case legacyMacroClick:
		if declaration.Key.Button < 1 || declaration.Key.Button > 5 {
			return false
		}
		currentAnyClick := declaration.Attributes&legacyMacroAnyClick != 0
		for _, previous := range earlier {
			if previous.Kind != legacyMacroClick || previous.Key.Button != declaration.Key.Button || previous.Key.Modifiers != declaration.Key.Modifiers {
				continue
			}
			previousAnyClick := previous.Attributes&legacyMacroAnyClick != 0
			if previousAnyClick || !currentAnyClick {
				return false
			}
		}
	}
	return true
}

func legacyMacroShadowsScriptBinding(declarations []legacyMacroDeclaration, combo string) bool {
	kind, binding, ok := scriptComboLegacyBinding(combo)
	if !ok {
		return false
	}
	for _, declaration := range declarations {
		matches := false
		switch kind {
		case legacyMacroKey, legacyMacroWheel:
			matches = declaration.Kind == kind && legacyMacroKeyMatches(declaration, binding.Name, binding.Modifiers)
		case legacyMacroClick:
			matches = declaration.Kind == kind && declaration.Key.Button == binding.Button && declaration.Key.Modifiers == binding.Modifiers &&
				declaration.Attributes&legacyMacroAnyClick != 0
		}
		if matches {
			return declaration.Attributes&legacyMacroNoOverride == 0
		}
	}
	return false
}

func scriptComboLegacyBinding(combo string) (legacyMacroKind, legacyMacroKeyBinding, bool) {
	parts := strings.Split(strings.TrimSpace(combo), "-")
	if len(parts) == 0 {
		return 0, legacyMacroKeyBinding{}, false
	}
	modifiers := make([]string, 0, len(parts)-1)
	for _, part := range parts[:len(parts)-1] {
		switch strings.ToLower(strings.TrimSpace(part)) {
		case "meta", "metaleft", "metaright", "command", "cmd":
			modifiers = append(modifiers, "command")
		case "ctrl", "control", "controlleft", "controlright":
			modifiers = append(modifiers, "control")
		case "alt", "altleft", "altright", "option":
			modifiers = append(modifiers, "option")
		case "shift", "shiftleft", "shiftright":
			modifiers = append(modifiers, "shift")
		default:
			return 0, legacyMacroKeyBinding{}, false
		}
	}

	trigger := strings.ToLower(strings.TrimSpace(parts[len(parts)-1]))
	key := trigger
	switch trigger {
	case "leftclick":
		key = "click"
	case "rightclick":
		key = "click2"
	case "middleclick":
		key = "click3"
	case "mouse4", "mouse 4":
		key = "click4"
	case "mouse5", "mouse 5":
		key = "click5"
	case "arrowup":
		key = "up"
	case "arrowdown":
		key = "down"
	case "arrowleft":
		key = "left"
	case "arrowright":
		key = "right"
	case "backquote":
		key = "`"
	case "backslash", "intlbackslash":
		key = "\\"
	case "backspace":
		key = "delete"
	case "bracketleft":
		key = "["
	case "bracketright":
		key = "]"
	case "comma":
		key = ","
	case "delete":
		key = "del"
	case "equal":
		key = "="
	case "enter":
		key = "return"
	case "insert":
		key = "help"
	case "period":
		key = "."
	case "quote":
		key = "'"
	case "semicolon":
		key = ";"
	case "slash":
		key = "/"
	default:
		if strings.HasPrefix(trigger, "digit") && len(trigger) == len("digit0") {
			key = trigger[len("digit"):]
		} else if strings.HasPrefix(trigger, "numpad") {
			modifiers = append(modifiers, "numpad")
			switch strings.TrimPrefix(trigger, "numpad") {
			case "add":
				key = "+"
			case "decimal":
				key = "."
			case "divide":
				key = "/"
			case "enter":
				key = "enter"
			case "equal":
				key = "="
			case "multiply":
				key = "*"
			case "subtract":
				key = "minus"
			default:
				key = strings.TrimPrefix(trigger, "numpad")
			}
		}
	}
	legacyTrigger := strings.Join(append(modifiers, key), "-")
	return parseLegacyMacroKeyBinding(legacyTrigger)
}
