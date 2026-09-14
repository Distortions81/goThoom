package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode"

	"gothoom/eui"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

const hotkeysFile = "global-hotkeys.json"
const hotkeyCommandInputHeight float32 = 20
const clientHotkeysVersion = 1

type clientHotkeyAction uint8

const (
	clientHotkeyCommandPalette clientHotkeyAction = iota
	clientHotkeyMoveLeft
	clientHotkeyMoveRight
	clientHotkeyMoveUp
	clientHotkeyMoveDown
	clientHotkeyRun
	clientHotkeyActionCount
)

type compiledClientHotkey struct {
	key       ebiten.Key
	modifiers uint8
}

type compiledClientHotkeys [clientHotkeyActionCount][]compiledClientHotkey

const (
	clientHotkeyModCtrl uint8 = 1 << iota
	clientHotkeyModAlt
	clientHotkeyModShift
	clientHotkeyModMeta
)

type HotkeyCommand struct {
	Command string `json:"command,omitempty"`
}

type Hotkey struct {
	Name         string          `json:"name,omitempty"`
	Combo        string          `json:"combo"`
	Commands     []HotkeyCommand `json:"commands"`
	Script       string          `json:"script,omitempty"`
	Disabled     bool            `json:"disabled,omitempty"`
	BuiltIn      bool            `json:"built_in,omitempty"`
	defaultCombo string
	registration scriptRegistrationHandle
}

var (
	hotkeys          []Hotkey
	hotkeysMu        sync.RWMutex
	hotkeysWin       *eui.WindowData
	hotkeysList      *eui.ItemData
	hotkeyEditWin    *eui.WindowData
	hotkeyComboText  *eui.ItemData
	hotkeyNameInput  *eui.ItemData
	hotkeyEnabledCB  *eui.ItemData
	hotkeyCmdSection *eui.ItemData
	hotkeyCmdInputs  []*eui.ItemData
	editingHotkey    int = -1

	recording     bool
	recordStart   time.Time
	recordTarget  *eui.ItemData
	recordedCombo string
	recordedMods  string

	scriptHotkeyMu sync.RWMutex

	// scriptHotkeyEnabled holds the persisted enabled state for script
	// hotkeys. The map is keyed first by script name and then by combo.
	scriptHotkeyEnabled  = map[string]map[string]bool{}
	clientHotkeyBindings atomic.Pointer[compiledClientHotkeys]
)

func loadHotkeys() {
	path := filepath.Join(dataDirPath, hotkeysFile)
	scriptHotkeyMu.Lock()
	scriptHotkeyEnabled = map[string]map[string]bool{}
	data, err := os.ReadFile(path)

	var newList []Hotkey
	if err == nil {
		type hotkeyJSON struct {
			Combo    string          `json:"combo"`
			Name     string          `json:"name,omitempty"`
			Commands []HotkeyCommand `json:"commands"`
			Command  string          `json:"command"`
			Text     string          `json:"text,omitempty"`
			Script   string          `json:"script,omitempty"`
			Disabled *bool           `json:"disabled,omitempty"`
			Enabled  *bool           `json:"enabled,omitempty"`
			BuiltIn  bool            `json:"built_in,omitempty"`
		}
		var raw []hotkeyJSON
		if err := json.Unmarshal(data, &raw); err != nil {
			scriptHotkeyMu.Unlock()
			return
		}
		for _, r := range raw {
			if r.Script != "" {
				m := scriptHotkeyEnabled[r.Script]
				if m == nil {
					m = map[string]bool{}
					scriptHotkeyEnabled[r.Script] = m
				}
				enabled := true
				if r.Enabled != nil {
					enabled = *r.Enabled
				} else if r.Disabled != nil {
					enabled = !*r.Disabled
				}
				m[r.Combo] = enabled
				continue
			}
			disabled := false
			if r.Disabled != nil {
				disabled = *r.Disabled
			}
			hk := Hotkey{Combo: r.Combo, Name: r.Name, Disabled: disabled, BuiltIn: r.BuiltIn}
			if len(r.Commands) > 0 {
				for _, c := range r.Commands {
					cmd := strings.TrimSpace(c.Command)
					if cmd != "" {
						hk.Commands = append(hk.Commands, HotkeyCommand{Command: cmd})
					}
				}
			} else if r.Command != "" {
				cmd := strings.TrimSpace(r.Command + " " + r.Text)
				if cmd != "" {
					hk.Commands = []HotkeyCommand{{Command: cmd}}
				}
			}
			newList = append(newList, hk)
		}
	} else if !os.IsNotExist(err) {
		scriptHotkeyMu.Unlock()
		return
	}
	scriptHotkeyMu.Unlock()
	var removedRetiredDefaults bool
	newList, removedRetiredDefaults = removeRetiredSessionTabHotkeys(newList)
	addedSessionDefaults := removedRetiredDefaults
	if !multiSessionWorkspace.TabHotkeysInitialized {
		newList, addedSessionDefaults = addDefaultSessionTabHotkeys(newList)
		multiSessionWorkspace.TabHotkeysInitialized = true
		markMultiSessionWorkspaceUsed()
		multiSessionWorkspaceDirty = true
	}
	if !multiSessionWorkspace.TabCycleInitialized {
		var added bool
		newList, added = addDefaultSessionTabCycleHotkeys(newList)
		addedSessionDefaults = addedSessionDefaults || added
		multiSessionWorkspace.TabCycleInitialized = true
		markMultiSessionWorkspaceUsed()
		multiSessionWorkspaceDirty = true
	}
	if multiSessionWorkspace.ClientHotkeysVersion < clientHotkeysVersion {
		var added bool
		newList, added = addDefaultClientHotkeys(newList)
		addedSessionDefaults = addedSessionDefaults || added
		multiSessionWorkspace.ClientHotkeysVersion = clientHotkeysVersion
		markMultiSessionWorkspaceUsed()
		multiSessionWorkspaceDirty = true
	}
	if markKnownBuiltInHotkeys(newList) {
		addedSessionDefaults = true
	}

	hotkeysMu.Lock()
	hotkeys = newList
	hotkeysMu.Unlock()
	rebuildClientHotkeyBindings(newList)
	refreshHotkeysList()
	if addedSessionDefaults {
		saveHotkeys()
	}
}

func addDefaultSessionTabHotkeys(list []Hotkey) ([]Hotkey, bool) {
	added := false
	for position := 1; position <= maxSessions; position++ {
		command := fmt.Sprintf("/tab %d", position)
		found := false
		for _, hotkey := range list {
			for _, entry := range hotkey.Commands {
				if strings.EqualFold(strings.TrimSpace(entry.Command), command) {
					found = true
					break
				}
			}
			if found {
				break
			}
		}
		if found {
			continue
		}
		key := position
		if position == 10 {
			key = 0
		}
		list = append(list, Hotkey{
			Name:     fmt.Sprintf("Select Session Tab %d", position),
			Combo:    fmt.Sprintf("Ctrl-%d", key),
			Commands: []HotkeyCommand{{Command: command}},
			BuiltIn:  true,
		})
		added = true
	}
	return list, added
}

func addDefaultSessionTabCycleHotkeys(list []Hotkey) ([]Hotkey, bool) {
	defaults := []Hotkey{
		{Name: "Next Session Tab", Combo: "Ctrl-Tab", Commands: []HotkeyCommand{{Command: "/tab next"}}, BuiltIn: true},
		{Name: "Previous Session Tab", Combo: "Ctrl-Shift-Tab", Commands: []HotkeyCommand{{Command: "/tab previous"}}, BuiltIn: true},
	}
	added := false
	for _, candidate := range defaults {
		found := false
		for _, existing := range list {
			if sameCombo(existing.Combo, candidate.Combo) {
				found = true
				break
			}
		}
		if found {
			continue
		}
		list = append(list, candidate)
		added = true
	}
	return list, added
}

func removeRetiredSessionTabHotkeys(list []Hotkey) ([]Hotkey, bool) {
	filtered := make([]Hotkey, 0, len(list))
	removed := false
	for _, hotkey := range list {
		command := ""
		if hotkey.Script == "" && len(hotkey.Commands) == 1 {
			command = strings.ToLower(strings.TrimSpace(hotkey.Commands[0].Command))
		}
		name := strings.ToLower(strings.TrimSpace(hotkey.Name))
		retiredName := (name == "previous session tab (alternate)" && command == "/tab previous") ||
			(name == "next session tab (alternate)" && command == "/tab next")
		retiredMarkedDefault := hotkey.BuiltIn && ((sameCombo(hotkey.Combo, "Ctrl-Comma") && command == "/tab previous") ||
			(sameCombo(hotkey.Combo, "Ctrl-Period") && command == "/tab next"))
		if retiredName || retiredMarkedDefault {
			removed = true
			continue
		}
		filtered = append(filtered, hotkey)
	}
	return filtered, removed
}

func addDefaultClientHotkeys(list []Hotkey) ([]Hotkey, bool) {
	paletteCombo := "Ctrl-Shift-P"
	if runtime.GOOS == "darwin" {
		paletteCombo = "Meta-Shift-P"
	}
	defaults := []Hotkey{
		{Name: "Toggle Fullscreen", Combo: "F12", Commands: []HotkeyCommand{{Command: "/fullscreen"}}, BuiltIn: true},
		{Name: "Open Command Palette", Combo: paletteCombo, Commands: []HotkeyCommand{{Command: "/palette"}}, BuiltIn: true},
		{Name: "Walk Left", Combo: "A", Commands: []HotkeyCommand{{Command: "/move left"}}, BuiltIn: true},
		{Name: "Walk Left (alternate)", Combo: "ArrowLeft", Commands: []HotkeyCommand{{Command: "/move left"}}, BuiltIn: true},
		{Name: "Walk Right", Combo: "D", Commands: []HotkeyCommand{{Command: "/move right"}}, BuiltIn: true},
		{Name: "Walk Right (alternate)", Combo: "ArrowRight", Commands: []HotkeyCommand{{Command: "/move right"}}, BuiltIn: true},
		{Name: "Walk Up", Combo: "W", Commands: []HotkeyCommand{{Command: "/move up"}}, BuiltIn: true},
		{Name: "Walk Up (alternate)", Combo: "ArrowUp", Commands: []HotkeyCommand{{Command: "/move up"}}, BuiltIn: true},
		{Name: "Walk Down", Combo: "S", Commands: []HotkeyCommand{{Command: "/move down"}}, BuiltIn: true},
		{Name: "Walk Down (alternate)", Combo: "ArrowDown", Commands: []HotkeyCommand{{Command: "/move down"}}, BuiltIn: true},
		{Name: "Run While Walking", Combo: "Shift", Commands: []HotkeyCommand{{Command: "/move run"}}, BuiltIn: true},
	}
	added := false
	for _, candidate := range defaults {
		command := candidate.Commands[0].Command
		if (command == "/fullscreen" || command == "/palette") && hotkeyListHasCommand(list, command) {
			continue
		}
		occupied := false
		for _, existing := range list {
			if sameCombo(existing.Combo, candidate.Combo) {
				occupied = true
				break
			}
		}
		if occupied {
			continue
		}
		list = append(list, candidate)
		added = true
	}
	return list, added
}

func markKnownBuiltInHotkeys(list []Hotkey) bool {
	definitions := map[string]string{
		"Next Session Tab":       "/tab next",
		"Previous Session Tab":   "/tab previous",
		"Toggle Fullscreen":      "/fullscreen",
		"Open Command Palette":   "/palette",
		"Walk Left":              "/move left",
		"Walk Left (alternate)":  "/move left",
		"Walk Right":             "/move right",
		"Walk Right (alternate)": "/move right",
		"Walk Up":                "/move up",
		"Walk Up (alternate)":    "/move up",
		"Walk Down":              "/move down",
		"Walk Down (alternate)":  "/move down",
		"Run While Walking":      "/move run",
	}
	for position := 1; position <= maxSessions; position++ {
		definitions[fmt.Sprintf("Select Session Tab %d", position)] = fmt.Sprintf("/tab %d", position)
	}
	changed := false
	for i := range list {
		hotkey := &list[i]
		if hotkey.BuiltIn || hotkey.Script != "" || len(hotkey.Commands) != 1 {
			continue
		}
		command, ok := definitions[hotkey.Name]
		if ok && strings.EqualFold(strings.TrimSpace(hotkey.Commands[0].Command), command) {
			hotkey.BuiltIn = true
			changed = true
		}
	}
	return changed
}

func hotkeyListHasCommand(list []Hotkey, command string) bool {
	for _, hotkey := range list {
		if hotkey.Script != "" {
			continue
		}
		for _, entry := range hotkey.Commands {
			if strings.EqualFold(strings.TrimSpace(entry.Command), command) {
				return true
			}
		}
	}
	return false
}

func hotkeyComboForCommand(command string) string {
	command = strings.TrimSpace(command)
	hotkeysMu.RLock()
	defer hotkeysMu.RUnlock()
	for _, hotkey := range hotkeys {
		if hotkey.Disabled || hotkey.Script != "" {
			continue
		}
		for _, entry := range hotkey.Commands {
			if strings.EqualFold(strings.TrimSpace(entry.Command), command) {
				return hotkey.Combo
			}
		}
	}
	return ""
}

func clientHotkeyActionForCommand(command string) (clientHotkeyAction, bool) {
	switch strings.ToLower(strings.TrimSpace(command)) {
	case "/palette":
		return clientHotkeyCommandPalette, true
	case "/move left":
		return clientHotkeyMoveLeft, true
	case "/move right":
		return clientHotkeyMoveRight, true
	case "/move up":
		return clientHotkeyMoveUp, true
	case "/move down":
		return clientHotkeyMoveDown, true
	case "/move run":
		return clientHotkeyRun, true
	default:
		return 0, false
	}
}

func compileClientHotkey(combo string) (compiledClientHotkey, bool) {
	parts := strings.Split(strings.TrimSpace(combo), "-")
	if len(parts) == 0 || strings.TrimSpace(parts[len(parts)-1]) == "" {
		return compiledClientHotkey{}, false
	}
	var modifiers uint8
	for _, part := range parts[:len(parts)-1] {
		switch strings.ToLower(strings.TrimSpace(part)) {
		case "ctrl", "control", "controlleft", "controlright":
			modifiers |= clientHotkeyModCtrl
		case "alt", "option", "altleft", "altright":
			modifiers |= clientHotkeyModAlt
		case "shift", "shiftleft", "shiftright":
			modifiers |= clientHotkeyModShift
		case "meta", "command", "cmd", "metaleft", "metaright":
			modifiers |= clientHotkeyModMeta
		default:
			return compiledClientHotkey{}, false
		}
	}
	trigger := strings.TrimSpace(parts[len(parts)-1])
	for key := ebiten.Key(0); key <= ebiten.KeyMax; key++ {
		if strings.EqualFold(key.String(), trigger) || sameCombo(scriptKeyName(key, nil, nil, false, false), trigger) {
			return compiledClientHotkey{key: key, modifiers: modifiers}, true
		}
	}
	return compiledClientHotkey{}, false
}

func rebuildClientHotkeyBindings(list []Hotkey) {
	compiled := &compiledClientHotkeys{}
	for _, hotkey := range list {
		if hotkey.Disabled || hotkey.Script != "" {
			continue
		}
		binding, ok := compileClientHotkey(hotkey.Combo)
		if !ok {
			continue
		}
		for _, entry := range hotkey.Commands {
			action, ok := clientHotkeyActionForCommand(entry.Command)
			if !ok {
				continue
			}
			compiled[action] = append(compiled[action], binding)
		}
	}
	clientHotkeyBindings.Store(compiled)
}

func currentClientHotkeyModifiers() uint8 {
	var modifiers uint8
	if ebiten.IsKeyPressed(ebiten.KeyControl) || ebiten.IsKeyPressed(ebiten.KeyControlLeft) || ebiten.IsKeyPressed(ebiten.KeyControlRight) {
		modifiers |= clientHotkeyModCtrl
	}
	if ebiten.IsKeyPressed(ebiten.KeyAlt) || ebiten.IsKeyPressed(ebiten.KeyAltLeft) || ebiten.IsKeyPressed(ebiten.KeyAltRight) {
		modifiers |= clientHotkeyModAlt
	}
	if ebiten.IsKeyPressed(ebiten.KeyShift) || ebiten.IsKeyPressed(ebiten.KeyShiftLeft) || ebiten.IsKeyPressed(ebiten.KeyShiftRight) {
		modifiers |= clientHotkeyModShift
	}
	if ebiten.IsKeyPressed(ebiten.KeyMeta) || ebiten.IsKeyPressed(ebiten.KeyMetaLeft) || ebiten.IsKeyPressed(ebiten.KeyMetaRight) {
		modifiers |= clientHotkeyModMeta
	}
	return modifiers
}

func clientHotkeyActionActive(action clientHotkeyAction, justPressed bool, accept func(ebiten.Key) bool) bool {
	bindings := clientHotkeyBindings.Load()
	if bindings == nil || action >= clientHotkeyActionCount {
		return false
	}
	modifiers := currentClientHotkeyModifiers()
	for _, binding := range bindings[action] {
		if clientHotkeyBindingActive(binding, modifiers, justPressed) && (accept == nil || accept(binding.key)) {
			return true
		}
	}
	return false
}

func clientHotkeyBindingActive(binding compiledClientHotkey, modifiers uint8, justPressed bool) bool {
	if modifiers&binding.modifiers != binding.modifiers {
		return false
	}
	if justPressed {
		return inpututil.IsKeyJustPressed(binding.key)
	}
	return ebiten.IsKeyPressed(binding.key)
}

func clientMovementHotkeyActive(bindings *compiledClientHotkeys, modifiers uint8, action clientHotkeyAction, consumed InputEvent) bool {
	for _, binding := range bindings[action] {
		if clientHotkeyBindingActive(binding, modifiers, false) &&
			!legacyMacroKeyConsumed(binding.key) && !scriptInputConsumesKey(consumed, binding.key) {
			return true
		}
	}
	return false
}

func clientMovementHotkeys(consumed InputEvent) (left, right, up, down, run bool) {
	bindings := clientHotkeyBindings.Load()
	if bindings == nil {
		return false, false, false, false, false
	}
	modifiers := currentClientHotkeyModifiers()
	return clientMovementHotkeyActive(bindings, modifiers, clientHotkeyMoveLeft, consumed),
		clientMovementHotkeyActive(bindings, modifiers, clientHotkeyMoveRight, consumed),
		clientMovementHotkeyActive(bindings, modifiers, clientHotkeyMoveUp, consumed),
		clientMovementHotkeyActive(bindings, modifiers, clientHotkeyMoveDown, consumed),
		clientMovementHotkeyActive(bindings, modifiers, clientHotkeyRun, consumed)
}

func saveHotkeys() {
	if isWASM {
		return
	}
	path := filepath.Join(dataDirPath, hotkeysFile)
	_ = os.MkdirAll(dataDirPath, 0o755)
	// snapshot under read lock
	hotkeysMu.RLock()
	snap := append([]Hotkey(nil), hotkeys...)
	hotkeysMu.RUnlock()
	rebuildClientHotkeyBindings(snap)
	type scriptState struct {
		Script  string `json:"script"`
		Combo   string `json:"combo"`
		Enabled bool   `json:"enabled"`
	}

	var out []any
	scriptHotkeyMu.Lock()
	for _, hk := range snap {
		if hk.Script != "" {
			m := scriptHotkeyEnabled[hk.Script]
			if m == nil {
				m = map[string]bool{}
				scriptHotkeyEnabled[hk.Script] = m
			}
			m[scriptHotkeyDefault(hk)] = !hk.Disabled
			continue
		}
		out = append(out, hk)
	}
	for plug, m := range scriptHotkeyEnabled {
		for combo, enabled := range m {
			out = append(out, scriptState{Script: plug, Combo: combo, Enabled: enabled})
		}
	}
	scriptHotkeyMu.Unlock()

	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(path, data, 0o644)
}

func scriptHotkeys(owner string) []Hotkey {
	hotkeysMu.RLock()
	defer hotkeysMu.RUnlock()
	var list []Hotkey
	for _, hk := range hotkeys {
		if hk.Script == owner {
			list = append(list, hk)
		}
	}
	return list
}

func removeScriptHotkeyRegistration(owner, combo string, preserveState bool) {
	hotkeysMu.RLock()
	var registrations []scriptRegistrationHandle
	for _, hk := range hotkeys {
		if hk.Script == owner && hk.Combo == combo && hk.registration.valid() {
			registrations = append(registrations, hk.registration)
		}
	}
	hotkeysMu.RUnlock()
	for _, registration := range registrations {
		registration.release()
	}
	hotkeysMu.Lock()
	for i := 0; i < len(hotkeys); i++ {
		hk := hotkeys[i]
		if hk.Script == owner && hk.Combo == combo {
			hotkeys = append(hotkeys[:i], hotkeys[i+1:]...)
			i--
		}
	}
	hotkeysMu.Unlock()
	if !preserveState {
		scriptHotkeyMu.Lock()
		if m := scriptHotkeyEnabled[owner]; m != nil {
			delete(m, combo)
			if len(m) == 0 {
				delete(scriptHotkeyEnabled, owner)
			}
		}
		scriptHotkeyMu.Unlock()
	}
	refreshHotkeysList()
	saveHotkeys()
}

func removeScriptHotkeyByHandle(registration scriptRegistrationHandle) {
	var owner, combo string
	hotkeysMu.Lock()
	for i := len(hotkeys) - 1; i >= 0; i-- {
		if hotkeys[i].registration == registration {
			owner = hotkeys[i].Script
			combo = hotkeys[i].Combo
			hotkeys = append(hotkeys[:i], hotkeys[i+1:]...)
		}
	}
	hotkeysMu.Unlock()
	if owner == "" {
		return
	}
	scriptHotkeyFnMu.Lock()
	if m := scriptHotkeyFns[owner]; m != nil {
		delete(m, combo)
		if len(m) == 0 {
			delete(scriptHotkeyFns, owner)
		}
	}
	scriptHotkeyFnMu.Unlock()
	refreshHotkeysList()
	saveHotkeys()
}

func makeHotkeysWindow() {
	if hotkeysWin != nil {
		return
	}
	hotkeysWin = eui.NewWindow()
	hotkeysWin.Title = "Hotkeys"
	hotkeysWin.Closable = true
	hotkeysWin.Movable = true
	hotkeysWin.Resizable = true
	hotkeysWin.AutoSize = true
	hotkeysWin.NoScroll = true
	hotkeysWin.SetZone(eui.HZoneCenter, eui.VZoneMiddleTop)

	root := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_HORIZONTAL, Fixed: true}
	hotkeysWin.AddItem(root)

	flow := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL, Fixed: true}
	flow.Size = eui.Point{X: 520, Y: hotkeysWin.Size.Y}
	root.AddItem(flow)

	btnRow := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_HORIZONTAL, Fixed: true}
	addBtn, addEvents := eui.NewButton()
	setMaterialIconOnly(addBtn, "add", "+")
	addBtn.SetTooltip("Create a new hotkey")
	addBtn.Size = eui.Point{X: 20, Y: 20}
	addBtn.FontSize = 14
	addEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			openHotkeyEditor(-1)
		}
	}
	btnRow.AddItem(addBtn)
	btnRow.Size = eui.Point{X: flow.Size.X, Y: addBtn.Size.Y}
	flow.AddItem(btnRow)

	hotkeysList = &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL, Scrollable: true}
	hotkeysList.Size = eui.Point{X: flow.Size.X, Y: flow.Size.Y - btnRow.Size.Y}
	flow.AddItem(hotkeysList)

	infoFlow := eui.NewColumn()
	infoText := "@right.clicked -> last right-clicked player\n@middle.clicked -> last middle-clicked player\n@<button>.<mod>.clicked -> clicked player with modifier (button: left|middle|right; mod: control|alt|shift)\n@hovered -> currently hovered player\n@selected.player -> selected player\n@selected.item -> selected item\n@equipped.left -> left hand item\n@equipped.belt -> belt item\n@equipped.<slot> -> item in wear slot"
	help := &eui.ItemData{ItemType: eui.ITEM_TEXT, Text: infoText}
	help.Size = eui.Point{X: 256, Y: 256}
	help.FontSize = 10
	infoFlow.AddItem(help)
	root.AddItem(infoFlow)

	hotkeysWin.AddWindow(false)
	refreshHotkeysList()
}

func refreshHotkeysList() {
	if hotkeysList == nil {
		return
	}
	hotkeysList.Contents = hotkeysList.Contents[:0]
	// snapshot to avoid concurrent mutation during UI build
	hotkeysMu.RLock()
	list := append([]Hotkey(nil), hotkeys...)
	hotkeysMu.RUnlock()

	// global hotkeys
	for i, hk := range list {
		if hk.Script != "" {
			continue
		}
		idx := i
		row := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_HORIZONTAL, Fixed: true}
		row.Size = eui.Point{X: 480, Y: 20}
		btn, events := eui.NewButton()
		btnText := hk.Combo
		if hk.Name != "" {
			btnText = hk.Name + " : " + hk.Combo
		}
		if len(hk.Commands) > 0 {
			text := hk.Commands[0].Command
			if len(hk.Commands) > 1 {
				text += " ..."
			}
			btnText += " -> " + text
		}
		if hk.Disabled {
			btnText = "Disabled — " + btnText
		}
		btn.Text = btnText
		btn.Size = eui.Point{X: 480, Y: 20}
		btn.FontSize = 10
		events.Handle = func(ev eui.UIEvent) {
			if ev.Type == eui.EventClick {
				openHotkeyEditor(idx)
			}
		}
		row.AddItem(btn)
		if !hk.BuiltIn {
			btn.Size.X = 460
			delBtn, delEvents := eui.NewButton()
			setMaterialIconOnly(delBtn, "delete", "x")
			delBtn.SetTooltip("Remove this hotkey")
			delBtn.Size = eui.Point{X: 20, Y: 20}
			delBtn.FontSize = 10
			delEvents.Handle = func(ev eui.UIEvent) {
				if ev.Type == eui.EventClick {
					confirmRemoveHotkey(idx)
				}
			}
			row.AddItem(delBtn)
		}
		hotkeysList.AddItem(row)
	}

	// script hotkeys header and list
	headerAdded := false
	for i, hk := range list {
		if hk.Script == "" {
			continue
		}
		if !headerAdded {
			label := &eui.ItemData{ItemType: eui.ITEM_TEXT, Text: "script Hotkeys", Fixed: true}
			label.Size = eui.Point{X: 480, Y: 20}
			label.FontSize = 10
			hotkeysList.AddItem(label)
			headerAdded = true
		}
		idx := i
		row := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_HORIZONTAL, Fixed: true}
		row.Size = eui.Point{X: 480, Y: 20}
		cb, cbEvents := eui.NewCheckbox()
		cb.Checked = !hk.Disabled
		cbEvents.Handle = func(ev eui.UIEvent) {
			if ev.Type == eui.EventClick {
				hotkeysMu.Lock()
				var hk Hotkey
				if idx >= 0 && idx < len(hotkeys) {
					hotkeys[idx].Disabled = !ev.Checked
					hk = hotkeys[idx]
				}
				hotkeysMu.Unlock()
				if hk.Script != "" {
					scriptHotkeyMu.Lock()
					m := scriptHotkeyEnabled[hk.Script]
					if m == nil {
						m = map[string]bool{}
						scriptHotkeyEnabled[hk.Script] = m
					}
					m[scriptHotkeyDefault(hk)] = !hk.Disabled
					scriptHotkeyMu.Unlock()
				}
				saveHotkeys()
			}
		}
		row.AddItem(cb)
		text := hk.Combo
		if hk.Name != "" {
			text = hk.Name + " : " + hk.Combo
		}
		disp := scriptDisplayNames[hk.Script]
		if disp == "" {
			disp = hk.Script
		}
		lbl := &eui.ItemData{ItemType: eui.ITEM_TEXT, Text: disp + " -> " + text, Fixed: true}
		lbl.Size = eui.Point{X: 460, Y: 20}
		lbl.FontSize = 10
		row.AddItem(lbl)
		hotkeysList.AddItem(row)
	}

	hotkeysList.Dirty = true
	if hotkeysWin != nil {
		hotkeysWin.Refresh()
	}
	refreshKeybindingsList()
}

func confirmRemoveHotkey(idx int) {
	hotkeysMu.RLock()
	if idx < 0 || idx >= len(hotkeys) {
		hotkeysMu.RUnlock()
		return
	}
	hk := hotkeys[idx]
	hotkeysMu.RUnlock()
	if hk.BuiltIn {
		return
	}
	eui.ShowPopup(
		"Remove Hotkey",
		fmt.Sprintf("Remove hotkey %s : %s?", hk.Name, hk.Combo),
		[]eui.PopupButton{
			{Text: "Cancel"},
			{Text: "Remove", Color: &eui.ColorDarkRed, HoverColor: &eui.ColorRed, Action: func() {
				hotkeysMu.Lock()
				if idx >= 0 && idx < len(hotkeys) {
					hotkeys = append(hotkeys[:idx], hotkeys[idx+1:]...)
				}
				hotkeysMu.Unlock()
				saveHotkeys()
				queueSessionWorkspaceUIUpdate()
				refreshHotkeysList()
			}},
		},
	)
}

func openHotkeyEditor(idx int) {
	if hotkeyEditWin != nil {
		return
	}
	editingHotkey = idx
	hotkeyEnabledCB = nil
	hotkeysMu.RLock()
	current := Hotkey{}
	hasCurrent := idx >= 0 && idx < len(hotkeys)
	if hasCurrent {
		current = hotkeys[idx]
	}
	hotkeysMu.RUnlock()
	hotkeyEditWin = eui.NewWindow()
	hotkeyEditWin.OnClose = func() {
		hotkeyEditWin = nil
		hotkeyEnabledCB = nil
	}
	hotkeyEditWin.Title = "Hotkey"
	hotkeyEditWin.Size = eui.Point{X: 400, Y: 160}
	hotkeyEditWin.AutoSize = true
	hotkeyEditWin.Closable = true
	hotkeyEditWin.Movable = true
	hotkeyEditWin.Resizable = false
	hotkeyEditWin.NoScroll = true
	hotkeyEditWin.SetZone(eui.HZoneCenter, eui.VZoneMiddleTop)

	flow := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL, Fixed: true}
	hotkeyEditWin.AddItem(flow)

	row := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_HORIZONTAL, Fixed: true}
	label, _ := eui.NewText()
	label.Text = "Keys:"
	label.Size = eui.Point{X: 40, Y: 20}
	label.FontSize = 12
	row.AddItem(label)
	hotkeyComboText, _ = eui.NewText()
	hotkeyComboText.Text = ""
	hotkeyComboText.Size = eui.Point{X: 200, Y: 20}
	hotkeyComboText.FontSize = 12
	row.AddItem(hotkeyComboText)
	hotkeyRecordBtn, recordEvents := eui.NewButton()
	hotkeyRecordBtn.Text = "Record"
	setMaterialButtonIcon(hotkeyRecordBtn, "fiber_manual_record")
	hotkeyRecordBtn.SetTooltip("Capture a key/mouse combo")
	hotkeyRecordBtn.Size = eui.Point{X: 60, Y: 20}
	hotkeyRecordBtn.FontSize = 12
	recordEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			startHotkeyRecording(hotkeyComboText)
		}
	}
	row.AddItem(hotkeyRecordBtn)
	flow.AddItem(row)

	nameRow := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_HORIZONTAL, Fixed: true}
	nameLabel, _ := eui.NewText()
	nameLabel.Text = "Name:"
	nameLabel.Size = eui.Point{X: 40, Y: 20}
	nameLabel.FontSize = 12
	nameRow.AddItem(nameLabel)
	hotkeyNameInput, _ = eui.NewInput()
	hotkeyNameInput.Size = eui.Point{X: hotkeyEditWin.Size.X - 40, Y: 20}
	hotkeyNameInput.FontSize = 12
	nameRow.AddItem(hotkeyNameInput)
	flow.AddItem(nameRow)
	if hasCurrent && current.BuiltIn {
		hotkeyEnabledCB, _ = eui.NewCheckbox()
		hotkeyEnabledCB.Text = "Enabled"
		hotkeyEnabledCB.Checked = !current.Disabled
		hotkeyEnabledCB.Size = eui.Point{X: hotkeyEditWin.Size.X - 40, Y: 20}
		hotkeyEnabledCB.SetTooltip("Use this built-in hotkey")
		flow.AddItem(hotkeyEnabledCB)
	}

	hotkeyCmdSection = &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL, Fixed: true}
	flow.AddItem(hotkeyCmdSection)
	hotkeyCmdInputs = nil

	// Row to add a command input
	addCmdRow, addCmdEvents := eui.NewButton()
	setMaterialIconOnly(addCmdRow, "add", "+")
	addCmdRow.SetTooltip("Add another command line")
	addCmdRow.Size = eui.Point{X: 20, Y: 20}
	addCmdRow.FontSize = 14
	addCmdEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			addHotkeyCommand("")
		}
	}
	flow.AddItem(addCmdRow)

	btnRow := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_HORIZONTAL, Fixed: true}
	okBtn, okEvents := eui.NewButton()
	okBtn.Text = "OK"
	setMaterialButtonIcon(okBtn, "check_circle")
	okBtn.Size = eui.Point{X: 80, Y: 20}
	okBtn.FontSize = 12
	okEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			finishHotkeyEdit(true)
		}
	}
	btnRow.AddItem(okBtn)

	cancelBtn, cancelEvents := eui.NewButton()
	cancelBtn.Text = "Cancel"
	setMaterialButtonIcon(cancelBtn, "close")
	cancelBtn.Size = eui.Point{X: 80, Y: 20}
	cancelBtn.FontSize = 12
	cancelEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			finishHotkeyEdit(false)
		}
	}
	btnRow.AddItem(cancelBtn)

	flow.AddItem(btnRow)

	if hasCurrent {
		hotkeyComboText.Text = current.Combo
		hotkeyNameInput.Text = current.Name
		if len(current.Commands) > 0 {
			for _, c := range current.Commands {
				addHotkeyCommand(c.Command)
			}
		} else {
			addHotkeyCommand("")
		}
	} else {
		addHotkeyCommand("")
	}

	hotkeyEditWin.AddWindow(true)
	hotkeyEditWin.MarkOpen()
	wrapHotkeyInputs()
}

func addHotkeyCommand(cmd string) {
	if hotkeyCmdSection == nil {
		return
	}
	cmdLabel, _ := eui.NewText()
	cmdLabel.Text = "Command:"
	cmdLabel.Size = eui.Point{X: hotkeyEditWin.Size.X - 40, Y: 20}
	cmdLabel.FontSize = 12
	hotkeyCmdSection.AddItem(cmdLabel)

	var cmdEvents *eui.EventHandler
	cmdInput, cmdEvents := eui.NewInput()
	cmdInput.Size = eui.Point{X: hotkeyEditWin.Size.X - 40, Y: hotkeyCommandInputHeight}
	cmdInput.FontSize = 12
	cmdInput.Text = cmd
	cmdEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventInputChanged {
			sanitizeHotkeyCommandInput(ev.Item)
		}
	}
	hotkeyCmdSection.AddItem(cmdInput)
	hotkeyCmdInputs = append(hotkeyCmdInputs, cmdInput)

	wrapHotkeyInputs()
}

func wrapHotkeyInputs() {
	if hotkeyEditWin == nil {
		return
	}
	for _, it := range hotkeyCmdInputs {
		sanitizeHotkeyCommandInput(it)
	}
	hotkeyEditWin.Refresh()
}

func sanitizeHotkeyCommandInput(it *eui.ItemData) {
	if it == nil {
		return
	}
	text := strings.NewReplacer("\r", " ", "\n", " ").Replace(it.Text)
	if text != it.Text {
		it.Text = text
	}
	if it.TextPtr != nil {
		*it.TextPtr = it.Text
	}
	if it.CursorPos > len([]rune(it.Text)) {
		it.CursorPos = len([]rune(it.Text))
	}
	if it.CursorPos < 0 {
		it.CursorPos = 0
	}
	it.Size.Y = hotkeyCommandInputHeight
	it.Dirty = true
}

func finishHotkeyEdit(save bool) {
	if save {
		combo := strings.ReplaceAll(hotkeyComboText.Text, "\n", " ")
		name := strings.ReplaceAll(hotkeyNameInput.Text, "\n", " ")
		cmds := []HotkeyCommand{}
		for _, in := range hotkeyCmdInputs {
			cmd := strings.ReplaceAll(in.Text, "\n", " ")
			if cmd != "" {
				cmds = append(cmds, HotkeyCommand{Command: cmd})
			}
		}
		if combo != "" {
			builtIn := false
			disabled := false
			hotkeysMu.RLock()
			if editingHotkey >= 0 && editingHotkey < len(hotkeys) {
				builtIn = hotkeys[editingHotkey].BuiltIn
				disabled = hotkeys[editingHotkey].Disabled
			}
			hotkeysMu.RUnlock()
			if builtIn && hotkeyEnabledCB != nil {
				disabled = !hotkeyEnabledCB.Checked
			}
			hotkeysMu.RLock()
			for i, hk := range hotkeys {
				if i == editingHotkey {
					continue
				}
				if !disabled && !hk.Disabled && strings.EqualFold(hk.Combo, combo) {
					hotkeysMu.RUnlock()
					name := hk.Name
					if name == "" {
						name = hk.Script
					}
					if name == "" {
						name = "another hotkey"
					}
					eui.ShowPopup("Error", fmt.Sprintf("%s already bound to %s", combo, name), []eui.PopupButton{{Text: "OK"}})
					return
				}
			}
			hotkeysMu.RUnlock()

			hk := Hotkey{Name: name, Combo: combo, Commands: cmds, Disabled: disabled, BuiltIn: builtIn}
			hotkeysMu.Lock()
			if editingHotkey >= 0 && editingHotkey < len(hotkeys) {
				hotkeys[editingHotkey] = hk
				hotkeysMu.Unlock()
				saveHotkeys()
				queueSessionWorkspaceUIUpdate()
				refreshHotkeysList()
			} else {
				hotkeys = append(hotkeys, hk)
				hotkeysMu.Unlock()
				saveHotkeys()
				queueSessionWorkspaceUIUpdate()
				refreshHotkeysList()
			}
		}
	}
	if hotkeyEditWin != nil {
		hotkeyEditWin.Close()
		hotkeyEditWin = nil
	}
}

func startHotkeyRecording(target *eui.ItemData) {
	recording = true
	recordStart = time.Now()
	recordTarget = target
	recordedCombo = ""
	recordedMods = ""
	if recordTarget != nil {
		recordTarget.Text = "Recording..."
		recordTarget.Dirty = true
		if hotkeyEditWin != nil {
			hotkeyEditWin.Refresh()
		}
	}
}

func finishRecording() {
	recording = false
	if recordTarget != nil {
		if recordedCombo == "" {
			recordTarget.Text = ""
		} else {
			recordTarget.Text = recordedCombo
		}
		recordTarget.Dirty = true
		if hotkeyEditWin != nil {
			hotkeyEditWin.Refresh()
		}
	}
}

func detectCombo() string {
	for button := ebiten.MouseButton(0); button <= ebiten.MouseButtonMax; button++ {
		if inpututil.IsMouseButtonJustPressed(button) {
			return comboFromMouseWithKey(button)
		}
	}
	wx, wy := ebiten.Wheel()
	if wy > 0 {
		return comboFromWheel("WheelUp")
	}
	if wy < 0 {
		return comboFromWheel("WheelDown")
	}
	if wx > 0 {
		return comboFromWheel("WheelRight")
	}
	if wx < 0 {
		return comboFromWheel("WheelLeft")
	}
	for _, k := range inpututil.AppendJustPressedKeys(nil) {
		if isModifier(k) {
			continue
		}
		return comboFromKey(k)
	}
	return ""
}

func comboFromKey(k ebiten.Key) string {
	mods := currentMods()
	shifted := ebiten.IsKeyPressed(ebiten.KeyShift) || ebiten.IsKeyPressed(ebiten.KeyShiftLeft) || ebiten.IsKeyPressed(ebiten.KeyShiftRight)
	alternate := ebiten.IsKeyPressed(ebiten.KeyAlt) || ebiten.IsKeyPressed(ebiten.KeyAltLeft) || ebiten.IsKeyPressed(ebiten.KeyAltRight)
	name := scriptKeyName(k, inpututil.AppendJustPressedKeys(nil), ebiten.AppendInputChars(nil),
		shifted, alternate)
	mods = append(mods, name)
	return strings.Join(mods, "-")
}

func scriptKeyName(key ebiten.Key, pressed []ebiten.Key, typed []rune, shifted, alternate bool) string {
	if (shifted || alternate) && len(pressed) == 1 && len(typed) == 1 && unicode.IsPrint(typed[0]) && !unicode.IsSpace(typed[0]) {
		return string(typed[0])
	}
	if shifted {
		if key >= ebiten.KeyDigit0 && key <= ebiten.KeyDigit9 {
			return string(")!@#$%^&*("[key-ebiten.KeyDigit0])
		}
		shiftedNames := map[ebiten.Key]string{
			ebiten.KeyMinus:         "_",
			ebiten.KeyEqual:         "+",
			ebiten.KeyBracketLeft:   "{",
			ebiten.KeyBracketRight:  "}",
			ebiten.KeyBackslash:     "|",
			ebiten.KeyIntlBackslash: "|",
			ebiten.KeySemicolon:     ":",
			ebiten.KeyQuote:         "\"",
			ebiten.KeyBackquote:     "~",
			ebiten.KeyComma:         "<",
			ebiten.KeyPeriod:        ">",
			ebiten.KeySlash:         "?",
		}
		if name := shiftedNames[key]; name != "" {
			return name
		}
	}
	if key >= ebiten.KeyDigit0 && key <= ebiten.KeyDigit9 {
		return string(rune('0') + rune(key-ebiten.KeyDigit0))
	}
	return key.String()
}

func comboFromWheel(dir string) string {
	mods := currentMods()
	mods = append(mods, dir)
	return strings.Join(mods, "-")
}

func comboFromMouseWithKey(b ebiten.MouseButton) string {
	mods := currentMods()
	keys := inpututil.AppendPressedKeys(nil)
	keyPart := ""
	for _, k := range keys {
		if isModifier(k) {
			continue
		}
		keyPart = k.String()
		break
	}
	if keyPart != "" {
		mods = append(mods, keyPart)
	}
	name := mouseButtonName(b)
	mods = append(mods, name)
	return strings.Join(mods, "-")
}

func currentMods() []string {
	mods := []string{}
	if ebiten.IsKeyPressed(ebiten.KeyMeta) || ebiten.IsKeyPressed(ebiten.KeyMetaLeft) || ebiten.IsKeyPressed(ebiten.KeyMetaRight) {
		mods = append(mods, "Meta")
	}
	if ebiten.IsKeyPressed(ebiten.KeyControl) || ebiten.IsKeyPressed(ebiten.KeyControlLeft) || ebiten.IsKeyPressed(ebiten.KeyControlRight) {
		mods = append(mods, "Ctrl")
	}
	if ebiten.IsKeyPressed(ebiten.KeyAlt) || ebiten.IsKeyPressed(ebiten.KeyAltLeft) || ebiten.IsKeyPressed(ebiten.KeyAltRight) {
		mods = append(mods, "Alt")
	}
	if ebiten.IsKeyPressed(ebiten.KeyShift) || ebiten.IsKeyPressed(ebiten.KeyShiftLeft) || ebiten.IsKeyPressed(ebiten.KeyShiftRight) {
		mods = append(mods, "Shift")
	}
	return mods
}

func mouseButtonName(b ebiten.MouseButton) string {
	switch b {
	case ebiten.MouseButtonLeft:
		return "LeftClick"
	case ebiten.MouseButtonRight:
		return "RightClick"
	case ebiten.MouseButtonMiddle:
		return "MiddleClick"
	default:
		return fmt.Sprintf("Mouse%d", int(b)+1)
	}
}

func isModifier(k ebiten.Key) bool {
	switch k {
	case ebiten.KeyShift, ebiten.KeyShiftLeft, ebiten.KeyShiftRight,
		ebiten.KeyControl, ebiten.KeyControlLeft, ebiten.KeyControlRight,
		ebiten.KeyAlt, ebiten.KeyAltLeft, ebiten.KeyAltRight,
		ebiten.KeyMeta, ebiten.KeyMetaLeft, ebiten.KeyMetaRight:
		return true
	}
	return false
}

var hotkeyClickVariableRE = regexp.MustCompile(`@([A-Za-z]+)((?:\.[A-Za-z]+)*)\.clicked`)

func applyHotkeyVars(cmd string) (string, bool) {
	return applyHotkeyVarsForSession(primarySession, cmd)
}

func applyHotkeyVarsForSession(session *Session, cmd string) (string, bool) {
	if session == nil {
		session = primarySession
	}
	// Resolve @hovered first (simple, unchanged)
	needHovered := strings.Contains(cmd, "@hovered")
	if needHovered {
		hoveredName := sessionHoverSnapshot(session).Mobile.Name
		if hoveredName == "" {
			return "", false
		}
		cmd = strings.ReplaceAll(cmd, "@hovered", hoveredName)
	}

	// Handle new click variables via regex replacement.
	re := hotkeyClickVariableRE
	out := re.ReplaceAllStringFunc(cmd, func(segment string) string {
		m := re.FindStringSubmatch(segment)
		if len(m) < 3 {
			return segment
		}
		button := strings.ToLower(m[1])
		modsPart := m[2]
		var info ClickInfo
		ok := false
		switch button {
		case "right", "rightclick":
			info, ok = sessionButtonClickSnapshot(session, ebiten.MouseButtonRight)
		case "middle", "middleclick":
			info, ok = sessionButtonClickSnapshot(session, ebiten.MouseButtonMiddle)
		case "left", "leftclick":
			info, ok = sessionButtonClickSnapshot(session, ebiten.MouseButtonLeft)
		default:
			ok = false
		}
		if !ok || info.Mobile.Name == "" {
			// Force overall failure by returning an impossible token; marker checked below.
			return "@@UNRESOLVED_CLICK@@"
		}
		if modsPart != "" {
			// modsPart is like ".control.shift"; split and verify each
			for _, raw := range strings.Split(strings.TrimPrefix(modsPart, "."), ".") {
				switch strings.ToLower(strings.TrimSpace(raw)) {
				case "control", "ctrl":
					if !info.Ctrl {
						return "@@UNRESOLVED_CLICK@@"
					}
				case "alt":
					if !info.Alt {
						return "@@UNRESOLVED_CLICK@@"
					}
				case "shift":
					if !info.Shift {
						return "@@UNRESOLVED_CLICK@@"
					}
				case "":
					// ignore
				default:
					return "@@UNRESOLVED_CLICK@@"
				}
			}
		}
		return info.Mobile.Name
	})
	if strings.Contains(out, "@@UNRESOLVED_CLICK@@") {
		return "", false
	}
	cmd = out

	return cmd, true
}

func sessionButtonClickSnapshot(session *Session, button ebiten.MouseButton) (ClickInfo, bool) {
	if session == nil {
		return ClickInfo{}, false
	}
	return session.input.buttonClickSnapshot(button)
}

func updateHotkeyRecording() {
	if !recording {
		return
	}
	if c := detectCombo(); c != "" {
		recordedCombo = c
		finishRecording()
		return
	}
	if modifiers := currentMods(); len(modifiers) > 0 {
		candidate := strings.Join(modifiers, "-")
		if recordedMods == "" || strings.Count(candidate, "-") > strings.Count(recordedMods, "-") {
			recordedMods = candidate
		}
	} else if recordedMods != "" {
		recordedCombo = recordedMods
		finishRecording()
		return
	}
	if time.Since(recordStart) > 5*time.Second {
		finishRecording()
	}
}

func hotkeyEquipAlreadyEquipped(cmd string) bool {
	fields := strings.Fields(cmd)
	if len(fields) < 2 {
		return false
	}
	id64, err := strconv.ParseUint(fields[1], 10, 16)
	if err != nil {
		return false
	}
	id := uint16(id64)
	items := getInventory()
	for _, it := range items {
		if it.ID == id && it.Equipped {
			name := it.Name
			if name == "" {
				name = fields[1]
			}
			consoleMessage(name + " already equipped, skipping")
			return true
		}
	}
	return false
}

func checkHotkeys(session *Session) InputEvent {
	if recording || bindingInputCaptured() {
		return InputEvent{}
	}
	typing := typingInUI()
	// Detect any just-pressed combo first.
	if combo := detectCombo(); combo != "" {
		if legacyMacroHotkeySuppressed(combo) {
			return InputEvent{}
		}
		// Bindings work while the game input bar is active. They pass through
		// by default, so ordinary typing is unchanged unless a handler calls
		// Consume. Other UI text fields keep printable keys for themselves.
		if typing {
			parts := strings.Split(combo, "-")
			if shouldBlockPrintableCombo(parts) {
				return InputEvent{}
			}
		}
		if session != nil {
			if hotkey, matched, enabled := session.sessionScriptHotkey(combo); matched {
				if enabled && hotkey.handler != nil {
					scriptLogEvent(hotkey.owner, "Hotkey", combo)
					event := makeScriptInputEventForSession(session, combo)
					if !hotkey.handler(event) {
						return event
					}
				}
				return InputEvent{}
			}
		}
		hotkeysMu.RLock()
		list := append([]Hotkey(nil), hotkeys...)
		hotkeysMu.RUnlock()
		for _, hk := range list {
			if !hk.Disabled && (hk.Combo == combo || strings.EqualFold(hk.Combo, combo) || sameCombo(hk.Combo, combo)) {
				// If this is a script hotkey with a function handler, call it.
				if hk.Script != "" {
					if fn, ok := scriptGetHotkeyFn(hk.Script, hk.Combo); ok && fn != nil {
						scriptLogEvent(hk.Script, "Hotkey", combo)
						ev := makeScriptInputEventForSession(session, combo)
						if !fn(ev) {
							return ev
						}
					}
				}
				for _, c := range hk.Commands {
					cmd := strings.TrimSpace(c.Command)
					lower := strings.ToLower(cmd)
					if lower == "/fullscreen" {
						SettingsLock.Lock()
						gs.Fullscreen = !gs.Fullscreen
						ebiten.SetFullscreen(gs.Fullscreen)
						ebiten.SetWindowFloating(gs.Fullscreen || gs.AlwaysOnTop)
						SettingsLock.Unlock()
						settingsDirty = true
						continue
					}
					// Show hotkey-triggered command as if it were typed
					var ok bool
					cmd, ok = applyHotkeyVarsForSession(session, cmd)
					if !ok {
						return InputEvent{}
					}
					if strings.HasPrefix(strings.ToLower(cmd), "/equip") {
						if hotkeyEquipAlreadyEquipped(cmd) {
							continue
						}
					}
					dispatchSubmittedCommand(session, cmd)
				}
				break
			}
		}
	}
	return InputEvent{}
}

func makeScriptInputEvent(combo string) InputEvent {
	return makeScriptInputEventForSession(primarySession, combo)
}

func makeScriptInputEventForSession(session *Session, combo string) InputEvent {
	parts := strings.Split(combo, "-")
	trigger := ""
	if len(parts) > 0 {
		trigger = parts[len(parts)-1]
	}
	event := InputEvent{
		Chord:    combo,
		decision: &inputEventDecision{continueInput: true},
	}
	for _, part := range parts {
		switch strings.ToLower(part) {
		case "ctrl", "control":
			event.Ctrl = true
			event.Modifiers = append(event.Modifiers, "Ctrl")
		case "alt":
			event.Alt = true
			event.Modifiers = append(event.Modifiers, "Alt")
		case "shift":
			event.Shift = true
			event.Modifiers = append(event.Modifiers, "Shift")
		case "meta", "command", "cmd":
			event.Meta = true
			event.Modifiers = append(event.Modifiers, "Meta")
		}
	}
	lowerTrigger := strings.ToLower(trigger)
	if strings.HasSuffix(lowerTrigger, "click") || strings.HasPrefix(lowerTrigger, "wheel") || strings.HasPrefix(lowerTrigger, "mouse") {
		event.Button = trigger
		for i := len(parts) - 2; i >= 0; i-- {
			part := parts[i]
			if !isInputModifierName(part) {
				event.Key = part
				break
			}
		}
	} else {
		event.Key = trigger
	}
	event.ScreenX, event.ScreenY = eui.PointerPosition()
	scale := worldScale
	if scale <= 0 {
		scale = 1
	}
	event.WorldX = int16(float64(event.ScreenX-worldOriginX)/scale - float64(fieldCenterX))
	event.WorldY = int16(float64(event.ScreenY-worldOriginY)/scale - float64(fieldCenterY))
	info := worldInfoAtSession(session, event.WorldX, event.WorldY)
	event.OnMobile = info.OnMobile
	event.Mobile = info.Mobile
	if info.OnPlayer {
		event.PlayerName = info.Mobile.Name
		event.SimpleName = scriptSimpleName(info.Mobile.Name)
	}
	return event
}

func scriptInputConsumesKey(event InputEvent, key ebiten.Key) bool {
	return !event.Continues() && event.Button == "" && strings.EqualFold(event.Key, key.String())
}

func scriptInputConsumesText(event InputEvent) bool {
	return !event.Continues() && event.Button == "" && event.Key != ""
}

func scriptInputConsumesButton(event InputEvent, button string) bool {
	return button != "" && !event.Continues() && strings.EqualFold(event.Button, button)
}

func scriptWheelButtonName(x, y float64) string {
	switch {
	case y > 0:
		return "WheelUp"
	case y < 0:
		return "WheelDown"
	case x > 0:
		return "WheelRight"
	case x < 0:
		return "WheelLeft"
	default:
		return ""
	}
}

func isInputModifierName(name string) bool {
	switch strings.ToLower(name) {
	case "ctrl", "control", "alt", "shift", "meta", "command", "cmd":
		return true
	default:
		return false
	}
}

func scriptSimpleName(name string) string {
	var simple strings.Builder
	for _, char := range name {
		if unicode.IsLetter(char) || unicode.IsDigit(char) {
			simple.WriteRune(char)
		}
	}
	return simple.String()
}

func shouldBlockPrintableCombo(parts []string) bool {
	if len(parts) == 0 {
		return false
	}
	if comboHasModifier(parts[:len(parts)-1]) {
		return false
	}
	trig := strings.ToLower(parts[len(parts)-1])
	return isTypingTrigger(trig)
}

func comboHasModifier(parts []string) bool {
	for _, part := range parts {
		switch strings.ToLower(part) {
		case "ctrl", "control", "alt", "shift", "meta", "command", "cmd":
			return true
		}
	}
	return false
}

func isTypingTrigger(trig string) bool {
	if trig == "" {
		return false
	}
	if trig == "space" || trig == "enter" {
		return true
	}
	if chars := []rune(trig); len(chars) == 1 && unicode.IsPrint(chars[0]) {
		return true
	}
	if strings.HasPrefix(trig, "digit") {
		suffix := trig[len("digit"):]
		if len(suffix) == 1 {
			b := suffix[0]
			if b >= '0' && b <= '9' {
				return true
			}
		}
	}
	if strings.HasPrefix(trig, "numpad") {
		suffix := trig[len("numpad"):]
		if len(suffix) == 1 {
			b := suffix[0]
			if b >= '0' && b <= '9' {
				return true
			}
		} else if suffix == "enter" || suffix == "space" {
			return true
		}
	}
	return false
}

// sameCombo compares two combo strings case-insensitively while ignoring the
// order and verbosity of modifier keys (Ctrl/Control, Alt, Shift). The final
// trigger token (e.g., "F3", "RightClick", "WheelUp") must match ignoring case.
func sameCombo(a, b string) bool {
	norm := func(s string) (mods map[string]bool, trig string) {
		parts := strings.Split(strings.TrimSpace(s), "-")
		if len(parts) == 0 {
			return map[string]bool{}, ""
		}
		trig = strings.ToLower(parts[len(parts)-1])
		if strings.HasPrefix(trig, "digit") && len(trig) == len("digit0") {
			if digit := trig[len(trig)-1]; digit >= '0' && digit <= '9' {
				trig = string(digit)
			}
		}
		// Be forgiving about spaces in manually entered mouse names while the
		// recorder and documentation always produce the compact spelling.
		if strings.HasPrefix(trig, "mouse") {
			if n, err := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(trig, "mouse"))); err == nil {
				trig = fmt.Sprintf("mouse%d", n)
			}
		}
		mods = map[string]bool{}
		for _, p := range parts[:len(parts)-1] {
			switch strings.ToLower(strings.TrimSpace(p)) {
			case "ctrl", "control", "controlleft", "controlright":
				mods["ctrl"] = true
			case "alt", "altleft", "altright":
				mods["alt"] = true
			case "shift", "shiftleft", "shiftright":
				mods["shift"] = true
			case "meta", "metaleft", "metaright", "command", "cmd":
				mods["meta"] = true
			default:
				// Treat any unknown modifier token as-is to be strict
				if p != "" {
					mods[strings.ToLower(p)] = true
				}
			}
		}
		return mods, trig
	}
	am, at := norm(a)
	bm, bt := norm(b)
	if at != bt {
		return false
	}
	if len(am) != len(bm) {
		return false
	}
	for k := range am {
		if !bm[k] {
			return false
		}
	}
	return true
}
