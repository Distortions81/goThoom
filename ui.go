package main

import (
	"context"
	"crypto/md5"
	"encoding/hex"

	"github.com/Distortions81/EUI/eui"
	"github.com/hajimehoshi/ebiten/v2"
)

var loginWin *eui.WindowData
var charactersList *eui.ItemData
var addCharWin *eui.WindowData
var connectingWin *eui.WindowData
var addCharName string
var addCharPass string
var addCharRemember bool
var loginCancel context.CancelFunc
var chatFontSize = 12
var labelFontSize = 12

func initUI() {
	openLoginWindow()

	overlay := &eui.ItemData{
		ItemType: eui.ITEM_FLOW,
		FlowType: eui.FLOW_HORIZONTAL,
		PinTo:    eui.PIN_BOTTOM_RIGHT,
	}
	playersBtn, playersEvents := eui.NewButton(&eui.ItemData{Text: "Players", Size: eui.Point{X: 128, Y: 24}, FontSize: 18})
	playersEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			if playersWin != nil {
				playersWin.RemoveWindow()
				playersWin = nil
			} else {
				openPlayersWindow()
			}
		}
	}
	overlay.AddItem(playersBtn)

	invBtn, invEvents := eui.NewButton(&eui.ItemData{Text: "Inventory", Size: eui.Point{X: 128, Y: 24}, FontSize: 18})
	invEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			if inventoryWin != nil {
				inventoryWin.RemoveWindow()
				inventoryWin = nil
			} else {
				openInventoryWindow()
			}
		}
	}
	overlay.AddItem(invBtn)

	btn, btnEvents := eui.NewButton(&eui.ItemData{Text: "Settings", Size: eui.Point{X: 128, Y: 24}, FontSize: 18})
	btnEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			if settingsWin != nil {
				settingsWin.RemoveWindow()
				settingsWin = nil
			} else {
				openSettingsWindow()
			}
		}
	}
	overlay.AddItem(btn)

	helpBtn, helpEvents := eui.NewButton(&eui.ItemData{Text: "Help", Size: eui.Point{X: 128, Y: 24}, FontSize: 18})
	helpEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			if helpWin != nil {
				helpWin.RemoveWindow()
				helpWin = nil
			} else {
				openHelpWindow()
			}
		}
	}
	overlay.AddItem(helpBtn)

	eui.AddOverlayFlow(overlay)
}

func updateCharacterButtons() {
	if charactersList == nil {
		return
	}
	charactersList.Contents = charactersList.Contents[:0]
	if len(characters) == 0 {
		empty, _ := eui.NewText(&eui.ItemData{Text: "empty", Size: eui.Point{X: 160, Y: 24}})
		charactersList.AddItem(empty)
		name = ""
		passHash = ""
	} else {
		for _, c := range characters {
			row := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_HORIZONTAL}
			radio, radioEvents := eui.NewRadio(&eui.ItemData{
				Text:       c.Name,
				RadioGroup: "characters",
				Size:       eui.Point{X: 160, Y: 24},
				Checked:    name == c.Name,
			})
			nameCopy := c.Name
			hashCopy := c.PassHash
			if name == c.Name {
				passHash = c.PassHash
			}
			radioEvents.Handle = func(ev eui.UIEvent) {
				if ev.Type == eui.EventRadioSelected {
					name = nameCopy
					passHash = hashCopy
				}
			}
			row.AddItem(radio)

			trash, trashEvents := eui.NewButton(&eui.ItemData{Text: "X", Size: eui.Point{X: 24, Y: 24}, Color: eui.ColorDarkRed, HoverColor: eui.ColorRed})
			delName := c.Name
			trashEvents.Handle = func(ev eui.UIEvent) {
				if ev.Type == eui.EventClick {
					removeCharacter(delName)
					if name == delName {
						name = ""
						passHash = ""
					}
					updateCharacterButtons()
					loginWin.Refresh()
				}
			}
			row.AddItem(trash)
			charactersList.AddItem(row)
		}
	}
	if loginWin != nil {
		loginWin.Refresh()
	}
}

func openAddCharacterWindow() {
	if addCharWin != nil {
		return
	}
	addCharWin = eui.NewWindow(&eui.WindowData{
		Title:     "Add Character",
		Closable:  false,
		Resizable: false,
		AutoSize:  true,
		Movable:   true,
		PinTo:     eui.PIN_MID_CENTER,
		Open:      true,
	})

	flow := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL}

	nameInput, _ := eui.NewInput(&eui.ItemData{Label: "Character", TextPtr: &addCharName, Size: eui.Point{X: 200, Y: 24}})
	flow.AddItem(nameInput)
	passInput, _ := eui.NewInput(&eui.ItemData{Label: "Password", TextPtr: &addCharPass, Size: eui.Point{X: 200, Y: 24}})
	flow.AddItem(passInput)
	rememberCB, rememberEvents := eui.NewCheckbox(&eui.ItemData{Text: "Remember", Size: eui.Point{X: 200, Y: 24}, Checked: addCharRemember})
	rememberEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			addCharRemember = ev.Checked
		}
	}
	flow.AddItem(rememberCB)
	addBtn, addEvents := eui.NewButton(&eui.ItemData{Text: "Add", Size: eui.Point{X: 200, Y: 24}})
	addEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			h := md5.Sum([]byte(addCharPass))
			hash := hex.EncodeToString(h[:])
			exists := false
			for i := range characters {
				if characters[i].Name == addCharName {
					characters[i].PassHash = hash
					exists = true
					break
				}
			}
			if !exists {
				characters = append(characters, Character{Name: addCharName, PassHash: hash})
			}
			if addCharRemember {
				saveCharacters()
			}
			name = addCharName
			passHash = hash
			updateCharacterButtons()
			if loginWin != nil {
				loginWin.Refresh()
			}
			addCharWin.RemoveWindow()
			addCharWin = nil
		}
	}
	flow.AddItem(addBtn)

	cancelBtn, cancelEvents := eui.NewButton(&eui.ItemData{Text: "Cancel", Size: eui.Point{X: 200, Y: 24}})
	cancelEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			addCharWin.RemoveWindow()
			addCharWin = nil
		}
	}
	flow.AddItem(cancelBtn)

	addCharWin.AddItem(flow)
	addCharWin.AddWindow(false)
}

func openConnectingWindow() {
	if connectingWin != nil {
		return
	}
	connectingWin = eui.NewWindow(&eui.WindowData{
		Title:     "Connecting...",
		Closable:  false,
		Resizable: false,
		AutoSize:  true,
		Movable:   false,
		PinTo:     eui.PIN_MID_CENTER,
		Open:      true,
	})
	flow := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL}
	cancelBtn, cancelEvents := eui.NewButton(&eui.ItemData{Text: "Cancel", Size: eui.Point{X: 200, Y: 24}})
	cancelEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			if loginCancel != nil {
				loginCancel()
			}
			connectingWin.RemoveWindow()
			connectingWin = nil
			openLoginWindow()
		}
	}
	flow.AddItem(cancelBtn)
	connectingWin.AddItem(flow)
	connectingWin.AddWindow(false)
}

func openLoginWindow() {
	if loginWin != nil {
		return
	}
	loginWin = eui.NewWindow(&eui.WindowData{
		Title:     "Login",
		Closable:  false,
		Resizable: false,
		AutoSize:  true,
		Movable:   false,
		PinTo:     eui.PIN_MID_CENTER,
		Open:      true,
	})

	loginFlow := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL}
	charactersList = &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL}
	loginFlow.AddItem(charactersList)
	updateCharacterButtons()

	connBtn, connEvents := eui.NewButton(&eui.ItemData{Text: "Connect", Size: eui.Point{X: 200, Y: 48}, Padding: 10})
	connEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			if name == "" {
				return
			}
			loginWin.RemoveWindow()
			loginWin = nil
			openConnectingWindow()
			var ctx context.Context
			ctx, loginCancel = context.WithCancel(gameCtx)
			go func() {
				login(ctx, clientVersion)
				if connectingWin != nil {
					connectingWin.RemoveWindow()
					connectingWin = nil
				}
			}()
		}
	}
	loginFlow.AddItem(connBtn)

	addBtn, addEvents := eui.NewButton(&eui.ItemData{Text: "Add Character", Size: eui.Point{X: 200, Y: 24}})
	addEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			addCharName = ""
			addCharPass = ""
			addCharRemember = false
			openAddCharacterWindow()
		}
	}
	loginFlow.AddItem(addBtn)

	loginWin.AddItem(loginFlow)
	loginWin.AddWindow(false)
}

func openSettingsWindow() {
	if settingsWin != nil {
		return
	}
	settingsWin = eui.NewWindow(&eui.WindowData{
		Title:     "Settings",
		Size:      eui.Point{X: 256, Y: 256},
		Position:  eui.Point{X: 8, Y: 8},
		Closable:  false,
		Resizable: true,
		AutoSize:  true,
		Movable:   true,
		Open:      true,
	})

	mainFlow := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL}
	var width float32 = 250

	label, _ := eui.NewText(&eui.ItemData{Text: "\nControls:", FontSize: 15, Size: eui.Point{X: 100, Y: 50}})
	mainFlow.AddItem(label)

	toggle, toggleEvents := eui.NewCheckbox(&eui.ItemData{Text: "Click-to-Toggle Walk", Size: eui.Point{X: width, Y: 24}, Checked: clickToToggle})
	toggleEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			clickToToggle = ev.Checked
			if !clickToToggle {
				walkToggled = false
			}
			settingsDirty = true
		}
	}
	mainFlow.AddItem(toggle)

	keySpeedSlider, keySpeedEvents := eui.NewSlider(&eui.ItemData{Label: "Keyboard Walk Speed", MinValue: 0.1, MaxValue: 1.0, Value: float32(keyWalkSpeed), Size: eui.Point{X: width - 10, Y: 24}})
	keySpeedEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventSliderChanged {
			keyWalkSpeed = float64(ev.Value)
			settingsDirty = true
		}
	}
	mainFlow.AddItem(keySpeedSlider)

	label, _ = eui.NewText(&eui.ItemData{Text: "\nText Sizes:", FontSize: 15, Size: eui.Point{X: 100, Y: 50}})
	mainFlow.AddItem(label)

	chatFontSlider, chatFontEvents := eui.NewSlider(&eui.ItemData{Label: "Chat", MinValue: 6, MaxValue: 24, IntOnly: true, Value: float32(chatFontSize), Size: eui.Point{X: width - 10, Y: 24}})
	chatFontEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventSliderChanged {
			chatFontSize = int(ev.Value)
			settingsDirty = true
		}
	}
	mainFlow.AddItem(chatFontSlider)

	labelFontSlider, labelFontEvents := eui.NewSlider(&eui.ItemData{Label: "Labels", MinValue: 6, MaxValue: 24, IntOnly: true, Value: float32(labelFontSize), Size: eui.Point{X: width - 10, Y: 24}})
	labelFontEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventSliderChanged {
			labelFontSize = int(ev.Value)
			settingsDirty = true
		}
	}
	mainFlow.AddItem(labelFontSlider)

	debugBtn, debugEvents := eui.NewButton(&eui.ItemData{Text: "Debug Settings", Size: eui.Point{X: width, Y: 24}})
	debugEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			if debugWin != nil {
				debugWin.RemoveWindow()
				debugWin = nil
			} else {
				openDebugWindow()
			}
		}
	}
	mainFlow.AddItem(debugBtn)

	settingsWin.AddItem(mainFlow)
	settingsWin.AddWindow(false)
}

func openDebugWindow() {
	if debugWin != nil {
		return
	}
	var width float32 = 250
	debugWin = eui.NewWindow(&eui.WindowData{
		Title:     "Debug Settings",
		Size:      eui.Point{X: 256, Y: 256},
		Position:  eui.Point{X: 272, Y: 8},
		Closable:  false,
		Resizable: true,
		AutoSize:  true,
		Movable:   true,
		Open:      true,
	})

	debugFlow := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL}

	vsyncCB, vsyncEvents := eui.NewCheckbox(&eui.ItemData{Text: "Vsync", Size: eui.Point{X: width, Y: 24}, Checked: vsync})
	vsyncEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			vsync = ev.Checked
			ebiten.SetVsyncEnabled(vsync)
			settingsDirty = true
		}
	}
	debugFlow.AddItem(vsyncCB)

	nightCB, nightEvents := eui.NewCheckbox(&eui.ItemData{Text: "Night Effects", Size: eui.Point{X: width, Y: 24}, Checked: nightMode})
	nightEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			nightMode = ev.Checked
			settingsDirty = true
		}
	}
	debugFlow.AddItem(nightCB)

	bubbleCB, bubbleEvents := eui.NewCheckbox(&eui.ItemData{Text: "Show Message Bubbles", Size: eui.Point{X: width, Y: 24}, Checked: showBubbles})
	bubbleEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			showBubbles = ev.Checked
			settingsDirty = true
		}
	}
	debugFlow.AddItem(bubbleCB)

	planesCB, planesEvents := eui.NewCheckbox(&eui.ItemData{Text: "Show image plane numbers", Size: eui.Point{X: width, Y: 24}, Checked: showPlanes})
	planesEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			showPlanes = ev.Checked
			settingsDirty = true
		}
	}
	debugFlow.AddItem(planesCB)

	hideMoveCB, hideMoveEvents := eui.NewCheckbox(&eui.ItemData{Text: "Hide Moving", Size: eui.Point{X: width, Y: 24}, Checked: hideMoving})
	hideMoveEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			hideMoving = ev.Checked
			settingsDirty = true
		}
	}
	debugFlow.AddItem(hideMoveCB)

	hideMobCB, hideMobEvents := eui.NewCheckbox(&eui.ItemData{Text: "Hide Mobiles", Size: eui.Point{X: width, Y: 24}, Checked: hideMobiles})
	hideMobEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			hideMobiles = ev.Checked
			settingsDirty = true
		}
	}
	debugFlow.AddItem(hideMobCB)

	debugWin.AddItem(debugFlow)
	debugWin.AddWindow(false)
}

func openInventoryWindow() {
	if inventoryWin != nil {
		return
	}
	inventoryWin = eui.NewWindow(&eui.WindowData{
		Title:     "Inventory",
		Closable:  false,
		Resizable: false,
		AutoSize:  true,
		Movable:   true,
		Open:      true,
	})
	inventoryList = &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL}
	title, _ := eui.NewText(&eui.ItemData{Text: "Inventory", Size: eui.Point{X: 256, Y: 128}})
	inventoryWin.AddItem(title)
	inventoryWin.AddItem(inventoryList)
	inventoryWin.AddWindow(false)
	updateInventoryWindow()
}

func openPlayersWindow() {
	if playersWin != nil {
		return
	}
	playersWin = eui.NewWindow(&eui.WindowData{
		Title:     "Players",
		Closable:  false,
		Resizable: false,
		AutoSize:  false,
		Movable:   true,
		Size:      eui.Point{X: 128, Y: 384},
		Open:      true,
	})
	playersList = &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL}
	playersWin.AddItem(playersList)
	playersWin.Resizable = true
	playersWin.AutoSize = true
	playersWin.AddWindow(false)
	updatePlayersWindow()
}

func openHelpWindow() {
	if helpWin != nil {
		return
	}
	helpWin = eui.NewWindow(&eui.WindowData{
		Title:     "Help",
		Closable:  false,
		Resizable: false,
		AutoSize:  true,
		Movable:   true,
		Open:      true,
	})
	helpFlow := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL}
	helpTexts := []string{
		"WASD or Arrow Keys - Walk",
		"Shift + Movement - Run",
		"Left Click - Walk toward cursor",
		"Click-to-Toggle Walk - Left click toggles walking",
		"Enter - Start typing / send command",
		"Escape - Cancel typing",
	}
	for _, line := range helpTexts {
		t, _ := eui.NewText(&eui.ItemData{Text: line, Size: eui.Point{X: 300, Y: 24}, FontSize: 15})
		helpFlow.AddItem(t)
	}
	helpWin.AddItem(helpFlow)
	helpWin.AddWindow(false)
}
