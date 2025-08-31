package main

import (
    "bytes"
    "encoding/json"
    "fmt"
    "math"
    "os"
    "path/filepath"
    "strconv"
    "strings"
    "sync"
    "time"

    "gothoom/eui"

    "github.com/hajimehoshi/ebiten/v2"
    "github.com/hajimehoshi/ebiten/v2/inpututil"
    text "github.com/hajimehoshi/ebiten/v2/text/v2"
    "golang.org/x/image/font/gofont/goregular"
    "regexp"
)

const hotkeysFile = "global-hotkeys.json"

type HotkeyCommand struct {
	Command string `json:"command,omitempty"`
}

type Hotkey struct {
	Name     string          `json:"name,omitempty"`
	Combo    string          `json:"combo"`
	Commands []HotkeyCommand `json:"commands"`
	Plugin   string          `json:"plugin,omitempty"`
	Disabled bool            `json:"disabled,omitempty"`
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

	pluginHotkeyMu sync.RWMutex

	// pluginHotkeyEnabled holds the persisted enabled state for plugin
	// hotkeys. The map is keyed first by plugin name and then by combo.
	pluginHotkeyEnabled = map[string]map[string]bool{}
)

func loadHotkeys() {
	path := filepath.Join(dataDirPath, hotkeysFile)
	pluginHotkeyMu.Lock()
	pluginHotkeyEnabled = map[string]map[string]bool{}
	data, err := os.ReadFile(path)

	var newList []Hotkey
	if err == nil {
		type hotkeyJSON struct {
			Combo    string          `json:"combo"`
			Name     string          `json:"name,omitempty"`
			Commands []HotkeyCommand `json:"commands"`
			Command  string          `json:"command"`
			Text     string          `json:"text,omitempty"`
			Plugin   string          `json:"plugin,omitempty"`
			Disabled *bool           `json:"disabled,omitempty"`
			Enabled  *bool           `json:"enabled,omitempty"`
		}
		var raw []hotkeyJSON
		if err := json.Unmarshal(data, &raw); err != nil {
			pluginHotkeyMu.Unlock()
			return
		}
		for _, r := range raw {
			if r.Plugin != "" {
				m := pluginHotkeyEnabled[r.Plugin]
				if m == nil {
					m = map[string]bool{}
					pluginHotkeyEnabled[r.Plugin] = m
				}
				enabled := false
				if r.Enabled != nil {
					enabled = *r.Enabled
				} else if r.Disabled != nil {
					enabled = !*r.Disabled
				}
				if enabled {
					m[r.Combo] = true
				}
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
	} else if !os.IsNotExist(err) {
		pluginHotkeyMu.Unlock()
		return
	}
	pluginHotkeyMu.Unlock()

	// Ensure default hotkeys exist.
    def := Hotkey{Name: "Click To Use", Combo: "RightClick", Commands: []HotkeyCommand{{Command: "/use @right.clicked"}}, Disabled: true}
	exists := false
	for _, hk := range newList {
		if hk.Combo == def.Combo && hk.Plugin == "" {
			exists = true
			break
		}
	}
	if !exists {
		newList = append(newList, def)
	}

	fs := Hotkey{Name: "Toggle Fullscreen", Combo: "F12", Commands: []HotkeyCommand{{Command: "/fullscreen"}}}
	exists = false
	for _, hk := range newList {
		if hk.Combo == fs.Combo && hk.Plugin == "" {
			exists = true
			break
		}
	}
	if !exists {
		newList = append(newList, fs)
	}

	hotkeysMu.Lock()
	hotkeys = newList
	hotkeysMu.Unlock()
	refreshHotkeysList()
}

func saveHotkeys() {
	path := filepath.Join(dataDirPath, hotkeysFile)
	_ = os.MkdirAll(dataDirPath, 0o755)
	// snapshot under read lock
	hotkeysMu.RLock()
	snap := append([]Hotkey(nil), hotkeys...)
	hotkeysMu.RUnlock()
	type pluginState struct {
		Plugin  string `json:"plugin,omitempty"`
		Combo   string `json:"combo"`
		Enabled bool   `json:"enabled,omitempty"`
	}

	var out []any
	pluginHotkeyMu.Lock()
	for _, hk := range snap {
		if hk.Plugin != "" {
			if hk.Disabled {
				if m := pluginHotkeyEnabled[hk.Plugin]; m != nil {
					delete(m, hk.Combo)
					if len(m) == 0 {
						delete(pluginHotkeyEnabled, hk.Plugin)
					}
				}
			} else {
				m := pluginHotkeyEnabled[hk.Plugin]
				if m == nil {
					m = map[string]bool{}
					pluginHotkeyEnabled[hk.Plugin] = m
				}
				m[hk.Combo] = true
			}
			continue
		}
		out = append(out, hk)
	}
	for plug, m := range pluginHotkeyEnabled {
		for combo := range m {
			out = append(out, pluginState{Plugin: plug, Combo: combo, Enabled: true})
		}
	}
	pluginHotkeyMu.Unlock()

	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(path, data, 0o644)
}

func pluginHotkeys(owner string) []Hotkey {
	hotkeysMu.RLock()
	defer hotkeysMu.RUnlock()
	var list []Hotkey
	for _, hk := range hotkeys {
		if hk.Plugin == owner {
			list = append(list, hk)
		}
	}
	return list
}

func pluginRemoveHotkey(owner, combo string) {
	hotkeysMu.Lock()
	for i := 0; i < len(hotkeys); i++ {
		hk := hotkeys[i]
		if hk.Plugin == owner && hk.Combo == combo {
			hotkeys = append(hotkeys[:i], hotkeys[i+1:]...)
			i--
		}
	}
	hotkeysMu.Unlock()
	pluginHotkeyMu.Lock()
	if m := pluginHotkeyEnabled[owner]; m != nil {
		delete(m, combo)
		if len(m) == 0 {
			delete(pluginHotkeyEnabled, owner)
		}
	}
	pluginHotkeyMu.Unlock()
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
		if hk.Plugin != "" {
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

	// plugin hotkeys header and list
	headerAdded := false
	for i, hk := range list {
		if hk.Plugin == "" {
			continue
		}
		if !headerAdded {
			label := &eui.ItemData{ItemType: eui.ITEM_TEXT, Text: "Plugin Hotkeys", Fixed: true}
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
				if hk.Plugin != "" {
					pluginHotkeyMu.Lock()
					if hk.Disabled {
						if m := pluginHotkeyEnabled[hk.Plugin]; m != nil {
							delete(m, hk.Combo)
							if len(m) == 0 {
								delete(pluginHotkeyEnabled, hk.Plugin)
							}
						}
					} else {
						m := pluginHotkeyEnabled[hk.Plugin]
						if m == nil {
							m = map[string]bool{}
							pluginHotkeyEnabled[hk.Plugin] = m
						}
						m[hk.Combo] = true
					}
					pluginHotkeyMu.Unlock()
				}
				saveHotkeys()
			}
		}
		row.AddItem(cb)
		text := hk.Combo
		if hk.Name != "" {
			text = hk.Name + " : " + hk.Combo
		}
		disp := pluginDisplayNames[hk.Plugin]
		if disp == "" {
			disp = hk.Plugin
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
	recordBtn, recordEvents := eui.NewButton()
	recordBtn.Text = "Record"
	recordBtn.Size = eui.Point{X: 60, Y: 20}
	recordBtn.FontSize = 12
	recordEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			startRecording(hotkeyComboText)
		}
	}
	row.AddItem(recordBtn)
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
	cmdInput.Size = eui.Point{X: hotkeyEditWin.Size.X - 40, Y: 20}
	cmdInput.FontSize = 12
	cmdInput.Scrollable = true
	cmdInput.Text = cmd
	cmdEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventInputChanged {
			wrapHotkeyInputs()
		}
	}
	hotkeyCmdSection.AddItem(cmdInput)
	hotkeyCmdInputs = append(hotkeyCmdInputs, cmdInput)

	hotkeyEditWin.Refresh()
	wrapHotkeyInputs()
}

func wrapHotkeyInputs() {
	if hotkeyEditWin == nil {
		return
	}
	ui := eui.UIScale()
	fs := float32(12)
	if len(hotkeyCmdInputs) > 0 {
		fs = hotkeyCmdInputs[0].FontSize
	}
	facePx := float64(fs * ui)
	src := eui.FontSource()
	if src == nil {
		if s, err := text.NewGoTextFaceSource(bytes.NewReader(goregular.TTF)); err == nil {
			src = s
		} else {
			return
		}
	}
	goFace := &text.GoTextFace{Source: src, Size: facePx}
	metrics := goFace.Metrics()
	linePx := math.Ceil(metrics.HAscent + metrics.HDescent + 2)
	rowUnits := float32(linePx) / ui
	padPx := float64(6) * float64(ui)

	resize := func(it *eui.ItemData) {
		if it == nil {
			return
		}
		raw := strings.ReplaceAll(it.Text, "\n", " ")
		_, lines := wrapText(raw, goFace, float64(it.Size.X*ui)-padPx)
		if len(lines) == 0 {
			lines = []string{""}
		}
		if n := len(raw) - len(strings.TrimRight(raw, " ")); n > 0 {
			lines[len(lines)-1] += strings.Repeat(" ", n)
		}
		it.Text = strings.Join(lines, "\n")
		if it.TextPtr != nil {
			*it.TextPtr = it.Text
		}
		it.Size.Y = rowUnits * float32(len(lines))
	}

	for _, it := range hotkeyCmdInputs {
		resize(it)
	}
	hotkeyEditWin.Refresh()
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
						name = hk.Plugin
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

func startRecording(target *eui.ItemData) {
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
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		if combo := comboFromMouseWithKey(ebiten.MouseButtonLeft); combo != "" {
			return combo
		}
	}
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonRight) {
		return comboFromMouse(ebiten.MouseButtonRight)
	}
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonMiddle) {
		return comboFromMouse(ebiten.MouseButtonMiddle)
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
	mods = append(mods, k.String())
	return strings.Join(mods, "-")
}

func comboFromMouse(b ebiten.MouseButton) string {
	mods := currentMods()
	name := mouseButtonName(b)
	mods = append(mods, name)
	return strings.Join(mods, "-")
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
	if keyPart == "" && len(mods) == 0 {
		return ""
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
		return fmt.Sprintf("Mouse %d", b)
	}
}

func isModifier(k ebiten.Key) bool {
	switch k {
	case ebiten.KeyShift, ebiten.KeyShiftLeft, ebiten.KeyShiftRight,
		ebiten.KeyControl, ebiten.KeyControlLeft, ebiten.KeyControlRight,
		ebiten.KeyAlt, ebiten.KeyAltLeft, ebiten.KeyAltRight:
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

func checkHotkeys() {
	if recording || inputActive || typingInUI() {
		return
	}
	if combo := detectCombo(); combo != "" {
		hotkeysMu.RLock()
		list := append([]Hotkey(nil), hotkeys...)
		hotkeysMu.RUnlock()
		for _, hk := range list {
			if hk.Combo == combo && !hk.Disabled {
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
						return
					}
					if strings.HasPrefix(strings.ToLower(cmd), "/equip") {
						if hotkeyEquipAlreadyEquipped(cmd) {
							continue
						}
					}
					if cmd != "" {
						consoleMessage("> " + cmd)
					}
					enqueueCommand(cmd)
				}
				nextCommand()
				break
			}
		}
	}
}
