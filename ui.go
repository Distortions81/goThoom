package main

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/Distortions81/EUI/eui"
	"github.com/dustin/go-humanize"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/sqweek/dialog"

	"go_client/climg"
)

var pTopLeft = eui.Point{X: 0, Y: 0}
var pTopRight = eui.Point{X: 8000, Y: 0}
var pBottomLeft = eui.Point{X: 0, Y: 8000}
var pBottomRight = eui.Point{X: 8000, Y: 8000}

var loginWin *eui.WindowData
var downloadWin *eui.WindowData
var charactersList *eui.ItemData
var addCharWin *eui.WindowData
var addCharName string
var addCharPass string
var addCharRemember bool
var windowsWin *eui.WindowData
var playersBox *eui.ItemData
var inventoryBox *eui.ItemData
var messagesBox *eui.ItemData

var (
	sheetCacheLabel  *eui.ItemData
	frameCacheLabel  *eui.ItemData
	mobileCacheLabel *eui.ItemData
	soundCacheLabel  *eui.ItemData
	totalCacheLabel  *eui.ItemData
	soundTestLabel   *eui.ItemData
	soundTestID      int
	recordBtn        *eui.ItemData
	recordStatus     *eui.ItemData
)

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
		PinTo:    eui.PIN_TOP_CENTER,
	}
	winBtn, winEvents := eui.NewButton(&eui.ItemData{Text: "Windows", Size: eui.Point{X: 128, Y: 24}, FontSize: 18})
	winEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			if windowsWin != nil {
				windowsWin.RemoveWindow()
				windowsWin = nil
			} else {
				openWindowsWindow()
			}
		}
	}
	overlay.AddItem(winBtn)

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

	recordBtn, recordEvents := eui.NewButton(&eui.ItemData{Text: "Record Movie", Size: eui.Point{X: 128, Y: 24}, FontSize: 18})
	recordEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type != eui.EventClick {
			return
		}
		if recorder != nil {
			if err := recorder.Close(); err != nil {
				logError("close recorder: %v", err)
			}
			recorder = nil
			recordBtn.Text = "Record Movie"
			recordBtn.Dirty = true
			if recordStatus != nil {
				recordStatus.Text = ""
				recordStatus.Dirty = true
			}
			return
		}
		recDir := filepath.Join(baseDir, "recordings")
		if err := os.MkdirAll(recDir, 0755); err != nil {
			logError("create recordings dir: %v", err)
			openErrorWindow("Error: Record Movie: " + err.Error())
			return
		}
		name := gs.LastCharacter
		if playerName != "" {
			name = playerName
		}
		if name == "" {
			name = "recording"
		}
		defName := fmt.Sprintf("%s_%s.clMov", name, time.Now().Format("20060102_150405"))
		filename, err := dialog.File().Filter("clMov files", "clMov", "clmov").SetStartDir(recDir).SetStartFile(defName).Title("Record Movie").Save()
		if err != nil {
			if err != dialog.ErrCancelled {
				logError("record movie save: %v", err)
				openErrorWindow("Error: Record Movie: " + err.Error())
			}
			return
		}
		if filename == "" {
			return
		}
		rec, err := newMovieRecorder(filename, clientVersion, int(movieRevision))
		if err != nil {
			logError("start recorder: %v", err)
			openErrorWindow("Error: Record Movie: " + err.Error())
			return
		}
		recorder = rec
		recordBtn.Text = "Stop Recording"
		recordBtn.Dirty = true
		if recordStatus != nil {
			recordStatus.Text = "REC"
			recordStatus.Dirty = true
		}
	}
	overlay.AddItem(recordBtn)
	recordStatus, _ = eui.NewText(&eui.ItemData{Text: "", Size: eui.Point{X: 80, Y: 24}, FontSize: 18, Color: eui.ColorRed})
	overlay.AddItem(recordStatus)

	eui.AddOverlayFlow(overlay)

	openMessagesWindow()
	openChatWindow()
}

func openDownloadsWindow(status dataFilesStatus) {
	if downloadWin != nil {
		return
	}

	downloadWin = eui.NewWindow(&eui.WindowData{})
	downloadWin.Title = "Downloads"
	downloadWin.Closable = false
	downloadWin.Resizable = false
	downloadWin.AutoSize = true
	downloadWin.Movable = false
	downloadWin.PinTo = eui.PIN_BOTTOM_CENTER
	downloadWin.Open = true

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
					clImages.Denoise = gs.DenoiseImages
					clImages.DenoiseSharpness = gs.DenoiseSharpness
					clImages.DenoisePercent = gs.DenoisePercent
					clearCaches()
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
		if gs.LastCharacter != "" {
			for _, c := range characters {
				if c.Name == gs.LastCharacter {
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
		empty, _ := eui.NewText(&eui.ItemData{Text: "empty", Size: eui.Point{X: 160, Y: 64}})
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
					gs.LastCharacter = nameCopy
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
	addCharWin = eui.NewWindow(&eui.WindowData{})
	addCharWin.Title = "Add Character"
	addCharWin.Closable = false
	addCharWin.Resizable = false
	addCharWin.AutoSize = true
	addCharWin.Movable = false
	addCharWin.Open = true
	addCharWin.PinTo = eui.PIN_BOTTOM_CENTER

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
			gs.LastCharacter = addCharName
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
	if clmov != "" {
		return
	}

	loginWin = eui.NewWindow(&eui.WindowData{})
	loginWin.Title = "Login"
	loginWin.Closable = false
	loginWin.Resizable = false
	loginWin.AutoSize = true
	loginWin.Movable = false
	loginWin.PinTo = eui.PIN_MID_CENTER
	loginWin.Open = true

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

	openBtn, openEvents := eui.NewButton(&eui.ItemData{Text: "Open clMov", Size: eui.Point{X: 200, Y: 24}})
	openEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			filename, err := dialog.File().Filter("clMov files", "clMov", "clmov").Load()
			if err != nil {
				if err != dialog.Cancelled {
					logError("open clMov: %v", err)
					openErrorWindow("Error: Open clMov: " + err.Error())
				}
				return
			}
			if filename == "" {
				return
			}
			clmov = filename
			loginWin.RemoveWindow()
			loginWin = nil
			go func() {
				drawStateEncrypted = false
				frames, err := parseMovie(filename, clientVersion)
				if err != nil {
					logError("parse movie: %v", err)
					clmov = ""
					openErrorWindow("Error: Open clMov: " + err.Error())
					openLoginWindow()
					return
				}
				playerName = extractMoviePlayerName(frames)
				ctx, cancel := context.WithCancel(gameCtx)
				mp := newMoviePlayer(frames, clMovFPS, cancel)
				mp.initUI()
				if gs.precacheAssets && !assetsPrecached {
					for !assetsPrecached {
						time.Sleep(100 * time.Millisecond)
					}
				}
				go mp.run(ctx)
			}()
		}
	}
	loginFlow.AddItem(openBtn)

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
			gs.LastCharacter = name
			saveSettings()
			loginWin.RemoveWindow()
			loginWin = nil
			go func() {
				ctx, cancel := context.WithCancel(gameCtx)
				loginMu.Lock()
				loginCancel = cancel
				loginMu.Unlock()
				if err := login(ctx, clientVersion); err != nil {
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
	win := eui.NewWindow(&eui.WindowData{})
	win.Title = "Error"
	win.Closable = false
	win.Resizable = false
	win.AutoSize = true
	win.Movable = false
	win.PinTo = eui.PIN_MID_CENTER
	win.Open = true

	flow := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL}
	text, _ := eui.NewText(&eui.ItemData{Text: msg, FontSize: 8, Size: eui.Point{X: 500, Y: 25}})
	flow.AddItem(text)
	okBtn, okEvents := eui.NewButton(&eui.ItemData{Text: "OK", Size: eui.Point{X: 200, Y: 24}})
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
	settingsWin = eui.NewWindow(&eui.WindowData{})
	settingsWin.Title = "Settings"
	settingsWin.Closable = true
	settingsWin.Resizable = false
	settingsWin.AutoSize = true
	settingsWin.Movable = false
	settingsWin.Open = true
	settingsWin.PinTo = eui.PART_TOP_LEFT

	mainFlow := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL}
	var width float32 = 250

	label, _ := eui.NewText(&eui.ItemData{Text: "\nControls:", FontSize: 15, Size: eui.Point{X: 100, Y: 50}})
	mainFlow.AddItem(label)

	toggle, toggleEvents := eui.NewCheckbox(&eui.ItemData{Text: "Click-to-toggle movement", Size: eui.Point{X: width, Y: 24}, Checked: gs.ClickToToggle})
	toggleEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			gs.ClickToToggle = ev.Checked
			if !gs.ClickToToggle {
				walkToggled = false
			}
			settingsDirty = true
		}
	}
	mainFlow.AddItem(toggle)

	keySpeedSlider, keySpeedEvents := eui.NewSlider(&eui.ItemData{Label: "Keyboard Walk Speed", MinValue: 0.1, MaxValue: 1.0, Value: float32(gs.KBWalkSpeed), Size: eui.Point{X: width - 10, Y: 24}})
	keySpeedEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventSliderChanged {
			gs.KBWalkSpeed = float64(ev.Value)
			settingsDirty = true
		}
	}
	mainFlow.AddItem(keySpeedSlider)

	label, _ = eui.NewText(&eui.ItemData{Text: "\nText Sizes:", FontSize: 15, Size: eui.Point{X: 100, Y: 50}})
	mainFlow.AddItem(label)

	chatFontSlider, chatFontEvents := eui.NewSlider(&eui.ItemData{Label: "Chat", MinValue: 6, MaxValue: 24, Value: float32(gs.BubbleFontSize), Size: eui.Point{X: width - 10, Y: 24}})
	chatFontEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventSliderChanged {
			gs.BubbleFontSize = float64(ev.Value)
			initFont()
			settingsDirty = true
		}
	}
	mainFlow.AddItem(chatFontSlider)

	labelFontSlider, labelFontEvents := eui.NewSlider(&eui.ItemData{Label: "Labels", MinValue: 6, MaxValue: 24, Value: float32(gs.MainFontSize), Size: eui.Point{X: width - 10, Y: 24}})
	labelFontEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventSliderChanged {
			gs.MainFontSize = float64(ev.Value)
			initFont()
			settingsDirty = true
		}
	}
	mainFlow.AddItem(labelFontSlider)

	label, _ = eui.NewText(&eui.ItemData{Text: "\nOpacity Settings:", FontSize: 15, Size: eui.Point{X: 150, Y: 50}})
	mainFlow.AddItem(label)

	bubbleOpSlider, bubbleOpEvents := eui.NewSlider(&eui.ItemData{Label: "Message Bubble", MinValue: 0, MaxValue: 1, Value: float32(gs.BubbleOpacity), Size: eui.Point{X: width - 10, Y: 24}})
	bubbleOpEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventSliderChanged {
			gs.BubbleOpacity = float64(ev.Value)
			settingsDirty = true
		}
	}
	mainFlow.AddItem(bubbleOpSlider)

	nameBgSlider, nameBgEvents := eui.NewSlider(&eui.ItemData{Label: "Name Background", MinValue: 0, MaxValue: 1, Value: float32(gs.NameBgOpacity), Size: eui.Point{X: width - 10, Y: 24}})
	nameBgEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventSliderChanged {
			gs.NameBgOpacity = float64(ev.Value)
			settingsDirty = true
		}
	}
	mainFlow.AddItem(nameBgSlider)

	label, _ = eui.NewText(&eui.ItemData{Text: "\nGraphics Settings:", FontSize: 15, Size: eui.Point{X: 150, Y: 50}})
	mainFlow.AddItem(label)

	uiScaleSlider, uiScaleEvents := eui.NewSlider(&eui.ItemData{Label: "UI Scaling", MinValue: 0.5, MaxValue: 2.5, Value: float32(gs.UIScale), Size: eui.Point{X: width - 10, Y: 24}})
	uiScaleEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventSliderChanged {
			gs.UIScale = float64(ev.Value)
			eui.SetUIScale(float32(gs.UIScale))
			settingsDirty = true
		}
	}
	mainFlow.AddItem(uiScaleSlider)

	worldSizeCB, worldSizeEvents := eui.NewCheckbox(&eui.ItemData{Text: "Allow More World Sizes", Size: eui.Point{X: width, Y: 24}, Checked: gs.MoreWorldSizes})
	worldSizeEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			gs.MoreWorldSizes = ev.Checked
			settingsDirty = true
		}
	}
	mainFlow.AddItem(worldSizeCB)

	denoiseCB, denoiseEvents := eui.NewCheckbox(&eui.ItemData{Text: "Image Denoise", Size: eui.Point{X: width, Y: 24}, Checked: gs.DenoiseImages})
	denoiseEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			gs.DenoiseImages = ev.Checked
			if clImages != nil {
				clImages.Denoise = ev.Checked
			}
			clearCaches()
			settingsDirty = true
		}
	}
	mainFlow.AddItem(denoiseCB)

	denoiseSharpSlider, denoiseSharpEvents := eui.NewSlider(&eui.ItemData{Label: "Denoise Sharpness", MinValue: 0.1, MaxValue: 8, Value: float32(gs.DenoiseSharpness), Size: eui.Point{X: width - 10, Y: 24}})
	denoiseSharpEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventSliderChanged {
			gs.DenoiseSharpness = float64(ev.Value)
			if clImages != nil {
				clImages.DenoiseSharpness = gs.DenoiseSharpness
			}
			clearCaches()
			settingsDirty = true
		}
	}
	mainFlow.AddItem(denoiseSharpSlider)

	denoiseAmtSlider, denoiseAmtEvents := eui.NewSlider(&eui.ItemData{Label: "Denoise Amount", MinValue: 0.1, MaxValue: 0.5, Value: float32(gs.DenoisePercent), Size: eui.Point{X: width - 10, Y: 24}})
	denoiseAmtEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventSliderChanged {
			gs.DenoisePercent = float64(ev.Value)
			if clImages != nil {
				clImages.DenoisePercent = gs.DenoisePercent
			}
			clearCaches()
			settingsDirty = true
		}
	}
	mainFlow.AddItem(denoiseAmtSlider)

	motion, motionEvents := eui.NewCheckbox(&eui.ItemData{Text: "Smooth Motion", Size: eui.Point{X: width, Y: 24}, Checked: gs.MotionSmoothing})
	motionEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			gs.MotionSmoothing = ev.Checked
			settingsDirty = true
		}
	}
	mainFlow.AddItem(motion)

	//moveSmoothCB, moveSmoothEvents := eui.NewCheckbox(&eui.ItemData{Text: "Smooth Moving Objects", Size: eui.Point{X: width, Y: 24}, Checked: gs.SmoothMoving})
	//moveSmoothEvents.Handle = func(ev eui.UIEvent) {
	//	if ev.Type == eui.EventCheckboxChanged {
	//		gs.SmoothMoving = ev.Checked
	//		settingsDirty = true
	//	}
	//}
	//mainFlow.AddItem(moveSmoothCB)

	anim, animEvents := eui.NewCheckbox(&eui.ItemData{Text: "Mobile Animation Blending", Size: eui.Point{X: width, Y: 24}, Checked: gs.BlendMobiles})
	animEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			gs.BlendMobiles = ev.Checked
			settingsDirty = true
		}
	}
	mainFlow.AddItem(anim)

	pictBlend, pictBlendEvents := eui.NewCheckbox(&eui.ItemData{Text: "World Animation Blending", Size: eui.Point{X: width, Y: 24}, Checked: gs.BlendPicts})
	pictBlendEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			gs.BlendPicts = ev.Checked
			settingsDirty = true
		}
	}
	mainFlow.AddItem(pictBlend)

	mobileBlendSlider, mobileBlendEvents := eui.NewSlider(&eui.ItemData{Label: "Mobile Blend Amount", MinValue: 0.3, MaxValue: 1.0, Value: float32(gs.MobileBlendAmount), Size: eui.Point{X: width - 10, Y: 24}})
	mobileBlendEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventSliderChanged {
			gs.MobileBlendAmount = float64(ev.Value)
			settingsDirty = true
		}
	}
	mainFlow.AddItem(mobileBlendSlider)

	blendSlider, blendEvents := eui.NewSlider(&eui.ItemData{Label: "Picture Blend Amount", MinValue: 0.3, MaxValue: 1.0, Value: float32(gs.BlendAmount), Size: eui.Point{X: width - 10, Y: 24}})
	blendEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventSliderChanged {
			gs.BlendAmount = float64(ev.Value)
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
	debugWin = eui.NewWindow(&eui.WindowData{})
	debugWin.Title = "Debug Settings"
	debugWin.Closable = true
	debugWin.Resizable = false
	debugWin.AutoSize = true
	debugWin.Movable = false
	debugWin.Open = true
	debugWin.PinTo = eui.PIN_MID_CENTER

	debugFlow := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL}

	nightCB, nightEvents := eui.NewCheckbox(&eui.ItemData{Text: "Night Effect", Size: eui.Point{X: width, Y: 24}, Checked: gs.nightEffect})
	nightEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			gs.nightEffect = ev.Checked
			settingsDirty = true
		}
	}
	debugFlow.AddItem(nightCB)

	lateInputCB, lateInputEvents := eui.NewCheckbox(&eui.ItemData{Text: "Late Input Updates", Size: eui.Point{X: width, Y: 24}, Checked: gs.lateInputUpdates})
	lateInputEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			gs.lateInputUpdates = ev.Checked
			settingsDirty = true
		}
	}
	debugFlow.AddItem(lateInputCB)

	showFPSCB, showFPSEvents := eui.NewCheckbox(&eui.ItemData{Text: "Show FPS", Size: eui.Point{X: width, Y: 24}, Checked: gs.ShowFPS})
	showFPSEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			gs.ShowFPS = ev.Checked
			settingsDirty = true
		}
	}
	debugFlow.AddItem(showFPSCB)

	precacheCB, precacheEvents := eui.NewCheckbox(&eui.ItemData{Text: "Precache Sounds and Sprites", Size: eui.Point{X: width, Y: 24}, Checked: gs.precacheAssets})
	precacheEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			gs.precacheAssets = ev.Checked
			settingsDirty = true
		}
	}
	debugFlow.AddItem(precacheCB)

	filt, filtEvents := eui.NewCheckbox(&eui.ItemData{Text: "Image Filtering", Size: eui.Point{X: width, Y: 24}, Checked: gs.textureFiltering})
	filtEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			gs.textureFiltering = ev.Checked
			if gs.textureFiltering {
				drawFilter = ebiten.FilterLinear
			} else {
				drawFilter = ebiten.FilterNearest
			}
			settingsDirty = true
		}
	}
	debugFlow.AddItem(filt)

	fastSound, fastSoundEvents := eui.NewCheckbox(&eui.ItemData{Text: "Low Quality Sound", Size: eui.Point{X: width, Y: 24}, Checked: gs.fastSound})
	fastSoundEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			gs.fastSound = ev.Checked
			settingsDirty = true

			pcmCache = make(map[uint16][]byte)

			if gs.fastSound {
				resample = resampleFast
			} else {
				resample = resampleSincHQ
			}
		}
	}
	debugFlow.AddItem(fastSound)

	bubbleCB, bubbleEvents := eui.NewCheckbox(&eui.ItemData{Text: "Message Bubbles", Size: eui.Point{X: width, Y: 24}, Checked: gs.speechBubbles})
	bubbleEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			gs.speechBubbles = ev.Checked
			settingsDirty = true
		}
	}
	debugFlow.AddItem(bubbleCB)

	bubbleMsgCB, bubbleMsgEvents := eui.NewCheckbox(&eui.ItemData{Text: "Bubble Text in Messages", Size: eui.Point{X: width, Y: 24}, Checked: gs.bubbleMessages})
	bubbleMsgEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			gs.bubbleMessages = ev.Checked
			settingsDirty = true
		}
	}
	debugFlow.AddItem(bubbleMsgCB)

	hideMoveCB, hideMoveEvents := eui.NewCheckbox(&eui.ItemData{Text: "Hide Moving", Size: eui.Point{X: width, Y: 24}, Checked: gs.hideMoving})
	hideMoveEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			gs.hideMoving = ev.Checked
			settingsDirty = true
		}
	}
	debugFlow.AddItem(hideMoveCB)

	hideMobCB, hideMobEvents := eui.NewCheckbox(&eui.ItemData{Text: "Hide Mobiles", Size: eui.Point{X: width, Y: 24}, Checked: gs.hideMobiles})
	hideMobEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			gs.hideMobiles = ev.Checked
			settingsDirty = true
		}
	}
	debugFlow.AddItem(hideMobCB)

	planesCB, planesEvents := eui.NewCheckbox(&eui.ItemData{Text: "Show image planes", Size: eui.Point{X: width, Y: 24}, Checked: gs.imgPlanesDebug})
	planesEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			gs.imgPlanesDebug = ev.Checked
			settingsDirty = true
		}
	}
	debugFlow.AddItem(planesCB)

	vsyncCB, vsyncEvents := eui.NewCheckbox(&eui.ItemData{Text: "Vsync", Size: eui.Point{X: width, Y: 24}, Checked: gs.vsync})
	vsyncEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			gs.vsync = ev.Checked
			ebiten.SetVsyncEnabled(gs.vsync)
			settingsDirty = true
		}
	}
	debugFlow.AddItem(vsyncCB)

	smoothinCB, smoothinEvents := eui.NewCheckbox(&eui.ItemData{Text: "Smoothing Debug", Size: eui.Point{X: width, Y: 24}, Checked: gs.smoothingDebug})
	smoothinEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			gs.smoothingDebug = ev.Checked
			settingsDirty = true
		}
	}
	debugFlow.AddItem(smoothinCB)

	cacheLabel, _ := eui.NewText(&eui.ItemData{Text: "Caches:", Size: eui.Point{X: width, Y: 24}, FontSize: 10})
	debugFlow.AddItem(cacheLabel)

	sheetCacheLabel, _ = eui.NewText(&eui.ItemData{Text: "", Size: eui.Point{X: width, Y: 24}, FontSize: 10})
	debugFlow.AddItem(sheetCacheLabel)

	frameCacheLabel, _ = eui.NewText(&eui.ItemData{Text: "", Size: eui.Point{X: width, Y: 24}, FontSize: 10})
	debugFlow.AddItem(frameCacheLabel)

	mobileCacheLabel, _ = eui.NewText(&eui.ItemData{Text: "", Size: eui.Point{X: width, Y: 24}, FontSize: 10})
	debugFlow.AddItem(mobileCacheLabel)

	soundCacheLabel, _ = eui.NewText(&eui.ItemData{Text: "", Size: eui.Point{X: width, Y: 24}, FontSize: 10})
	debugFlow.AddItem(soundCacheLabel)

	clearCacheBtn, clearCacheEvents := eui.NewButton(&eui.ItemData{Text: "Clear All Caches", Size: eui.Point{X: width, Y: 24}})
	clearCacheEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			clearCaches()
			updateDebugStats()
		}
	}
	debugFlow.AddItem(clearCacheBtn)
	totalCacheLabel, _ = eui.NewText(&eui.ItemData{Text: "", Size: eui.Point{X: width, Y: 24}, FontSize: 10})
	debugFlow.AddItem(totalCacheLabel)

	debugWin.AddItem(debugFlow)

	soundTestFlow := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_HORIZONTAL}

	minusTenBtn, minusTenEvents := eui.NewButton(&eui.ItemData{Text: "--", Size: eui.Point{X: 24, Y: 24}})
	minusTenEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			soundTestID -= 10
			if soundTestID < 0 {
				soundTestID = 0
			}
			updateSoundTestLabel()
			playSound(uint16(soundTestID))
		}
	}
	soundTestFlow.AddItem(minusTenBtn)

	minusBtn, minusEvents := eui.NewButton(&eui.ItemData{Text: "-", Size: eui.Point{X: 24, Y: 24}})
	minusEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			soundTestID--
			if soundTestID < 0 {
				soundTestID = 0
			}
			updateSoundTestLabel()
			playSound(uint16(soundTestID))
		}
	}
	soundTestFlow.AddItem(minusBtn)

	soundTestLabel, _ = eui.NewText(&eui.ItemData{Text: "0", Size: eui.Point{X: 40, Y: 24}, FontSize: 10})
	soundTestFlow.AddItem(soundTestLabel)

	plusBtn, plusEvents := eui.NewButton(&eui.ItemData{Text: "+", Size: eui.Point{X: 24, Y: 24}})
	plusEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			soundTestID++
			updateSoundTestLabel()
			playSound(uint16(soundTestID))
		}
	}
	soundTestFlow.AddItem(plusBtn)

	plusTenBtn, plusTenEvents := eui.NewButton(&eui.ItemData{Text: "++", Size: eui.Point{X: 24, Y: 24}})
	plusTenEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			soundTestID += 10
			updateSoundTestLabel()
			playSound(uint16(soundTestID))
		}
	}
	soundTestFlow.AddItem(plusTenBtn)

	playBtn, playEvents := eui.NewButton(&eui.ItemData{Text: "Play", Size: eui.Point{X: 40, Y: 24}})
	playEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			playSound(uint16(soundTestID))
		}
	}
	soundTestFlow.AddItem(playBtn)

	debugFlow.AddItem(soundTestFlow)

	debugWin.AddWindow(false)
	updateSoundTestLabel()
	updateDebugStats()
}

// updateDebugStats refreshes the cache statistics displayed in the debug window.
func updateDebugStats() {
	if debugWin == nil {
		return
	}

	sheetCount, sheetBytes, frameCount, frameBytes, mobileCount, mobileBytes := imageCacheStats()
	soundCount, soundBytes := soundCacheStats()

	if sheetCacheLabel != nil {
		sheetCacheLabel.Text = fmt.Sprintf("Sprite Sheets: %d (%s)", sheetCount, humanize.Bytes(uint64(sheetBytes)))
		sheetCacheLabel.Dirty = true
	}
	if frameCacheLabel != nil {
		frameCacheLabel.Text = fmt.Sprintf("Animation Frames: %d (%s)", frameCount, humanize.Bytes(uint64(frameBytes)))
		frameCacheLabel.Dirty = true
	}
	if mobileCacheLabel != nil {
		mobileCacheLabel.Text = fmt.Sprintf("Mobile Animation Frames: %d (%s)", mobileCount, humanize.Bytes(uint64(mobileBytes)))
		mobileCacheLabel.Dirty = true
	}
	if soundCacheLabel != nil {
		soundCacheLabel.Text = fmt.Sprintf("Sounds: %d (%s)", soundCount, humanize.Bytes(uint64(soundBytes)))
		soundCacheLabel.Dirty = true
	}
	if totalCacheLabel != nil {
		totalCacheLabel.Text = fmt.Sprintf("Total: %s", humanize.Bytes(uint64(sheetBytes+frameBytes+mobileBytes+soundBytes)))
		totalCacheLabel.Dirty = true
	}
}

func updateSoundTestLabel() {
	if soundTestLabel != nil {
		soundTestLabel.Text = fmt.Sprintf("%d", soundTestID)
		soundTestLabel.Dirty = true
	}
}

func openWindowsWindow() {
	if windowsWin != nil {
		if windowsWin.Open {
			return
		}
		windowsWin = nil
	}
	windowsWin = eui.NewWindow(&eui.WindowData{})
	windowsWin.Title = "Windows"
	windowsWin.Closable = false
	windowsWin.Resizable = false
	windowsWin.AutoSize = true
	windowsWin.Movable = false
	windowsWin.Open = true
	windowsWin.PinTo = eui.PIN_TOP_CENTER

	flow := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL}

	playersBox, playersBoxEvents := eui.NewCheckbox(&eui.ItemData{Text: "Players", Size: eui.Point{X: 128, Y: 24}})
	playersBoxEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			if ev.Checked {
				openPlayersWindow()
			} else if playersWin != nil {
				playersWin.RemoveWindow()
				playersWin = nil
			}
			if playersBox != nil {
				playersBox.Dirty = true
			}
		}
	}
	flow.AddItem(playersBox)

	inventoryBox, inventoryBoxEvents := eui.NewCheckbox(&eui.ItemData{Text: "Inventory", Size: eui.Point{X: 128, Y: 24}, Checked: inventoryWin != nil})
	inventoryBoxEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			if ev.Checked {
				openInventoryWindow()
			} else if inventoryWin != nil {
				inventoryWin.RemoveWindow()
				inventoryWin = nil
			}
			if inventoryBox != nil {
				inventoryBox.Dirty = true
			}
		}
	}
	flow.AddItem(inventoryBox)

	messagesBox, messagesBoxEvents := eui.NewCheckbox(&eui.ItemData{Text: "Messages", Size: eui.Point{X: 128, Y: 24}, Checked: messagesWin != nil})
	messagesBoxEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			if ev.Checked {
				openMessagesWindow()
			} else if messagesWin != nil {
				messagesWin.RemoveWindow()
				messagesWin = nil
			}
			if messagesBox != nil {
				messagesBox.Dirty = true
			}
		}
	}
	flow.AddItem(messagesBox)

	windowsWin.AddItem(flow)
	windowsWin.AddWindow(false)
}

func openInventoryWindow() {
	if inventoryWin != nil {
		return
	}
	inventoryWin = eui.NewWindow(&eui.WindowData{})
	inventoryWin.Title = "Inventory"
	inventoryWin.Closable = false
	inventoryWin.Resizable = false
	inventoryWin.AutoSize = true
	inventoryWin.Movable = false
	inventoryWin.PinTo = eui.PIN_TOP_LEFT
	inventoryWin.Open = true

	inventoryList = &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL}
	title, _ := eui.NewText(&eui.ItemData{Text: "Inventory", Size: eui.Point{X: 256, Y: 128}})
	inventoryWin.AddItem(title)
	inventoryWin.AddItem(inventoryList)
	inventoryWin.AddWindow(false)
	inventoryWin.Position = eui.Point{X: 0, Y: 0}
	inventoryWin.Refresh()
	inventoryDirty.Store(true)
	if inventoryBox != nil {
		inventoryBox.Checked = true
		inventoryBox.Dirty = true
	}
}

func openPlayersWindow() {
	if playersWin != nil {
		if playersWin.Open {
			return
		}
	}
	playersWin = eui.NewWindow(&eui.WindowData{})
	playersWin.Title = "Players"
	playersWin.Size = eui.Point{X: 300, Y: 600}
	playersWin.Closable = false
	playersWin.Resizable = false
	playersWin.Movable = false
	playersWin.PinTo = eui.PIN_TOP_RIGHT
	playersWin.Open = true

	playersList = &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL}
	playersWin.AddItem(playersList)
	playersWin.AddWindow(false)
	playersWin.Refresh()
	playersDirty.Store(true)
}

func openHelpWindow() {
	if helpWin != nil {
		return
	}
	helpWin = eui.NewWindow(&eui.WindowData{})
	helpWin.Title = "Help"
	helpWin.Closable = true
	helpWin.Resizable = false
	helpWin.AutoSize = true
	helpWin.Movable = false
	helpWin.PinTo = eui.PIN_MID_CENTER
	helpWin.Open = true

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
