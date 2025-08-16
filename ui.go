package main

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"gothoom/eui"

	"github.com/dustin/go-humanize"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/sqweek/dialog"

	"gothoom/climg"
	"gothoom/clsnd"

	text "github.com/hajimehoshi/ebiten/v2/text/v2"
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

// Keep references to inputs so we can clear text programmatically.
var addCharNameInput *eui.ItemData
var addCharPassInput *eui.ItemData
var windowsWin *eui.WindowData

// Checkboxes in the Windows window so we can update their state live
var windowsPlayersCB *eui.ItemData
var windowsInventoryCB *eui.ItemData
var windowsChatCB *eui.ItemData
var windowsConsoleCB *eui.ItemData
var toolbarWin *eui.WindowData
var hudWin *eui.WindowData
var rightHandImg *eui.ItemData
var leftHandImg *eui.ItemData

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

// lastWhoRequest tracks the last time we requested a backend who list so we
// can avoid spamming the server when the Players window is toggled rapidly.
var lastWhoRequest time.Time

func init() {
	eui.WindowStateChanged = func() {
		// Keep the Windows window's checkboxes in sync
		if windowsPlayersCB != nil {
			windowsPlayersCB.Checked = playersWin != nil && playersWin.IsOpen()
			windowsPlayersCB.Dirty = true
		}
		if windowsInventoryCB != nil {
			windowsInventoryCB.Checked = inventoryWin != nil && inventoryWin.IsOpen()
			windowsInventoryCB.Dirty = true
		}
		if windowsChatCB != nil {
			windowsChatCB.Checked = chatWin != nil && chatWin.IsOpen()
			windowsChatCB.Dirty = true
		}
		if windowsConsoleCB != nil {
			windowsConsoleCB.Checked = consoleWin != nil && consoleWin.IsOpen()
			windowsConsoleCB.Dirty = true
		}
		if windowsWin != nil {
			windowsWin.Refresh()
		}

		// If the Players window just opened (or is open) and it's been a few
		// seconds since our last request, trigger a backend who scan so the
		// list includes everyone online, not just nearby mobiles.
		if playersWin != nil && playersWin.IsOpen() {
			if time.Since(lastWhoRequest) > 5*time.Second {
				pendingCommand = "/be-who"
				lastWhoRequest = time.Now()
			}
		}
	}
}

func initUI() {
	var err error
	status, err = checkDataFiles(clientVersion)
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
	makeQualityWindow()
	makeDebugWindow()
	makeWindowsWindow()
	makeInventoryWindow()
	makePlayersWindow()
	makeHelpWindow()
	makeHUDWindow()
	makeToolbarWindow()

	// Load any persisted players data (e.g., from prior sessions) so
	// avatars/classes can show up immediately.
	loadPlayersPersist()

	chatWin.MarkOpen()
	consoleWin.MarkOpen()
	inventoryWin.MarkOpen()
	playersWin.MarkOpen()

	if status.NeedImages || status.NeedSounds {
		downloadWin.MarkOpen()
	} else if clmov == "" && pcapPath == "" {
		loginWin.MarkOpen()
	}
}

func buildToolbar(toolFontSize, buttonWidth, buttonHeight float32, twoRows bool) *eui.ItemData {
	var row1, row2, menu *eui.ItemData
	if twoRows {
		row1 = &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_HORIZONTAL}
		row2 = &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_HORIZONTAL}
		menu = &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL}
	} else {
		menu = &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_HORIZONTAL}
	}

	winBtn, winEvents := eui.NewButton()
	winBtn.Text = "Windows"
	winBtn.Size = eui.Point{X: buttonWidth, Y: buttonHeight}
	winBtn.FontSize = toolFontSize
	winEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			windowsWin.ToggleNear(ev.Item)
		}
	}
	if twoRows {
		row1.AddItem(winBtn)
	} else {
		menu.AddItem(winBtn)
	}

	btn, setEvents := eui.NewButton()
	btn.Text = "Settings"
	btn.Size = eui.Point{X: buttonWidth, Y: buttonHeight}
	btn.FontSize = toolFontSize
	setEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			settingsWin.ToggleNear(ev.Item)
		}
	}
	if twoRows {
		row1.AddItem(btn)
	} else {
		menu.AddItem(btn)
	}

	helpBtn, helpEvents := eui.NewButton()
	helpBtn.Text = "Help"
	helpBtn.Size = eui.Point{X: buttonWidth, Y: buttonHeight}
	helpBtn.FontSize = toolFontSize
	helpEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			helpWin.ToggleNear(ev.Item)
		}
	}
	if twoRows {
		row1.AddItem(helpBtn)
	} else {
		menu.AddItem(helpBtn)
	}

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
	if twoRows {
		row2.AddItem(volumeSlider)
	} else {
		menu.AddItem(volumeSlider)
	}

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
	if twoRows {
		row2.AddItem(muteBtn)
	} else {
		menu.AddItem(muteBtn)
	}

	exitSessBtn, exitSessEv := eui.NewButton()
	exitSessBtn.Text = "Exit"
	exitSessBtn.Size = eui.Point{X: buttonWidth, Y: buttonHeight}
	exitSessBtn.FontSize = toolFontSize
	exitSessEv.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			confirmExitSession()
		}
	}
	if twoRows {
		row2.AddItem(exitSessBtn)
	} else {
		menu.AddItem(exitSessBtn)
	}

	recordStatus, _ = eui.NewText()
	recordStatus.Text = ""
	recordStatus.Size = eui.Point{X: 80, Y: buttonHeight}
	recordStatus.FontSize = toolFontSize
	recordStatus.Color = eui.ColorRed
	if twoRows {
		row2.AddItem(recordStatus)
	} else {
		menu.AddItem(recordStatus)
	}

	if twoRows {
		menu.AddItem(row1)
		menu.AddItem(row2)
	}

	return menu
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
	toolbarWin.Size = eui.Point{X: 500, Y: 35}
	toolbarWin.SetZone(eui.HZoneCenter, eui.VZoneTop)

	toolbarWin.AddItem(buildToolbar(toolFontSize, buttonWidth, buttonHeight, false))
	toolbarWin.AddWindow(false)
	toolbarWin.MarkOpen()

	//eui.TreeMode = true
}

func makeHUDWindow() {
	if hudWin != nil {
		return
	}
	var toolFontSize float32 = 10
	var buttonHeight float32 = 15
	var buttonWidth float32 = 64

	hudWin = eui.NewWindow()
	hudWin.Title = "HUD"
	hudWin.Closable = true
	hudWin.Resizable = false
	hudWin.AutoSize = true
	hudWin.Movable = true
	hudWin.NoScroll = true
	hudWin.SetZone(eui.HZoneLeft, eui.VZoneTop)

	flow := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_HORIZONTAL}
	hands := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_HORIZONTAL}
	leftHandImg, _ = eui.NewImageItem(32, 32)
	leftHandImg.Margin = 2
	rightHandImg, _ = eui.NewImageItem(32, 32)
	rightHandImg.Margin = 2
	hands.AddItem(leftHandImg)
	hands.AddItem(rightHandImg)
	flow.AddItem(hands)
	flow.AddItem(buildToolbar(toolFontSize, buttonWidth, buttonHeight, true))

	hudWin.AddItem(flow)
	hudWin.AddWindow(false)
	hudWin.MarkOpen()
	updateHandsWindow()
}

func updateHandsWindow() {
	if rightHandImg == nil || leftHandImg == nil {
		return
	}
	rightID, leftID := equippedItemPicts()
	var rightImg, leftImg *ebiten.Image
	if rightID == 0 && leftID == 0 {
		img := loadImage(defaultHandPictID)
		if img == nil {
			return
		}
		rightImg = img
		leftImg = mirrorImage(img)
	} else {
		if rightID != 0 {
			rightImg = loadImage(rightID)
		}
		if leftID != 0 {
			if img := loadImage(leftID); img != nil {
				leftImg = mirrorImage(img)
			}
		}
	}
	if rightImg != nil {
		rightHandImg.Image = rightImg
		rightHandImg.Size = eui.Point{X: float32(rightImg.Bounds().Dx()), Y: float32(rightImg.Bounds().Dy())}
		rightHandImg.Dirty = true
	}
	if leftImg != nil {
		leftHandImg.Image = leftImg
		leftHandImg.Size = eui.Point{X: float32(leftImg.Bounds().Dx()), Y: float32(leftImg.Bounds().Dy())}
		leftHandImg.Dirty = true
	}
	if hudWin != nil {
		hudWin.Refresh()
	}
}

func confirmExitSession() {
	if playingMovie {
		showPopup("Exit Movie", "Stop playback and return to login?", []popupButton{
			{Text: "Cancel"},
			{Text: "Exit", Color: &eui.ColorDarkRed, HoverColor: &eui.ColorRed, Action: func() {
				if movieWin != nil {
					movieWin.Close()
				} else {
					// Fallback: ensure login is visible
					loginWin.MarkOpen()
				}
			}},
		})
		return
	}
	if tcpConn != nil { // Connected to server
		showPopup("Exit Session", "Disconnect and return to login?", []popupButton{
			{Text: "Cancel"},
			{Text: "Disconnect", Color: &eui.ColorDarkRed, HoverColor: &eui.ColorRed, Action: func() {
				handleDisconnect()
			}},
		})
		return
	}
	// No active session; just go to login
	loginWin.MarkOpen()
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
	downloadWin.SetZone(eui.HZoneCenter, eui.VZoneMiddleTop)

	startedDownload := false

	flow := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL}

	// Live status line updated during downloads
	statusText, _ := eui.NewText()
	statusText.Text = ""
	statusText.FontSize = 13
	statusText.Size = eui.Point{X: 700, Y: 20}
	flow.AddItem(statusText)

	// Progress bar for downloads (barber pole when size unknown)
	pb, _ := eui.NewProgressBar()
	pb.Size = eui.Point{X: 700, Y: 14}
	pb.MinValue = 0
	pb.MaxValue = 1
	pb.Value = 0
	pb.Indeterminate = true
	flow.AddItem(pb)
	// Track throughput for kb/s and ETA
	var dlStart time.Time
	var currentName string
	downloadStatus = func(s string) {
		// Clear initial descriptive text once download actually begins
		statusText.Text = s
		statusText.Dirty = true
		if downloadWin != nil {
			downloadWin.Refresh()
		}
	}
	downloadProgress = func(name string, read, total int64) {
		if dlStart.IsZero() || name != currentName {
			dlStart = time.Now()
			currentName = name
		}
		// Update progress bar
		if total > 0 {
			pb.Indeterminate = false
			// Use absolute scale so ratio = (Value-Min)/(Max-Min) is robust
			pb.MinValue = 0
			pb.MaxValue = float32(total)
			pb.Value = float32(read)
		} else {
			pb.Indeterminate = true
		}
		pb.Dirty = true

		// Compose status with kb/s and ETA when possible
		elapsed := time.Since(dlStart).Seconds()
		rate := float64(read)
		if elapsed > 0 {
			rate = rate / elapsed // bytes/sec
		} else {
			rate = 0
		}
		var etaStr string
		if total > 0 && rate > 1 {
			remain := float64(total-read) / rate
			if remain < 0 {
				remain = 0
			}
			eta := time.Duration(remain) * time.Second
			// Format as M:SS for compactness
			m := int(eta.Minutes())
			s := int(eta.Seconds()) % 60
			etaStr = fmt.Sprintf(" ETA %d:%02d", m, s)
		}
		var pct string
		if total > 0 {
			pct = fmt.Sprintf(" (%.1f%%)", 100*float64(read)/float64(total))
		}
		statusText.Text = fmt.Sprintf("Downloading %s: %s/%s%s  %s/s%s",
			name,
			humanize.Bytes(uint64(read)),
			func() string {
				if total > 0 {
					return humanize.Bytes(uint64(total))
				} else {
					return "?"
				}
			}(),
			pct,
			humanize.Bytes(uint64(rate)),
			etaStr,
		)
		statusText.Dirty = true
		if downloadWin != nil {
			downloadWin.Refresh()
		}
	}

	t, _ := eui.NewText()
	t.Text = "Files we must download:"
	t.FontSize = 15
	t.Size = eui.Point{X: 320, Y: 25}
	flow.AddItem(t)

	for _, f := range status.Files {
		t, _ := eui.NewText()
		t.Text = f
		t.FontSize = 15
		t.Size = eui.Point{X: 320, Y: 25}
		flow.AddItem(t)
	}

	z, _ := eui.NewText()
	z.Text = ""
	z.FontSize = 15
	z.Size = eui.Point{X: 320, Y: 25}
	flow.AddItem(z)

	// Helper to start the download process; reused by Download and Retry
	var startDownload func()
	startDownload = func() {
		if startedDownload {
			return
		}
		startedDownload = true
		// Reset UI state
		dlStart = time.Time{}
		currentName = ""
		pb.Indeterminate = true
		pb.MinValue = 0
		pb.MaxValue = 1
		pb.Value = 0
		pb.Dirty = true
		statusText.Dirty = true
		// Show only the live status + progress while downloading
		flow.Contents = []*eui.ItemData{statusText, pb}
		downloadWin.Refresh()
		go func() {
			dlMutex.Lock()
			defer dlMutex.Unlock()

			if err := downloadDataFiles(clientVersion, status); err != nil {
				logError("download data files: %v", err)
				// Present inline Retry and Quit buttons
				retryRow := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_HORIZONTAL}
				retryBtn, retryEvents := eui.NewButton()
				retryBtn.Text = "Retry"
				retryBtn.Size = eui.Point{X: 100, Y: 24}
				retryEvents.Handle = func(ev eui.UIEvent) {
					if ev.Type == eui.EventClick {
						startedDownload = false
						startDownload()
					}
				}
				retryRow.AddItem(retryBtn)

				quitBtn, quitEvents := eui.NewButton()
				quitBtn.Text = "Quit"
				quitBtn.Size = eui.Point{X: 100, Y: 24}
				quitEvents.Handle = func(ev eui.UIEvent) {
					if ev.Type == eui.EventClick {
						confirmQuit()
					}
				}
				retryRow.AddItem(quitBtn)

				flow.AddItem(retryRow)
				startedDownload = false
				downloadWin.Refresh()
				return
			}
			img, err := climg.Load(filepath.Join(dataDirPath, CL_ImagesFile))
			if err != nil {
				logError("failed to load CL_Images: %v", err)
				return
			} else {
				img.Denoise = gs.DenoiseImages
				img.DenoiseSharpness = gs.DenoiseSharpness
				img.DenoisePercent = gs.DenoisePercent
				clImages = img
			}

			clSounds, err = clsnd.Load(filepath.Join("data/CL_Sounds"))
			if err != nil {
				logError("failed to load CL_Sounds: %v", err)
				return
			}
			// Reload characters in case data dir was created during download
			loadCharacters()
			// Force reselect from LastCharacter if available
			name = ""
			passHash = ""
			updateCharacterButtons()
			if loginWin != nil {
				loginWin.Refresh()
			}
			// Clear the callback to avoid stray updates after closing.
			downloadStatus = nil
			downloadProgress = nil
			downloadWin.Close()
			loginWin.MarkOpen()
		}()
	}

	btnFlow := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_HORIZONTAL}
	dlBtn, dlEvents := eui.NewButton()
	dlBtn.Text = "Download"
	dlBtn.Size = eui.Point{X: 100, Y: 24}
	dlEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			startDownload()
		}
	}
	btnFlow.AddItem(dlBtn)

	closeBtn, closeEvents := eui.NewButton()
	closeBtn.Text = "Quit"
	closeBtn.Size = eui.Point{X: 100, Y: 24}
	closeEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			confirmQuit()
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
		empty.Text = "No characters, click add!"
		empty.FontSize = 14
		empty.Size = eui.Point{X: 200, Y: 64}
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
					// Rebuild the list so only the selected radio is checked
					// across all rows and refresh the login UI immediately.
					updateCharacterButtons()
					if loginWin != nil {
						loginWin.Refresh()
					}
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
	//addCharWin.SetZone(eui.HZoneCenterLeft, eui.VZoneMiddleTop)

	flow := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL}

	nameInput, _ := eui.NewInput()
	nameInput.Label = "Character"
	nameInput.TextPtr = &addCharName
	nameInput.Size = eui.Point{X: 200, Y: 24}
	addCharNameInput = nameInput
	flow.AddItem(nameInput)
	passInput, _ := eui.NewInput()
	passInput.Label = "Password"
	passInput.TextPtr = &addCharPass
	passInput.Size = eui.Point{X: 200, Y: 24}
	addCharPassInput = passInput
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
				// Reload to ensure in-memory state matches persisted data.
				loadCharacters()
			}
			// Update selection to the newly added character
			name = addCharName
			passHash = hash
			gs.LastCharacter = addCharName
			saveSettings()
			// Ensure the login window is open before updating its contents
			if loginWin != nil {
				loginWin.MarkOpen()
			}
			// Refresh the login UI to show the new character immediately
			updateCharacterButtons()
			if loginWin != nil {
				loginWin.Refresh()
			}
			// Clear the add-character inputs for good UX on repeat adds
			addCharName = ""
			addCharPass = ""
			if addCharNameInput != nil {
				addCharNameInput.Text = ""
				addCharNameInput.Dirty = true
			}
			if addCharPassInput != nil {
				addCharPassInput.Text = ""
				addCharPassInput.Dirty = true
			}
			// Return user to login (already open above)
			addCharWin.Close()
		}
	}
	flow.AddItem(addBtn)

	cancelBtn, cancelEvents := eui.NewButton()
	cancelBtn.Text = "Cancel"
	cancelBtn.Size = eui.Point{X: 200, Y: 24}
	cancelEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			addCharWin.Close()
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
	loginWin.SetZone(eui.HZoneCenter, eui.VZoneMiddleTop)
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
			addCharWin.MarkOpenNear(ev.Item)
		}
	}
	loginFlow.AddItem(addBtn)

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
				makeErrorWindow("Error: Login: login is empty")
				return
			}
			// Ensure a password exists (either stored hash or plain) before attempting login
			if !demo && passHash == "" && pass == "" {
				makeErrorWindow("Error: Login: password is empty")
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
					// Bring login forward first so the popup stays on top
					loginWin.MarkOpen()
					makeErrorWindow("Error: Login: " + err.Error())
				}
			}()
		}
	}
	loginFlow.AddItem(connBtn)

	label2, _ := eui.NewText()
	label2.Text = ""
	label2.FontSize = 15
	label2.Size = eui.Point{X: 1, Y: 25}
	loginFlow.AddItem(label2)

	openBtn, openEvents := eui.NewButton()
	openBtn.Text = "Play movie file (clMov)"
	openBtn.Size = eui.Point{X: 200, Y: 24}
	openEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			filename, err := dialog.File().Filter("clMov files", "clMov", "clmov").Load()
			if err != nil {
				if err != dialog.Cancelled {
					logError("open clMov: %v", err)
					// Keep popup on top of login
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
					loginWin.MarkOpen()
					makeErrorWindow("Error: Open clMov: " + err.Error())
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
	quitBttn, quitEvn := eui.NewButton()
	quitBttn.Text = "Quit"
	quitBttn.Size = eui.Point{X: 200, Y: 24}
	quitEvn.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			confirmQuit()
		}
	}
	loginFlow.AddItem(quitBttn)
	loginWin.AddItem(loginFlow)
	loginWin.AddWindow(false)
}

// explainError returns a plain-English explanation and suggestions for an error message.
func explainError(msg string) string {
	m := strings.ToLower(msg)
	switch {
	case strings.Contains(m, "login is empty"):
		return "No character selected. Choose a character or add one before connecting."
	case strings.Contains(m, "password is empty"):
		return "No password provided. Enter or save a password for this character, then try again."
	case strings.Contains(m, "tcp connect") || strings.Contains(m, "udp connect") || strings.Contains(m, "connection refused") || strings.Contains(m, "dial"):
		return "Can't reach the server. Check your internet connection, the server address/port, and any firewall/VPN rules."
	case strings.Contains(m, "auto update") || strings.Contains(m, "download ") || strings.Contains(m, "http error") || strings.Contains(m, "gzip reader"):
		return "The game data download failed. Check network connectivity, disk space, and that the data directory is writable, then try again."
	case strings.Contains(m, "permission denied"):
		return "Operation not permitted. Ensure the app has permission to read/write the required files or try a different folder."
	case strings.Contains(m, "no such file") || strings.Contains(m, "file not found"):
		return "The file path does not exist. Verify the path and that the file is present."
	case strings.Contains(m, "open clmov"):
		return "Couldn't open the .clMov file. Make sure the file exists and is readable."
	case strings.Contains(m, "record movie"):
		return "Couldn't start recording. Ensure the destination folder is writable and there is enough free space."
	case strings.Contains(m, "login failed") || strings.Contains(m, "error: login"):
		return "Login failed. Verify your character name and password, and that the account has available characters."
	case strings.Contains(m, "x11") || strings.Contains(m, "display"):
		return "No display detected. If running remotely/headless, set DISPLAY or run in a desktop session."
	default:
		// Try to extract a kError code from the message and convert it.
		re := regexp.MustCompile(`-?\d+`)
		if loc := re.FindString(msg); loc != "" {
			if v, err := strconv.Atoi(loc); err == nil {
				if desc, name, ok := describeKError(int16(v)); ok {
					return fmt.Sprintf("%s (%s %d)", desc, name, v)
				}
			}
		}
		return "An error occurred. Try again. If it persists, check the console logs for details."
	}
}

func makeErrorWindow(msg string) {
	body := msg + "\n" + explainError(msg)
	showPopup("Error", body, []popupButton{{Text: "OK"}})
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

	// Split settings into two columns: simple (left) and advanced (right)
	var leftW float32 = 270
	var rightW float32 = 270
	outer := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_HORIZONTAL}
	left := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL}
	left.Size = eui.Point{X: leftW, Y: 10}
	right := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL}
	right.Size = eui.Point{X: rightW, Y: 10}

	// (Reset button added at the bottom-right later)

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
	themeDD.Size = eui.Point{X: rightW, Y: 24}
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
	right.AddItem(themeDD)

	label, _ := eui.NewText()
	label.Text = "\nControls:"
	label.FontSize = 15
	label.Size = eui.Point{X: leftW, Y: 50}
	left.AddItem(label)

	toggle, toggleEvents := eui.NewCheckbox()
	toggle.Text = "Click-to-toggle movement"
	toggle.Size = eui.Point{X: leftW, Y: 24}
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
	left.AddItem(toggle)

	keySpeedSlider, keySpeedEvents := eui.NewSlider()
	keySpeedSlider.Label = "Keyboard Walk Speed"
	keySpeedSlider.MinValue = 0.1
	keySpeedSlider.MaxValue = 1.0
	keySpeedSlider.Value = float32(gs.KBWalkSpeed)
	keySpeedSlider.Size = eui.Point{X: leftW - 10, Y: 24}
	keySpeedEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventSliderChanged {
			gs.KBWalkSpeed = float64(ev.Value)
			settingsDirty = true
		}
	}
	left.AddItem(keySpeedSlider)

	label, _ = eui.NewText()
	label.Text = "\nWindow Behavior:"
	label.FontSize = 15
	label.Size = eui.Point{X: rightW, Y: 50}
	right.AddItem(label)

	tilingCB, tilingEvents := eui.NewCheckbox()
	tilingCB.Text = "Tiling window mode (buggy)"
	tilingCB.Size = eui.Point{X: rightW, Y: 24}
	tilingCB.Checked = gs.WindowTiling
	tilingCB.Tooltip = "Prevent windows from overlapping"
	tilingEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			gs.WindowTiling = ev.Checked
			eui.SetWindowTiling(ev.Checked)
			settingsDirty = true
		}
	}
	right.AddItem(tilingCB)

	snapCB, snapEvents := eui.NewCheckbox()
	snapCB.Text = "Window snapping (buggy)"
	snapCB.Size = eui.Point{X: rightW, Y: 24}
	snapCB.Checked = gs.WindowSnapping
	snapCB.Tooltip = "Snap windows to edges and others"
	snapEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			gs.WindowSnapping = ev.Checked
			eui.SetWindowSnapping(ev.Checked)
			settingsDirty = true
		}
	}
	right.AddItem(snapCB)

	// Screen size settings in-place (moved from separate window)
	uiScaleSlider, uiScaleEvents := eui.NewSlider()
	uiScaleSlider.Label = "UI Scaling"
	uiScaleSlider.MinValue = 1.0
	uiScaleSlider.MaxValue = 2.5
	uiScaleSlider.Value = float32(gs.UIScale)
	pendingUIScale := gs.UIScale
	uiScaleEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventSliderChanged {
			pendingUIScale = float64(ev.Value)
		}
	}

	uiScaleApplyBtn, uiScaleApplyEvents := eui.NewButton()
	uiScaleApplyBtn.Text = "Apply UI Scale"
	uiScaleApplyBtn.Size = eui.Point{X: 140, Y: 24}
	uiScaleApplyEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			gs.UIScale = pendingUIScale
			eui.SetUIScale(float32(gs.UIScale))
			updateGameWindowSize()
			settingsDirty = true
		}
	}

	// Place the slider and button on the same row
	uiScaleRow := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_HORIZONTAL}
	// Fit slider to remaining width in the row
	uiScaleSlider.Size = eui.Point{X: rightW - uiScaleApplyBtn.Size.X - 10, Y: 24}
	uiScaleRow.AddItem(uiScaleSlider)
	uiScaleRow.AddItem(uiScaleApplyBtn)
	right.AddItem(uiScaleRow)

	fullscreenCB, fullscreenEvents := eui.NewCheckbox()
	fullscreenCB.Text = "Fullscreen"
	fullscreenCB.Size = eui.Point{X: rightW, Y: 24}
	fullscreenCB.Checked = gs.Fullscreen
	fullscreenEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			gs.Fullscreen = ev.Checked
			ebiten.SetFullscreen(gs.Fullscreen)
			ebiten.SetWindowFloating(gs.Fullscreen)
			settingsDirty = true
		}
	}
	right.AddItem(fullscreenCB)

	renderScale, renderScaleEvents := eui.NewSlider()
	renderScale.Label = "Render Scale"
	renderScale.MinValue = 1
	renderScale.MaxValue = 4
	renderScale.IntOnly = true
	if gs.GameScale < 1 {
		gs.GameScale = 1
	}
	if gs.GameScale > 10 {
		gs.GameScale = 10
	}
	renderScale.Value = float32(math.Round(gs.GameScale))
	renderScale.Size = eui.Point{X: rightW - 10, Y: 24}
	renderScale.Tooltip = "Game render zoom (1x–10x). In Integer mode uses nearest filtering."
	renderScaleEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventSliderChanged {
			v := math.Round(float64(ev.Value))
			if v < 1 {
				v = 1
			}
			if v > 10 {
				v = 10
			}
			gs.GameScale = v
			renderScale.Value = float32(v)
			settingsDirty = true
			if gameWin != nil {
				gameWin.Refresh()
			}
		}
	}
	right.AddItem(renderScale)

	/*
		intCB, intEvents := eui.NewCheckbox()
		intCB.Text = "Integer scale (sharper, faster)"
		intCB.Size = eui.Point{X: rightW, Y: 24}
		intCB.Checked = gs.IntegerScaling
		intEvents.Handle = func(ev eui.UIEvent) {
			if ev.Type == eui.EventCheckboxChanged {
				gs.IntegerScaling = ev.Checked
				initFont()
				if gameWin != nil {
					gameWin.Refresh()
				}
				if graphicsWin != nil {
					graphicsWin.Refresh()
				}
				if debugWin != nil {
					debugWin.Refresh()
				}
			}
		}
		right.AddItem(intCB)
	*/

	label, _ = eui.NewText()
	label.Text = "\nText Sizes:"
	label.FontSize = 15
	label.Size = eui.Point{X: leftW, Y: 50}
	left.AddItem(label)

	chatFontSlider, chatFontEvents := eui.NewSlider()
	chatFontSlider.Label = "Chat Bubble Font Size"
	chatFontSlider.MinValue = 6
	chatFontSlider.MaxValue = 48
	chatFontSlider.Value = float32(gs.BubbleFontSize)
	chatFontSlider.Size = eui.Point{X: leftW - 10, Y: 24}
	chatFontEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventSliderChanged {
			gs.BubbleFontSize = float64(ev.Value)
			initFont()
			settingsDirty = true
		}
	}
	left.AddItem(chatFontSlider)

	labelFontSlider, labelFontEvents := eui.NewSlider()
	labelFontSlider.Label = "Name Font Size"
	labelFontSlider.MinValue = 5
	labelFontSlider.MaxValue = 48
	labelFontSlider.Value = float32(gs.MainFontSize)
	labelFontSlider.Size = eui.Point{X: leftW - 10, Y: 24}
	labelFontEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventSliderChanged {
			gs.MainFontSize = float64(ev.Value)
			initFont()
			settingsDirty = true
		}
	}
	left.AddItem(labelFontSlider)

	// Inventory font size slider
	invFontSlider, invFontEvents := eui.NewSlider()
	invFontSlider.Label = "Inventory Font Size"
	invFontSlider.MinValue = 5
	invFontSlider.MaxValue = 48
	invFontSlider.Value = func() float32 {
		if gs.InventoryFontSize > 0 {
			return float32(gs.InventoryFontSize)
		}
		return float32(gs.ConsoleFontSize)
	}()
	invFontSlider.Size = eui.Point{X: leftW - 10, Y: 24}
	invFontEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventSliderChanged {
			gs.InventoryFontSize = float64(ev.Value)
			settingsDirty = true
			updateInventoryWindow()
		}
	}
	left.AddItem(invFontSlider)

	// Players list font size slider
	plFontSlider, plFontEvents := eui.NewSlider()
	plFontSlider.Label = "Players List Font Size"
	plFontSlider.MinValue = 5
	plFontSlider.MaxValue = 48
	plFontSlider.Value = func() float32 {
		if gs.PlayersFontSize > 0 {
			return float32(gs.PlayersFontSize)
		}
		return float32(gs.ConsoleFontSize)
	}()
	plFontSlider.Size = eui.Point{X: leftW - 10, Y: 24}
	plFontEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventSliderChanged {
			gs.PlayersFontSize = float64(ev.Value)
			settingsDirty = true
			updatePlayersWindow()
			if playersWin != nil {
				playersWin.Refresh()
			}
		}
	}
	left.AddItem(plFontSlider)

	consoleFontSlider, consoleFontEvents := eui.NewSlider()
	consoleFontSlider.Label = "Console Font Size"
	consoleFontSlider.MinValue = 6
	consoleFontSlider.MaxValue = 24
	consoleFontSlider.Value = float32(gs.ConsoleFontSize)
	consoleFontSlider.Size = eui.Point{X: leftW - 10, Y: 24}
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
	left.AddItem(consoleFontSlider)

	chatWindowFontSlider, chatWindowFontEvents := eui.NewSlider()
	chatWindowFontSlider.Label = "Chat Window Font Size"
	chatWindowFontSlider.MinValue = 6
	chatWindowFontSlider.MaxValue = 24
	chatWindowFontSlider.Value = float32(gs.ChatFontSize)
	chatWindowFontSlider.Size = eui.Point{X: leftW - 10, Y: 24}
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
	left.AddItem(chatWindowFontSlider)

	label, _ = eui.NewText()
	label.Text = "\nOpacity Settings:"
	label.FontSize = 15
	label.Size = eui.Point{X: leftW, Y: 30}
	left.AddItem(label)

	bubbleOpSlider, bubbleOpEvents := eui.NewSlider()
	bubbleOpSlider.Label = "Bubble Opacity"
	bubbleOpSlider.MinValue = 0
	bubbleOpSlider.MaxValue = 1
	bubbleOpSlider.Value = float32(gs.BubbleOpacity)
	bubbleOpSlider.Size = eui.Point{X: leftW - 10, Y: 24}
	bubbleOpEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventSliderChanged {
			gs.BubbleOpacity = float64(ev.Value)
			settingsDirty = true
		}
	}
	left.AddItem(bubbleOpSlider)

	nameBgSlider, nameBgEvents := eui.NewSlider()
	nameBgSlider.Label = "Name Background Opacity"
	nameBgSlider.MinValue = 0
	nameBgSlider.MaxValue = 1
	nameBgSlider.Value = float32(gs.NameBgOpacity)
	nameBgSlider.Size = eui.Point{X: leftW - 10, Y: 24}
	nameBgEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventSliderChanged {
			gs.NameBgOpacity = float64(ev.Value)
			settingsDirty = true
		}
	}
	left.AddItem(nameBgSlider)

	label, _ = eui.NewText()
	label.Text = "\nQuality Settings:"
	label.FontSize = 15
	label.Size = eui.Point{X: rightW, Y: 50}
	right.AddItem(label)

	qualityPresetDD, qpEvents := eui.NewDropdown()
	qualityPresetDD.Options = []string{"Ultra-Low", "Low", "Standard", "High", "Ultimate", "Custom"}
	qualityPresetDD.Size = eui.Point{X: rightW, Y: 24}
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
	right.AddItem(qualityPresetDD)

	qualityBtn, qualityEvents := eui.NewButton()
	qualityBtn.Text = "Quality Options"
	qualityBtn.Size = eui.Point{X: rightW, Y: 24}
	qualityEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			qualityWin.ToggleNear(ev.Item)
		}
	}
	right.AddItem(qualityBtn)

	label, _ = eui.NewText()
	label.Text = ""
	label.Size = eui.Point{X: rightW, Y: 15}
	right.AddItem(label)

	debugBtn, debugEvents := eui.NewButton()
	debugBtn.Text = "Debug Settings"
	debugBtn.Size = eui.Point{X: rightW, Y: 24}
	debugEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			debugWin.ToggleNear(ev.Item)
		}
	}
	right.AddItem(debugBtn)

	// Bottom-right: Reset All Settings
	resetBtn, resetEv := eui.NewButton()
	resetBtn.Text = "Reset All Settings"
	resetBtn.Size = eui.Point{X: rightW, Y: 24}
	resetBtn.Color = eui.ColorDarkRed
	resetBtn.HoverColor = eui.ColorRed
	resetBtn.Tooltip = "Restore defaults and reapply"
	resetEv.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			confirmResetSettings()
		}
	}
	right.AddItem(resetBtn)

	outer.AddItem(left)
	outer.AddItem(right)
	settingsWin.AddItem(outer)
	settingsWin.AddWindow(false)
}

// resetAllSettings restores gs to defaults, reapplies, and refreshes windows.
func resetAllSettings() {
	gs = gsdef
	clampWindowSettings()
	applySettings()
	updateGameWindowSize()
	saveSettings()
	settingsDirty = false

	if consoleWin != nil {
		updateConsoleWindow()
		consoleWin.Refresh()
	}
	if chatWin != nil {
		updateChatWindow()
		chatWin.Refresh()
	}
	if graphicsWin != nil {
		graphicsWin.Refresh()
	}
	if qualityWin != nil {
		qualityWin.Refresh()
	}

	// Rebuild the Settings window UI so control values match defaults
	if settingsWin != nil {
		settingsWin.Close()
		settingsWin = nil
		makeSettingsWindow()
		settingsWin.MarkOpen()
	}
}

// popupButton defines a button in a popup dialog.
type popupButton struct {
	Text       string
	Color      *eui.Color
	HoverColor *eui.Color
	Action     func()
}

// showPopup creates a simple modal-like popup with a message and buttons.
func showPopup(title, message string, buttons []popupButton) *eui.WindowData {
	win := eui.NewWindow()
	win.Title = title
	win.Closable = false
	win.Resizable = false
	win.AutoSize = true
	win.Movable = true
	win.SetZone(eui.HZoneCenter, eui.VZoneMiddleTop)
	// Add some breathing room so text doesn't hug the border
	win.Padding = 8
	win.BorderPad = 4

	flow := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL}
	// Message (wrapped to a reasonable width)
	uiScale := eui.UIScale()
	targetWidthPx := float64(520)
	// Add horizontal padding on both sides to avoid right-edge clipping.
	hpadPx := float64(24)
	padUnits := float32(hpadPx / float64(uiScale))
	// targetWidthUnits not used directly; inner width sets actual text width
	// Match renderer size: (FontSize*uiScale)+2
	facePx := float64(12*uiScale + 2)
	var face text.Face
	if src := eui.FontSource(); src != nil {
		face = &text.GoTextFace{Source: src, Size: facePx}
	} else {
		face = &text.GoTextFace{Size: facePx}
	}
	// Wrap to inner width (minus horizontal padding)
	innerPx := targetWidthPx - 2*hpadPx
	if innerPx < 50 {
		innerPx = 50
	}
	_, lines := wrapText(message, face, innerPx)
	wrapped := strings.Join(lines, "\n")
	gm := face.Metrics()
	lineHpx := float64(gm.HAscent + gm.HDescent)
	if lineHpx < 14 {
		lineHpx = 14
	}
	heightUnits := float32((lineHpx*float64(len(lines)) + 8) / float64(uiScale))
	if heightUnits < 24 {
		heightUnits = 24
	}
	txt, _ := eui.NewText()
	txt.Text = wrapped
	txt.FontSize = 12
	// Slight width fudge to avoid right-edge clipping from rounding
	fudgeUnits := float32(2.0 / float64(uiScale))
	txt.Size = eui.Point{X: float32(innerPx/float64(uiScale)) + fudgeUnits, Y: heightUnits}
	txt.Position = eui.Point{X: padUnits, Y: 0}
	flow.AddItem(txt)

	// Buttons row
	btnRow := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_HORIZONTAL}
	for _, b := range buttons {
		btn, ev := eui.NewButton()
		btn.Text = b.Text
		btn.Size = eui.Point{X: 120, Y: 24}
		if b.Color != nil {
			btn.Color = *b.Color
		}
		if b.HoverColor != nil {
			btn.HoverColor = *b.HoverColor
		}
		action := b.Action
		ev.Handle = func(ev eui.UIEvent) {
			if ev.Type == eui.EventClick {
				if action != nil {
					action()
				}
				win.Close()
			}
		}
		btnRow.AddItem(btn)
	}
	flow.AddItem(btnRow)

	win.AddItem(flow)
	win.AddWindow(false)
	win.MarkOpen()
	return win
}

func confirmResetSettings() {
	// Use a red confirm button to indicate a destructive action
	showPopup(
		"Confirm Reset",
		"Reset all settings to defaults? This cannot be undone.",
		[]popupButton{
			{Text: "Cancel"},
			{Text: "Reset", Color: &eui.ColorDarkRed, HoverColor: &eui.ColorRed, Action: func() { resetAllSettings() }},
		},
	)
}

func confirmQuit() {
	showPopup(
		"Confirm Quit",
		"Quit the app? Unsaved state will be lost.",
		[]popupButton{
			{Text: "Cancel"},
			{Text: "Quit", Color: &eui.ColorDarkRed, HoverColor: &eui.ColorRed, Action: func() {
				saveCharacters()
				saveSettings()
				os.Exit(0)
			}},
		},
	)
}

func makeGraphicsWindow() {
	if graphicsWin != nil {
		return
	}
	// Column widths
	var leftW float32 = 260
	var rightW float32 = 260

	graphicsWin = eui.NewWindow()
	graphicsWin.Title = "Screen Size Settings"
	graphicsWin.Closable = true
	graphicsWin.Resizable = false
	graphicsWin.AutoSize = true
	graphicsWin.Movable = true
	graphicsWin.SetZone(eui.HZoneCenterLeft, eui.VZoneMiddleTop)

	// Outer horizontal flow with two vertical columns
	outer := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_HORIZONTAL}

	simple := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL}
	simple.Size = eui.Point{X: leftW, Y: 10}

	advanced := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL}
	advanced.Size = eui.Point{X: rightW, Y: 10}

	// Simple (left) controls
	uiScaleSlider, uiScaleEvents := eui.NewSlider()
	uiScaleSlider.Label = "UI Scaling"
	uiScaleSlider.MinValue = 1.0
	uiScaleSlider.MaxValue = 2.5
	uiScaleSlider.Value = float32(gs.UIScale)
	pendingUIScale := gs.UIScale
	uiScaleEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventSliderChanged {
			pendingUIScale = float64(ev.Value)
		}
	}

	uiScaleApplyBtn, uiScaleApplyEvents := eui.NewButton()
	uiScaleApplyBtn.Text = "Apply UI Scale"
	uiScaleApplyBtn.Size = eui.Point{X: 140, Y: 24}
	uiScaleApplyEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			gs.UIScale = pendingUIScale
			eui.SetUIScale(float32(gs.UIScale))
			updateGameWindowSize()
			settingsDirty = true
		}
	}

	// Place the slider and button on the same row
	uiScaleRow := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_HORIZONTAL}
	uiScaleSlider.Size = eui.Point{X: leftW - uiScaleApplyBtn.Size.X - 10, Y: 24}
	uiScaleRow.AddItem(uiScaleSlider)
	uiScaleRow.AddItem(uiScaleApplyBtn)
	simple.AddItem(uiScaleRow)

	fullscreenCB, fullscreenEvents := eui.NewCheckbox()
	fullscreenCB.Text = "Fullscreen"
	fullscreenCB.Size = eui.Point{X: leftW, Y: 24}
	fullscreenCB.Checked = gs.Fullscreen
	fullscreenEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			gs.Fullscreen = ev.Checked
			ebiten.SetFullscreen(gs.Fullscreen)
			ebiten.SetWindowFloating(gs.Fullscreen)
			settingsDirty = true
		}
	}
	simple.AddItem(fullscreenCB)

	// Render scale (world zoom) 1x..10x, defaults to 2x
	renderScale, renderScaleEvents := eui.NewSlider()
	renderScale.Label = "Render Scale"
	renderScale.MinValue = 1
	renderScale.MaxValue = 10
	renderScale.IntOnly = true
	// Snap to integer steps by rounding the current setting
	if gs.GameScale < 1 {
		gs.GameScale = 1
	}
	if gs.GameScale > 10 {
		gs.GameScale = 10
	}
	renderScale.Value = float32(math.Round(gs.GameScale))
	renderScale.Size = eui.Point{X: leftW - 10, Y: 24}
	renderScale.Tooltip = "Game render zoom (1x–10x). In Integer mode uses nearest filtering."
	renderScaleEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventSliderChanged {
			// Round to integer steps and clamp
			v := math.Round(float64(ev.Value))
			if v < 1 {
				v = 1
			}
			if v > 10 {
				v = 10
			}
			gs.GameScale = v
			// Reflect the snapped value on the slider UI
			renderScale.Value = float32(v)
			settingsDirty = true
			if gameWin != nil {
				gameWin.Refresh()
			}
		}
	}
	simple.AddItem(renderScale)

	// Advanced (right) controls
	intCB, intEvents := eui.NewCheckbox()
	intCB.Text = "Integer scale (sharper, faster)"
	intCB.Size = eui.Point{X: rightW, Y: 24}
	intCB.Checked = gs.IntegerScaling
	intEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			gs.IntegerScaling = ev.Checked
			initFont()
			if gameWin != nil {
				gameWin.Refresh()
			}
			settingsDirty = true
		}
	}
	advanced.AddItem(intCB)

	tbMaxCB, tbMaxEvents := eui.NewCheckbox()
	tbMaxCB.Text = "Show titlebar Maximize button"
	tbMaxCB.Size = eui.Point{X: rightW, Y: 24}
	tbMaxCB.Checked = gs.TitlebarMaximize
	tbMaxEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			gs.TitlebarMaximize = ev.Checked
			if gameWin != nil {
				gameWin.Maximizable = ev.Checked
				gameWin.Refresh()
			}
			settingsDirty = true
		}
	}
	advanced.AddItem(tbMaxCB)

	outer.AddItem(simple)
	outer.AddItem(advanced)
	graphicsWin.AddItem(outer)
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

	nsCB, noSmoothEvents := eui.NewCheckbox()
	noSmoothCB = nsCB
	noSmoothCB.Text = "Smooth moving objects,glitchy WIP"
	noSmoothCB.Size = eui.Point{X: width, Y: 24}
	noSmoothCB.Checked = gs.smoothMoving
	noSmoothCB.Tooltip = "Smooth moving objects that are not 'mobiles' such as chains, clouds, etc"
	noSmoothEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			gs.smoothMoving = ev.Checked
			settingsDirty = true
		}
	}
	flow.AddItem(noSmoothCB)

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
	//windowsWin.SetZone(eui.HZoneCenterLeft, eui.VZoneMiddleTop)

	flow := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL}

	playersBox, playersBoxEvents := eui.NewCheckbox()
	windowsPlayersCB = playersBox
	playersBox.Text = "Players"
	playersBox.Size = eui.Point{X: 128, Y: 24}
	playersBox.Checked = playersWin != nil && playersWin.IsOpen()
	playersBoxEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			if ev.Checked {
				playersWin.MarkOpenNear(ev.Item)
			} else {
				playersWin.Close()
			}
		}
	}
	flow.AddItem(playersBox)

	inventoryBox, inventoryBoxEvents := eui.NewCheckbox()
	windowsInventoryCB = inventoryBox
	inventoryBox.Text = "Inventory"
	inventoryBox.Size = eui.Point{X: 128, Y: 24}
	inventoryBox.Checked = inventoryWin != nil && inventoryWin.IsOpen()
	inventoryBoxEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			if ev.Checked {
				inventoryWin.MarkOpenNear(ev.Item)
			} else {
				inventoryWin.Close()
			}
		}
	}
	flow.AddItem(inventoryBox)

	chatBox, chatBoxEvents := eui.NewCheckbox()
	windowsChatCB = chatBox
	chatBox.Text = "Chat"
	chatBox.Size = eui.Point{X: 128, Y: 24}
	chatBox.Checked = chatWin != nil && chatWin.IsOpen()
	chatBoxEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			if ev.Checked {
				chatWin.MarkOpenNear(ev.Item)
			} else {
				chatWin.Close()
			}
		}
	}
	flow.AddItem(chatBox)

	consoleBox, consoleBoxEvents := eui.NewCheckbox()
	windowsConsoleCB = consoleBox
	consoleBox.Text = "Console"
	consoleBox.Size = eui.Point{X: 128, Y: 24}
	consoleBox.Checked = consoleWin != nil && consoleWin.IsOpen()
	consoleBoxEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			if ev.Checked {
				consoleWin.MarkOpenNear(ev.Item)
			} else {
				consoleWin.Close()
			}
		}
	}
	flow.AddItem(consoleBox)

	windowsWin.AddItem(flow)
	windowsWin.AddWindow(false)

}

func makePlayersWindow() {
	if playersWin != nil {
		return
	}
	// Use the common text window scaffold to get an inner scrollable list
	// and consistent padding/behavior with Inventory/Chat windows.
	playersWin, playersList, _ = makeTextWindow("Players", eui.HZoneRight, eui.VZoneTop, false)
	// Restore saved geometry if present, otherwise keep defaults from helper.
	if gs.PlayersWindow.Size.X > 0 && gs.PlayersWindow.Size.Y > 0 {
		playersWin.Size = eui.Point{X: float32(gs.PlayersWindow.Size.X), Y: float32(gs.PlayersWindow.Size.Y)}
	}
	if gs.PlayersWindow.Position.X != 0 || gs.PlayersWindow.Position.Y != 0 {
		playersWin.Position = eui.Point{X: float32(gs.PlayersWindow.Position.X), Y: float32(gs.PlayersWindow.Position.Y)}
	}
	// Refresh contents on resize so word-wrapping and row sizing stay correct.
	playersWin.OnResize = func() { updatePlayersWindow() }
	updatePlayersWindow()
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
	//helpWin.SetZone(eui.HZoneCenterLeft, eui.VZoneMiddleTop)
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
