package main

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"gothoom/eui"
)

func openLegacyTriggerEditor(entry legacyMacroLibraryEntry, keys bool) *eui.WindowData {
	doc, err := loadLegacyTriggerDocument(entry, keys)
	if err != nil {
		legacyMacroLibraryReport(err.Error())
		return nil
	}
	win := eui.NewWindow()
	kind := "Commands"
	if keys {
		kind = "Keybinds"
	}
	win.Title = entry.Name + " — " + kind
	win.Closable = true
	win.Movable = true
	win.Resizable = true
	win.NoScroll = true
	win.Size = eui.Point{X: 760, Y: 500}
	win.SetZone(eui.HZoneCenterLeft, eui.VZoneMiddleTop)
	root := eui.NewColumn()
	root.Fixed = true
	introText := "Rename typed commands and abbreviations. Changes apply to every character using these files."
	if keys {
		introText = "Change keys, clicks, or wheel bindings. Changes apply to every character using these files."
	}
	intro := eui.NewLabel(introText)
	intro.Size = eui.Point{X: 720, Y: 40}
	root.AddItem(intro)
	list := eui.NewColumn()
	list.Fixed = true
	list.Scrollable = true
	root.AddItem(list)
	inputs := make([]*eui.ItemData, len(doc.fields))
	status := eui.NewLabel("")
	status.Size = eui.Point{X: 720, Y: 56}
	if len(doc.program.Diagnostics) > 0 {
		status.Text = fmt.Sprintf("%d existing source diagnostics. Saving checks for new errors.", len(doc.program.Diagnostics))
	}
	setStatus := func(message string) { status.Text = message; status.Dirty = true; win.Refresh() }
	for index, field := range doc.fields {
		path, err := filepath.Rel(legacyMacrosDir(), field.declaration.Header.Path)
		if err != nil {
			path = field.declaration.Header.Path
		}
		heading := eui.NewLabel(fmt.Sprintf("%s:%d  ·  %s", path, field.declaration.Header.Line, field.declaration.Trigger))
		heading.Size = eui.Point{X: 700, Y: 28}
		if index > 0 {
			heading.Position.Y = 10
		}
		list.AddItem(heading)
		input, _ := eui.NewInput()
		input.Text = field.declaration.Trigger
		input.Size = eui.Point{X: 460, Y: 28}
		inputs[index] = input
		row := eui.NewRow(input)
		if keys {
			record, events := eui.NewButton()
			record.Text = "Record"
			record.Size = eui.Point{X: 90, Y: 28}
			record.Position.X = 8
			events.Handle = func(ev eui.UIEvent) {
				if ev.Type == eui.EventClick {
					startLegacyTriggerRecording(win, input, setStatus)
				}
			}
			row.AddItem(record)
		}
		reset, events := eui.NewButton()
		reset.Text = "Reset"
		reset.Size = eui.Point{X: 70, Y: 28}
		reset.Position.X = 8
		original := field.declaration.Trigger
		events.Handle = func(ev eui.UIEvent) {
			if ev.Type == eui.EventClick {
				input.Text = original
				input.Dirty = true
				win.Refresh()
			}
		}
		row.AddItem(reset)
		list.AddItem(row)
	}
	if len(inputs) == 0 {
		list.AddItem(eui.NewLabel("No editable " + strings.ToLower(kind) + " in this macro."))
	}
	root.AddItem(status)
	buttons := eui.NewRow()
	buttons.Fixed = true
	buttons.Size = eui.Point{X: 720, Y: 28}
	save, saveEvents := eui.NewButton()
	save.Text = "Save"
	save.Size = eui.Point{X: 100, Y: 28}
	save.Disabled = isWASM || len(inputs) == 0
	saveEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type != eui.EventClick {
			return
		}
		values := make([]string, len(inputs))
		for i, input := range inputs {
			values[i] = input.Text
		}
		if err := doc.save(values); err != nil {
			setStatus(err.Error())
			return
		}
		legacyMacroLibraryReload()
		win.Close()
	}
	cancel, cancelEvents := eui.NewButton()
	cancel.Text = "Cancel"
	cancel.Size = eui.Point{X: 100, Y: 28}
	cancel.Position.X = 12
	cancelEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			win.Close()
		}
	}
	buttons.AddItem(save)
	buttons.AddItem(cancel)
	root.AddItem(buttons)
	win.AddItem(root)
	win.OnResize = func() { eui.LayoutWindowBody(win, root, list) }

	win.OnClose = func() {
		if legacyTriggerRecorder != nil && legacyTriggerRecorder.window == win {
			legacyTriggerRecorder = nil
		}
		win.RemoveWindow()
	}
	win.AddWindow(false)
	win.MarkOpen()
	win.OnResize()
	win.Refresh()
	return win
}

type legacyTriggerRecordState struct {
	window      *eui.WindowData
	target      *eui.ItemData
	status      func(string)
	started     time.Time
	armed, done bool
}

var legacyTriggerRecorder *legacyTriggerRecordState
var legacyTriggerRecordFrame bool

func startLegacyTriggerRecording(win *eui.WindowData, target *eui.ItemData, status func(string)) {
	legacyTriggerRecorder = &legacyTriggerRecordState{window: win, target: target, status: status, started: time.Now()}
	status("Press a key or click with modifiers. Esc cancels; recording times out after 10 seconds.")
}

func bindingInputCaptured() bool {
	return keyboardTestSuppressingInput() || legacyTriggerRecordFrame || legacyTriggerRecorder != nil
}
func bindingCaptureFrameActive() bool { return keyboardTestFrameActive || legacyTriggerRecordFrame }

func legacyTriggerRecordInputReleased() bool {
	for b := ebiten.MouseButton(0); b <= ebiten.MouseButtonMax; b++ {
		if ebiten.IsMouseButtonPressed(b) {
			return false
		}
	}
	for _, key := range inpututil.AppendPressedKeys(nil) {
		if !isModifier(key) {
			return false
		}
	}
	return true
}

func legacyTriggerBeginInputFrame() {
	legacyTriggerRecordFrame = legacyTriggerRecorder != nil
	r := legacyTriggerRecorder
	if r == nil {
		return
	}
	if !r.window.IsOpen() {
		legacyTriggerRecorder = nil
		return
	}
	if r.done {
		if legacyTriggerRecordInputReleased() {
			legacyTriggerRecorder = nil
		}
		return
	}
	if !ebiten.IsFocused() || time.Since(r.started) > 10*time.Second {
		r.status("Recording cancelled. The binding has not changed.")
		r.done = true
		return
	}
	if !r.armed {
		r.armed = legacyTriggerRecordInputReleased()
		return
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		r.status("Recording cancelled. The binding has not changed.")
		r.done = true
		return
	}
	name := ""
	modifiers := legacyMacroCurrentModifiers(false)
	for button := ebiten.MouseButton(0); button <= ebiten.MouseButtonMax; button++ {
		if inpututil.IsMouseButtonJustPressed(button) {
			name = keyboardTestLegacyMouseName(button)
			break
		}
	}
	if name == "" {
		x, y := ebiten.Wheel()
		name, modifiers = legacyMacroWheelInput(x, y, modifiers)
	}
	if name == "" {
		keys := inpututil.AppendJustPressedKeys(nil)
		for _, key := range keys {
			physical, numpad, ok := legacyMacroKeyName(key)
			if !ok || isModifier(key) {
				continue
			}
			modifiers = legacyMacroCurrentModifiers(numpad)
			name = legacyMacroPrintedKeyName(key, physical, modifiers, keys, ebiten.AppendInputChars(nil))
			break
		}
	}
	if name == "" {
		return
	}
	combo := legacyTriggerRecordedName(name, modifiers)
	field := legacyTriggerField{}
	if _, err := formatLegacyTrigger(field, combo); err != nil {
		r.status(err.Error())
		r.done = true
		return
	}
	r.target.Text = combo
	r.target.Dirty = true
	r.done = true
	r.status("Recorded " + combo + ". Save to apply it.")
}

func legacyTriggerRecordedName(name string, modifiers legacyMacroModifiers) string {
	var parts []string
	for _, m := range []struct {
		flag legacyMacroModifiers
		name string
	}{{legacyMacroModCommand, "Command"}, {legacyMacroModControl, "Control"}, {legacyMacroModNumpad, "Numpad"}, {legacyMacroModOption, "Option"}, {legacyMacroModShift, "Shift"}} {
		if modifiers&m.flag != 0 {
			parts = append(parts, m.name)
		}
	}
	return strings.Join(append(parts, name), "-")
}
