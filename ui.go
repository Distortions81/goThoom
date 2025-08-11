package main

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/Distortions81/EUI/eui"
	"github.com/dustin/go-humanize"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/sqweek/dialog"

	"go_client/climg"
	"go_client/clsnd"
)

const cval = 8000

var (
	TOP_RIGHT = eui.Point{X: cval, Y: 0}
	TOP_LEFT  = eui.Point{X: 0, Y: 0}

	BOTTOM_LEFT  = eui.Point{X: 0, Y: cval}
	BOTTOM_RIGHT = eui.Point{X: cval, Y: cval}
)

var loginWin *eui.WindowData
var downloadWin *eui.WindowData
var charactersList *eui.ItemData
var addCharWin *eui.WindowData
var addCharName string
var addCharPass string
var addCharRemember bool
var windowsWin *eui.WindowData
var toolbarWin *eui.WindowData

var (
	sheetCacheLabel  *eui.ItemData
	frameCacheLabel  *eui.ItemData
	mobileCacheLabel *eui.ItemData
	soundCacheLabel  *eui.ItemData
	mobileBlendLabel *eui.ItemData
	pictBlendLabel   *eui.ItemData
	totalCacheLabel  *eui.ItemData

	soundTestLabel *eui.ItemData
	soundTestID    int
	recordBtn      *eui.ItemData
	recordStatus   *eui.ItemData
)

func initUI() {
	status, err := checkDataFiles(clientVersion)
	if err != nil {
		logError("check data files: %v", err)
	}

	makeGameWindow()
	makeDownloadsWindow()
	makeLoginWindow()
	makeChatWindow()
	makeConsoleWindow()
	makeSettingsWindow()
	makeQualityWindow()
	makeDebugWindow()
	makeWindowsWindow()
	makeInventoryWindow()
	makePlayersWindow()
	makeHelpWindow()
	makeToolbarWindow()

	loginWin.Open()
	chatWin.Open()
	messagesWin.Open()
	inventoryWin.Open()
	playersWin.Open()

	if status.NeedImages || status.NeedSounds {
		downloadWin.Open()
	} else {
		loginWin.Open()
	}
}

func makeToolbarWindow() {
	toolbarWin = eui.NewWindow()
	toolbarWin.Closable = false
	toolbarWin.Resizable = false
	toolbarWin.AutoSize = false
	toolbarWin.NoScroll = true
	toolbarWin.ShowDragbar = false
	toolbarWin.Movable = true
	toolbarWin.Title = ""
	toolbarWin.SetTitleSize(4)
	xs, _ := eui.ScreenSize()
	tbs := eui.Point{X: 930, Y: 48}
	toolbarWin.Size = tbs
	toolbarWin.Position = eui.Point{X: float32(xs/2) - (tbs.X / 2), Y: 0}

	gameMenu := &eui.ItemData{
		ItemType: eui.ITEM_FLOW,
		FlowType: eui.FLOW_HORIZONTAL,
	}
	winBtn, winEvents := eui.NewButton()
	winBtn.Text = "Windows"
	winBtn.Size = eui.Point{X: 128, Y: 24}
	winBtn.FontSize = 18
	winEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			windowsWin.Toggle()
		}
	}
	gameMenu.AddItem(winBtn)

	btn, setEvents := eui.NewButton()
	btn.Text = "Settings"
	btn.Size = eui.Point{X: 128, Y: 24}
	btn.FontSize = 18
	setEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			settingsWin.Toggle()
		}
	}
	gameMenu.AddItem(btn)

	helpBtn, helpEvents := eui.NewButton()
	helpBtn.Text = "Help"
	helpBtn.Size = eui.Point{X: 128, Y: 24}
	helpBtn.FontSize = 18
	helpEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			helpWin.Toggle()
		}
	}
	gameMenu.AddItem(helpBtn)

	volumeSlider, volumeEvents := eui.NewSlider()
	volumeSlider.Label = ""
	volumeSlider.MinValue = 0
	volumeSlider.MaxValue = 1
	volumeSlider.Log = true
	volumeSlider.LogValue = 10
	volumeSlider.Value = float32(gs.Volume)
	volumeSlider.Size = eui.Point{X: 300, Y: 24}
	volumeSlider.FontSize = 9
	volumeEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventSliderChanged {
			gs.Volume = float64(ev.Value)
			settingsDirty = true
			updateSoundVolume()
		}
	}
	gameMenu.AddItem(volumeSlider)

	muteBtn, muteEvents := eui.NewButton()
	muteBtn.Text = "Mute"
	if gs.Mute {
		muteBtn.Text = "Unmute"
	}
	muteBtn.Size = eui.Point{X: 64, Y: 24}
	muteBtn.FontSize = 18
	muteEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			gs.Mute = !gs.Mute
			if gs.Mute {
				muteBtn.Text = "Unmute"
			} else {
				muteBtn.Text = "Mute"
			}
			muteBtn.Dirty = true
			settingsDirty = true
			updateSoundVolume()
		}
	}
	gameMenu.AddItem(muteBtn)

	recordBtn, recordEvents := eui.NewButton()
	recordBtn.Text = "Record"
	recordBtn.Size = eui.Point{X: 128, Y: 24}
	recordBtn.FontSize = 18
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
		recDir := filepath.Join("recordings")
		if err := os.MkdirAll(recDir, 0755); err != nil {
			logError("create recordings dir: %v", err)
			makeErrorWindow("Error: Record Movie: " + err.Error())
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
				makeErrorWindow("Error: Record Movie: " + err.Error())
			}
			return
		}
		if filename == "" {
			return
		}
		rec, err := newMovieRecorder(filename, clientVersion, int(movieRevision))
		if err != nil {
			logError("start recorder: %v", err)
			makeErrorWindow("Error: Record Movie: " + err.Error())
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
	gameMenu.AddItem(recordBtn)
	recordStatus, _ = eui.NewText()
	recordStatus.Text = ""
	recordStatus.Size = eui.Point{X: 80, Y: 24}
	recordStatus.FontSize = 18
	recordStatus.Color = eui.ColorRed
	gameMenu.AddItem(recordStatus)

	toolbarWin.AddItem(gameMenu)
	toolbarWin.AddWindow(false)
	toolbarWin.Open()
}

var dlMutex sync.Mutex

func makeDownloadsWindow() {
	var status dataFilesStatus
	if downloadWin != nil {
		return
	}
	downloadWin = eui.NewWindow()
	downloadWin.Title = "Downloads"
	downloadWin.Closable = false
	downloadWin.Resizable = false
	downloadWin.AutoSize = true
	downloadWin.Movable = true

	startedDownload := false

	flow := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL}

	t, _ := eui.NewText()
	t.Text = "Files we must download:"
	t.FontSize = 15
	t.Size = eui.Point{X: 200, Y: 25}
	flow.AddItem(t)

	for _, f := range status.Files {
		t, _ := eui.NewText()
		t.Text = f
		t.FontSize = 15
		t.Size = eui.Point{X: 200, Y: 25}
		flow.AddItem(t)
	}

	btnFlow := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_HORIZONTAL}
	dlBtn, dlEvents := eui.NewButton()
	dlBtn.Text = "Download"
	dlBtn.Size = eui.Point{X: 100, Y: 24}
	dlEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			if startedDownload {
				return
			}
			startedDownload = true
			go func() {
				dlMutex.Lock()
				defer dlMutex.Unlock()

				if err := downloadDataFiles(clientVersion, status); err != nil {
					logError("download data files: %v", err)
					makeErrorWindow("Error: Download Data Files: " + err.Error())
					return
				}
				clImages, err := climg.Load(filepath.Join(dataDirPath, CL_ImagesFile))
				if err != nil {
					logError("failed to load CL_Images: %v", err)
					return
				} else {
					clImages.Denoise = gs.DenoiseImages
					clImages.DenoiseSharpness = gs.DenoiseSharpness
					clImages.DenoisePercent = gs.DenoisePercent
				}

				clSounds, err = clsnd.Load(filepath.Join("data/CL_Sounds"))
				if err != nil {
					logError("failed to load CL_Sounds: %v", err)
					return
				}
				downloadWin.Close()
				makeLoginWindow()
			}()
		}
	}
	btnFlow.AddItem(dlBtn)

	closeBtn, closeEvents := eui.NewButton()
	closeBtn.Text = "Quit"
	closeBtn.Size = eui.Point{X: 100, Y: 24}
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
	if loginWin == nil || !loginWin.IsOpen() {
		return
	}
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
	for i := range charactersList.Contents {
		charactersList.Contents[i] = nil
	}
	charactersList.Contents = charactersList.Contents[:0]
	if len(characters) == 0 {
		empty, _ := eui.NewText()
		empty.Text = "empty"
		empty.Size = eui.Point{X: 160, Y: 64}
		charactersList.AddItem(empty)
		name = ""
		passHash = ""
	} else {
		for _, c := range characters {
			row := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_HORIZONTAL}
			radio, radioEvents := eui.NewRadio()
			radio.Text = c.Name
			radio.RadioGroup = "characters"
			radio.Size = eui.Point{X: 160, Y: 24}
			radio.Checked = name == c.Name
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

			trash, trashEvents := eui.NewButton()
			trash.Text = "X"
			trash.Size = eui.Point{X: 24, Y: 24}
			trash.Color = eui.ColorDarkRed
			trash.HoverColor = eui.ColorRed
			delName := c.Name
			trashEvents.Handle = func(ev eui.UIEvent) {
				if ev.Type == eui.EventClick {
					removeCharacter(delName)
					if name == delName {
						name = ""
						passHash = ""
					}
					updateCharacterButtons()
					//loginWin.Refresh()
				}
			}
			row.AddItem(trash)
			charactersList.AddItem(row)
		}
	}
	//loginWin.Refresh()
}

func makeAddCharacterWindow() {
	if addCharWin != nil {
		return
	}
	addCharWin = eui.NewWindow()
	addCharWin.Title = "Add Character"
	addCharWin.Closable = false
	addCharWin.Resizable = false
	addCharWin.AutoSize = true
	addCharWin.Movable = true

	flow := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL}

	nameInput, _ := eui.NewInput()
	nameInput.Label = "Character"
	nameInput.TextPtr = &addCharName
	nameInput.Size = eui.Point{X: 200, Y: 24}
	flow.AddItem(nameInput)
	passInput, _ := eui.NewInput()
	passInput.Label = "Password"
	passInput.TextPtr = &addCharPass
	passInput.Size = eui.Point{X: 200, Y: 24}
	flow.AddItem(passInput)
	rememberCB, rememberEvents := eui.NewCheckbox()
	rememberCB.Text = "Remember"
	rememberCB.Size = eui.Point{X: 200, Y: 24}
	rememberCB.Checked = addCharRemember
	rememberEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			addCharRemember = ev.Checked
		}
	}
	flow.AddItem(rememberCB)
	addBtn, addEvents := eui.NewButton()
	addBtn.Text = "Add"
	addBtn.Size = eui.Point{X: 200, Y: 24}
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
			if loginWin != nil && loginWin.IsOpen() {
				loginWin.Refresh()
			}
			addCharWin.Open()
		}
	}
	flow.AddItem(addBtn)

	cancelBtn, cancelEvents := eui.NewButton()
	cancelBtn.Text = "Cancel"
	cancelBtn.Size = eui.Point{X: 200, Y: 24}
	cancelEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			addCharWin.Open()
		}
	}
	flow.AddItem(cancelBtn)

	addCharWin.AddItem(flow)
	addCharWin.AddWindow(false)
}

func makeLoginWindow() {
	if loginWin != nil {
		return
	}

	loginWin = eui.NewWindow()
	loginWin.Title = "Login"
	loginWin.Closable = false
	loginWin.Resizable = false
	loginWin.AutoSize = true
	loginWin.Movable = true
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

	addBtn, addEvents := eui.NewButton()
	addBtn.Text = "Add Character"
	addBtn.RadioGroup = "Characters"
	addBtn.Size = eui.Point{X: 200, Y: 24}
	addEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			addCharName = ""
			addCharPass = ""
			addCharRemember = false
			makeAddCharacterWindow()
		}
	}
	loginFlow.AddItem(addBtn)

	openBtn, openEvents := eui.NewButton()
	openBtn.Text = "Open clMov"
	openBtn.Size = eui.Point{X: 200, Y: 24}
	openEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			filename, err := dialog.File().Filter("clMov files", "clMov", "clmov").Load()
			if err != nil {
				if err != dialog.Cancelled {
					logError("open clMov: %v", err)
					makeErrorWindow("Error: Open clMov: " + err.Error())
				}
				return
			}
			if filename == "" {
				return
			}
			clmov = filename
			loginWin.Close()
			go func() {
				drawStateEncrypted = false
				frames, err := parseMovie(filename, clientVersion)
				if err != nil {
					logError("parse movie: %v", err)
					clmov = ""
					makeErrorWindow("Error: Open clMov: " + err.Error())
					makeLoginWindow()
					return
				}
				playerName = extractMoviePlayerName(frames)
				ctx, cancel := context.WithCancel(gameCtx)
				mp := newMoviePlayer(frames, clMovFPS, cancel)
				mp.makePlaybackWindow()
				if (gs.precacheSounds || gs.precacheImages) && !assetsPrecached {
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

	label, _ := eui.NewText()
	label.Text = ""
	label.FontSize = 15
	label.Size = eui.Point{X: 1, Y: 25}
	loginFlow.AddItem(label)

	connBtn, connEvents := eui.NewButton()
	connBtn.Text = "Connect"
	connBtn.Size = eui.Point{X: 200, Y: 48}
	connBtn.Padding = 10
	connEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			if name == "" {
				return
			}
			gs.LastCharacter = name
			saveSettings()
			loginWin.Close()
			go func() {
				ctx, cancel := context.WithCancel(gameCtx)
				loginMu.Lock()
				loginCancel = cancel
				loginMu.Unlock()
				if err := login(ctx, clientVersion); err != nil {
					logError("login: %v", err)
					makeErrorWindow("Error: Login: " + err.Error())
					makeLoginWindow()
				}
			}()
		}
	}
	loginFlow.AddItem(connBtn)

	loginWin.AddItem(loginFlow)
	loginWin.AddWindow(false)
}

func makeErrorWindow(msg string) {
	win := eui.NewWindow()
	win.Title = "Error"
	win.Closable = false
	win.Resizable = false
	win.AutoSize = true
	win.Movable = true

	flow := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL}
	text, _ := eui.NewText()
	text.Text = msg
	text.FontSize = 8
	text.Size = eui.Point{X: 500, Y: 25}
	flow.AddItem(text)
	okBtn, okEvents := eui.NewButton()
	okBtn.Text = "OK"
	okBtn.Size = eui.Point{X: 200, Y: 24}
	okEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			win.Close()
		}
	}
	flow.AddItem(okBtn)
	win.AddItem(flow)
	win.AddWindow(false)
	win.Open()
}

func makeSettingsWindow() {
	if settingsWin != nil {
		return
	}
	settingsWin = eui.NewWindow()
	settingsWin.Title = "Settings"
	settingsWin.Closable = true
	settingsWin.Resizable = false
	settingsWin.AutoSize = true
	settingsWin.Movable = true

	mainFlow := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL}
	var width float32 = 250

	label, _ := eui.NewText()
	label.Text = "\nControls:"
	label.FontSize = 15
	label.Size = eui.Point{X: 100, Y: 50}
	mainFlow.AddItem(label)

	toggle, toggleEvents := eui.NewCheckbox()
	toggle.Text = "Click-to-toggle movement"
	toggle.Size = eui.Point{X: width, Y: 24}
	toggle.Checked = gs.ClickToToggle
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

	keySpeedSlider, keySpeedEvents := eui.NewSlider()
	keySpeedSlider.Label = "Keyboard Walk Speed"
	keySpeedSlider.MinValue = 0.1
	keySpeedSlider.MaxValue = 1.0
	keySpeedSlider.Value = float32(gs.KBWalkSpeed)
	keySpeedSlider.Size = eui.Point{X: width - 10, Y: 24}
	keySpeedEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventSliderChanged {
			gs.KBWalkSpeed = float64(ev.Value)
			settingsDirty = true
		}
	}
	mainFlow.AddItem(keySpeedSlider)

	label, _ = eui.NewText()
	label.Text = "\nText Sizes:"
	label.FontSize = 15
	label.Size = eui.Point{X: 100, Y: 50}
	mainFlow.AddItem(label)

	chatFontSlider, chatFontEvents := eui.NewSlider()
	chatFontSlider.Label = "Chat"
	chatFontSlider.MinValue = 6
	chatFontSlider.MaxValue = 24
	chatFontSlider.Value = float32(gs.BubbleFontSize)
	chatFontSlider.Size = eui.Point{X: width - 10, Y: 24}
	chatFontEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventSliderChanged {
			gs.BubbleFontSize = float64(ev.Value)
			initFont()
			settingsDirty = true
		}
	}
	mainFlow.AddItem(chatFontSlider)

	labelFontSlider, labelFontEvents := eui.NewSlider()
	labelFontSlider.Label = "Labels"
	labelFontSlider.MinValue = 6
	labelFontSlider.MaxValue = 24
	labelFontSlider.Value = float32(gs.MainFontSize)
	labelFontSlider.Size = eui.Point{X: width - 10, Y: 24}
	labelFontEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventSliderChanged {
			gs.MainFontSize = float64(ev.Value)
			initFont()
			settingsDirty = true
		}
	}
	mainFlow.AddItem(labelFontSlider)

	label, _ = eui.NewText()
	label.Text = "\nOpacity Settings:"
	label.FontSize = 15
	label.Size = eui.Point{X: 150, Y: 50}
	mainFlow.AddItem(label)

	bubbleOpSlider, bubbleOpEvents := eui.NewSlider()
	bubbleOpSlider.Label = "Message Bubble"
	bubbleOpSlider.MinValue = 0
	bubbleOpSlider.MaxValue = 1
	bubbleOpSlider.Value = float32(gs.BubbleOpacity)
	bubbleOpSlider.Size = eui.Point{X: width - 10, Y: 24}
	bubbleOpEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventSliderChanged {
			gs.BubbleOpacity = float64(ev.Value)
			settingsDirty = true
		}
	}
	mainFlow.AddItem(bubbleOpSlider)

	nameBgSlider, nameBgEvents := eui.NewSlider()
	nameBgSlider.Label = "Name Background"
	nameBgSlider.MinValue = 0
	nameBgSlider.MaxValue = 1
	nameBgSlider.Value = float32(gs.NameBgOpacity)
	nameBgSlider.Size = eui.Point{X: width - 10, Y: 24}
	nameBgEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventSliderChanged {
			gs.NameBgOpacity = float64(ev.Value)
			settingsDirty = true
		}
	}
	mainFlow.AddItem(nameBgSlider)

	label, _ = eui.NewText()
	label.Text = "\nGraphics Settings:"
	label.FontSize = 15
	label.Size = eui.Point{X: 150, Y: 50}
	mainFlow.AddItem(label)

	uiScaleSlider, uiScaleEvents := eui.NewSlider()
	uiScaleSlider.Label = "UI Scaling"
	uiScaleSlider.MinValue = 0.5
	uiScaleSlider.MaxValue = 2.5
	uiScaleSlider.Value = float32(gs.UIScale)
	uiScaleSlider.Size = eui.Point{X: width - 10, Y: 24}
	uiScaleEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventSliderChanged {
			gs.UIScale = float64(ev.Value)
			eui.SetUIScale(float32(gs.UIScale))
			settingsDirty = true
		}
	}
	mainFlow.AddItem(uiScaleSlider)

	fullscreenCB, fullscreenEvents := eui.NewCheckbox()
	fullscreenCB.Text = "Fullscreen"
	fullscreenCB.Size = eui.Point{X: width, Y: 24}
	fullscreenCB.Checked = gs.Fullscreen
	fullscreenEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			gs.Fullscreen = ev.Checked
			ebiten.SetFullscreen(gs.Fullscreen)
			settingsDirty = true
		}
	}
	mainFlow.AddItem(fullscreenCB)

	qualityBtn, qualityEvents := eui.NewButton()
	qualityBtn.Text = "Quality Options"
	qualityBtn.Size = eui.Point{X: width, Y: 24}
	qualityEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			qualityWin.Toggle()
		}
	}
	mainFlow.AddItem(qualityBtn)

	debugBtn, debugEvents := eui.NewButton()
	debugBtn.Text = "Debug Settings"
	debugBtn.Size = eui.Point{X: width, Y: 24}
	debugEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			debugWin.Toggle()
		}
	}
	mainFlow.AddItem(debugBtn)

	settingsWin.AddItem(mainFlow)
	settingsWin.AddWindow(false)
}

func makeQualityWindow() {
	if qualityWin != nil {
		return
	}
	var width float32 = 250
	qualityWin = eui.NewWindow()
	qualityWin.Title = "Quality Options"
	qualityWin.Closable = true
	qualityWin.Resizable = false
	qualityWin.AutoSize = true
	qualityWin.Movable = true

	flow := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL}

	denoiseCB, denoiseEvents := eui.NewCheckbox()
	denoiseCB.Text = "Image Denoise"
	denoiseCB.Size = eui.Point{X: width, Y: 24}
	denoiseCB.Checked = gs.DenoiseImages
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
	flow.AddItem(denoiseCB)

	denoiseSharpSlider, denoiseSharpEvents := eui.NewSlider()
	denoiseSharpSlider.Label = "Denoise Sharpness"
	denoiseSharpSlider.MinValue = 0.1
	denoiseSharpSlider.MaxValue = 8
	denoiseSharpSlider.Value = float32(gs.DenoiseSharpness)
	denoiseSharpSlider.Size = eui.Point{X: width - 10, Y: 24}
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
	flow.AddItem(denoiseSharpSlider)

	denoiseAmtSlider, denoiseAmtEvents := eui.NewSlider()
	denoiseAmtSlider.Label = "Denoise Amount"
	denoiseAmtSlider.MinValue = 0.1
	denoiseAmtSlider.MaxValue = 0.5
	denoiseAmtSlider.Value = float32(gs.DenoisePercent)
	denoiseAmtSlider.Size = eui.Point{X: width - 10, Y: 24}
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
	flow.AddItem(denoiseAmtSlider)

	motion, motionEvents := eui.NewCheckbox()
	motion.Text = "Smooth Motion"
	motion.Size = eui.Point{X: width, Y: 24}
	motion.Checked = gs.MotionSmoothing
	motionEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			gs.MotionSmoothing = ev.Checked
			settingsDirty = true
		}
	}
	flow.AddItem(motion)

	anim, animEvents := eui.NewCheckbox()
	anim.Text = "Mobile Animation Blending"
	anim.Size = eui.Point{X: width, Y: 24}
	anim.Checked = gs.BlendMobiles
	animEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			gs.BlendMobiles = ev.Checked
			settingsDirty = true
		}
	}
	flow.AddItem(anim)

	pictBlend, pictBlendEvents := eui.NewCheckbox()
	pictBlend.Text = "World Animation Blending"
	pictBlend.Size = eui.Point{X: width, Y: 24}
	pictBlend.Checked = gs.BlendPicts
	pictBlendEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			gs.BlendPicts = ev.Checked
			settingsDirty = true
		}
	}
	flow.AddItem(pictBlend)

	mobileBlendSlider, mobileBlendEvents := eui.NewSlider()
	mobileBlendSlider.Label = "Mobile Blend Amount"
	mobileBlendSlider.MinValue = 0.3
	mobileBlendSlider.MaxValue = 1.0
	mobileBlendSlider.Value = float32(gs.MobileBlendAmount)
	mobileBlendSlider.Size = eui.Point{X: width - 10, Y: 24}
	mobileBlendEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventSliderChanged {
			gs.MobileBlendAmount = float64(ev.Value)
			settingsDirty = true
		}
	}
	flow.AddItem(mobileBlendSlider)

	blendSlider, blendEvents := eui.NewSlider()
	blendSlider.Label = "Picture Blend Amount"
	blendSlider.MinValue = 0.3
	blendSlider.MaxValue = 1.0
	blendSlider.Value = float32(gs.BlendAmount)
	blendSlider.Size = eui.Point{X: width - 10, Y: 24}
	blendEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventSliderChanged {
			gs.BlendAmount = float64(ev.Value)
			settingsDirty = true
		}
	}
	flow.AddItem(blendSlider)

	mobileFramesSlider, mobileFramesEvents := eui.NewSlider()
	mobileFramesSlider.Label = "Mobile Blend Frames"
	mobileFramesSlider.MinValue = 3
	mobileFramesSlider.MaxValue = 30
	mobileFramesSlider.Value = float32(gs.MobileBlendFrames)
	mobileFramesSlider.Size = eui.Point{X: width - 10, Y: 24}
	mobileFramesSlider.IntOnly = true
	mobileFramesEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventSliderChanged {
			gs.MobileBlendFrames = int(ev.Value)
			settingsDirty = true
		}
	}
	flow.AddItem(mobileFramesSlider)

	pictFramesSlider, pictFramesEvents := eui.NewSlider()
	pictFramesSlider.Label = "Picture Blend Frames"
	pictFramesSlider.MinValue = 3
	pictFramesSlider.MaxValue = 30
	pictFramesSlider.Value = float32(gs.PictBlendFrames)
	pictFramesSlider.Size = eui.Point{X: width - 10, Y: 24}
	pictFramesSlider.IntOnly = true
	pictFramesEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventSliderChanged {
			gs.PictBlendFrames = int(ev.Value)
			settingsDirty = true
		}
	}
	flow.AddItem(pictFramesSlider)

	showFPSCB, showFPSEvents := eui.NewCheckbox()
	showFPSCB.Text = "Show FPS / UPS"
	showFPSCB.Size = eui.Point{X: width, Y: 24}
	showFPSCB.Checked = gs.ShowFPS
	showFPSEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			gs.ShowFPS = ev.Checked
			settingsDirty = true
		}
	}
	flow.AddItem(showFPSCB)

	precacheSoundCB, precacheSoundEvents := eui.NewCheckbox()
	precacheSoundCB.Text = "Precache Sounds"
	precacheSoundCB.Size = eui.Point{X: width, Y: 24}
	precacheSoundCB.Checked = gs.precacheSounds
	precacheSoundEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			gs.precacheSounds = ev.Checked
			settingsDirty = true
		}
	}
	flow.AddItem(precacheSoundCB)

	precacheImageCB, precacheImageEvents := eui.NewCheckbox()
	precacheImageCB.Text = "Precache Images"
	precacheImageCB.Size = eui.Point{X: width, Y: 24}
	precacheImageCB.Checked = gs.precacheImages
	precacheImageEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			gs.precacheImages = ev.Checked
			settingsDirty = true
		}
	}
	flow.AddItem(precacheImageCB)

	filt, filtEvents := eui.NewCheckbox()
	filt.Text = "Image Filtering"
	filt.Size = eui.Point{X: width, Y: 24}
	filt.Checked = gs.textureFiltering
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
	flow.AddItem(filt)

	fastSound, fastSoundEvents := eui.NewCheckbox()
	fastSound.Text = "Low Quality Sound"
	fastSound.Size = eui.Point{X: width, Y: 24}
	fastSound.Checked = gs.fastSound
	fastSoundEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			gs.fastSound = ev.Checked
			settingsDirty = true

			pcmCache = make(map[uint16][]byte)

			if gs.fastSound {
				resample = resampleLinear
			} else {
				initSinc()
				resample = resampleSincHQ
			}
			soundMu.Lock()
			pcmCache = make(map[uint16][]byte)
			soundMu.Unlock()
		}
	}
	flow.AddItem(fastSound)

	vsyncCB, vsyncEvents := eui.NewCheckbox()
	vsyncCB.Text = "Vsync"
	vsyncCB.Size = eui.Point{X: width, Y: 24}
	vsyncCB.Checked = gs.vsync
	vsyncEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			gs.vsync = ev.Checked
			ebiten.SetVsyncEnabled(gs.vsync)
			settingsDirty = true
		}
	}
	flow.AddItem(vsyncCB)

	qualityWin.AddItem(flow)
	qualityWin.AddWindow(false)
}

func makeDebugWindow() {
	if debugWin != nil {
		return
	}

	var width float32 = 250
	debugWin = eui.NewWindow()
	debugWin.Title = "Debug Settings"
	debugWin.Closable = true
	debugWin.Resizable = false
	debugWin.AutoSize = true
	debugWin.Movable = true

	debugFlow := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL}

	nightCB, nightEvents := eui.NewCheckbox()
	nightCB.Text = "Night Effect"
	nightCB.Size = eui.Point{X: width, Y: 24}
	nightCB.Checked = gs.nightEffect
	nightEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			gs.nightEffect = ev.Checked
			settingsDirty = true
		}
	}
	debugFlow.AddItem(nightCB)

	lateInputCB, lateInputEvents := eui.NewCheckbox()
	lateInputCB.Text = "Late Input Updates"
	lateInputCB.Size = eui.Point{X: width, Y: 24}
	lateInputCB.Checked = gs.lateInputUpdates
	lateInputEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			gs.lateInputUpdates = ev.Checked
			settingsDirty = true
		}
	}
	debugFlow.AddItem(lateInputCB)

	recordStatsCB, recordStatsEvents := eui.NewCheckbox()
	recordStatsCB.Text = "Record Asset Stats"
	recordStatsCB.Size = eui.Point{X: width, Y: 24}
	recordStatsCB.Checked = gs.recordAssetStats
	recordStatsEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			gs.recordAssetStats = ev.Checked
			settingsDirty = true
		}
	}
	debugFlow.AddItem(recordStatsCB)

	bubbleCB, bubbleEvents := eui.NewCheckbox()
	bubbleCB.Text = "Message Bubbles"
	bubbleCB.Size = eui.Point{X: width, Y: 24}
	bubbleCB.Checked = gs.SpeechBubbles
	bubbleEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			gs.SpeechBubbles = ev.Checked
			settingsDirty = true
		}
	}
	debugFlow.AddItem(bubbleCB)

	bubbleMsgCB, bubbleMsgEvents := eui.NewCheckbox()
	bubbleMsgCB.Text = "Chat to console"
	bubbleMsgCB.Size = eui.Point{X: width, Y: 24}
	bubbleMsgCB.Checked = gs.bubbleMessages
	bubbleMsgEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			gs.bubbleMessages = ev.Checked
			settingsDirty = true
		}
	}
	debugFlow.AddItem(bubbleMsgCB)

	hideMoveCB, hideMoveEvents := eui.NewCheckbox()
	hideMoveCB.Text = "Hide Moving"
	hideMoveCB.Size = eui.Point{X: width, Y: 24}
	hideMoveCB.Checked = gs.hideMoving
	hideMoveEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			gs.hideMoving = ev.Checked
			settingsDirty = true
		}
	}
	debugFlow.AddItem(hideMoveCB)

	hideMobCB, hideMobEvents := eui.NewCheckbox()
	hideMobCB.Text = "Hide Mobiles"
	hideMobCB.Size = eui.Point{X: width, Y: 24}
	hideMobCB.Checked = gs.hideMobiles
	hideMobEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			gs.hideMobiles = ev.Checked
			settingsDirty = true
		}
	}
	debugFlow.AddItem(hideMobCB)

	planesCB, planesEvents := eui.NewCheckbox()
	planesCB.Text = "Show image planes"
	planesCB.Size = eui.Point{X: width, Y: 24}
	planesCB.Checked = gs.imgPlanesDebug
	planesEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			gs.imgPlanesDebug = ev.Checked
			settingsDirty = true
		}
	}
	debugFlow.AddItem(planesCB)
	smoothinCB, smoothinEvents := eui.NewCheckbox()
	smoothinCB.Text = "Smoothing Debug"
	smoothinCB.Size = eui.Point{X: width, Y: 24}
	smoothinCB.Checked = gs.smoothingDebug
	smoothinEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			gs.smoothingDebug = ev.Checked
			settingsDirty = true
		}
	}
	debugFlow.AddItem(smoothinCB)
	cacheLabel, _ := eui.NewText()
	cacheLabel.Text = "Caches:"
	cacheLabel.Size = eui.Point{X: width, Y: 24}
	cacheLabel.FontSize = 10
	debugFlow.AddItem(cacheLabel)

	sheetCacheLabel, _ = eui.NewText()
	sheetCacheLabel.Text = ""
	sheetCacheLabel.Size = eui.Point{X: width, Y: 24}
	sheetCacheLabel.FontSize = 10
	debugFlow.AddItem(sheetCacheLabel)

	frameCacheLabel, _ = eui.NewText()
	frameCacheLabel.Text = ""
	frameCacheLabel.Size = eui.Point{X: width, Y: 24}
	frameCacheLabel.FontSize = 10
	debugFlow.AddItem(frameCacheLabel)

	mobileCacheLabel, _ = eui.NewText()
	mobileCacheLabel.Text = ""
	mobileCacheLabel.Size = eui.Point{X: width, Y: 24}
	mobileCacheLabel.FontSize = 10
	debugFlow.AddItem(mobileCacheLabel)

	soundCacheLabel, _ = eui.NewText()
	soundCacheLabel.Text = ""
	soundCacheLabel.Size = eui.Point{X: width, Y: 24}
	soundCacheLabel.FontSize = 10
	debugFlow.AddItem(soundCacheLabel)

	mobileBlendLabel, _ = eui.NewText()
	mobileBlendLabel.Text = ""
	mobileBlendLabel.Size = eui.Point{X: width, Y: 24}
	mobileBlendLabel.FontSize = 10
	debugFlow.AddItem(mobileBlendLabel)

	pictBlendLabel, _ = eui.NewText()
	pictBlendLabel.Text = ""
	pictBlendLabel.Size = eui.Point{X: width, Y: 24}
	pictBlendLabel.FontSize = 10
	debugFlow.AddItem(pictBlendLabel)

	clearCacheBtn, clearCacheEvents := eui.NewButton()
	clearCacheBtn.Text = "Clear All Caches"
	clearCacheBtn.Size = eui.Point{X: width, Y: 24}
	clearCacheEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			clearCaches()
			updateDebugStats()
		}
	}
	debugFlow.AddItem(clearCacheBtn)
	totalCacheLabel, _ = eui.NewText()
	totalCacheLabel.Text = ""
	totalCacheLabel.Size = eui.Point{X: width, Y: 24}
	totalCacheLabel.FontSize = 10
	debugFlow.AddItem(totalCacheLabel)

	debugWin.AddItem(debugFlow)

	soundTestFlow := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_HORIZONTAL}

	minusTenBtn, minusTenEvents := eui.NewButton()
	minusTenBtn.Text = "--"
	minusTenBtn.Size = eui.Point{X: 24, Y: 24}
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

	minusBtn, minusEvents := eui.NewButton()
	minusBtn.Text = "-"
	minusBtn.Size = eui.Point{X: 24, Y: 24}
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

	soundTestLabel, _ = eui.NewText()
	soundTestLabel.Text = "0"
	soundTestLabel.Size = eui.Point{X: 40, Y: 24}
	soundTestLabel.FontSize = 10
	soundTestFlow.AddItem(soundTestLabel)

	plusBtn, plusEvents := eui.NewButton()
	plusBtn.Text = "+"
	plusBtn.Size = eui.Point{X: 24, Y: 24}
	plusEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			soundTestID++
			updateSoundTestLabel()
			playSound(uint16(soundTestID))
		}
	}
	soundTestFlow.AddItem(plusBtn)

	plusTenBtn, plusTenEvents := eui.NewButton()
	plusTenBtn.Text = "++"
	plusTenBtn.Size = eui.Point{X: 24, Y: 24}
	plusTenEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			soundTestID += 10
			updateSoundTestLabel()
			playSound(uint16(soundTestID))
		}
	}
	soundTestFlow.AddItem(plusTenBtn)

	playBtn, playEvents := eui.NewButton()
	playBtn.Text = "Play"
	playBtn.Size = eui.Point{X: 40, Y: 24}
	playEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			playSound(uint16(soundTestID))
		}
	}
	soundTestFlow.AddItem(playBtn)

	debugFlow.AddItem(soundTestFlow)

	debugWin.AddWindow(false)
}

// updateDebugStats refreshes the cache statistics displayed in the debug window.
func updateDebugStats() {
	if debugWin == nil || !debugWin.IsOpen() {
		return
	}

	sheetCount, sheetBytes, frameCount, frameBytes, mobileCount, mobileBytes, mobileBlendCount, mobileBlendBytes, pictBlendCount, pictBlendBytes := imageCacheStats()
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
	if mobileBlendLabel != nil {
		mobileBlendLabel.Text = fmt.Sprintf("Mobile Blend Frames: %d (%s)", mobileBlendCount, humanize.Bytes(uint64(mobileBlendBytes)))
		mobileBlendLabel.Dirty = true
	}
	if pictBlendLabel != nil {
		pictBlendLabel.Text = fmt.Sprintf("World Blend Frames: %d (%s)", pictBlendCount, humanize.Bytes(uint64(pictBlendBytes)))
		pictBlendLabel.Dirty = true
	}
	if soundCacheLabel != nil {
		soundCacheLabel.Text = fmt.Sprintf("Sounds: %d (%s)", soundCount, humanize.Bytes(uint64(soundBytes)))
		soundCacheLabel.Dirty = true
	}
	if totalCacheLabel != nil {
		totalCacheLabel.Text = fmt.Sprintf("Total: %s", humanize.Bytes(uint64(sheetBytes+frameBytes+mobileBytes+soundBytes+mobileBlendBytes+pictBlendBytes)))
		totalCacheLabel.Dirty = true
	}
}

func updateSoundTestLabel() {
	if soundTestLabel != nil {
		soundTestLabel.Text = fmt.Sprintf("%d", soundTestID)
		soundTestLabel.Dirty = true
	}
}

func makeWindowsWindow() {
	if windowsWin != nil {
		return
	}
	windowsWin = eui.NewWindow()
	windowsWin.Title = "Windows"
	windowsWin.Closable = true
	windowsWin.Resizable = false
	windowsWin.AutoSize = true
	windowsWin.Movable = true

	flow := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL}

	playersBox, playersBoxEvents := eui.NewCheckbox()
	playersBox.Text = "Players"
	playersBox.Size = eui.Point{X: 128, Y: 24}
	playersBox.Checked = playersWin != nil
	playersBoxEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			if ev.Checked {
				playersWin.Open()
			} else {
				playersWin.Close()
			}
		}
	}
	flow.AddItem(playersBox)

	inventoryBox, inventoryBoxEvents := eui.NewCheckbox()
	inventoryBox.Text = "Inventory"
	inventoryBox.Size = eui.Point{X: 128, Y: 24}
	inventoryBox.Checked = inventoryWin != nil
	inventoryBoxEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			if ev.Checked {
				inventoryWin.Open()
			} else {
				inventoryWin.Close()
			}
		}
	}
	flow.AddItem(inventoryBox)

	messagesBox, messagesBoxEvents := eui.NewCheckbox()
	messagesBox.Text = "Messages"
	messagesBox.Size = eui.Point{X: 128, Y: 24}
	messagesBox.Checked = messagesWin != nil
	messagesBoxEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			if ev.Checked {
				messagesWin.Open()
			} else {
				messagesWin.Close()
			}
		}
	}
	flow.AddItem(messagesBox)

	windowsWin.AddItem(flow)
	windowsWin.AddWindow(false)

}

func makeInventoryWindow() {
	if inventoryWin != nil {
		return
	}
	inventoryWin = eui.NewWindow()
	inventoryWin.Title = "Inventory"
	inventoryWin.Closable = true
	inventoryWin.Resizable = true
	inventoryWin.Movable = true
	inventoryWin.Size = eui.Point{X: 425, Y: 600}

	if gs.InventoryWindow.Size.X > 0 && gs.InventoryWindow.Size.Y > 0 {
		inventoryWin.Size = eui.Point{X: float32(gs.InventoryWindow.Size.X), Y: float32(gs.InventoryWindow.Size.Y)}
	}
	if gs.InventoryWindow.Position.X != 0 || gs.InventoryWindow.Position.Y != 0 {
		inventoryWin.Position = eui.Point{X: float32(gs.InventoryWindow.Position.X), Y: float32(gs.InventoryWindow.Position.Y)}
	} else {
		inventoryWin.Position = eui.Point{X: 0, Y: 0}
	}

	inventoryList = &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL}
	title, _ := eui.NewText()
	title.Text = "Inventory"
	title.Size = eui.Point{X: 256, Y: 128}
	inventoryWin.AddItem(title)
	inventoryWin.AddItem(inventoryList)
	inventoryWin.AddWindow(false)
}

func makePlayersWindow() {
	if playersWin != nil {
		return
	}
	playersWin = eui.NewWindow()
	playersWin.Title = "Players"
	if gs.PlayersWindow.Size.X > 0 && gs.PlayersWindow.Size.Y > 0 {
		playersWin.Size = eui.Point{X: float32(gs.PlayersWindow.Size.X), Y: float32(gs.PlayersWindow.Size.Y)}
	} else {
		playersWin.Size = eui.Point{X: 425, Y: 600}
	}
	playersWin.Closable = true
	playersWin.Resizable = true
	playersWin.Movable = true
	playersWin.Position = TOP_RIGHT
	if gs.PlayersWindow.Position.X != 0 || gs.PlayersWindow.Position.Y != 0 {
		playersWin.Position = eui.Point{X: float32(gs.PlayersWindow.Position.X), Y: float32(gs.PlayersWindow.Position.Y)}
	}

	playersList = &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL}
	playersWin.AddItem(playersList)
	playersWin.AddWindow(false)
}

func makeHelpWindow() {
	if helpWin != nil {
		return
	}
	helpWin = eui.NewWindow()
	helpWin.Title = "Help"
	helpWin.Closable = true
	helpWin.Resizable = false
	helpWin.AutoSize = true
	helpWin.Movable = true
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
		t, _ := eui.NewText()
		t.Text = line
		t.Size = eui.Point{X: 300, Y: 24}
		t.FontSize = 15
		helpFlow.AddItem(t)
	}
	helpWin.AddItem(helpFlow)
	helpWin.AddWindow(false)
}
