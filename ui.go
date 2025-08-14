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

	"gothoom/eui"

	"github.com/dustin/go-humanize"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/sqweek/dialog"

	"gothoom/climg"
	"gothoom/clsnd"
)

const cval = 1000

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

	soundTestLabel  *eui.ItemData
	soundTestID     int
	recordBtn       *eui.ItemData
	recordStatus    *eui.ItemData
	qualityPresetDD *eui.ItemData
	denoiseCB       *eui.ItemData
	motionCB        *eui.ItemData
	noSmoothCB      *eui.ItemData
	animCB          *eui.ItemData
	pictBlendCB     *eui.ItemData
	precacheSoundCB *eui.ItemData
	precacheImageCB *eui.ItemData
	noCacheCB       *eui.ItemData
	potatoCB        *eui.ItemData
)

func init() {
	eui.WindowStateChanged = func() {
		if windowsWin != nil {
			windowsWin.Refresh()
		}
	}
}

func initUI() {
	status, err := checkDataFiles(clientVersion)
	if err != nil {
		logError("check data files: %v", err)
	}

	makeGameWindow()
	makeDownloadsWindow()
	makeLoginWindow()
	makeAddCharacterWindow()
	makeChatWindow()
	makeConsoleWindow()
	makeSettingsWindow()
	makeGraphicsWindow()
	makeQualityWindow()
	makeDebugWindow()
	makeWindowsWindow()
	makeInventoryWindow()
	makePlayersWindow()
	makeHelpWindow()
	makeToolbarWindow()

	chatWin.MarkOpen()
	consoleWin.MarkOpen()
	inventoryWin.MarkOpen()
	playersWin.MarkOpen()

	if status.NeedImages || status.NeedSounds {
		downloadWin.MarkOpen()
	} else if clmov == "" {
		loginWin.MarkOpen()
	}
}

func makeToolbarWindow() {

	var toolFontSize float32 = 10
	var buttonHeight float32 = 15
	var buttonWidth float32 = 64

	toolbarWin = eui.NewWindow()
	toolbarWin.Title = ""
	toolbarWin.SetTitleSize(8)
	toolbarWin.Closable = false
	toolbarWin.Resizable = false
	toolbarWin.AutoSize = false
	toolbarWin.ShowDragbar = false
	toolbarWin.Movable = true
	toolbarWin.NoScroll = true
	toolbarWin.SetZone(eui.HZoneCenter, eui.VZoneTop)
	toolbarWin.Size = eui.Point{X: 500, Y: 35}

	gameMenu := &eui.ItemData{
		ItemType: eui.ITEM_FLOW,
		FlowType: eui.FLOW_HORIZONTAL,
	}
	winBtn, winEvents := eui.NewButton()
	winBtn.Text = "Windows"
	winBtn.Size = eui.Point{X: buttonWidth, Y: buttonHeight}
	winBtn.FontSize = toolFontSize
	winEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			windowsWin.Toggle()
		}
	}
	gameMenu.AddItem(winBtn)

	btn, setEvents := eui.NewButton()
	btn.Text = "Settings"
	btn.Size = eui.Point{X: buttonWidth, Y: buttonHeight}
	btn.FontSize = toolFontSize
	setEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			settingsWin.Toggle()
		}
	}
	gameMenu.AddItem(btn)

	helpBtn, helpEvents := eui.NewButton()
	helpBtn.Text = "Help"
	helpBtn.Size = eui.Point{X: buttonWidth, Y: buttonHeight}
	helpBtn.FontSize = toolFontSize
	helpEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			helpWin.Toggle()
		}
	}
	gameMenu.AddItem(helpBtn)

	volumeSlider, volumeEvents := eui.NewSlider()
	volumeSlider.MinValue = 0
	volumeSlider.MaxValue = 1
	volumeSlider.Value = float32(gs.Volume)
	volumeSlider.Size = eui.Point{X: 150, Y: buttonHeight}
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
	muteBtn.Size = eui.Point{X: 64, Y: buttonHeight}
	muteBtn.FontSize = 12
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

	/*
		recordBtn, recordEvents := eui.NewButton()
		recordBtn.Text = "Record"
		recordBtn.Size = eui.Point{X: buttonWidth, Y: buttonHeight}
		recordBtn.FontSize = toolFontSize
		recordBtn.Disabled = true
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
	*/

	recordStatus, _ = eui.NewText()
	recordStatus.Text = ""
	recordStatus.Size = eui.Point{X: 80, Y: buttonHeight}
	recordStatus.FontSize = toolFontSize
	recordStatus.Color = eui.ColorRed
	gameMenu.AddItem(recordStatus)

	toolbarWin.AddItem(gameMenu)
	toolbarWin.AddWindow(false)
	toolbarWin.MarkOpen()

	//eui.TreeMode = true
}

var dlMutex sync.Mutex
var status dataFilesStatus

func makeDownloadsWindow() {

	if downloadWin != nil {
		return
	}
	downloadWin = eui.NewWindow()
	downloadWin.Title = "Downloads"
	downloadWin.Closable = false
	downloadWin.Resizable = false
	downloadWin.AutoSize = true
	downloadWin.Movable = true
	downloadWin.SetZone(eui.HZoneCenterLeft, eui.VZoneMiddleTop)

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
				loginWin.MarkOpen()
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
					// Preserve window position while contents change size
					loginWin.Refresh()
				}
			}
			row.AddItem(trash)
			charactersList.AddItem(row)
		}
	}
	// Preserve window position while contents change size
	loginWin.Refresh()
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
	addCharWin.SetZone(eui.HZoneCenterLeft, eui.VZoneMiddleTop)

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
			loginWin.MarkOpen()
			addCharWin.Close()
		}
	}
	flow.AddItem(addBtn)

	cancelBtn, cancelEvents := eui.NewButton()
	cancelBtn.Text = "Cancel"
	cancelBtn.Size = eui.Point{X: 200, Y: 24}
	cancelEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			loginWin.MarkOpen()
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
	loginWin.SetZone(eui.HZoneCenterLeft, eui.VZoneMiddleTop)
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
	addBtn.Size = eui.Point{X: 200, Y: 24}
	addEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			addCharName = ""
			addCharPass = ""
			addCharRemember = true
			loginWin.Close()
			addCharWin.MarkOpen()
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
					loginWin.MarkOpen()
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
					loginWin.MarkOpen()
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
	win.MarkOpen()
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
	settingsWin.SetZone(eui.HZoneCenterLeft, eui.VZoneMiddleTop)

	mainFlow := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL}
	var width float32 = 250

	themeDD, themeEvents := eui.NewDropdown()
	themeDD.Label = "Theme"
	if opts, err := eui.ListThemes(); err == nil {
		themeDD.Options = opts
		cur := eui.CurrentThemeName()
		for i, n := range opts {
			if n == cur {
				themeDD.Selected = i
				break
			}
		}
	}
	themeDD.Size = eui.Point{X: width, Y: 24}
	themeEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventDropdownSelected {
			name := themeDD.Options[ev.Index]
			if err := eui.LoadTheme(name); err == nil {
				gs.Theme = name
				settingsDirty = true
				settingsWin.Refresh()
			}
		}
	}
	mainFlow.AddItem(themeDD)

	label, _ := eui.NewText()
	label.Text = "\nControls:"
	label.FontSize = 15
	label.Size = eui.Point{X: 100, Y: 50}
	mainFlow.AddItem(label)

	toggle, toggleEvents := eui.NewCheckbox()
	toggle.Text = "Click-to-toggle movement"
	toggle.Size = eui.Point{X: width, Y: 24}
	toggle.Checked = gs.ClickToToggle
	toggle.Tooltip = "Click once to start walking, click again to stop."
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
	label.Text = "\nWindow Behavior:"
	label.FontSize = 15
	label.Size = eui.Point{X: 150, Y: 50}
	mainFlow.AddItem(label)

	tilingCB, tilingEvents := eui.NewCheckbox()
	tilingCB.Text = "Tiling window mode"
	tilingCB.Size = eui.Point{X: width, Y: 24}
	tilingCB.Checked = gs.WindowTiling
	tilingCB.Tooltip = "Prevent windows from overlapping"
	tilingEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			gs.WindowTiling = ev.Checked
			eui.SetWindowTiling(ev.Checked)
			settingsDirty = true
		}
	}
	mainFlow.AddItem(tilingCB)

	snapCB, snapEvents := eui.NewCheckbox()
	snapCB.Text = "Window snapping (buggy)"
	snapCB.Size = eui.Point{X: width, Y: 24}
	snapCB.Checked = gs.WindowSnapping
	snapCB.Tooltip = "Snap windows to edges and others"
	snapEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			gs.WindowSnapping = ev.Checked
			eui.SetWindowSnapping(ev.Checked)
			settingsDirty = true
		}
	}
	mainFlow.AddItem(snapCB)

	graphicsBtn, graphicsEvents := eui.NewButton()
	graphicsBtn.Text = "Screen Size Settings"
	graphicsBtn.Size = eui.Point{X: width, Y: 24}
	graphicsEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			graphicsWin.Toggle()
		}
	}
	mainFlow.AddItem(graphicsBtn)

	label, _ = eui.NewText()
	label.Text = "\nText Sizes:"
	label.FontSize = 15
	label.Size = eui.Point{X: 100, Y: 50}
	mainFlow.AddItem(label)

	chatFontSlider, chatFontEvents := eui.NewSlider()
	chatFontSlider.Label = "Chat Bubble Font Size"
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
	labelFontSlider.Label = "Name Font Size"
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

	consoleFontSlider, consoleFontEvents := eui.NewSlider()
	consoleFontSlider.Label = "Console Font Size"
	consoleFontSlider.MinValue = 6
	consoleFontSlider.MaxValue = 24
	consoleFontSlider.Value = float32(gs.ConsoleFontSize)
	consoleFontSlider.Size = eui.Point{X: width - 10, Y: 24}
	consoleFontEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventSliderChanged {
			gs.ConsoleFontSize = float64(ev.Value)
			updateConsoleWindow()
			if consoleWin != nil {
				consoleWin.Refresh()
			}
			settingsDirty = true
		}
	}
	mainFlow.AddItem(consoleFontSlider)

	chatWindowFontSlider, chatWindowFontEvents := eui.NewSlider()
	chatWindowFontSlider.Label = "Chat Window Font Size"
	chatWindowFontSlider.MinValue = 6
	chatWindowFontSlider.MaxValue = 24
	chatWindowFontSlider.Value = float32(gs.ChatFontSize)
	chatWindowFontSlider.Size = eui.Point{X: width - 10, Y: 24}
	chatWindowFontEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventSliderChanged {
			gs.ChatFontSize = float64(ev.Value)
			updateChatWindow()
			if chatWin != nil {
				chatWin.Refresh()
			}
			settingsDirty = true
		}
	}
	mainFlow.AddItem(chatWindowFontSlider)

	label, _ = eui.NewText()
	label.Text = "\nOpacity Settings:"
	label.FontSize = 15
	label.Size = eui.Point{X: 150, Y: 50}
	mainFlow.AddItem(label)

	bubbleOpSlider, bubbleOpEvents := eui.NewSlider()
	bubbleOpSlider.Label = "Bubble Opacity"
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
	nameBgSlider.Label = "Name Background Opacity"
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
	label.Text = "\nQuality Settings:"
	label.FontSize = 15
	label.Size = eui.Point{X: 150, Y: 50}
	mainFlow.AddItem(label)

	qualityPresetDD, qpEvents := eui.NewDropdown()
	qualityPresetDD.Options = []string{"Ultra-Low", "Low", "Standard", "High", "Ultimate", "Custom"}
	qualityPresetDD.Size = eui.Point{X: width, Y: 24}
	qualityPresetDD.Selected = detectQualityPreset()
	qualityPresetDD.FontSize = 12
	qpEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventDropdownSelected {
			switch ev.Index {
			case 0:
				applyQualityPreset("Ultra Low")
			case 1:
				applyQualityPreset("Low")
			case 2:
				applyQualityPreset("Standard")
			case 3:
				applyQualityPreset("High")
			case 4:
				applyQualityPreset("Ultimate")
			}
			qualityPresetDD.Selected = detectQualityPreset()
		}
	}
	mainFlow.AddItem(qualityPresetDD)

	qualityBtn, qualityEvents := eui.NewButton()
	qualityBtn.Text = "Quality Options"
	qualityBtn.Size = eui.Point{X: width, Y: 24}
	qualityEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			qualityWin.Toggle()
		}
	}
	mainFlow.AddItem(qualityBtn)

	label, _ = eui.NewText()
	label.Text = ""
	label.Size = eui.Point{X: 150, Y: 15}
	mainFlow.AddItem(label)

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

func makeGraphicsWindow() {
	if graphicsWin != nil {
		return
	}
	var width float32 = 250
	graphicsWin = eui.NewWindow()
	graphicsWin.Title = "Screen Size Settings"
	graphicsWin.Closable = true
	graphicsWin.Resizable = false
	graphicsWin.AutoSize = true
	graphicsWin.Movable = true
	graphicsWin.SetZone(eui.HZoneCenterLeft, eui.VZoneMiddleTop)

	flow := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL}

	uiScaleSlider, uiScaleEvents := eui.NewSlider()
	uiScaleSlider.Label = "UI Scaling"
	uiScaleSlider.MinValue = 1.0
	uiScaleSlider.MaxValue = 2.5
	uiScaleSlider.Value = float32(gs.UIScale)
	uiScaleSlider.Size = eui.Point{X: width - 10, Y: 24}
	pendingUIScale := gs.UIScale
	uiScaleEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventSliderChanged {
			pendingUIScale = float64(ev.Value)
		}
	}
	flow.AddItem(uiScaleSlider)

	uiScaleApplyBtn, uiScaleApplyEvents := eui.NewButton()
	uiScaleApplyBtn.Text = "Apply UI Scale"
	uiScaleApplyBtn.Size = eui.Point{X: 100, Y: 24}
	uiScaleApplyEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			gs.UIScale = pendingUIScale
			eui.SetUIScale(float32(gs.UIScale))
			updateGameWindowSize()
			settingsDirty = true
		}
	}
	flow.AddItem(uiScaleApplyBtn)

	gameSizeSlider, gameSizeEvents := eui.NewSlider()
	gameSizeSlider.Label = "Game Window Magnify (Sharp)"
	gameSizeSlider.MinValue = 1
	gameSizeSlider.MaxValue = 5
	gameSizeSlider.IntOnly = true
	gsVal := gs.GameScale
	if gsVal < 1 {
		gsVal = 1
	} else if gsVal > 5 {
		gsVal = 5
	}
	gameSizeSlider.Value = float32(gsVal)
	gameSizeSlider.Size = eui.Point{X: width - 10, Y: 24}
	gameSizeSlider.Disabled = gs.AnyGameWindowSize
	gameSizeEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventSliderChanged {
			gs.GameScale = float64(ev.Value)
			updateGameWindowSize()
			initFont()
			settingsDirty = true
		}
	}
	flow.AddItem(gameSizeSlider)

	/*
		anySizeWarn, _ := eui.NewText()
		anySizeWarn.Text = "Warning: this option will\nproduce blurrier graphics"
		anySizeWarn.FontSize = 10
		anySizeWarn.Color = eui.ColorRed
		anySizeWarn.Size = eui.Point{X: width, Y: 32}
		anySizeWarn.Invisible = !gs.AnyGameWindowSize

			anySizeCB, anySizeEvents := eui.NewCheckbox()
			anySizeCB.Text = "Any size game window"
			anySizeCB.Size = eui.Point{X: width, Y: 24}
			anySizeCB.Checked = gs.AnyGameWindowSize
			anySizeEvents.Handle = func(ev eui.UIEvent) {
				if ev.Type == eui.EventCheckboxChanged {
					gs.AnyGameWindowSize = ev.Checked
					gameSizeSlider.Disabled = ev.Checked
					anySizeWarn.Invisible = !ev.Checked
					updateGameWindowSize()
					settingsDirty = true
				}
			}
			flow.AddItem(anySizeCB)
			flow.AddItem(anySizeWarn)
	*/

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
	flow.AddItem(fullscreenCB)

	graphicsWin.AddItem(flow)
	graphicsWin.AddWindow(false)
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
	qualityWin.SetZone(eui.HZoneCenterLeft, eui.VZoneMiddleTop)

	flow := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL}

	dCB, denoiseEvents := eui.NewCheckbox()
	denoiseCB = dCB
	denoiseCB.Text = "Blend Image Dithering"
	denoiseCB.Size = eui.Point{X: width, Y: 24}
	denoiseCB.Checked = gs.DenoiseImages
	denoiseCB.Tooltip = "Attempts to blend image dithering to recover color information"
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
	denoiseSharpSlider.Label = "Sharpness"
	denoiseSharpSlider.MinValue = 0.1
	denoiseSharpSlider.MaxValue = 8
	denoiseSharpSlider.Value = float32(gs.DenoiseSharpness)
	denoiseSharpSlider.Size = eui.Point{X: width - 10, Y: 24}
	denoiseSharpSlider.Tooltip = "High is bias for not losing fine details"
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
	denoiseAmtSlider.Label = "Denoise strength"
	denoiseAmtSlider.MinValue = 0.1
	denoiseAmtSlider.MaxValue = 0.5
	denoiseAmtSlider.Value = float32(gs.DenoisePercent)
	denoiseAmtSlider.Size = eui.Point{X: width - 10, Y: 24}
	denoiseAmtSlider.Tooltip = "How strongly to blend dithered areas"
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

	mCB, motionEvents := eui.NewCheckbox()
	motionCB = mCB
	motionCB.Text = "Smooth Motion"
	motionCB.Size = eui.Point{X: width, Y: 24}
	motionCB.Checked = gs.MotionSmoothing
	motionCB.Tooltip = "Interpolate camera and mobile movement, looks very nice"
	motionEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			gs.MotionSmoothing = ev.Checked
			settingsDirty = true
		}
	}
	flow.AddItem(motionCB)

	smo, noSmoothEvents := eui.NewCheckbox()
	smo.Text = "Smooth moving objects,glitchy WIP"
	smo.Size = eui.Point{X: width, Y: 24}
	smo.Checked = gs.smoothMoving
	smo.Tooltip = "Smooth moving objects that are not 'mobiles' such as chains, clouds, etc"
	noSmoothEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			gs.smoothMoving = ev.Checked
			settingsDirty = true
		}
	}
	flow.AddItem(smo)

	aCB, animEvents := eui.NewCheckbox()
	animCB = aCB
	animCB.Text = "Mobile Animation Blending"
	animCB.Size = eui.Point{X: width, Y: 24}
	animCB.Checked = gs.BlendMobiles
	animCB.Tooltip = "Gives appearance of more frames of animation at cost of latency."
	animEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			gs.BlendMobiles = ev.Checked
			settingsDirty = true
			mobileBlendCache = map[mobileBlendKey]*ebiten.Image{}
		}
	}
	flow.AddItem(animCB)

	pCB, pictBlendEvents := eui.NewCheckbox()
	pictBlendCB = pCB
	pictBlendCB.Text = "World Animation Blending"
	pictBlendCB.Size = eui.Point{X: width, Y: 24}
	pictBlendCB.Checked = gs.BlendPicts
	pictBlendCB.Tooltip = "Gives appearance of more frames of animation for water, grass, etc. Looks amazing!"
	pictBlendEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			gs.BlendPicts = ev.Checked
			settingsDirty = true
			pictBlendCache = map[pictBlendKey]*ebiten.Image{}
		}
	}
	flow.AddItem(pictBlendCB)

	mobileBlendSlider, mobileBlendEvents := eui.NewSlider()
	mobileBlendSlider.Label = "Mobile Animation Blend Amount"
	mobileBlendSlider.MinValue = 0.1
	mobileBlendSlider.MaxValue = 1.0
	mobileBlendSlider.Value = float32(gs.MobileBlendAmount)
	mobileBlendSlider.Size = eui.Point{X: width - 10, Y: 24}
	mobileBlendSlider.Tooltip = "Generally looks best at 0.25-0.5, increases latency"
	mobileBlendEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventSliderChanged {
			gs.MobileBlendAmount = float64(ev.Value)
			settingsDirty = true
		}
	}
	flow.AddItem(mobileBlendSlider)

	blendSlider, blendEvents := eui.NewSlider()
	blendSlider.Label = "World Animation Blending Strength"
	blendSlider.MinValue = 0.1
	blendSlider.MaxValue = 1.0
	blendSlider.Value = float32(gs.BlendAmount)
	blendSlider.Size = eui.Point{X: width - 10, Y: 24}
	blendSlider.Tooltip = "This looks amazing at max (1.0)"
	blendEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventSliderChanged {
			gs.BlendAmount = float64(ev.Value)
			settingsDirty = true
		}
	}
	flow.AddItem(blendSlider)

	mobileFramesSlider, mobileFramesEvents := eui.NewSlider()
	mobileFramesSlider.Label = "Mobile Animation Blend Frames"
	mobileFramesSlider.MinValue = 3
	mobileFramesSlider.MaxValue = 30
	mobileFramesSlider.Value = float32(gs.MobileBlendFrames)
	mobileFramesSlider.Size = eui.Point{X: width - 10, Y: 24}
	mobileFramesSlider.IntOnly = true
	mobileFramesSlider.Tooltip = "Number of blending steps. 10 blend frames = ~60fps"
	mobileFramesEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventSliderChanged {
			gs.MobileBlendFrames = int(ev.Value)
			settingsDirty = true
		}
	}
	flow.AddItem(mobileFramesSlider)

	pictFramesSlider, pictFramesEvents := eui.NewSlider()
	pictFramesSlider.Label = "World Animation Blend Frames"
	pictFramesSlider.MinValue = 3
	pictFramesSlider.MaxValue = 30
	pictFramesSlider.Value = float32(gs.PictBlendFrames)
	pictFramesSlider.Size = eui.Point{X: width - 10, Y: 24}
	pictFramesSlider.IntOnly = true
	pictFramesSlider.Tooltip = "Number of blending steps. 10 blend frames = ~60fps"
	pictFramesEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventSliderChanged {
			gs.PictBlendFrames = int(ev.Value)
			settingsDirty = true
		}
	}
	flow.AddItem(pictFramesSlider)

	showFPSCB, showFPSEvents := eui.NewCheckbox()
	showFPSCB.Text = "Show FPS + UPS"
	showFPSCB.Size = eui.Point{X: width, Y: 24}
	showFPSCB.Checked = gs.ShowFPS
	showFPSCB.Tooltip = "Display frames per second, and updates per second"
	showFPSEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			gs.ShowFPS = ev.Checked
			settingsDirty = true
		}
	}
	flow.AddItem(showFPSCB)

	psCB, precacheSoundEvents := eui.NewCheckbox()
	precacheSoundCB = psCB
	precacheSoundCB.Text = "Precache Sounds"
	precacheSoundCB.Size = eui.Point{X: width, Y: 24}
	precacheSoundCB.Checked = gs.precacheSounds
	precacheSoundCB.Tooltip = "Load and pre-process all sounds, uses RAM but runs smoother (400MB)"
	precacheSoundCB.Disabled = gs.NoCaching
	precacheSoundEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			gs.precacheSounds = ev.Checked
			if ev.Checked {
				gs.NoCaching = false
				if noCacheCB != nil {
					noCacheCB.Checked = false
				}
				go precacheAssets()
			}
			settingsDirty = true
			if qualityWin != nil {
				qualityWin.Refresh()
			}
			if graphicsWin != nil {
				graphicsWin.Refresh()
			}
			if debugWin != nil {
				debugWin.Refresh()
			}
		}
	}
	flow.AddItem(precacheSoundCB)

	piCB, precacheImageEvents := eui.NewCheckbox()
	precacheImageCB = piCB
	precacheImageCB.Text = "Precache Images"
	precacheImageCB.Size = eui.Point{X: width, Y: 24}
	precacheImageCB.Checked = gs.precacheImages
	precacheImageCB.Tooltip = "Load and pre-process all images, more RAM but runs smoother (2GB)"
	precacheImageCB.Disabled = gs.NoCaching
	precacheImageEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			gs.precacheImages = ev.Checked
			if ev.Checked {
				gs.NoCaching = false
				if noCacheCB != nil {
					noCacheCB.Checked = false
				}
				go precacheAssets()
			}
			settingsDirty = true
			if qualityWin != nil {
				qualityWin.Refresh()
			}
			if graphicsWin != nil {
				graphicsWin.Refresh()
			}
			if debugWin != nil {
				debugWin.Refresh()
			}
		}
	}
	flow.AddItem(precacheImageCB)

	ncCB, noCacheEvents := eui.NewCheckbox()
	noCacheCB = ncCB
	noCacheCB.Text = "No caching (Low RAM)"
	noCacheCB.Tooltip = "Save around 100-200MB RAM at cost of more CPU."
	noCacheCB.Size = eui.Point{X: width, Y: 24}
	noCacheCB.Checked = gs.NoCaching
	noCacheEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			gs.NoCaching = ev.Checked
			precacheSoundCB.Disabled = ev.Checked
			precacheImageCB.Disabled = ev.Checked
			if ev.Checked {
				gs.precacheSounds = false
				gs.precacheImages = false
				precacheSoundCB.Checked = false
				precacheImageCB.Checked = false
				clearCaches()
			}
			settingsDirty = true
			if qualityPresetDD != nil {
				qualityPresetDD.Selected = detectQualityPreset()
			}
			if qualityWin != nil {
				qualityWin.Refresh()
			}
			if graphicsWin != nil {
				graphicsWin.Refresh()
			}
			if debugWin != nil {
				debugWin.Refresh()
			}
		}
	}
	flow.AddItem(noCacheCB)

	pcCB, potatoEvents := eui.NewCheckbox()
	potatoCB = pcCB
	potatoCB.Text = "Potato GPU (low VRAM)"
	potatoCB.Tooltip = "Work-around for GPUs that only support 4096x4096 size sprites"
	potatoCB.Size = eui.Point{X: width, Y: 24}
	potatoCB.Checked = gs.PotatoComputer
	potatoEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			gs.PotatoComputer = ev.Checked
			applySettings()
			if ev.Checked {
				clearCaches()
			}
			settingsDirty = true
			if qualityPresetDD != nil {
				qualityPresetDD.Selected = detectQualityPreset()
			}
		}
	}
	flow.AddItem(potatoCB)

	vsyncCB, vsyncEvents := eui.NewCheckbox()
	vsyncCB.Text = "VSync - Limit FPS"
	vsyncCB.Size = eui.Point{X: width, Y: 24}
	vsyncCB.Checked = gs.vsync
	vsyncCB.Tooltip = "Limit framerate to monitor Hz. OFF can improve speed"
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
	debugWin.SetZone(eui.HZoneCenterLeft, eui.VZoneMiddleTop)

	debugFlow := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL}

	bubbleCB, bubbleEvents := eui.NewCheckbox()
	bubbleCB.Text = "Message Bubbles"
	bubbleCB.Size = eui.Point{X: width, Y: 24}
	bubbleCB.Checked = gs.SpeechBubbles
	bubbleCB.Tooltip = "Show speech bubbles in game"
	bubbleEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			gs.SpeechBubbles = ev.Checked
			settingsDirty = true
		}
	}
	debugFlow.AddItem(bubbleCB)

	nightCB, nightEvents := eui.NewCheckbox()
	nightCB.Text = "Night Effect"
	nightCB.Size = eui.Point{X: width, Y: 24}
	nightCB.Checked = gs.nightEffect
	nightCB.Tooltip = "Enable night vingette effect"
	nightEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			gs.nightEffect = ev.Checked
			settingsDirty = true
		}
	}
	debugFlow.AddItem(nightCB)

	lateInputCB, lateInputEvents := eui.NewCheckbox()
	lateInputCB.Text = "Late Input Updates (experimental)"
	lateInputCB.Size = eui.Point{X: width, Y: 24}
	lateInputCB.Checked = gs.lateInputUpdates
	lateInputCB.Tooltip = "Polls for user input at last moment, sends update to server early by predicted ping"
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
	recordStatsCB.Tooltip = "Writes stats.json with number of times image-id is loaded"
	recordStatsEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			gs.recordAssetStats = ev.Checked
			settingsDirty = true
		}
	}
	debugFlow.AddItem(recordStatsCB)

	bubbleMsgCB, bubbleMsgEvents := eui.NewCheckbox()
	bubbleMsgCB.Text = "Send chat to console window"
	bubbleMsgCB.Tooltip = "Nice for single-window text"
	bubbleMsgCB.Size = eui.Point{X: width, Y: 24}
	bubbleMsgCB.Checked = gs.MessagesToConsole
	bubbleMsgEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			gs.MessagesToConsole = ev.Checked
			settingsDirty = true
		}
	}
	debugFlow.AddItem(bubbleMsgCB)

	hideMoveCB, hideMoveEvents := eui.NewCheckbox()
	hideMoveCB.Text = "Hide Moving Objects"
	hideMoveCB.Tooltip = "Helpful for screenshots"
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
	hideMobCB.Tooltip = "Helpful for screenshots"
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
	planesCB.Tooltip = "Shows plane (layer) number on each sprite"
	planesCB.Size = eui.Point{X: width, Y: 24}
	planesCB.Checked = gs.imgPlanesDebug
	planesEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			gs.imgPlanesDebug = ev.Checked
			settingsDirty = true
		}
	}
	debugFlow.AddItem(planesCB)

	pictIDCB, pictIDEvents := eui.NewCheckbox()
	pictIDCB.Text = "Show picture IDs"
	pictIDCB.Tooltip = "Shows picture ID on each sprite"
	pictIDCB.Size = eui.Point{X: width, Y: 24}
	pictIDCB.Checked = gs.pictIDDebug
	pictIDEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			gs.pictIDDebug = ev.Checked
			settingsDirty = true
		}
	}
	debugFlow.AddItem(pictIDCB)

	smoothinCB, smoothinEvents := eui.NewCheckbox()
	smoothinCB.Text = "Tint moving objects red"
	smoothinCB.Size = eui.Point{X: width, Y: 24}
	smoothinCB.Checked = gs.smoothingDebug
	smoothinEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			gs.smoothingDebug = ev.Checked
			settingsDirty = true
		}
	}
	debugFlow.AddItem(smoothinCB)
	pictAgainCB, pictAgainEvents := eui.NewCheckbox()
	pictAgainCB.Text = "Tint pictAgain blue"
	pictAgainCB.Size = eui.Point{X: width, Y: 24}
	pictAgainCB.Checked = gs.pictAgainDebug
	pictAgainEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			gs.pictAgainDebug = ev.Checked
			settingsDirty = true
		}
	}
	debugFlow.AddItem(pictAgainCB)
	noMobSmoothCB, noMobSmoothEvents := eui.NewCheckbox()
	noMobSmoothCB.Text = "Don't smooth mobile motion"
	noMobSmoothCB.Size = eui.Point{X: width, Y: 24}
	noMobSmoothCB.Checked = gs.noMobileSmoothing
	noMobSmoothCB.Tooltip = "Disable motion interpolation for mobiles"
	noMobSmoothEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			gs.noMobileSmoothing = ev.Checked
			settingsDirty = true
		}
	}
	debugFlow.AddItem(noMobSmoothCB)
	shiftSpriteCB, shiftSpriteEvents := eui.NewCheckbox()
	shiftSpriteCB.Text = "Don't shift new sprites"
	shiftSpriteCB.Size = eui.Point{X: width, Y: 24}
	shiftSpriteCB.Checked = gs.dontShiftNewSprites
	shiftSpriteEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			gs.dontShiftNewSprites = ev.Checked
			settingsDirty = true
		}
	}
	debugFlow.AddItem(shiftSpriteCB)

	zoomCB, zoomEvents := eui.NewCheckbox()
	zoomCB.Text = "Enable Scroll Wheel Zoom"
	zoomCB.Size = eui.Point{X: width, Y: 24}
	zoomCB.Checked = gs.ZoomEnabled
	zoomCB.Tooltip = "Use mouse wheel over game to zoom"
	zoomEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			gs.ZoomEnabled = ev.Checked
			settingsDirty = true
		}
	}
	debugFlow.AddItem(zoomCB)
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
	clearCacheBtn.Tooltip = "Clear cached assets"
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
	windowsWin.SetZone(eui.HZoneCenterLeft, eui.VZoneMiddleTop)

	flow := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL}

	playersBox, playersBoxEvents := eui.NewCheckbox()
	playersBox.Text = "Players"
	playersBox.Size = eui.Point{X: 128, Y: 24}
	playersBox.Checked = playersWin != nil
	playersBoxEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			if ev.Checked {
				playersWin.MarkOpen()
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
				inventoryWin.MarkOpen()
			} else {
				inventoryWin.Close()
			}
		}
	}
	flow.AddItem(inventoryBox)

	chatBox, chatBoxEvents := eui.NewCheckbox()
	chatBox.Text = "Chat"
	chatBox.Size = eui.Point{X: 128, Y: 24}
	chatBox.Checked = consoleWin != nil
	chatBoxEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			if ev.Checked {
				chatWin.MarkOpen()
			} else {
				chatWin.Close()
			}
		}
	}
	flow.AddItem(chatBox)

	consoleBox, consoleBoxEvents := eui.NewCheckbox()
	consoleBox.Text = "Console"
	consoleBox.Size = eui.Point{X: 128, Y: 24}
	consoleBox.Checked = consoleWin.Open
	consoleBoxEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			if ev.Checked {
				consoleWin.MarkOpen()
			} else {
				consoleWin.Close()
			}
		}
	}
	flow.AddItem(consoleBox)

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
	inventoryWin.SetZone(eui.HZoneLeft, eui.VZoneTop)
	inventoryWin.Size = eui.Point{X: 410, Y: 600}

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
		playersWin.Size = eui.Point{X: 410, Y: 600}
	}
	playersWin.Closable = true
	playersWin.Resizable = true
	playersWin.Movable = true
	playersWin.SetZone(eui.HZoneRight, eui.VZoneTop)
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
	helpWin.SetZone(eui.HZoneCenterLeft, eui.VZoneMiddleTop)
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
