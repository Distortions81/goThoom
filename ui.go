package main

import (
	"log"

	"github.com/Distortions81/EUI/eui"
	"github.com/hajimehoshi/ebiten/v2"
)

var loginWin *eui.WindowData
var remember bool
var charactersList *eui.ItemData
var accountWin *eui.WindowData

func initUI() {

	if !noSplash {
		loginWin = eui.NewWindow(&eui.WindowData{
			Title:     "Login",
			Open:      true,
			Closable:  false,
			Resizable: false,
			AutoSize:  true,
			Movable:   true,
			PinTo:     eui.PIN_MID_CENTER,
		})

		loginFlow := &eui.ItemData{
			ItemType: eui.ITEM_FLOW,
			FlowType: eui.FLOW_VERTICAL,
		}

		nameInput, _ := eui.NewInput(&eui.ItemData{Label: "Character", TextPtr: &name, Size: eui.Point{X: 200, Y: 24}})
		loginFlow.AddItem(nameInput)

		passInput, _ := eui.NewInput(&eui.ItemData{Label: "Password", TextPtr: &pass, Size: eui.Point{X: 200, Y: 24}})
		loginFlow.AddItem(passInput)

		rememberCB, rememberEvents := eui.NewCheckbox(&eui.ItemData{Text: "Remember", Size: eui.Point{X: 200, Y: 24}, Checked: remember})
		rememberEvents.Handle = func(ev eui.UIEvent) {
			if ev.Type == eui.EventCheckboxChanged {
				remember = ev.Checked
			}
		}
		loginFlow.AddItem(rememberCB)

		connBtn, connEvents := eui.NewButton(&eui.ItemData{Text: "Connect", Size: eui.Point{X: 200, Y: 48}, Padding: 10})
		connEvents.Handle = func(ev eui.UIEvent) {
			if ev.Type == eui.EventClick {
				if remember {
					rememberCharacter(name, pass)
					updateCharacterButtons()
				}
			}
		}
		loginFlow.AddItem(connBtn)

		manageBtn, manageEvents := eui.NewButton(&eui.ItemData{Text: "Manage Account", Size: eui.Point{X: 200, Y: 24}})
		manageEvents.Handle = func(ev eui.UIEvent) {
			if ev.Type == eui.EventClick {
				if accountWin != nil {
					accountWin.Open = true
				}
			}
		}
		loginFlow.AddItem(manageBtn)

		updateBtn, updateEvents := eui.NewButton(&eui.ItemData{Text: "Update Characters", Size: eui.Point{X: 200, Y: 24}})
		updateEvents.Handle = func(ev eui.UIEvent) {
			if ev.Type == eui.EventClick {
				updateCharacterButtons()
			}
		}
		loginFlow.AddItem(updateBtn)

		charactersList = &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL}
		loginFlow.AddItem(charactersList)
		updateCharacterButtons()

		loginWin.AddItem(loginFlow)
		loginWin.AddWindow(false)
		initAccountWindow()
	}

	settingsWin = eui.NewWindow(&eui.WindowData{
		Title:     "Settings",
		Size:      eui.Point{X: 256, Y: 256},
		Position:  eui.Point{X: 8, Y: 8},
		Open:      false,
		Closable:  false,
		Resizable: true,
		AutoSize:  true,
		Movable:   true,
	})

	mainFlow := &eui.ItemData{
		ItemType: eui.ITEM_FLOW,
		FlowType: eui.FLOW_VERTICAL,
	}
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

	textSlider, textEvents := eui.NewSlider(&eui.ItemData{Label: "Text Size", MinValue: 3, MaxValue: 24, Value: float32(mainFontSize), Size: eui.Point{X: width - 10, Y: 24}, IntOnly: true})
	textEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventSliderChanged {
			mainFontSize = float64(ev.Value)
			initFont()
			inputBg = nil
			settingsDirty = true
		}
	}
	mainFlow.AddItem(textSlider)

	bubbleTextSlider, bubbleTextEvents := eui.NewSlider(&eui.ItemData{Label: "Bubble Text Size", MinValue: 3, MaxValue: 24, Value: float32(bubbleFontSize), Size: eui.Point{X: width - 10, Y: 24}, IntOnly: true})
	bubbleTextEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventSliderChanged {
			bubbleFontSize = float64(ev.Value)
			initFont()
			settingsDirty = true
		}
	}
	mainFlow.AddItem(bubbleTextSlider)

	label, _ = eui.NewText(&eui.ItemData{Text: "\nGraphics Settings:", FontSize: 15, Size: eui.Point{X: 150, Y: 50}})
	mainFlow.AddItem(label)

	scaleSlider, scaleEvents := eui.NewSlider(&eui.ItemData{Label: "Scaling", MinValue: 2, MaxValue: 5, Value: float32(scale), Size: eui.Point{X: width, Y: 24}, IntOnly: true})
	scaleEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventSliderChanged {
			scale = int(ev.Value)
			initFont()
			inputBg = nil
			ebiten.SetWindowSize(gameAreaSizeX*scale, gameAreaSizeY*scale)
			settingsDirty = true
		}
	}
	mainFlow.AddItem(scaleSlider)

	filt, filtEvents := eui.NewCheckbox(&eui.ItemData{Text: "Image Filtering", Size: eui.Point{X: width, Y: 24}, Checked: linear})
	filtEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			linear = ev.Checked
			if linear {
				drawFilter = ebiten.FilterLinear
			} else {
				drawFilter = ebiten.FilterNearest
			}
			settingsDirty = true
		}
	}
	mainFlow.AddItem(filt)

	motion, motionEvents := eui.NewCheckbox(&eui.ItemData{Text: "Smooth Motion", Size: eui.Point{X: width, Y: 24}, Checked: interp})
	motionEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			interp = ev.Checked
			settingsDirty = true
		}
	}
	mainFlow.AddItem(motion)

	moveSmoothCB, moveSmoothEvents := eui.NewCheckbox(&eui.ItemData{Text: "Smooth Moving Objects", Size: eui.Point{X: width, Y: 24}, Checked: smoothMoving})
	moveSmoothEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			smoothMoving = ev.Checked
			settingsDirty = true
		}
	}
	mainFlow.AddItem(moveSmoothCB)

	anim, animEvents := eui.NewCheckbox(&eui.ItemData{Text: "Character Frame Blending", Size: eui.Point{X: width, Y: 24}, Checked: onion})
	animEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			onion = ev.Checked
			settingsDirty = true
		}
	}
	mainFlow.AddItem(anim)

	pictBlend, pictBlendEvents := eui.NewCheckbox(&eui.ItemData{Text: "Object Frame Blending", Size: eui.Point{X: width, Y: 24}, Checked: blendPicts})
	pictBlendEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			blendPicts = ev.Checked
			settingsDirty = true
		}
	}
	mainFlow.AddItem(pictBlend)

	blendSlider, blendEvents := eui.NewSlider(&eui.ItemData{Label: "Blend Rate", MinValue: 0.3, MaxValue: 1.0, Value: float32(blendRate), Size: eui.Point{X: width - 10, Y: 24}})
	blendEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventSliderChanged {
			blendRate = float64(ev.Value)
			settingsDirty = true
		}
	}
	mainFlow.AddItem(blendSlider)

	debugBtn, debugEvents := eui.NewButton(&eui.ItemData{Text: "Debug Settings", Size: eui.Point{X: width, Y: 24}})
	debugEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			debugWin.Open = !debugWin.Open
		}
	}
	mainFlow.AddItem(debugBtn)

	settingsWin.AddItem(mainFlow)
	settingsWin.AddWindow(false)
	settingsWin.Open = false

	debugWin = eui.NewWindow(&eui.WindowData{
		Title:     "Debug Settings",
		Size:      eui.Point{X: 256, Y: 256},
		Position:  eui.Point{X: 272, Y: 8},
		Open:      false,
		Closable:  false,
		Resizable: true,
		AutoSize:  true,
		Movable:   true,
	})

	debugFlow := &eui.ItemData{
		ItemType: eui.ITEM_FLOW,
		FlowType: eui.FLOW_VERTICAL,
	}

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
	debugWin.Open = false

	inventoryWin = eui.NewWindow(&eui.WindowData{
		Title:     "Inventory",
		Open:      false,
		Closable:  false,
		Resizable: false,
		AutoSize:  true,
		Movable:   true,
	})
	inventoryList = &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL}
	title, _ := eui.NewText(&eui.ItemData{Text: "Inventory", Size: eui.Point{X: 256, Y: 128}})
	inventoryWin.AddItem(title)
	inventoryWin.AddItem(inventoryList)
	inventoryWin.Open = false
	inventoryWin.AddWindow(false)

	playersWin = eui.NewWindow(&eui.WindowData{
		Title:     "Players",
		Open:      false,
		Closable:  false,
		Resizable: false,
		AutoSize:  false,
		Movable:   true,
		Size:      eui.Point{X: 128, Y: 384},
	})
	playersList = &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL}
	playersWin.AddItem(playersList)
	playersWin.Open = false
	playersWin.AddWindow(false)
	playersWin.Resizable = true
	playersWin.AutoSize = true

	helpWin = eui.NewWindow(&eui.WindowData{
		Title:     "Help",
		Open:      false,
		Closable:  false,
		Resizable: false,
		AutoSize:  true,
		Movable:   true,
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
	helpWin.Open = false
	helpWin.AddWindow(false)

	overlay := &eui.ItemData{
		ItemType: eui.ITEM_FLOW,
		FlowType: eui.FLOW_HORIZONTAL,
		PinTo:    eui.PIN_BOTTOM_RIGHT,
	}
	playersBtn, playersEvents := eui.NewButton(&eui.ItemData{Text: "Players", Size: eui.Point{X: 128, Y: 24}, FontSize: 18})
	playersEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			playersWin.Open = !playersWin.Open
			if playersWin.Open {
				updatePlayersWindow()
			}
		}
	}
	overlay.AddItem(playersBtn)

	invBtn, invEvents := eui.NewButton(&eui.ItemData{Text: "Inventory", Size: eui.Point{X: 128, Y: 24}, FontSize: 18})
	invEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			inventoryWin.Open = !inventoryWin.Open
			if inventoryWin.Open {
				updateInventoryWindow()
			}
		}
	}
	overlay.AddItem(invBtn)

	btn, btnEvents := eui.NewButton(&eui.ItemData{Text: "Settings", Size: eui.Point{X: 128, Y: 24}, FontSize: 18})
	btnEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			settingsWin.Open = !settingsWin.Open
		}
	}
	overlay.AddItem(btn)

	helpBtn, helpEvents := eui.NewButton(&eui.ItemData{Text: "Help", Size: eui.Point{X: 128, Y: 24}, FontSize: 18})
	helpEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			helpWin.Open = !helpWin.Open
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
	for _, c := range characters {
		row := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_HORIZONTAL}
		radio, radioEvents := eui.NewRadio(&eui.ItemData{
			Text:       c.Name,
			RadioGroup: "characters",
			Size:       eui.Point{X: 160, Y: 24},
			Checked:    name == c.Name,
		})
		nameCopy := c.Name
		radioEvents.Handle = func(ev eui.UIEvent) {
			if ev.Type == eui.EventRadioSelected {
				name = nameCopy
			}
		}
		row.AddItem(radio)

		trash, trashEvents := eui.NewButton(&eui.ItemData{Text: "X", Size: eui.Point{X: 24, Y: 24}, Color: eui.ColorDarkRed, HoverColor: eui.ColorRed})
		delName := c.Name
		trashEvents.Handle = func(ev eui.UIEvent) {
			if ev.Type == eui.EventClick {
				removeCharacter(delName)
				updateCharacterButtons()
				loginWin.Refresh()
			}
		}
		row.AddItem(trash)
		charactersList.AddItem(row)
	}
	loginWin.Refresh()
}

func initAccountWindow() {
	accountWin = eui.NewWindow(&eui.WindowData{
		Title:     "Account",
		Open:      false,
		Closable:  true,
		Resizable: false,
		AutoSize:  true,
		Movable:   true,
		PinTo:     eui.PIN_MID_CENTER,
	})

	flow := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL}
	acctInput, _ := eui.NewInput(&eui.ItemData{Label: "Account", TextPtr: &account, Size: eui.Point{X: 200, Y: 24}})
	flow.AddItem(acctInput)
	passInput, _ := eui.NewInput(&eui.ItemData{Label: "Password", TextPtr: &accountPass, Size: eui.Point{X: 200, Y: 24}})
	flow.AddItem(passInput)

	createBtn, createEvents := eui.NewButton(&eui.ItemData{Text: "Create", Size: eui.Point{X: 200, Y: 24}})
	createEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			log.Printf("create account %s", account)
		}
	}
	flow.AddItem(createBtn)

	deleteBtn, deleteEvents := eui.NewButton(&eui.ItemData{Text: "Delete", Size: eui.Point{X: 200, Y: 24}})
	deleteEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			log.Printf("delete account %s", account)
		}
	}
	flow.AddItem(deleteBtn)

	changeBtn, changeEvents := eui.NewButton(&eui.ItemData{Text: "Change Password", Size: eui.Point{X: 200, Y: 24}})
	changeEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			log.Printf("change password for %s", account)
		}
	}
	flow.AddItem(changeBtn)

	accountWin.AddItem(flow)
	accountWin.AddWindow(false)
}
