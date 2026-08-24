package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"gothoom/eui"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

const hotkeysFile = "global-hotkeys.json"
const hotkeyCommandInputHeight float32 = 20

type HotkeyCommand struct {
	Command string `json:"command,omitempty"`
}

type Hotkey struct {
	Name         string          `json:"name,omitempty"`
	Combo        string          `json:"combo"`
	Commands     []HotkeyCommand `json:"commands"`
	Script       string          `json:"script,omitempty"`
	Disabled     bool            `json:"disabled,omitempty"`
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
	hotkeyCmdSection *eui.ItemData
	hotkeyCmdInputs  []*eui.ItemData
	editingHotkey    int = -1

	recording     bool
	recordStart   time.Time
	recordTarget  *eui.ItemData
	recordedCombo string

	scriptHotkeyMu sync.RWMutex

	// scriptHotkeyEnabled holds the persisted enabled state for script
	// hotkeys. The map is keyed first by script name and then by combo.
	scriptHotkeyEnabled = map[string]map[string]bool{}
)

func loadHotkeys() {
	path := filepath.Join(dataDirPath, hotkeysFile)
	scriptHotkeyMu.Lock()
	scriptHotkeyEnabled = map[string]map[string]bool{}
	data, err := os.ReadFile(path)

	var newList []Hotkey
	noFile := false
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
			hk := Hotkey{Combo: r.Combo, Name: r.Name, Disabled: disabled}
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
	} else if os.IsNotExist(err) {
		noFile = true
	} else {
		scriptHotkeyMu.Unlock()
		return
	}
	scriptHotkeyMu.Unlock()

	// Add default hotkeys only on first run (no existing config file).
	if noFile {
		fs := Hotkey{Name: "Toggle Fullscreen", Combo: "F12", Commands: []HotkeyCommand{{Command: "/fullscreen"}}}
		exists := false
		for _, hk := range newList {
			if hk.Combo == fs.Combo && hk.Script == "" {
				exists = true
				break
			}
		}
		if !exists {
			newList = append(newList, fs)
		}
	}

	hotkeysMu.Lock()
	hotkeys = newList
	hotkeysMu.Unlock()
	refreshHotkeysList()
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
			m[hk.Combo] = !hk.Disabled
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
	addBtn.Text = "+"
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

	infoFlow := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL}
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
		btn.Text = btnText
		btn.Size = eui.Point{X: 460, Y: 20}
		btn.FontSize = 10
		events.Handle = func(ev eui.UIEvent) {
			if ev.Type == eui.EventClick {
				openHotkeyEditor(idx)
			}
		}
		row.AddItem(btn)
		delBtn, delEvents := eui.NewButton()
		delBtn.Text = "x"
		delBtn.SetTooltip("Remove this hotkey")
		delBtn.Size = eui.Point{X: 20, Y: 20}
		delBtn.FontSize = 10
		delEvents.Handle = func(ev eui.UIEvent) {
			if ev.Type == eui.EventClick {
				confirmRemoveHotkey(idx)
			}
		}
		row.AddItem(delBtn)
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
					m[hk.Combo] = !hk.Disabled
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
}

func confirmRemoveHotkey(idx int) {
	hotkeysMu.RLock()
	if idx < 0 || idx >= len(hotkeys) {
		hotkeysMu.RUnlock()
		return
	}
	hk := hotkeys[idx]
	hotkeysMu.RUnlock()
	showPopup(
		"Remove Hotkey",
		fmt.Sprintf("Remove hotkey %s : %s?", hk.Name, hk.Combo),
		[]popupButton{
			{Text: "Cancel"},
			{Text: "Remove", Color: &eui.ColorDarkRed, HoverColor: &eui.ColorRed, Action: func() {
				hotkeysMu.Lock()
				if idx >= 0 && idx < len(hotkeys) {
					hotkeys = append(hotkeys[:idx], hotkeys[idx+1:]...)
				}
				hotkeysMu.Unlock()
				saveHotkeys()
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
	hotkeyEditWin = eui.NewWindow()
	hotkeyEditWin.OnClose = func() { hotkeyEditWin = nil }
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

	hotkeyCmdSection = &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL, Fixed: true}
	flow.AddItem(hotkeyCmdSection)
	hotkeyCmdInputs = nil

	// Row to add a command input
	addCmdRow, addCmdEvents := eui.NewButton()
	addCmdRow.Text = "+"
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
	cancelBtn.Size = eui.Point{X: 80, Y: 20}
	cancelBtn.FontSize = 12
	cancelEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			finishHotkeyEdit(false)
		}
	}
	btnRow.AddItem(cancelBtn)

	flow.AddItem(btnRow)

	hotkeysMu.RLock()
	curLen := len(hotkeys)
	if idx >= 0 && idx < curLen {
		hk := hotkeys[idx]
		hotkeysMu.RUnlock()
		hotkeyComboText.Text = hk.Combo
		hotkeyNameInput.Text = hk.Name
		if len(hk.Commands) > 0 {
			for _, c := range hk.Commands {
				addHotkeyCommand(c.Command)
			}
		} else {
			addHotkeyCommand("")
		}
	} else {
		hotkeysMu.RUnlock()
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
			hotkeysMu.RLock()
			for i, hk := range hotkeys {
				if i == editingHotkey {
					continue
				}
				if strings.EqualFold(hk.Combo, combo) {
					hotkeysMu.RUnlock()
					name := hk.Name
					if name == "" {
						name = hk.Script
					}
					if name == "" {
						name = "another hotkey"
					}
					showPopup("Error", fmt.Sprintf("%s already bound to %s", combo, name), []popupButton{{Text: "OK"}})
					return
				}
			}
			hotkeysMu.RUnlock()

			hk := Hotkey{Name: name, Combo: combo, Commands: cmds}
			hotkeysMu.Lock()
			if editingHotkey >= 0 && editingHotkey < len(hotkeys) {
				hotkeys[editingHotkey] = hk
				hotkeysMu.Unlock()
				saveHotkeys()
				refreshHotkeysList()
			} else {
				hotkeys = append(hotkeys, hk)
				hotkeysMu.Unlock()
				saveHotkeys()
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

func applyHotkeyVars(cmd string) (string, bool) {
	// Resolve @hovered first (simple, unchanged)
	needHovered := strings.Contains(cmd, "@hovered")
	if needHovered {
		var hoveredName string
		lastHoverMu.Lock()
		hoveredName = lastHover.Mobile.Name
		lastHoverMu.Unlock()
		if hoveredName == "" {
			return "", false
		}
		cmd = strings.ReplaceAll(cmd, "@hovered", hoveredName)
	}

	// Handle new click variables via regex replacement.
	re := regexp.MustCompile(`@([A-Za-z]+)((?:\.[A-Za-z]+)*)\.clicked`)
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
			lastClickByButtonMu.Lock()
			info, ok = lastClickByButton[ebiten.MouseButtonRight]
			lastClickByButtonMu.Unlock()
		case "middle", "middleclick":
			lastClickByButtonMu.Lock()
			info, ok = lastClickByButton[ebiten.MouseButtonMiddle]
			lastClickByButtonMu.Unlock()
		case "left", "leftclick":
			lastClickByButtonMu.Lock()
			info, ok = lastClickByButton[ebiten.MouseButtonLeft]
			lastClickByButtonMu.Unlock()
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

func updateHotkeyRecording() {
	if !recording {
		return
	}
	if time.Since(recordStart) > 5*time.Second {
		finishRecording()
		return
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
		finishRecording()
		return
	}
	if c := detectCombo(); c != "" {
		recordedCombo = c
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

func checkHotkeys() InputEvent {
	if recording {
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
		hotkeysMu.RLock()
		list := append([]Hotkey(nil), hotkeys...)
		hotkeysMu.RUnlock()
		for _, hk := range list {
			if !hk.Disabled && (hk.Combo == combo || strings.EqualFold(hk.Combo, combo) || sameCombo(hk.Combo, combo)) {
				// If this is a script hotkey with a function handler, call it.
				if hk.Script != "" {
					if fn, ok := scriptGetHotkeyFn(hk.Script, hk.Combo); ok && fn != nil {
						scriptLogEvent(hk.Script, "Hotkey", combo)
						ev := makeScriptInputEvent(combo)
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
					cmd, ok = applyHotkeyVars(cmd)
					if !ok {
						return InputEvent{}
					}
					if strings.HasPrefix(strings.ToLower(cmd), "/equip") {
						if hotkeyEquipAlreadyEquipped(cmd) {
							continue
						}
					}
					enqueueCommand(cmd)
				}
				nextCommand()
				break
			}
		}
	}
	return InputEvent{}
}

func makeScriptInputEvent(combo string) InputEvent {
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
	info := worldInfoAt(event.WorldX, event.WorldY)
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
