package main

import (
	"fmt"
	"image/color"
	"strconv"
	"strings"

	"gothoom/eui"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	text "github.com/hajimehoshi/ebiten/v2/text/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

var (
	joystickWin           *eui.WindowData
	controllerDD          *eui.ItemData
	controllerEvents      *eui.EventHandler
	axesText, buttonsText *eui.ItemData
	walkStickDD           *eui.ItemData
	walkEvents            *eui.EventHandler
	cursorStickDD         *eui.ItemData
	cursorEvents          *eui.EventHandler
	walkDeadzoneSlider    *eui.ItemData
	walkDZEvents          *eui.EventHandler
	cursorDeadzoneSlider  *eui.ItemData
	cursorDZEvents        *eui.EventHandler
	click1Input           *eui.ItemData
	click2Input           *eui.ItemData
	click3Input           *eui.ItemData
	inputImgItem          *eui.ItemData
	inputImg              *ebiten.Image
	joystickIDs           []ebiten.GamepadID
	joystickNames         []string
	selectedJoystick      int
	lastAxisCount         int
)

const (
	joystickImgW = 260
	joystickImgH = 150
)

func makeJoystickWindow() {
	if joystickWin != nil {
		return
	}
	joystickWin = eui.NewWindow()
	joystickWin.Title = "Gamepad"
	joystickWin.Closable = true
	joystickWin.Movable = true
	joystickWin.Resizable = true
	joystickWin.AutoSize = true
	joystickWin.NoScroll = true
	joystickWin.SetZone(eui.HZoneCenter, eui.VZoneMiddleTop)

	root := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL, Fixed: true}
	root.Size = eui.Point{X: 350, Y: 200}
	joystickWin.AddItem(root)

	// Prominent notice that this feature is WIP
	wipLabel, _ := eui.NewText()
	wipLabel.Text = "Work in progress, does not function"
	wipLabel.FontSize = 18
	wipLabel.Size = eui.Point{X: 350, Y: 28}
	root.AddItem(wipLabel)

	joystickIDs = ebiten.AppendGamepadIDs(joystickIDs[:0])
	joystickNames = joystickNames[:0]
	for _, id := range joystickIDs {
		joystickNames = append(joystickNames, ebiten.GamepadName(id))
	}

	controllerDD, controllerEvents = eui.NewDropdown()
	controllerDD.Label = "Controller"
	controllerDD.Options = joystickNames
	controllerDD.Size = eui.Point{X: 350, Y: 24}
	controllerEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventDropdownSelected {
			selectedJoystick = ev.Index
			if selectedJoystick >= 0 && selectedJoystick < len(joystickIDs) {
				updateStickOptions(ebiten.GamepadAxisCount(joystickIDs[selectedJoystick]))
			} else {
				updateStickOptions(0)
			}
		}
	}
	root.AddItem(controllerDD)

	refreshBtn, refreshEvents := eui.NewButton()
	refreshBtn.Text = "Refresh"
	refreshBtn.Size = eui.Point{X: 80, Y: 24}
	refreshEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			joystickIDs = ebiten.AppendGamepadIDs(joystickIDs[:0])
			joystickNames = joystickNames[:0]
			for _, id := range joystickIDs {
				joystickNames = append(joystickNames, ebiten.GamepadName(id))
			}
			controllerDD.Options = joystickNames
			controllerDD.Dirty = true
			if selectedJoystick >= len(joystickIDs) {
				selectedJoystick = len(joystickIDs) - 1
				controllerDD.Selected = selectedJoystick
			}
			if selectedJoystick >= 0 && selectedJoystick < len(joystickIDs) {
				updateStickOptions(ebiten.GamepadAxisCount(joystickIDs[selectedJoystick]))
			} else {
				updateStickOptions(0)
			}
			joystickWin.Refresh()
		}
	}
	root.AddItem(refreshBtn)

	axesText, _ = eui.NewText()
	axesText.FontSize = 12
	axesText.Size = eui.Point{X: 350, Y: 24}
	root.AddItem(axesText)

	buttonsText, _ = eui.NewText()
	buttonsText.FontSize = 12
	buttonsText.Size = eui.Point{X: 350, Y: 24}
	root.AddItem(buttonsText)

	enableCB, enableEvents := eui.NewCheckbox()
	enableCB.Text = "Enable Gamepad"
	enableCB.Checked = gs.JoystickEnabled
	enableEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			gs.JoystickEnabled = ev.Checked
			settingsDirty = true
		}
	}
	root.AddItem(enableCB)

	walkStickDD, walkEvents = eui.NewDropdown()
	walkStickDD.Label = "Walk Stick"
	walkStickDD.Size = eui.Point{X: 260, Y: 24}
	walkEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventDropdownSelected {
			gs.JoystickWalkStick = ev.Index - 1
			settingsDirty = true
		}
	}
	root.AddItem(walkStickDD)

	walkDeadzoneSlider, walkDZEvents = eui.NewSlider()
	walkDeadzoneSlider.Label = "Walk Deadzone"
	walkDeadzoneSlider.MinValue = 0.01
	walkDeadzoneSlider.MaxValue = 0.2
	walkDeadzoneSlider.Value = float32(gs.JoystickWalkDeadzone)
	walkDeadzoneSlider.Size = eui.Point{X: 260, Y: 24}
	walkDZEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventSliderChanged {
			gs.JoystickWalkDeadzone = float64(ev.Value)
			settingsDirty = true
		}
	}
	root.AddItem(walkDeadzoneSlider)

	cursorStickDD, cursorEvents = eui.NewDropdown()
	cursorStickDD.Label = "Cursor Stick"
	cursorStickDD.Size = eui.Point{X: 260, Y: 24}
	cursorEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventDropdownSelected {
			gs.JoystickCursorStick = ev.Index - 1
			settingsDirty = true
		}
	}
	root.AddItem(cursorStickDD)

	cursorDeadzoneSlider, cursorDZEvents = eui.NewSlider()
	cursorDeadzoneSlider.Label = "Cursor Deadzone"
	cursorDeadzoneSlider.MinValue = 0.01
	cursorDeadzoneSlider.MaxValue = 0.2
	cursorDeadzoneSlider.Value = float32(gs.JoystickCursorDeadzone)
	cursorDeadzoneSlider.Size = eui.Point{X: 260, Y: 24}
	cursorDZEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventSliderChanged {
			gs.JoystickCursorDeadzone = float64(ev.Value)
			settingsDirty = true
		}
	}
	root.AddItem(cursorDeadzoneSlider)

	click1Input, _ = eui.NewInput()
	click1Input.Label = "Click1 Button"
	click1Input.Size = eui.Point{X: 350, Y: 24}
	root.AddItem(click1Input)

	click2Input, _ = eui.NewInput()
	click2Input.Label = "Click2 Button"
	click2Input.Size = eui.Point{X: 350, Y: 24}
	root.AddItem(click2Input)

	click3Input, _ = eui.NewInput()
	click3Input.Label = "Click3 Button"
	click3Input.Size = eui.Point{X: 350, Y: 24}
	root.AddItem(click3Input)

	inputImgItem, inputImg = eui.NewImageItem(joystickImgW, joystickImgH)
	root.AddItem(inputImgItem)

	if gs.JoystickBindings != nil {
		if b, ok := gs.JoystickBindings["click1"]; ok {
			click1Input.Text = strconv.Itoa(int(b))
		}
		if b, ok := gs.JoystickBindings["click2"]; ok {
			click2Input.Text = strconv.Itoa(int(b))
		}
		if b, ok := gs.JoystickBindings["click3"]; ok {
			click3Input.Text = strconv.Itoa(int(b))
		}
	}

	if len(joystickIDs) > 0 {
		updateStickOptions(ebiten.GamepadAxisCount(joystickIDs[selectedJoystick]))
	} else {
		updateStickOptions(0)
	}

	joystickWin.AddWindow(false)
}

func updateStickOptions(axisCount int) {
	stickCount := axisCount / 2
	opts := make([]string, stickCount+1)
	opts[0] = "none"
	for i := 0; i < stickCount; i++ {
		opts[i+1] = fmt.Sprintf("Stick %d", i+1)
	}
	if walkStickDD != nil {
		walkStickDD.Options = opts
		walkStickDD.Disabled = stickCount == 0
		if gs.JoystickWalkStick >= stickCount {
			gs.JoystickWalkStick = stickCount - 1
		}
		if gs.JoystickWalkStick < -1 {
			gs.JoystickWalkStick = -1
		}
		walkStickDD.Selected = gs.JoystickWalkStick + 1
		walkStickDD.Dirty = true
	}
	if cursorStickDD != nil {
		cursorStickDD.Options = opts
		cursorStickDD.Disabled = stickCount == 0
		if gs.JoystickCursorStick >= stickCount {
			gs.JoystickCursorStick = stickCount - 1
		}
		if gs.JoystickCursorStick < -1 {
			gs.JoystickCursorStick = -1
		}
		cursorStickDD.Selected = gs.JoystickCursorStick + 1
		cursorStickDD.Dirty = true
	}
	lastAxisCount = axisCount
	if joystickWin != nil {
		joystickWin.Refresh()
	}
}

func drawJoystickDisplay(id ebiten.GamepadID) {
	if inputImg == nil {
		return
	}
	inputImg.Clear()

	drawStick := func(cx, cy float32, stick int, label string, dz float64) {
		vector.FillCircle(inputImg, cx, cy, 40, color.NRGBA{64, 64, 64, 255}, true)
		if dz > 0 {
			vector.FillCircle(inputImg, cx, cy, float32(dz)*40, color.NRGBA{32, 32, 32, 255}, true)
		}
		if stick >= 0 {
			axisIndex := stick * 2
			if axisIndex+1 < ebiten.GamepadAxisCount(id) {
				ax := ebiten.GamepadAxisValue(id, axisIndex)
				ay := ebiten.GamepadAxisValue(id, axisIndex+1)
				vector.FillCircle(inputImg, cx+float32(ax)*40, cy+float32(ay)*40, 5, color.NRGBA{0, 255, 0, 255}, true)
			}
		}
		metrics := mainFont.Metrics()
		txtW, _ := text.Measure(label, mainFont, 0)
		scale := 0.7
		op := &text.DrawOptions{}
		op.GeoM.Translate(float64(cx)-float64(txtW)*scale/2, float64(cy)+40+metrics.HAscent*scale)
		op.GeoM.Scale(scale, scale)
		text.Draw(inputImg, label, mainFont, op)
	}

	drawStick(60, 60, gs.JoystickWalkStick, "Walk", gs.JoystickWalkDeadzone)
	drawStick(190, 90, gs.JoystickCursorStick, "Cursor", gs.JoystickCursorDeadzone)

	drawBtn := func(cx, cy, r float32, btn ebiten.GamepadButton, lbl string) {
		col := color.NRGBA{128, 128, 128, 255}
		if ebiten.IsGamepadButtonPressed(id, btn) {
			col = color.NRGBA{0, 255, 0, 255}
		}
		vector.FillCircle(inputImg, cx, cy, r, col, true)
		metrics := mainFont.Metrics()
		txtW, _ := text.Measure(lbl, mainFont, 0)
		scale := 0.5
		op := &text.DrawOptions{}
		op.GeoM.Translate(float64(cx)-float64(txtW)*scale/2, float64(cy)+float64(r)+metrics.HAscent*scale)
		op.GeoM.Scale(scale, scale)
		text.Draw(inputImg, lbl, mainFont, op)
	}

	btns := []struct {
		btn     ebiten.GamepadButton
		x, y, r float32
		lbl     string
	}{
		{ebiten.GamepadButton0, 230, 80, 10, "A"},
		{ebiten.GamepadButton1, 250, 60, 10, "B"},
		{ebiten.GamepadButton2, 210, 60, 10, "X"},
		{ebiten.GamepadButton3, 230, 40, 10, "Y"},
		{ebiten.GamepadButton4, 80, 30, 10, "LB"},
		{ebiten.GamepadButton5, 180, 30, 10, "RB"},
		{ebiten.GamepadButton6, 80, 10, 10, "LT"},
		{ebiten.GamepadButton7, 180, 10, 10, "RT"},
		{ebiten.GamepadButton8, 120, 60, 10, "Back"},
		{ebiten.GamepadButton9, 150, 60, 10, "Start"},
		{ebiten.GamepadButton10, 60, 60, 6, "L3"},
		{ebiten.GamepadButton11, 190, 90, 6, "R3"},
		{ebiten.GamepadButton12, 80, 100, 8, "Up"},
		{ebiten.GamepadButton13, 90, 110, 8, "Right"},
		{ebiten.GamepadButton14, 80, 120, 8, "Down"},
		{ebiten.GamepadButton15, 70, 110, 8, "Left"},
	}
	for _, b := range btns {
		drawBtn(b.x, b.y, b.r, b.btn, b.lbl)
	}

	inputImgItem.Dirty = true
}

func updateJoystickWindow() {
	joystickWin.Refresh()
	newIDs := inpututil.AppendJustConnectedGamepadIDs(nil)
	if len(newIDs) > 0 {
		for _, id := range newIDs {
			joystickIDs = append(joystickIDs, id)
			joystickNames = append(joystickNames, ebiten.GamepadName(id))
		}
		if controllerDD != nil {
			controllerDD.Options = joystickNames
			controllerDD.Dirty = true
			joystickWin.Refresh()
		}
	}
	for i := 0; i < len(joystickIDs); {
		id := joystickIDs[i]
		if inpututil.IsGamepadJustDisconnected(id) {
			joystickIDs = append(joystickIDs[:i], joystickIDs[i+1:]...)
			joystickNames = append(joystickNames[:i], joystickNames[i+1:]...)
			if controllerDD != nil {
				controllerDD.Options = joystickNames
				controllerDD.Dirty = true
				if selectedJoystick >= len(joystickIDs) {
					selectedJoystick = len(joystickIDs) - 1
					controllerDD.Selected = selectedJoystick
				}
				joystickWin.Refresh()
			}
			continue
		}
		i++
	}

	if len(joystickIDs) == 0 {
		updateStickOptions(0)
	}

	if selectedJoystick < 0 || selectedJoystick >= len(joystickIDs) {
		updateStickOptions(0)
		axesText.Text = ""
		buttonsText.Text = ""
		axesText.Dirty = true
		buttonsText.Dirty = true
		return
	}

	id := joystickIDs[selectedJoystick]

	axisCount := ebiten.GamepadAxisCount(id)
	if axisCount != lastAxisCount {
		updateStickOptions(axisCount)
	}
	axes := make([]string, axisCount)
	for a := 0; a < axisCount; a++ {
		axes[a] = fmt.Sprintf("%d:%.2f", a, ebiten.GamepadAxisValue(id, a))
	}
	axesText.Text = "Axes: " + strings.Join(axes, " ")
	axesText.Dirty = true

	buttonCount := ebiten.GamepadButtonCount(id)
	pressed := []string{}
	for b := 0; b < buttonCount; b++ {
		if ebiten.IsGamepadButtonPressed(id, ebiten.GamepadButton(b)) {
			pressed = append(pressed, strconv.Itoa(b))
		}
	}
	buttonsText.Text = "Pressed: " + strings.Join(pressed, " ")
	buttonsText.Dirty = true

	btns := inpututil.AppendJustPressedGamepadButtons(id, nil)
	if len(btns) > 0 {
		if click1Input != nil && click1Input.Focused {
			if gs.JoystickBindings == nil {
				gs.JoystickBindings = make(map[string]ebiten.GamepadButton)
			}
			gs.JoystickBindings["click1"] = btns[len(btns)-1]
			click1Input.Text = strconv.Itoa(int(gs.JoystickBindings["click1"]))
			click1Input.Dirty = true
			settingsDirty = true
		} else if click2Input != nil && click2Input.Focused {
			if gs.JoystickBindings == nil {
				gs.JoystickBindings = make(map[string]ebiten.GamepadButton)
			}
			gs.JoystickBindings["click2"] = btns[len(btns)-1]
			click2Input.Text = strconv.Itoa(int(gs.JoystickBindings["click2"]))
			click2Input.Dirty = true
			settingsDirty = true
		} else if click3Input != nil && click3Input.Focused {
			if gs.JoystickBindings == nil {
				gs.JoystickBindings = make(map[string]ebiten.GamepadButton)
			}
			gs.JoystickBindings["click3"] = btns[len(btns)-1]
			click3Input.Text = strconv.Itoa(int(gs.JoystickBindings["click3"]))
			click3Input.Dirty = true
			settingsDirty = true
		}
	}

	drawJoystickDisplay(id)
}
