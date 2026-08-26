package main

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"gothoom/eui"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

type keyboardTestKeySpec struct {
	Key   ebiten.Key
	Label string
	Width float32
	Blank bool
}

var (
	keyboardTestWin         *eui.WindowData
	keyboardTestPressedList *eui.ItemData
	keyboardTestKeys        map[ebiten.Key]*eui.ItemData
	keyboardTestKeyStates   map[ebiten.Key]bool
	keyboardTestMouse       map[ebiten.MouseButton]*eui.ItemData
	keyboardTestMouseStates map[ebiten.MouseButton]bool
	keyboardTestMouseUntil  map[ebiten.MouseButton]time.Time
	keyboardTestWheel       map[string]*eui.ItemData
	keyboardTestWheelStates map[string]bool
	keyboardTestWheelUntil  map[string]time.Time
	keyboardTestSignature   string
	keyboardTestFrameActive bool
)

var (
	keyboardTestIdleColor    = eui.NewColor(48, 52, 60, 255)
	keyboardTestPressedColor = eui.NewColor(210, 48, 48, 255)
)

func openKeyboardTestWindow() {
	makeKeyboardTestWindow()
	keyboardTestSignature = ""
	clear(keyboardTestMouseUntil)
	clear(keyboardTestWheelUntil)
	refreshKeyboardTestPressedList(nil)
	keyboardTestWin.MarkOpen()
}

func makeKeyboardTestWindow() {
	if keyboardTestWin != nil {
		return
	}
	keyboardTestWin = eui.NewWindow()
	keyboardTestWin.Title = "Test Keyboard/Mouse"
	keyboardTestWin.Size = eui.Point{X: 960, Y: 360}
	keyboardTestWin.Closable = true
	keyboardTestWin.Movable = true
	keyboardTestWin.Resizable = false
	keyboardTestWin.NoScroll = true
	keyboardTestWin.SetZone(eui.HZoneCenter, eui.VZoneMiddleTop)

	root := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL, Fixed: true}
	keyboardTestWin.AddItem(root)
	note, _ := eui.NewText()
	note.Text = "Press keys, mouse buttons, or the wheel. Bindings are paused while open."
	note.FontSize = 11
	note.Size = eui.Point{X: 930, Y: 22}
	root.AddItem(note)

	body := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_HORIZONTAL, Fixed: true}
	body.Size = eui.Point{X: 930, Y: 290}
	root.AddItem(body)

	keyboard := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL, Fixed: true}
	keyboard.Size = eui.Point{X: 760, Y: 290}
	body.AddItem(keyboard)
	keyboardTestKeys = make(map[ebiten.Key]*eui.ItemData)
	keyboardTestKeyStates = make(map[ebiten.Key]bool)
	addKeyboardTestRows(keyboard, 760, keyboardTestFunctionLayout())

	keyBlocks := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_HORIZONTAL, Fixed: true}
	keyBlocks.Size = eui.Point{X: 760, Y: 145}
	keyboard.AddItem(keyBlocks)
	addKeyboardTestSection(keyBlocks, 480, keyboardTestMainLayout())
	addKeyboardTestSection(keyBlocks, 122, keyboardTestNavigationLayout())
	addKeyboardTestSection(keyBlocks, 150, keyboardTestNumpadLayout())
	addKeyboardTestMouseRow(keyboard)

	pressed := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL, Fixed: true, Margin: 4}
	pressed.Size = eui.Point{X: 160, Y: 290}
	body.AddItem(pressed)
	heading, _ := eui.NewText()
	heading.Text = "Detected input"
	heading.FontSize = 11
	heading.Size = eui.Point{X: 155, Y: 22}
	pressed.AddItem(heading)
	keyboardTestPressedList = &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL, Scrollable: true, Fixed: true}
	keyboardTestPressedList.Size = eui.Point{X: 155, Y: 260}
	pressed.AddItem(keyboardTestPressedList)

	keyboardTestWin.AddWindow(false)
	refreshKeyboardTestPressedList(nil)
}

func addKeyboardTestSection(parent *eui.ItemData, width float32, rows [][]keyboardTestKeySpec) {
	section := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL, Fixed: true, Margin: 2}
	section.Size = eui.Point{X: width, Y: 145}
	parent.AddItem(section)
	addKeyboardTestRows(section, width, rows)
}

func addKeyboardTestRows(parent *eui.ItemData, width float32, rows [][]keyboardTestKeySpec) {
	for _, specs := range rows {
		row := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_HORIZONTAL, Fixed: true}
		row.Size = eui.Point{X: width, Y: 27}
		for _, spec := range specs {
			keyWidth := spec.Width
			if keyWidth == 0 {
				keyWidth = 32
			}
			if spec.Blank {
				row.AddItem(&eui.ItemData{ItemType: eui.ITEM_TEXT, Size: eui.Point{X: keyWidth, Y: 24}, Fixed: true})
				continue
			}
			key := newKeyboardTestIndicator(spec.Label, keyWidth)
			row.AddItem(key)
			keyboardTestKeys[spec.Key] = key
		}
		parent.AddItem(row)
	}
}

func addKeyboardTestMouseRow(parent *eui.ItemData) {
	row := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_HORIZONTAL, Fixed: true}
	row.Size = eui.Point{X: 760, Y: 31}
	keyboardTestMouse = make(map[ebiten.MouseButton]*eui.ItemData)
	keyboardTestMouseStates = make(map[ebiten.MouseButton]bool)
	keyboardTestMouseUntil = make(map[ebiten.MouseButton]time.Time)
	for _, button := range []ebiten.MouseButton{
		ebiten.MouseButtonLeft,
		ebiten.MouseButtonMiddle,
		ebiten.MouseButtonRight,
		ebiten.MouseButton3,
		ebiten.MouseButton4,
	} {
		item := newKeyboardTestIndicator(keyboardTestMouseButtonName(button), 72)
		row.AddItem(item)
		keyboardTestMouse[button] = item
	}
	keyboardTestWheel = make(map[string]*eui.ItemData)
	keyboardTestWheelStates = make(map[string]bool)
	keyboardTestWheelUntil = make(map[string]time.Time)
	for _, direction := range []string{"Wheel Up", "Wheel Down", "Wheel Left", "Wheel Right"} {
		item := newKeyboardTestIndicator(direction, 90)
		row.AddItem(item)
		keyboardTestWheel[direction] = item
	}
	parent.AddItem(row)
}

func newKeyboardTestIndicator(label string, width float32) *eui.ItemData {
	return &eui.ItemData{
		ItemType:       eui.ITEM_TEXT,
		Text:           " " + label,
		Size:           eui.Point{X: width, Y: 24},
		FontSize:       9,
		Margin:         1,
		Fixed:          true,
		Filled:         true,
		Outlined:       true,
		Border:         1,
		Color:          keyboardTestIdleColor,
		OutlineColor:   eui.NewColor(128, 132, 140, 255),
		TextColor:      eui.NewColor(255, 255, 255, 255),
		ForceTextColor: true,
	}
}

func keyboardTestCapturing() bool {
	return keyboardTestWin != nil && keyboardTestWin.IsOpen()
}

func keyboardTestBeginInputFrame() {
	keyboardTestFrameActive = keyboardTestCapturing()
}

func keyboardTestSuppressingInput() bool {
	return keyboardTestFrameActive || keyboardTestCapturing()
}

func updateKeyboardTest() {
	if !keyboardTestSuppressingInput() {
		return
	}
	pressed := inpututil.AppendPressedKeys(nil)
	pressedSet := make(map[ebiten.Key]bool, len(pressed))
	for _, key := range pressed {
		pressedSet[key] = true
		legacyMacroMarkKeyConsumed(key, key.String())
	}
	for key, item := range keyboardTestKeys {
		down := pressedSet[key]
		keyboardTestSetIndicator(item, down, keyboardTestKeyStates[key])
		keyboardTestKeyStates[key] = down
	}

	names := keyboardTestPressedNames(pressed)
	now := time.Now()
	for button, item := range keyboardTestMouse {
		down := ebiten.IsMouseButtonPressed(button)
		if inpututil.IsMouseButtonJustPressed(button) {
			keyboardTestMouseUntil[button] = now.Add(400 * time.Millisecond)
		}
		active := down || now.Before(keyboardTestMouseUntil[button])
		keyboardTestSetIndicator(item, active, keyboardTestMouseStates[button])
		keyboardTestMouseStates[button] = active
		if active {
			names = append(names, keyboardTestMouseButtonName(button))
		}
		if down {
			legacyMacroMarkMouseConsumed(button, keyboardTestLegacyMouseName(button))
		}
	}

	wheelX, wheelY := ebiten.Wheel()
	for _, direction := range keyboardTestWheelDirections(wheelX, wheelY) {
		keyboardTestWheelUntil[direction] = now.Add(400 * time.Millisecond)
		legacyMacroMarkInputConsumed(strings.ReplaceAll(strings.ToLower(direction), " ", ""))
	}
	for direction, item := range keyboardTestWheel {
		active := now.Before(keyboardTestWheelUntil[direction])
		keyboardTestSetIndicator(item, active, keyboardTestWheelStates[direction])
		keyboardTestWheelStates[direction] = active
		if active {
			names = append(names, direction)
		}
	}
	sort.Strings(names)
	signature := strings.Join(names, "\x00")
	if signature != keyboardTestSignature {
		keyboardTestSignature = signature
		refreshKeyboardTestPressedList(names)
	}
}

func keyboardTestSetIndicator(item *eui.ItemData, active, previous bool) {
	if item == nil || active == previous {
		return
	}
	item.Color = keyboardTestIdleColor
	if active {
		item.Color = keyboardTestPressedColor
	}
	item.Dirty = true
}

func keyboardTestMouseButtonName(button ebiten.MouseButton) string {
	switch button {
	case ebiten.MouseButtonLeft:
		return "Left Mouse"
	case ebiten.MouseButtonMiddle:
		return "Middle Mouse"
	case ebiten.MouseButtonRight:
		return "Right Mouse"
	case ebiten.MouseButton3:
		return "Mouse 4"
	case ebiten.MouseButton4:
		return "Mouse 5"
	default:
		return fmt.Sprintf("Mouse %d", int(button)+1)
	}
}

func keyboardTestLegacyMouseName(button ebiten.MouseButton) string {
	switch button {
	case ebiten.MouseButtonLeft:
		return "click"
	case ebiten.MouseButtonMiddle:
		return "click3"
	case ebiten.MouseButtonRight:
		return "click2"
	default:
		return fmt.Sprintf("click%d", int(button)+1)
	}
}

func keyboardTestWheelDirections(x, y float64) []string {
	directions := make([]string, 0, 2)
	if y > 0 {
		directions = append(directions, "Wheel Up")
	} else if y < 0 {
		directions = append(directions, "Wheel Down")
	}
	if x > 0 {
		directions = append(directions, "Wheel Right")
	} else if x < 0 {
		directions = append(directions, "Wheel Left")
	}
	return directions
}

func refreshKeyboardTestPressedList(names []string) {
	if keyboardTestPressedList == nil {
		return
	}
	keyboardTestPressedList.Contents = keyboardTestPressedList.Contents[:0]
	if len(names) == 0 {
		names = []string{"None"}
	}
	for _, name := range names {
		item, _ := eui.NewText()
		item.Text = name
		item.FontSize = 10
		item.Size = eui.Point{X: 140, Y: 20}
		keyboardTestPressedList.AddItem(item)
	}
	keyboardTestPressedList.Dirty = true
	if keyboardTestWin != nil {
		keyboardTestWin.Refresh()
	}
}

func keyboardTestPressedNames(keys []ebiten.Key) []string {
	names := make([]string, 0, len(keys))
	for _, key := range keys {
		identifier := key.String()
		if identifier == "" {
			identifier = fmt.Sprintf("Key(%d)", key)
		}
		if osName := ebiten.KeyName(key); osName != "" && !strings.EqualFold(osName, identifier) {
			identifier += " (" + osName + ")"
		}
		names = append(names, identifier)
	}
	sort.Strings(names)
	return names
}

func keyboardTestLayout() [][]keyboardTestKeySpec {
	var layout [][]keyboardTestKeySpec
	layout = append(layout, keyboardTestFunctionLayout()...)
	layout = append(layout, keyboardTestMainLayout()...)
	layout = append(layout, keyboardTestNavigationLayout()...)
	layout = append(layout, keyboardTestNumpadLayout()...)
	return layout
}

func keyboardTestFunctionLayout() [][]keyboardTestKeySpec {
	return [][]keyboardTestKeySpec{
		{
			{Key: ebiten.KeyEscape, Label: "Esc", Width: 42},
			{Key: ebiten.KeyF1, Label: "F1"}, {Key: ebiten.KeyF2, Label: "F2"},
			{Key: ebiten.KeyF3, Label: "F3"}, {Key: ebiten.KeyF4, Label: "F4"},
			{Key: ebiten.KeyF5, Label: "F5"}, {Key: ebiten.KeyF6, Label: "F6"},
			{Key: ebiten.KeyF7, Label: "F7"}, {Key: ebiten.KeyF8, Label: "F8"},
			{Key: ebiten.KeyF9, Label: "F9"}, {Key: ebiten.KeyF10, Label: "F10"},
			{Key: ebiten.KeyF11, Label: "F11"}, {Key: ebiten.KeyF12, Label: "F12"},
			{Key: ebiten.KeyPrintScreen, Label: "Print", Width: 44},
			{Key: ebiten.KeyScrollLock, Label: "Scroll", Width: 44},
			{Key: ebiten.KeyPause, Label: "Pause", Width: 44},
		},
		{
			{Blank: true, Width: 42},
			{Key: ebiten.KeyF13, Label: "F13"}, {Key: ebiten.KeyF14, Label: "F14"},
			{Key: ebiten.KeyF15, Label: "F15"}, {Key: ebiten.KeyF16, Label: "F16"},
			{Key: ebiten.KeyF17, Label: "F17"}, {Key: ebiten.KeyF18, Label: "F18"},
			{Key: ebiten.KeyF19, Label: "F19"}, {Key: ebiten.KeyF20, Label: "F20"},
			{Key: ebiten.KeyF21, Label: "F21"}, {Key: ebiten.KeyF22, Label: "F22"},
			{Key: ebiten.KeyF23, Label: "F23"}, {Key: ebiten.KeyF24, Label: "F24"},
		},
	}
}

func keyboardTestMainLayout() [][]keyboardTestKeySpec {
	return [][]keyboardTestKeySpec{
		{
			{Key: ebiten.KeyBackquote, Label: "`"}, {Key: ebiten.KeyDigit1, Label: "1"},
			{Key: ebiten.KeyDigit2, Label: "2"}, {Key: ebiten.KeyDigit3, Label: "3"},
			{Key: ebiten.KeyDigit4, Label: "4"}, {Key: ebiten.KeyDigit5, Label: "5"},
			{Key: ebiten.KeyDigit6, Label: "6"}, {Key: ebiten.KeyDigit7, Label: "7"},
			{Key: ebiten.KeyDigit8, Label: "8"}, {Key: ebiten.KeyDigit9, Label: "9"},
			{Key: ebiten.KeyDigit0, Label: "0"}, {Key: ebiten.KeyMinus, Label: "-"},
			{Key: ebiten.KeyEqual, Label: "="}, {Key: ebiten.KeyBackspace, Label: "Back", Width: 56},
		},
		{
			{Key: ebiten.KeyTab, Label: "Tab", Width: 46},
			{Key: ebiten.KeyQ, Label: "Q"}, {Key: ebiten.KeyW, Label: "W"},
			{Key: ebiten.KeyE, Label: "E"}, {Key: ebiten.KeyR, Label: "R"},
			{Key: ebiten.KeyT, Label: "T"}, {Key: ebiten.KeyY, Label: "Y"},
			{Key: ebiten.KeyU, Label: "U"}, {Key: ebiten.KeyI, Label: "I"},
			{Key: ebiten.KeyO, Label: "O"}, {Key: ebiten.KeyP, Label: "P"},
			{Key: ebiten.KeyBracketLeft, Label: "["}, {Key: ebiten.KeyBracketRight, Label: "]"},
			{Key: ebiten.KeyBackslash, Label: "\\", Width: 46},
		},
		{
			{Key: ebiten.KeyCapsLock, Label: "Caps", Width: 56},
			{Key: ebiten.KeyA, Label: "A"}, {Key: ebiten.KeyS, Label: "S"},
			{Key: ebiten.KeyD, Label: "D"}, {Key: ebiten.KeyF, Label: "F"},
			{Key: ebiten.KeyG, Label: "G"}, {Key: ebiten.KeyH, Label: "H"},
			{Key: ebiten.KeyJ, Label: "J"}, {Key: ebiten.KeyK, Label: "K"},
			{Key: ebiten.KeyL, Label: "L"}, {Key: ebiten.KeySemicolon, Label: ";"},
			{Key: ebiten.KeyQuote, Label: "'"}, {Key: ebiten.KeyEnter, Label: "Enter", Width: 64},
		},
		{
			{Key: ebiten.KeyShiftLeft, Label: "Shift", Width: 68},
			{Key: ebiten.KeyZ, Label: "Z"}, {Key: ebiten.KeyX, Label: "X"},
			{Key: ebiten.KeyC, Label: "C"}, {Key: ebiten.KeyV, Label: "V"},
			{Key: ebiten.KeyB, Label: "B"}, {Key: ebiten.KeyN, Label: "N"},
			{Key: ebiten.KeyM, Label: "M"}, {Key: ebiten.KeyComma, Label: ","},
			{Key: ebiten.KeyPeriod, Label: "."}, {Key: ebiten.KeySlash, Label: "/"},
			{Key: ebiten.KeyShiftRight, Label: "Shift", Width: 84},
		},
		{
			{Key: ebiten.KeyControlLeft, Label: "Ctrl", Width: 46},
			{Key: ebiten.KeyMetaLeft, Label: "Meta", Width: 46},
			{Key: ebiten.KeyAltLeft, Label: "Alt", Width: 42},
			{Key: ebiten.KeySpace, Label: "Space", Width: 168},
			{Key: ebiten.KeyAltRight, Label: "Alt", Width: 42},
			{Key: ebiten.KeyMetaRight, Label: "Meta", Width: 46},
			{Key: ebiten.KeyControlRight, Label: "Ctrl", Width: 46},
		},
	}
}

func keyboardTestNavigationLayout() [][]keyboardTestKeySpec {
	return [][]keyboardTestKeySpec{
		{
			{Key: ebiten.KeyInsert, Label: "Ins", Width: 38},
			{Key: ebiten.KeyHome, Label: "Home", Width: 38},
			{Key: ebiten.KeyPageUp, Label: "PgUp", Width: 38},
		},
		{
			{Key: ebiten.KeyDelete, Label: "Del", Width: 38},
			{Key: ebiten.KeyEnd, Label: "End", Width: 38},
			{Key: ebiten.KeyPageDown, Label: "PgDn", Width: 38},
		},
		{{Blank: true, Width: 114}},
		{{Blank: true, Width: 38}, {Key: ebiten.KeyArrowUp, Label: "Up", Width: 38}, {Blank: true, Width: 38}},
		{
			{Key: ebiten.KeyArrowLeft, Label: "Left", Width: 38},
			{Key: ebiten.KeyArrowDown, Label: "Down", Width: 38},
			{Key: ebiten.KeyArrowRight, Label: "Right", Width: 38},
		},
	}
}

func keyboardTestNumpadLayout() [][]keyboardTestKeySpec {
	return [][]keyboardTestKeySpec{
		{
			{Key: ebiten.KeyNumLock, Label: "Num", Width: 36},
			{Key: ebiten.KeyNumpadDivide, Label: "/", Width: 36},
			{Key: ebiten.KeyNumpadMultiply, Label: "*", Width: 36},
			{Key: ebiten.KeyNumpadSubtract, Label: "-", Width: 36},
		},
		{
			{Key: ebiten.KeyNumpad7, Label: "7", Width: 36}, {Key: ebiten.KeyNumpad8, Label: "8", Width: 36},
			{Key: ebiten.KeyNumpad9, Label: "9", Width: 36}, {Key: ebiten.KeyNumpadAdd, Label: "+", Width: 36},
		},
		{
			{Key: ebiten.KeyNumpad4, Label: "4", Width: 36}, {Key: ebiten.KeyNumpad5, Label: "5", Width: 36},
			{Key: ebiten.KeyNumpad6, Label: "6", Width: 36}, {Blank: true, Width: 36},
		},
		{
			{Key: ebiten.KeyNumpad1, Label: "1", Width: 36}, {Key: ebiten.KeyNumpad2, Label: "2", Width: 36},
			{Key: ebiten.KeyNumpad3, Label: "3", Width: 36}, {Key: ebiten.KeyNumpadEnter, Label: "Enter", Width: 36},
		},
		{
			{Key: ebiten.KeyNumpad0, Label: "0", Width: 72},
			{Key: ebiten.KeyNumpadDecimal, Label: ".", Width: 36}, {Blank: true, Width: 36},
		},
	}
}
