package main

import (
	"crypto/md5"
	"encoding/hex"
	"os"
	"path/filepath"

	"github.com/Distortions81/EUI/eui"
	"github.com/hajimehoshi/ebiten/v2"

	"go_client/climg"
)

var loginWin *eui.WindowData
var downloadWin *eui.WindowData
var charactersList *eui.ItemData
var addCharWin *eui.WindowData
var addCharName string
var addCharPass string
var addCharRemember bool
var chatFontSize = 12
var labelFontSize = 12

func initUI() {
	status, err := checkDataFiles(dataDir, clientVersion)
	if err != nil {
		logError("check data files: %v", err)
	}
	if status.NeedImages || status.NeedSounds {
		openDownloadsWindow(status)
	} else {
		openLoginWindow()
	}

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

func openDownloadsWindow(status dataFilesStatus) {
	if downloadWin != nil {
		return
	}

	downloadWin = eui.NewWindow(&eui.WindowData{
		Title:     "Downloads",
		Closable:  false,
		Resizable: false,
		AutoSize:  true,
		Movable:   false,
		PinTo:     eui.PIN_MID_CENTER,
		Open:      true,
	})

	startedDownload := false

	flow := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL}

	t, _ := eui.NewText(&eui.ItemData{Text: "Files we must download:", FontSize: 15, Size: eui.Point{X: 200, Y: 25}})
	flow.AddItem(t)

	for _, f := range status.Files {
		t, _ := eui.NewText(&eui.ItemData{Text: f, FontSize: 15, Size: eui.Point{X: 200, Y: 25}})
		flow.AddItem(t)
	}

	btnFlow := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_HORIZONTAL}
	dlBtn, dlEvents := eui.NewButton(&eui.ItemData{Text: "Download", Size: eui.Point{X: 100, Y: 24}})
	dlEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			if startedDownload {
				return
			}
			startedDownload = true
			go func() {
				if err := downloadDataFiles(dataDir, clientVersion, status); err != nil {
					logError("download data files: %v", err)
					openErrorWindow("Error: Download Data Files: " + err.Error())
					return
				}
				if imgs, err := climg.Load(filepath.Join(dataDir, "CL_Images")); err == nil {
					clImages = imgs
				} else {
					logError("load CL_Images: %v", err)
					openErrorWindow("Error: Load CL_Images: " + err.Error())
				}
				downloadWin.RemoveWindow()
				downloadWin = nil
				openLoginWindow()
			}()
		}
	}
	btnFlow.AddItem(dlBtn)

	closeBtn, closeEvents := eui.NewButton(&eui.ItemData{Text: "Close", Size: eui.Point{X: 100, Y: 24}})
	closeEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			os.Exit(0)
		}
	}
	btnFlow.AddItem(closeBtn)
	flow.AddItem(btnFlow)

	downloadWin.AddItem(flow)
	downloadWin.AddWindow(false)
}

func updateCharacterButtons() {
	if charactersList == nil {
		return
	}
	if name == "" {
		if lastCharacter != "" {
			for _, c := range characters {
				if c.Name == lastCharacter {
					name = c.Name
					passHash = c.PassHash
					break
				}
			}
		}
		if name == "" && len(characters) == 1 {
			name = characters[0].Name
			passHash = characters[0].PassHash
		}
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
					lastCharacter = nameCopy
					saveSettings()
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
		Movable:   false,
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
			lastCharacter = addCharName
			saveSettings()
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
	addCharWin.BringForward()
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

	/*
		manBtn, manBtnEvents := eui.NewButton(&eui.ItemData{Text: "Manage account", Size: eui.Point{X: 200, Y: 24}})
		manBtnEvents.Handle = func(ev eui.UIEvent) {
			if ev.Type == eui.EventClick {
				//Add manage account window here
			}
		}
		loginFlow.AddItem(manBtn)
	*/

	addBtn, addEvents := eui.NewButton(&eui.ItemData{Text: "Add Character", RadioGroup: "Characters", Size: eui.Point{X: 200, Y: 24}})
	addEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			addCharName = ""
			addCharPass = ""
			addCharRemember = false
			openAddCharacterWindow()
		}
	}
	loginFlow.AddItem(addBtn)

	loginFlow.AddItem(charactersList)
	updateCharacterButtons()

	label, _ := eui.NewText(&eui.ItemData{Text: "", FontSize: 15, Size: eui.Point{X: 1, Y: 25}})
	loginFlow.AddItem(label)

	connBtn, connEvents := eui.NewButton(&eui.ItemData{Text: "Connect", Size: eui.Point{X: 200, Y: 48}, Padding: 10})
	connEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			if name == "" {
				return
			}
			lastCharacter = name
			saveSettings()
			loginWin.RemoveWindow()
			loginWin = nil
			go func() {
				if err := login(gameCtx, clientVersion); err != nil {
					logError("login: %v", err)
					openErrorWindow("Error: Login: " + err.Error())
					openLoginWindow()
				}
			}()
		}
	}
	loginFlow.AddItem(connBtn)

	loginWin.AddItem(loginFlow)
	loginWin.AddWindow(false)
}

func openErrorWindow(msg string) {
	win := eui.NewWindow(&eui.WindowData{
		Title: "Error",

		Closable:  false,
		Resizable: false,
		AutoSize:  true,
		Movable:   false,
		PinTo:     eui.PIN_TOP_CENTER,
		Open:      true,
	})
	flow := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL}
	text, _ := eui.NewText(&eui.ItemData{Text: msg, FontSize: 8, Size: eui.Point{X: 500, Y: 25}})
	flow.AddItem(text)
	okBtn, okEvents := eui.NewButton(&eui.ItemData{Text: "OK", Size: eui.Point{X: 200, Y: 24}, PinTo: eui.PIN_BOTTOM_CENTER})
	okEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			win.RemoveWindow()
		}
	}
	flow.AddItem(okBtn)
	win.AddItem(flow)
	win.AddWindow(false)
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
