package main

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"gothoom/eui"

	"unicode"

	"github.com/dustin/go-humanize"
	"github.com/hajimehoshi/ebiten/v2"
	open "github.com/skratchdot/open-golang/open"

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
var precacheWin *eui.WindowData
var charactersList *eui.ItemData
var advancedWin *eui.WindowData
var connectWin *eui.WindowData
var connectStatusText *eui.ItemData
var addCharWin *eui.WindowData
var addCharName string
var addCharPass string
var addCharRemember bool
var passWin *eui.WindowData
var passInput *eui.ItemData
var passWarn *eui.ItemData
var passPrev string
var passRemember bool
var passRememberCB *eui.ItemData

var changelogWin *eui.WindowData

// applyBoldFace sets a bold text face for the given item based on its current
// FontSize and the active UI scale, so it renders as a bold section label.
func applyBoldFace(it *eui.ItemData) {
	if it == nil {
		return
	}
	sz := float64(it.FontSize*eui.UIScale() + 2)
	if src := eui.BoldFontSource(); src != nil {
		it.Face = &text.GoTextFace{Source: src, Size: sz}
	} else {
		it.Face = &text.GoTextFace{Size: sz}
	}
}

var changelogList *eui.ItemData
var changelogPrevBtn *eui.ItemData
var changelogNextBtn *eui.ItemData

// Keep references to inputs so we can clear text programmatically.
var addCharNameInput *eui.ItemData
var addCharPassInput *eui.ItemData
var addCharPassWarn *eui.ItemData
var addCharPassPrev string
var windowsWin *eui.WindowData
var scriptsWin *eui.WindowData
var scriptsList *eui.ItemData
var scriptDetails *eui.ItemData
var selectedscript string
var scriptConfigWin *eui.WindowData
var scriptConfigOwner string
var scriptDebugList *eui.ItemData

// Checkboxes in the Windows window so we can update their state live
var windowsPlayersCB *eui.ItemData
var windowsInventoryCB *eui.ItemData
var windowsChatCB *eui.ItemData
var windowsConsoleCB *eui.ItemData
var windowsHelpCB *eui.ItemData
var hudWin *eui.WindowData
var rightHandImg *eui.ItemData
var leftHandImg *eui.ItemData
var shaderWarnWin *eui.WindowData
var shaderWarnDontShowCB *eui.ItemData

var (
	sheetCacheLabel        *eui.ItemData
	frameCacheLabel        *eui.ItemData
	scaledFrameCacheLabel  *eui.ItemData
	mobileCacheLabel       *eui.ItemData
	scaledMobileCacheLabel *eui.ItemData
	soundCacheLabel        *eui.ItemData
	mobileBlendLabel       *eui.ItemData
	pictBlendLabel         *eui.ItemData
	totalCacheLabel        *eui.ItemData

	recordBtn          *eui.ItemData
	recordStatus       *eui.ItemData
	recordPath         string
	qualityPresetDD    *eui.ItemData
	shaderLightSlider  *eui.ItemData
	shaderGlowSlider   *eui.ItemData
	gammaCorrectionCB  *eui.ItemData
	spriteGammaSlider  *eui.ItemData
	monitorGammaSlider *eui.ItemData
	denoiseCB          *eui.ItemData
	motionCB           *eui.ItemData
	animCB             *eui.ItemData
	pictBlendCB        *eui.ItemData
	shaderLightingCB   *eui.ItemData
	upscaleFilterCB    *eui.ItemData
	throttleSoundCB    *eui.ItemData
	soundEnhanceCB     *eui.ItemData
	soundEnhanceSlider *eui.ItemData
	musicEnhanceCB     *eui.ItemData
	resampleAudioCB    *eui.ItemData
	precacheSoundCB    *eui.ItemData
	precacheImageCB    *eui.ItemData
	noCacheCB          *eui.ItemData
	potatoCB           *eui.ItemData
	volumeSlider       *eui.ItemData
	muteBtn            *eui.ItemData
	mixerWin           *eui.WindowData
	gameMixSlider      *eui.ItemData
	musicMixSlider     *eui.ItemData
	ttsMixSlider       *eui.ItemData
	notifMixSlider     *eui.ItemData
	mixMuteBtn         *eui.ItemData
	musicMixCB         *eui.ItemData
	ttsMixCB           *eui.ItemData
)

var ttsTestPhrase = "The quick brown fox jumps over the lazy dog"

// lastWhoRequest tracks the last time we requested a backend who list so we
// can avoid spamming the server when the Players window is toggled rapidly.
var lastWhoRequest time.Time

func capsLockToggled() {
	clearCapsWarnings()
}

func clearCapsWarnings() {
	if addCharPassWarn != nil {
		addCharPassWarn.Text = ""
		addCharPassWarn.Dirty = true
	}
	if passWarn != nil {
		passWarn.Text = ""
		passWarn.Dirty = true
	}
}

func checkCapsWarning(prev *string, curr string, warn *eui.ItemData) {
	if warn == nil {
		*prev = curr
		return
	}
	if len(curr) > len(*prev) {
		r := rune(curr[len(curr)-1])
		shift := eui.ShiftPressed
		if unicode.IsLetter(r) && ((unicode.IsUpper(r) && !shift) || (unicode.IsLower(r) && shift)) {
			warn.Text = "Caps lock may be on"
			warn.TextColor = eui.NewColor(255, 0, 0, 255)
		} else {
			warn.Text = ""
		}
		warn.Dirty = true
	} else if len(curr) <= len(*prev) {
		warn.Text = ""
		warn.Dirty = true
	}
	*prev = curr
}

func init() {
	eui.CapsLockToggleHandler = capsLockToggled
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
		if windowsHelpCB != nil {
			windowsHelpCB.Checked = helpWin != nil && helpWin.IsOpen()
			windowsHelpCB.Dirty = true
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
	status, err = checkDataFiles(clVersion)
	if err != nil {
		logError("check data files: %v", err)
	}

	loadHotkeys()
	// Load persisted user/global shortcuts before showing UI or handling input
	loadShortcuts()

	eui.SetUIScale(float32(gs.UIScale))

	makeGameWindow()
	makeDownloadsWindow()
	makeLoginWindow()
	makeAddCharacterWindow()
	makeChatWindow()
	makeConsoleWindow()
	makeSettingsWindow()
	makeQualityWindow()
	makeNotificationsWindow()
	makeBubbleWindow()
	makeDebugWindow()
	initHelpUI()
	initAboutUI()
	makeWindowsWindow()
	makeInventoryWindow()
	makePlayersWindow()
	makeShortcutsWindow()
	makeHotkeysWindow()
	makeTriggersWindow()
	makeJoystickWindow()
	makescriptsWindow()
	makeMixerWindow()
	makeToolbar()

	// Load any persisted players data (e.g., from prior sessions) so
	// avatars/classes can show up immediately.
	loadPlayersPersist()
	backfillCharactersFromPlayers()

	if status.NeedImages || status.NeedSounds {
		downloadWin.MarkOpen()
	} else if clmov == "" && pcapPath == "" && !fake {
		loginWin.MarkOpen()
	}
	uiReady = true
	if !windowsRestored {
		restoreWindowSettings()
	}
	if clmov == "" && pcapPath == "" && !fake && shouldShowSetupWizard(settingsLoaded, gs.SetupWizardVersion, appVersion) {
		openSetupWizard(false)
	}
}

func buildToolbar(toolFontSize, buttonWidth, buttonHeight float32) *eui.ItemData {
	var row1, row2, menu *eui.ItemData

	row1 = &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_HORIZONTAL}
	row2 = &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_HORIZONTAL}
	menu = &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL}

	winBtn, winEvents := eui.NewButton()
	winBtn.Text = "Windows"
	winBtn.SetTooltip("Manage windows layout and visibility")
	winBtn.Size = eui.Point{X: buttonWidth, Y: buttonHeight}
	winBtn.FontSize = toolFontSize
	winEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			windowsWin.ToggleNear(ev.Item)
		}
	}
	row1.AddItem(winBtn)

	btn, setEvents := eui.NewButton()
	btn.Text = "Settings"
	btn.SetTooltip("Open settings")
	btn.Size = eui.Point{X: buttonWidth, Y: buttonHeight}
	btn.FontSize = toolFontSize
	setEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			settingsWin.ToggleNear(ev.Item)
		}
	}
	row1.AddItem(btn)

	actionsBtn, actionsEvents := eui.NewButton()
	actionsBtn.Text = "Actions"
	actionsBtn.SetTooltip("Hotkeys, Shortcuts, Triggers, Scripts, Legacy Macros")
	actionsBtn.Size = eui.Point{X: buttonWidth, Y: buttonHeight}
	actionsBtn.FontSize = toolFontSize
	actionsEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type != eui.EventClick {
			return
		}
		r := ev.Item.DrawRect
		options := []string{
			"Hotkeys",
			"Shortcuts",
			"Triggers",
			"Scripts",
			"Legacy Macros",
			"Saved Data",
		}
		eui.ShowContextMenu(options, r.X0, r.Y1, func(i int) {
			switch i {
			case 0:
				hotkeysWin.ToggleNear(actionsBtn)
			case 1:
				refreshShortcutsList()
				shortcutsWin.ToggleNear(actionsBtn)
			case 2:
				refreshTriggersList()
				triggersWin.ToggleNear(actionsBtn)
			case 3:
				refreshscriptsWindow()
				scriptsWin.ToggleNear(actionsBtn)
			case 4:
				makeLegacyMacroLibraryWindow()
				refreshLegacyMacroLibraryWindow()
				legacyMacroLibraryWin.ToggleNear(actionsBtn)
			case 5:
				makeSavedDataWindow()
				savedDataWin.ToggleNear(actionsBtn)
			}
		})
	}
	row1.AddItem(actionsBtn)

	var recordEvents *eui.EventHandler
	recordBtn, recordEvents = eui.NewButton()
	recordBtn.Text = "Record"
	recordBtn.SetTooltip("Start/stop recording (.clmov)")
	recordBtn.Size = eui.Point{X: buttonWidth, Y: buttonHeight}
	recordBtn.Color = eui.ColorDarkRed
	recordBtn.FontSize = toolFontSize
	recordEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			if setupWizardPreviewActive {
				return
			}
			// STOP during playback
			if playingMovie && !setupWizardPreviewActive {
				if movieWin != nil {
					movieWin.Close()
				} else {
					playingMovie = false
					movieMode = false
				}
				updateRecordButton()
				return
			}
			// Cancel arming when disconnected
			if recorder == nil && recordingMovie && tcpConn == nil {
				recordingMovie = false
				consoleMessage("recording canceled; will not start on connect")
				updateRecordButton()
				return
			}
			toggleRecording()
		}
	}
	row2.AddItem(recordBtn)

	helpBtn, helpEvents := eui.NewButton()
	helpBtn.Text = "Help"
	helpBtn.SetTooltip("Open help")
	helpBtn.Size = eui.Point{X: buttonWidth, Y: buttonHeight}
	helpBtn.FontSize = toolFontSize
	helpEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			toggleHelpWindow(ev.Item)
		}
	}
	row2.AddItem(helpBtn)

	shotBtn, shotEvents := eui.NewButton()
	shotBtn.Text = "Snapshot"
	shotBtn.SetTooltip("Save screenshot")
	shotBtn.Size = eui.Point{X: buttonWidth, Y: buttonHeight}
	shotBtn.FontSize = toolFontSize
	shotEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			takeScreenshot()
		}
	}
	row2.AddItem(shotBtn)

	exitSessBtn, exitSessEv := eui.NewButton()
	exitSessBtn.Text = "Exit"
	exitSessBtn.SetTooltip("Exit session")
	exitSessBtn.Size = eui.Point{X: buttonWidth, Y: buttonHeight}
	exitSessBtn.FontSize = toolFontSize
	exitSessEv.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			confirmExitSession()
		}
	}
	row2.AddItem(exitSessBtn)

	mixBtn, mixEvents := eui.NewButton()
	mixBtn.Text = "Mixer"
	mixBtn.SetTooltip("Open audio mixer")
	mixBtn.Size = eui.Point{X: buttonWidth, Y: buttonHeight}
	mixBtn.FontSize = toolFontSize
	mixEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			mixerWin.ToggleNear(ev.Item)
		}
	}
	row1.AddItem(mixBtn)

	/*
	   stopBtn, stopEvents := eui.NewButton()
	   stopBtn.Text = "Stop scripts"
	   stopBtn.Size = eui.Point{X: buttonWidth * 2, Y: buttonHeight}
	   stopBtn.FontSize = toolFontSize

	   stopBtnTheme := *stopBtn.Theme
	   stopBtnTheme.Button.Color = eui.ColorDarkRed
	   stopBtnTheme.Button.HoverColor = eui.ColorRed
	   stopBtnTheme.Button.ClickColor = eui.ColorLightRed
	   stopBtn.Theme = &stopBtnTheme
	   stopEvents.Handle = func(ev eui.UIEvent) {
	           if ev.Type == eui.EventClick {
	                   stopAllscripts()
	           }
	   }
	   row2.AddItem(stopBtn)
	*/

	// Removed toolbar volume slider and mute button (use Mixer instead)

	recordStatus, _ = eui.NewText()
	recordStatus.Text = ""
	recordStatus.Size = eui.Point{X: 80, Y: buttonHeight}
	recordStatus.FontSize = toolFontSize
	recordStatus.Color = eui.ColorRed
	row2.AddItem(recordStatus)

	menu.AddItem(row1)
	menu.AddItem(row2)

	return menu
}

func makescriptsWindow() {
	if scriptsWin != nil {
		return
	}
	scriptsWin = eui.NewWindow()
	scriptsWin.Title = "Scripts"
	scriptsWin.Closable = true
	scriptsWin.Resizable = false
	scriptsWin.AutoSize = true
	scriptsWin.Movable = true
	scriptsWin.SetZone(eui.HZoneCenterLeft, eui.VZoneMiddleTop)

	root := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL, Scrollable: true}
	scriptsWin.AddItem(root)

	main := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_HORIZONTAL}
	root.AddItem(main)

	list := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL}
	scriptsList = list
	main.AddItem(list)

	scriptDetails = &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL}
	main.AddItem(scriptDetails)

	buttonsBottom := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_HORIZONTAL}
	root.AddItem(buttonsBottom)

	refreshBtn, rh := eui.NewButton()
	refreshBtn.Text = "Refresh"
	refreshBtn.SetTooltip("Rescan scripts and reload list")
	refreshBtn.Size = eui.Point{X: 64, Y: 24}
	rh.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			rescanscripts()
		}
	}
	buttonsBottom.AddItem(refreshBtn)

	openBtn, oh := eui.NewButton()
	openBtn.Text = "Open scripts folder"
	// Label already clear; no tooltip.
	openBtn.Size = eui.Point{X: 160, Y: 24}
	oh.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			open.Run(userScriptsDir())
		}
	}
	buttonsBottom.AddItem(openBtn)

	debugFlow := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL}
	root.AddItem(debugFlow)
	debugCB, debugEvents := eui.NewCheckbox()
	debugCB.Text = "Debug events"
	debugCB.Size = eui.Point{X: 160, Y: 24}
	debugCB.Checked = gs.scriptEventDebug
	debugEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			gs.scriptEventDebug = ev.Checked
			scriptDebugList.Invisible = !ev.Checked
			if ev.Checked {
				refreshscriptDebug()
			}
		}
	}
	debugFlow.AddItem(debugCB)
	dbg := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL, Scrollable: true}
	dbg.Size = eui.Point{X: 480, Y: 120}
	dbg.Invisible = !gs.scriptEventDebug
	scriptDebugList = dbg
	debugFlow.AddItem(dbg)

	scriptsWin.AddWindow(false)
	refreshscriptsWindow()
}

func refreshscriptsWindow() {
	if scriptsList == nil {
		return
	}
	checkSize := eui.Point{X: 32, Y: 32}
	scriptSize := eui.Point{X: 256, Y: 32}

	scriptsList.Contents = scriptsList.Contents[:0]
	legend := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_HORIZONTAL}
	charTxt, _ := eui.NewText()
	charTxt.Text = "Player"
	charTxt.FontSize = 9
	charTxt.Size = checkSize
	legend.AddItem(charTxt)
	allTxt, _ := eui.NewText()
	allTxt.Text = "Global"
	allTxt.FontSize = 9
	allTxt.Size = checkSize
	legend.AddItem(allTxt)
	plugTxt, _ := eui.NewText()
	plugTxt.Text = "script"
	plugTxt.FontSize = 9
	plugTxt.Size = scriptSize
	legend.AddItem(plugTxt)
	scriptsList.AddItem(legend)

	type entry struct {
		owner   string
		name    string
		cat     string
		sub     string
		invalid bool
	}
	scriptMu.RLock()
	cats := make(map[string][]entry)
	for o, n := range scriptDisplayNames {
		cats[scriptCategories[o]] = append(cats[scriptCategories[o]], entry{
			owner:   o,
			name:    n,
			cat:     scriptCategories[o],
			sub:     scriptSubCategories[o],
			invalid: scriptInvalid[o],
		})
	}
	scriptMu.RUnlock()
	var catList []string
	for c := range cats {
		catList = append(catList, c)
	}
	sort.Strings(catList)
	for _, cat := range catList {
		row := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_HORIZONTAL}
		spacer1 := &eui.ItemData{ItemType: eui.ITEM_TEXT, Size: checkSize, Fixed: true}
		spacer2 := &eui.ItemData{ItemType: eui.ITEM_TEXT, Size: checkSize, Fixed: true}
		row.AddItem(spacer1)
		row.AddItem(spacer2)
		txt, _ := eui.NewText()
		label := cat
		if label == "" {
			label = "Other"
		}
		txt.Text = label
		txt.FontSize = 12
		txt.Size = scriptSize
		row.AddItem(txt)
		scriptsList.AddItem(row)

		plist := cats[cat]
		sort.Slice(plist, func(i, j int) bool {
			return strings.ToLower(plist[i].name) < strings.ToLower(plist[j].name)
		})
		for _, e := range plist {
			row := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_HORIZONTAL}
			charCB, charEvents := eui.NewCheckbox()
			charCB.Size = checkSize
			allCB, allEvents := eui.NewCheckbox()
			allCB.Size = checkSize
			// Consider LastCharacter before login so the per-character
			// checkbox reflects the saved preference.
			effChar := playerName
			if effChar == "" {
				effChar = gs.LastCharacter
			}
			label := e.name
			if e.sub != "" {
				label += " [" + e.sub + "]"
			}
			owner := e.owner
			scriptMu.RLock()
			scope := scriptEnabledFor[owner]
			scriptMu.RUnlock()
			charCB.Checked = effChar != "" && scope.Chars != nil && scope.Chars[effChar]
			charCB.Disabled = e.invalid || effChar == ""
			allCB.Checked = scope.All
			allCB.Disabled = e.invalid
			click := func() { selectscript(owner) }
			if selectedscript == owner {
				row.Filled = true
				if scriptsWin != nil && scriptsWin.Theme != nil {
					row.Color = scriptsWin.Theme.Button.SelectedColor
				}
			}
			if !e.invalid {
				charEvents.Handle = func(ev eui.UIEvent) {
					if ev.Type == eui.EventCheckboxChanged {
						// Character/all are mutually exclusive. Prioritize the
						// clicked box and clear the other to reflect scope.
						if ev.Checked {
							setscriptEnabled(owner, true, false)
						} else {
							// Unchecking character when not selecting "all" disables.
							setscriptEnabled(owner, false, allCB.Checked)
						}
					}
				}
				allEvents.Handle = func(ev eui.UIEvent) {
					if ev.Type == eui.EventCheckboxChanged {
						if ev.Checked {
							setscriptEnabled(owner, false, true)
						} else {
							// Unchecking "All" should fully disable the script,
							// regardless of the per-character box state.
							clearscriptScope(owner)
						}
					}
				}
			}
			row.AddItem(charCB)
			row.AddItem(allCB)
			nameTxt, _ := eui.NewText()
			nameTxt.Text = label
			nameTxt.FontSize = 12
			nameTxt.Size = scriptSize
			nameTxt.Disabled = e.invalid
			nameTxt.Action = click
			row.Action = click
			row.AddItem(nameTxt)

			if !e.invalid {
				reloadBtn, rh := eui.NewButton()
				reloadBtn.Text = "Reload"
				reloadBtn.SetTooltip("Restart this script if enabled")
				reloadBtn.Size = eui.Point{X: 55, Y: 24}
				rh.Handle = func(ev eui.UIEvent) {
					if ev.Type == eui.EventClick {
						scriptMu.RLock()
						enabled := !scriptDisabled[owner]
						scriptMu.RUnlock()
						if enabled {
							disablescript(owner, "reloaded")
							enablescript(owner)
						}
					}
				}
				row.AddItem(reloadBtn)

				scriptConfigMu.RLock()
				cfg := scriptConfigEntries[owner]
				scriptConfigMu.RUnlock()
				if len(cfg) > 0 {
					cfgBtn, ch := eui.NewButton()
					cfgBtn.Text = "Configure"
					cfgBtn.Size = eui.Point{X: 70, Y: 24}
					ch.Handle = func(ev eui.UIEvent) {
						if ev.Type == eui.EventClick {
							openscriptConfigWindow(owner)
						}
					}
					row.AddItem(cfgBtn)
				}
			}
			nameTxt, _ = eui.NewText()
			nameTxt.FontSize = 12
			nameTxt.Size = eui.Point{X: 10, Y: 24}
			nameTxt.Disabled = e.invalid
			nameTxt.Action = click
			row.Action = click
			row.AddItem(nameTxt)

			scriptsList.AddItem(row)
		}
	}
	if scriptsWin != nil {
		refreshscriptDetails()
		scriptsWin.Refresh()
	}
}

func selectscript(owner string) {
	if selectedscript == owner {
		return
	}
	selectedscript = owner
	refreshscriptsWindow()
}

func refreshscriptDetails() {

	infoSize := eui.Point{X: 256, Y: 24}
	if scriptDetails == nil {
		return
	}
	scriptDetails.Contents = scriptDetails.Contents[:0]
	owner := selectedscript
	if owner == "" {
		txt, _ := eui.NewText()
		txt.Text = "Select a script"
		txt.FontSize = 12
		txt.Size = infoSize
		scriptDetails.AddItem(txt)
		return
	}

	scriptMu.RLock()
	name := scriptDisplayNames[owner]
	author := scriptAuthors[owner]
	cat := scriptCategories[owner]
	sub := scriptSubCategories[owner]
	disabled := scriptDisabled[owner]
	invalid := scriptInvalid[owner]
	scriptMu.RUnlock()

	status := "Enabled"
	if invalid {
		status = "Invalid"
	} else if disabled {
		status = "Disabled"
	}

	line := func(s string) {
		item, _ := eui.NewText()
		item.Text = s
		item.FontSize = 12
		item.Size = infoSize
		scriptDetails.AddItem(item)
	}

	line("Name: " + name)
	line("Author: " + author)
	catLabel := cat
	if sub != "" {
		if catLabel != "" {
			catLabel += " / "
		}
		catLabel += sub
	}
	line("Category: " + catLabel)
	line("Status: " + status)
	errText := "None"
	if invalid {
		errText = "Invalid script"
	}
	line("Errors: " + errText)

	shortcutMu.RLock()
	m := shortcutMaps[owner]
	shortcutMu.RUnlock()
	if len(m) == 0 {
		line("Shortcuts: none")
	} else {
		line("Shortcuts:")
		type pair struct{ short, full string }
		var list []pair
		for k, v := range m {
			list = append(list, pair{k, v})
		}
		sort.Slice(list, func(i, j int) bool { return list[i].short < list[j].short })
		for _, p := range list {
			t, _ := eui.NewText()
			t.Text = "  " + p.short + " = " + strings.TrimSpace(p.full)
			t.FontSize = 12
			t.Size = infoSize
			scriptDetails.AddItem(t)
		}
	}

	triggerHandlersMu.RLock()
	var triggers []string
	for phrase, hs := range scriptTriggers {
		for _, h := range hs {
			if h.owner == owner {
				triggers = append(triggers, phrase)
				break
			}
		}
	}
	triggerHandlersMu.RUnlock()
	if len(triggers) == 0 {
		line("Triggers: none")
	} else {
		line("Triggers:")
		sort.Strings(triggers)
		for _, t := range triggers {
			txt, _ := eui.NewText()
			txt.Text = "  " + t
			txt.FontSize = 12
			txt.Size = infoSize
			scriptDetails.AddItem(txt)
		}
	}

	if scriptsWin != nil {
		scriptsWin.Refresh()
	}
}

func refreshscriptDebug() {
	if scriptDebugList == nil {
		return
	}
	scriptDebugList.Contents = scriptDebugList.Contents[:0]
	scriptDebugMu.Lock()
	lines := append([]string(nil), scriptDebugLines...)
	scriptDebugMu.Unlock()
	for _, ln := range lines {
		t, _ := eui.NewText()
		t.Text = ln
		t.FontSize = 12
		t.Size = eui.Point{X: 400, Y: 16}
		scriptDebugList.AddItem(t)
	}
	if scriptsWin != nil {
		scriptsWin.Refresh()
	}
}

func openscriptConfigWindow(owner string) {
	scriptConfigMu.RLock()
	entries := scriptConfigEntries[owner]
	scriptConfigMu.RUnlock()
	if len(entries) == 0 {
		return
	}
	if scriptConfigWin != nil {
		scriptConfigWin.Close()
	}
	scriptMu.RLock()
	name := scriptDisplayNames[owner]
	scriptMu.RUnlock()
	scriptConfigWin = eui.NewWindow()
	scriptConfigWin.Title = "Configure: " + name
	scriptConfigWin.Closable = true
	scriptConfigWin.Resizable = false
	scriptConfigWin.AutoSize = true
	scriptConfigWin.Movable = true
	scriptConfigWin.SetZone(eui.HZoneCenterLeft, eui.VZoneMiddleTop)

	root := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL}
	scriptConfigWin.AddItem(root)

	for _, ce := range entries {
		row := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_HORIZONTAL}
		lbl, _ := eui.NewText()
		lbl.Text = ce.Name
		lbl.FontSize = 12
		lbl.Size = eui.Point{X: 120, Y: 24}
		row.AddItem(lbl)

		switch ce.Type {
		case "int-slider", "float-slider":
			s, _ := eui.NewSlider()
			s.MinValue = 0
			s.MaxValue = 100
			s.Size = eui.Point{X: 120, Y: 24}
			row.AddItem(s)
		case "check-box":
			cb, _ := eui.NewCheckbox()
			cb.Size = eui.Point{X: 24, Y: 24}
			row.AddItem(cb)
		case "text-box":
			inp, _ := eui.NewInput()
			inp.Size = eui.Point{X: 120, Y: 24}
			row.AddItem(inp)
		case "item-selector":
			dd, _ := eui.NewDropdown()
			dd.Size = eui.Point{X: 120, Y: 24}
			row.AddItem(dd)
		default:
			t, _ := eui.NewText()
			t.Text = ce.Type
			t.FontSize = 12
			t.Size = eui.Point{X: 120, Y: 24}
			row.AddItem(t)
		}
		root.AddItem(row)
	}

	scriptConfigWin.AddWindow(false)
	scriptConfigOwner = owner
}

func makeMixerWindow() {
	if mixerWin != nil {
		return
	}
	mixerWin = eui.NewWindow()
	mixerWin.Title = "Mixer"
	mixerWin.Closable = true
	mixerWin.Resizable = false
	mixerWin.AutoSize = true
	mixerWin.Movable = true

	flow := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_HORIZONTAL}

	addSpacer := func() {
		sp, _ := eui.NewText()
		sp.Text = ""
		sp.Size = eui.Point{X: 16, Y: 1}
		flow.AddItem(sp)
	}
	addBigSpacer := func() {
		sp, _ := eui.NewText()
		sp.Text = ""
		sp.Size = eui.Point{X: 28, Y: 1}
		flow.AddItem(sp)
	}

	// Main/master volume column to match other channel columns
	mainCol := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL, Size: eui.Point{X: 64, Y: 140}}
	masterMixSlider, h := eui.NewSlider()
	masterMixSlider.Vertical = true
	masterMixSlider.MinValue = 0
	masterMixSlider.MaxValue = 1
	masterMixSlider.Value = float32(gs.MasterVolume)
	masterMixSlider.Size = eui.Point{X: 24, Y: 100}
	masterMixSlider.AuxSize = eui.Point{X: 16, Y: 8}
	h.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventSliderChanged {
			if gs.Mute {
				ev.Item.Value = 0
				ev.Item.Dirty = true
				return
			}
			gs.MasterVolume = float64(ev.Value)
			if volumeSlider != nil {
				volumeSlider.Value = ev.Item.Value
				volumeSlider.Dirty = true
			}
			settingsDirty = true
			updateSoundVolume()
		}
	}
	mainCol.AddItem(masterMixSlider)
	mainLbl, _ := eui.NewText()
	mainLbl.Text = "Main"
	mainLbl.Size = eui.Point{X: 64, Y: 24}
	mainLbl.FontSize = 12
	mainCol.AddItem(mainLbl)
	flow.AddItem(mainCol)

	// Add a slightly larger gap before sub-channel sliders for clarity
	addBigSpacer()

	makeMix := func(val float64, enabled bool, name string, slide func(ev eui.UIEvent), check func(ev eui.UIEvent)) (*eui.ItemData, *eui.ItemData) {
		col := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL, Size: eui.Point{X: 64, Y: 140}}
		s, sh := eui.NewSlider()
		s.Vertical = true
		s.MinValue = 0
		s.MaxValue = 1
		s.Value = float32(val)
		s.Size = eui.Point{X: 24, Y: 100}
		s.AuxSize = eui.Point{X: 16, Y: 8}
		s.Disabled = !enabled
		sh.Handle = slide
		col.AddItem(s)
		cb, cbh := eui.NewCheckbox()
		cb.Text = name
		cb.Checked = enabled
		cb.Size = eui.Point{X: 64, Y: 24}
		cbh.Handle = check
		col.AddItem(cb)
		flow.AddItem(col)
		return s, cb
	}

	gameMixSlider, _ = makeMix(gs.GameVolume, gs.GameSound, "Game",
		func(ev eui.UIEvent) {
			if ev.Type == eui.EventSliderChanged {
				gs.GameVolume = float64(ev.Value)
				settingsDirty = true
				updateSoundVolume()
			}
		},
		func(ev eui.UIEvent) {
			if ev.Type == eui.EventCheckboxChanged {
				gs.GameSound = ev.Checked
				gameMixSlider.Disabled = !ev.Checked
				if !ev.Checked {
					stopAllSounds()
				}
				settingsDirty = true
				updateSoundVolume()
			}
		})

	addSpacer()

	musicMixSlider, musicMixCB = makeMix(gs.MusicVolume, gs.Music, "Music",
		func(ev eui.UIEvent) {
			if ev.Type == eui.EventSliderChanged {
				gs.MusicVolume = float64(ev.Value)
				settingsDirty = true
				updateSoundVolume()
			}
		},
		func(ev eui.UIEvent) {
			if ev.Type == eui.EventCheckboxChanged {
				if ev.Checked {
					gs.Music = true
					musicMixSlider.Disabled = false
					if s, err := checkDataFiles(clVersion); err == nil {
						status = s
						if status.NeedSoundfont {
							disableMusic()
							if downloadWin != nil {
								downloadWin.Close()
								downloadWin = nil
							}
							makeDownloadsWindow()
							if downloadWin != nil {
								downloadWin.MarkOpen()
							}
							return
						}
					}
					settingsDirty = true
					updateSoundVolume()
				} else {
					disableMusic()
				}
			}
		})

	addSpacer()

	ttsMixSlider, ttsMixCB = makeMix(gs.ChatTTSVolume, gs.ChatTTS, "TTS",
		func(ev eui.UIEvent) {
			if ev.Type == eui.EventSliderChanged {
				gs.ChatTTSVolume = float64(ev.Value)
				settingsDirty = true
				updateSoundVolume()
			}
		},
		func(ev eui.UIEvent) {
			if ev.Type == eui.EventCheckboxChanged {
				if ev.Checked {
					gs.ChatTTS = true
					ttsMixSlider.Disabled = false
					if s, err := checkDataFiles(clVersion); err == nil {
						status = s
						if status.NeedPiper || status.NeedPiperFem || status.NeedPiperMale {
							disableTTS()
							if downloadWin != nil {
								downloadWin.Close()
								downloadWin = nil
							}
							makeDownloadsWindow()
							if downloadWin != nil {
								downloadWin.MarkOpen()
							}
							return
						}
					}
					settingsDirty = true
					updateSoundVolume()
				} else {
					disableTTS()
				}
			}
		})

	addSpacer()

	notifMixSlider, _ = makeMix(gs.NotificationVolume, gs.NotificationBeep, "Notif",
		func(ev eui.UIEvent) {
			if ev.Type == eui.EventSliderChanged {
				gs.NotificationVolume = float64(ev.Value)
				settingsDirty = true
				updateSoundVolume()
			}
		},
		func(ev eui.UIEvent) {
			if ev.Type == eui.EventCheckboxChanged {
				gs.NotificationBeep = ev.Checked
				notifMixSlider.Disabled = !ev.Checked
				settingsDirty = true
				updateSoundVolume()
			}
		})

	addSpacer()

	var mixMuteEvents *eui.EventHandler
	mixMuteBtn, mixMuteEvents = eui.NewButton()
	mixMuteBtn.Text = "Mute"
	if gs.Mute {
		mixMuteBtn.Text = "Unmute"
	}
	// Make the mute button wider to accommodate label and adjacent checkbox context
	mixMuteBtn.Size = eui.Point{X: 192, Y: 24}
	mixMuteEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			gs.Mute = !gs.Mute
			if gs.Mute {
				mixMuteBtn.Text = "Unmute"
				if volumeSlider != nil {
					volumeSlider.Value = 0
				}
				if masterMixSlider != nil {
					masterMixSlider.Value = 0
					masterMixSlider.Dirty = true
				}
				if muteBtn != nil {
					muteBtn.Text = "Unmute"
					muteBtn.Dirty = true
				}
				stopAllAudioPlayers()
				clearTuneQueue()
			} else {
				mixMuteBtn.Text = "Mute"
				if volumeSlider != nil {
					volumeSlider.Value = float32(gs.MasterVolume)
				}
				if masterMixSlider != nil {
					masterMixSlider.Value = float32(gs.MasterVolume)
					masterMixSlider.Dirty = true
				}
				if muteBtn != nil {
					muteBtn.Text = "Mute"
					muteBtn.Dirty = true
				}
			}
			mixMuteBtn.Dirty = true
			if volumeSlider != nil {
				volumeSlider.Dirty = true
			}
			settingsDirty = true
			updateSoundVolume()
		}
	}
	// Place mute-unfocused checkbox directly under Mute button in its own column
	muteUnfocusCB, muteUnfocusEvents := eui.NewCheckbox()
	muteUnfocusCB.Text = "Mute when unfocused"
	// Match mute button width so the text fits comfortably
	muteUnfocusCB.Size = eui.Point{X: 192, Y: 24}
	muteUnfocusCB.Checked = gs.MuteWhenUnfocused
	muteUnfocusCB.SetTooltip("Temporarily mute audio when window is not focused")
	muteUnfocusEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			gs.MuteWhenUnfocused = ev.Checked
			if ev.Checked {
				if !ebiten.IsFocused() {
					focusMuted = true
				}
			} else {
				focusMuted = false
			}
			settingsDirty = true
			updateSoundVolume()
		}
	}
	// Make the column 3x standard width so the mixer window grows accordingly
	muteCol := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL, Size: eui.Point{X: 192, Y: 60}}
	muteCol.AddItem(mixMuteBtn)
	muteCol.AddItem(muteUnfocusCB)
	flow.AddItem(muteCol)

	mixerWin.AddItem(flow)
}

func makeToolbar() {
	if hudWin != nil {
		return
	}
	var toolFontSize float32 = 12
	var buttonHeight float32 = 18
	var buttonWidth float32 = 80

	hudWin = eui.NewWindow()
	hudWin.Title = "Toolbar"
	hudWin.Closable = false
	hudWin.Resizable = false
	hudWin.AutoSize = false
	hudWin.Size = eui.Point{X: buttonWidth * 5.5, Y: 85}
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
	flow.AddItem(buildToolbar(toolFontSize, buttonWidth, buttonHeight))

	hudWin.AddItem(flow)
	hudWin.AddWindow(false)
	updateHandsWindow()
	// Ensure record button reflects current state (playback/armed/recording)
	updateRecordButton()

	go func() {
		for {
			time.Sleep(time.Second * 5)
			hudWin.Title = fmt.Sprintf("Toolbar - FPS: %4.0f Loss: %0.0f%% Ping: %-3v Jit: %-3v",
				ebiten.ActualFPS(), droppedPercent(), netLatency.Milliseconds(), netJitter.Milliseconds())
			hudWin.Refresh()

		}
	}()
}

var (
	overlayHandOpts = &ebiten.DrawImageOptions{Filter: ebiten.FilterLinear, DisableMipmaps: true}
	overlayItemOpts = &ebiten.DrawImageOptions{Filter: ebiten.FilterLinear, DisableMipmaps: true}
)

func overlayItemOnHand(hand, item *ebiten.Image) *ebiten.Image {
	if hand == nil {
		return item
	}
	if item == nil {
		return hand
	}
	w := hand.Bounds().Dx()
	h := hand.Bounds().Dy()
	iw, ih := item.Bounds().Dx(), item.Bounds().Dy()
	if iw > w {
		w = iw
	}
	if ih > h {
		h = ih
	}
	out := newImage(w, h)
	offX := (w - hand.Bounds().Dx()) / 2
	offY := (h - hand.Bounds().Dy()) / 2
	opHand := overlayHandOpts
	opHand.ColorScale.Reset()
	opHand.ColorScale.ScaleAlpha(0.5)
	opHand.GeoM.Reset()
	opHand.GeoM.Translate(float64(offX), float64(offY))
	out.DrawImage(hand, opHand)
	offX = (w - iw) / 2
	offY = (h - ih) / 2
	opItem := overlayItemOpts
	opItem.ColorScale.Reset()
	opItem.GeoM.Reset()
	opItem.GeoM.Translate(float64(offX), float64(offY))
	out.DrawImage(item, opItem)
	return out
}

func updateHandsWindow() {
	if rightHandImg == nil || leftHandImg == nil {
		return
	}
	baseHand := loadImage(defaultHandPictID)
	if baseHand == nil {
		return
	}
	rightID, leftID := equippedItemPicts()

	rightImg := baseHand
	if rightID != 0 {
		if item := loadImage(rightID); item != nil {
			rightImg = overlayItemOnHand(baseHand, item)
		}
	}

	leftHand := mirrorImage(baseHand)
	leftImg := leftHand
	if leftID != 0 {
		if item := loadImage(leftID); item != nil {
			leftImg = overlayItemOnHand(leftHand, mirrorImage(item))
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
	if playingMovie && !setupWizardPreviewActive {
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

func startRecording() {
	if isWASM {
		consoleMessage("movie recording unavailable in browser build")
		return
	}
	dir := filepath.Join(dataDirPath, "Movies")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		logError("record movie: %v", err)
		return
	}
	ts := time.Now().Format("2006-01-02-15-04-05")
	base := gs.LastCharacter
	if base == "" {
		base = "movie"
	}
	recordPath = filepath.Join(dir, fmt.Sprintf("%s__%s.clMov", base, ts))
	// Use clVersion for the .clMov header version field as requested.
	mr, err := newMovieRecorder(recordPath, clVersion, int(movieRevision))
	if err != nil {
		logError("record movie: %v", err)
		recordPath = ""
		return
	}
	stateMu.Lock()
	snapshot := cloneDrawState(state)
	stateMu.Unlock()
	mr.AddStateSnapshot(snapshot, uint16(clVersion))
	recorder = mr
	consoleMessage(fmt.Sprintf("recording to %s", filepath.Base(recordPath)))
	updateRecordButton()
}

func stopRecording() {
	if recorder == nil {
		return
	}
	if err := recorder.Close(); err != nil {
		logError("record movie: %v", err)
	}
	recorder = nil
	if recordPath != "" {
		saved := recordPath
		consoleMessage(fmt.Sprintf("saved movie: %s", filepath.Base(saved)))
		if gs.AutoRecord {
			go func(src string) {
				outName := filepath.Base(src) + ".zip"
				dst := filepath.Join(filepath.Dir(src), outName)
				if err := compressZip(src, dst); err != nil {
					logError("zip compress: %v", err)
					consoleMessage("compress failed: " + err.Error())
				} else {
					consoleMessage("compressed: " + outName)
					os.Remove(src)
				}
			}(saved)
		} else if gs.PromptOnSaveRecording {
			showRecordingSaveDialog(saved)
		}
		recordPath = ""
	}
	updateRecordButton()
}

func toggleRecording() {
	if recorder != nil {
		stopRecording()
		return
	}
	if clmov != "" || playingMovie || pcapPath != "" || fake {
		consoleMessage("cannot record during playback or replay")
		return
	}
	if tcpConn == nil { // not connected yet: arm and start on connect
		recordingMovie = true
		consoleMessage("recording will start on connect")
		updateRecordButton()
		return
	}
	startRecording()
}

var dlMutex sync.Mutex
var status dataFilesStatus

// ===== Recording save/rename/compress dialog =====
var recordSaveWin *eui.WindowData
var recordSaveInput *eui.ItemData
var recordSaveCompressCB *eui.ItemData
var recordSaveDontShowCB *eui.ItemData

func showRecordingSaveDialog(path string) {
	dir := filepath.Dir(path)
	base := filepath.Base(path)
	if recordSaveWin == nil {
		recordSaveWin = eui.NewWindow()
		recordSaveWin.Title = "Save Recording"
		recordSaveWin.Closable = true
		recordSaveWin.Resizable = false
		recordSaveWin.AutoSize = true
		recordSaveWin.Movable = true
		recordSaveWin.NoScroll = true
		recordSaveWin.SetZone(eui.HZoneCenter, eui.VZoneMiddleTop)
	}
	recordSaveWin.Contents = nil

	flow := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL, Fixed: true}
	info, _ := eui.NewText()
	info.Text = "Rename the .clMov file and optionally create a .zip archive (about half smaller)."
	info.Size = eui.Point{X: 420, Y: 36}
	info.FontSize = 10
	flow.AddItem(info)

	row := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_HORIZONTAL, Fixed: true}
	lbl, _ := eui.NewText()
	lbl.Text = "Filename:"
	lbl.Size = eui.Point{X: 64, Y: 24}
	lbl.FontSize = 12
	row.AddItem(lbl)
	recordSaveInput, _ = eui.NewInput()
	recordSaveInput.Size = eui.Point{X: 340, Y: 24}
	recordSaveInput.FontSize = 12
	recordSaveInput.Text = base
	row.AddItem(recordSaveInput)
	flow.AddItem(row)

	recordSaveCompressCB, _ = eui.NewCheckbox()
	recordSaveCompressCB.Text = ".zip compress (about half smaller)"
	recordSaveCompressCB.Checked = true
	recordSaveCompressCB.Size = eui.Point{X: 420, Y: 24}
	flow.AddItem(recordSaveCompressCB)

	recordSaveDontShowCB, _ = eui.NewCheckbox()
	recordSaveDontShowCB.Text = "Don't show this again"
	recordSaveDontShowCB.Size = eui.Point{X: 420, Y: 24}
	flow.AddItem(recordSaveDontShowCB)

	// Buttons
	btnRow := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_HORIZONTAL, Fixed: true, Alignment: eui.ALIGN_RIGHT}
	btnRow.Size = eui.Point{X: 420, Y: 28}
	cancelBtn, cancelEv := eui.NewButton()
	cancelBtn.Text = "Skip"
	cancelBtn.Size = eui.Point{X: 80, Y: 24}
	cancelEv.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			if recordSaveWin != nil {
				recordSaveWin.Close()
			}
		}
	}
	saveBtn, saveEv := eui.NewButton()
	saveBtn.Text = "Save"
	saveBtn.Size = eui.Point{X: 80, Y: 24}
	saveEv.Handle = func(ev eui.UIEvent) {
		if ev.Type != eui.EventClick {
			return
		}
		// Apply don't-show preference
		if recordSaveDontShowCB != nil && recordSaveDontShowCB.Checked {
			gs.PromptOnSaveRecording = false
			settingsDirty = true
			saveSettings()
		}
		// Resolve new path
		name := base
		if recordSaveInput != nil && strings.TrimSpace(recordSaveInput.Text) != "" {
			name = strings.TrimSpace(recordSaveInput.Text)
		}
		// Ensure extension
		if !strings.EqualFold(filepath.Ext(name), ".clmov") {
			name += ".clMov"
		}
		newPath := filepath.Join(dir, name)
		// Rename if changed
		if newPath != path {
			if err := os.Rename(path, newPath); err != nil {
				logError("rename recording: %v", err)
				consoleMessage("rename failed: " + err.Error())
			} else {
				consoleMessage("renamed to: " + filepath.Base(newPath))
				path = newPath
			}
		}
		// Compress if requested (to .zip using archive/zip)
		if recordSaveCompressCB != nil && recordSaveCompressCB.Checked {
			go func(src string) {
				outName := filepath.Base(src) + ".zip"
				dst := filepath.Join(filepath.Dir(src), outName)
				if err := compressZip(src, dst); err != nil {
					logError("zip compress: %v", err)
					consoleMessage("compress failed: " + err.Error())
				} else {
					consoleMessage("compressed: " + outName)
				}
			}(path)
		}
		if recordSaveWin != nil {
			recordSaveWin.Close()
		}
	}
	btnRow.AddItem(cancelBtn)
	btnRow.AddItem(saveBtn)
	flow.AddItem(btnRow)

	recordSaveWin.AddItem(flow)
	recordSaveWin.AddWindow(true)
	recordSaveWin.MarkOpen()
}

// handleDownloadAssetError presents error options when a required asset fails to load.
// It resets the download state and provides Retry and Quit buttons so the user
// can recover or exit.
func handleDownloadAssetError(flow, statusText, pb *eui.ItemData, retryFn func(), started *bool, msg string) {
	if downloadStatus != nil {
		downloadStatus(msg)
	}
	flow.Contents = []*eui.ItemData{statusText, pb}
	retryRow := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_HORIZONTAL}
	retryBtn, retryEvents := eui.NewButton()
	retryBtn.Text = "Retry"
	retryBtn.Size = eui.Point{X: 100, Y: 24}
	retryEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			*started = false
			retryFn()
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
	*started = false
	downloadStatus = nil
	downloadProgress = nil
	if downloadWin != nil {
		downloadWin.Refresh()
	}
}

func makeDownloadsWindow() {

	if downloadWin != nil {
		return
	}
	downloadWin = eui.NewWindow()
	downloadWin.Title = "Downloads"
	downloadWin.Closable = !(status.NeedImages || status.NeedSounds)
	downloadWin.Resizable = false
	downloadWin.AutoSize = true
	downloadWin.Movable = true
	downloadWin.SetZone(eui.HZoneCenter, eui.VZoneMiddleTop)

	startedDownload := false
	var downloadSoundfontCB *eui.ItemData
	var downloadTTSCB *eui.ItemData

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
	eui.SetProgressIndeterminate(pb, true)
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
			eui.SetProgressIndeterminate(pb, false)
			// Use absolute scale so ratio = (Value-Min)/(Max-Min) is robust
			pb.MinValue = 0
			pb.MaxValue = float32(total)
			pb.Value = float32(read)
		} else {
			eui.SetProgressIndeterminate(pb, true)
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
	applyBoldFace(t)
	flow.AddItem(t)

	for _, f := range status.Files {
		t, _ := eui.NewText()
		if f.Size > 0 {
			t.Text = fmt.Sprintf("%s (%s)", f.Name, humanize.Bytes(uint64(f.Size)))
		} else {
			t.Text = f.Name
		}
		t.FontSize = 15
		t.Size = eui.Point{X: 320, Y: 25}
		flow.AddItem(t)
	}

	if status.NeedSoundfont || status.NeedPiper || status.NeedPiperFem || status.NeedPiperMale {
		opt, _ := eui.NewText()
		opt.Text = "Optional downloads:"
		opt.FontSize = 15
		opt.Size = eui.Point{X: 320, Y: 25}
		applyBoldFace(opt)
		flow.AddItem(opt)

		info, _ := eui.NewText()
		info.Text = "Download TTS voices and the music soundfont."
		info.FontSize = 13
		info.Size = eui.Point{X: 320, Y: 25}
		flow.AddItem(info)
	}
	if status.NeedSoundfont {
		sfCB, _ := eui.NewCheckbox()
		label := "Download soundfont (music)"
		if status.SoundfontSize > 0 {
			label = fmt.Sprintf("Download soundfont (%s) (Music)", humanize.Bytes(uint64(status.SoundfontSize)))
		}
		sfCB.Text = label
		sfCB.Size = eui.Point{X: 320, Y: 24}
		sfCB.Checked = true
		downloadSoundfontCB = sfCB
		flow.AddItem(sfCB)
	}
	if status.NeedPiper || status.NeedPiperFem || status.NeedPiperMale {
		pc, _ := eui.NewCheckbox()
		total := status.PiperSize + status.PiperFemSize + status.PiperMaleSize
		label := "Download Piper files (TTS)"
		if total > 0 {
			label = fmt.Sprintf("Download Piper files (%s) (TTS)", humanize.Bytes(uint64(total)))
		}
		pc.Text = label
		pc.Size = eui.Point{X: 320, Y: 24}
		pc.Checked = false
		downloadTTSCB = pc
		flow.AddItem(pc)
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
		// Create a cancellable context for in-flight downloads.
		downloadCtx, downloadCancel = context.WithCancel(context.Background())
		// Reset UI state
		dlStart = time.Time{}
		currentName = ""
		eui.SetProgressIndeterminate(pb, true)
		pb.MinValue = 0
		pb.MaxValue = 1
		pb.Value = 0
		pb.Dirty = true
		statusText.Dirty = true
		downloadSoundfont, downloadTTS := optionalDownloadSelections(downloadSoundfontCB, downloadTTSCB)
		// Show the live status + progress and provide a cancel button
		cancelRow := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_HORIZONTAL}
		cancelBtn, cancelEvents := eui.NewButton()
		cancelBtn.Text = "Cancel"
		cancelBtn.Size = eui.Point{X: 100, Y: 24}
		cancelEvents.Handle = func(ev eui.UIEvent) {
			if ev.Type == eui.EventClick {
				if downloadCancel != nil {
					downloadCancel()
				}
				if downloadStatus != nil {
					downloadStatus("Download canceled")
				}
			}
		}
		cancelRow.AddItem(cancelBtn)
		flow.Contents = []*eui.ItemData{statusText, pb, cancelRow}
		downloadWin.Refresh()
		go func() {
			dlMutex.Lock()
			curStatus := status
			dlMutex.Unlock()

			if err := downloadDataFiles(clVersion, curStatus, downloadSoundfont, downloadTTS, downloadTTS, downloadTTS); err != nil {
				logError("download data files: %v", err)
				// Present inline Retry and Quit buttons
				flow.Contents = []*eui.ItemData{statusText, pb}
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
			imgStart := time.Now()
			var img *climg.CLImages
			var err error
			if isWASM && len(wasmCLImagesData) > 0 {
				img, err = climg.LoadBytes(wasmCLImagesData)
			} else {
				img, err = climg.Load(filepath.Join(dataDirPath, CL_ImagesFile))
			}
			if err != nil {
				logError("failed to load CL_Images: %v", err)
				handleDownloadAssetError(flow, statusText, pb, startDownload, &startedDownload, "Failed to load CL_Images")
				return
			} else {
				img.Denoise = gs.DenoiseImages
				img.DenoiseSharpness = gs.DenoiseSharpness
				img.DenoiseAmount = gs.DenoiseAmount
				clImages = img
				markWorldStateChanged()
				if measureLoads {
					dtms := float64(time.Since(imgStart).Nanoseconds()) / 1e6
					log.Printf("measure: CL_Images archive loaded in %.2fms frame=%d", dtms, frameCounter)
				}
				// Refresh windows that depend on CL_Images now that
				// the archive is available so icons appear without
				// requiring a manual resize.
				inventoryDirty = true
				playersDirty = true
			}

			sndStart := time.Now()
			if isWASM && len(wasmCLSoundsData) > 0 {
				clSounds, err = clsnd.LoadBytes(wasmCLSoundsData)
			} else {
				clSounds, err = clsnd.Load(filepath.Join("data/CL_Sounds"))
			}
			if err != nil {
				logError("failed to load CL_Sounds: %v", err)
				handleDownloadAssetError(flow, statusText, pb, startDownload, &startedDownload, "Failed to load CL_Sounds")
				return
			} else if measureLoads {
				dtms := float64(time.Since(sndStart).Nanoseconds()) / 1e6
				log.Printf("measure: CL_Sounds archive loaded in %.2fms frame=%d", dtms, frameCounter)
			}
			if s, err := checkDataFiles(clVersion); err == nil {
				dlMutex.Lock()
				status = s
				dlMutex.Unlock()
			}
			if name == "" && loginWin != nil {
				// Force reselect from LastCharacter if available
				passHash = ""
				pass = ""
				updateCharacterButtons()
				loginWin.Refresh()
			}
			// Clear the callback to avoid stray updates after closing.
			downloadStatus = nil
			downloadProgress = nil
			downloadWin.Close()
			if setupWizardWin != nil && setupWizardWin.IsOpen() {
				startSetupWizardPreview()
			}
			if name == "" && loginWin != nil && clmov == "" && !playingMovie && pcapPath == "" && !fake {
				loginWin.MarkOpen()
			}
		}()
	}

	// Auto-start download in WASM to avoid extra click; keep window open for progress.
	if isWASM {
		startDownload()
	}

	btnFlow := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_HORIZONTAL}
	if !isWASM {
		dlBtn, dlEvents := eui.NewButton()
		dlBtn.Text = "Download"
		dlBtn.Size = eui.Point{X: 100, Y: 24}
		dlEvents.Handle = func(ev eui.UIEvent) {
			if ev.Type == eui.EventClick {
				startDownload()
			}
		}
		btnFlow.AddItem(dlBtn)
	}

	closeBtn, closeEvents := eui.NewButton()
	closeBtn.Size = eui.Point{X: 100, Y: 24}
	if status.NeedImages || status.NeedSounds {
		closeBtn.Text = "Quit"
		closeEvents.Handle = func(ev eui.UIEvent) {
			if ev.Type == eui.EventClick {
				confirmQuit()
			}
		}
	} else {
		closeBtn.Text = "Close"
		closeEvents.Handle = func(ev eui.UIEvent) {
			if ev.Type == eui.EventClick {
				downloadWin.Close()
			}
		}
	}
	btnFlow.AddItem(closeBtn)
	flow.AddItem(btnFlow)

	downloadWin.AddItem(flow)
	downloadWin.AddWindow(false)
}

func optionalDownloadSelections(soundfontCB, ttsCB *eui.ItemData) (soundfont, tts bool) {
	if soundfontCB != nil {
		soundfont = soundfontCB.Checked
	}
	if ttsCB != nil {
		tts = ttsCB.Checked
	}
	return soundfont, tts
}

const charWinWidth = 500

func updateCharacterButtons() {
	if loginWin == nil || !loginWin.IsOpen() {
		return
	}
	if charactersList == nil {
		return
	}
	// Preserve current scroll position while rebuilding the list
	prevScroll := charactersList.Scroll
	if name == "" {
		if gs.LastCharacter != "" {
			for _, c := range characters {
				if c.Name == gs.LastCharacter {
					name = c.Name
					passHash = c.passHash
					pass = ""
					break
				}
			}
		}
		if name == "" && len(characters) == 1 {
			name = characters[0].Name
			passHash = characters[0].passHash
			pass = ""
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
		empty.Size = eui.Point{X: charWinWidth, Y: 64}
		charactersList.AddItem(empty)
		name = ""
		passHash = ""
		pass = ""
	} else {
		for _, c := range characters {
			row := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_HORIZONTAL}

			profItem, _ := eui.NewImageItem(48, 48)
			profItem.Margin = 4
			profItem.Border = 0
			profItem.Filled = false
			if pid := professionPictID(c.Profession); pid != 0 {
				if img := loadImage(pid); img != nil {
					profItem.Image = img
					profItem.ImageName = "prof:cl:" + fmt.Sprint(pid)
				}
			}
			row.AddItem(profItem)

			avItem, _ := eui.NewImageItem(48, 48)
			avItem.Margin = 4
			avItem.Border = 0
			avItem.Filled = false
			var img *ebiten.Image
			if c.PictID != 0 {
				if m := loadMobileFrame(c.PictID, 0, c.Colors); m != nil {
					img = m
				} else if im := loadImage(c.PictID); im != nil {
					img = im
				}
			}
			if img == nil {
				if gid := defaultMobilePictID(genderUnknown); gid != 0 {
					if m := loadMobileFrame(gid, 0, nil); m != nil {
						img = m
					} else if im := loadImage(gid); im != nil {
						img = im
					}
				}
			}
			if img != nil {
				avItem.Image = img
			}
			row.AddItem(avItem)

			radio, radioEvents := eui.NewRadio()
			radio.Text = c.Name
			radio.RadioGroup = "characters"
			radio.Size = eui.Point{X: 350, Y: 48}
			radio.FontSize = 20
			radio.Checked = name == c.Name
			nameCopy := c.Name
			hashCopy := c.passHash
			if name == c.Name {
				passHash = c.passHash
				pass = ""
			}
			radioEvents.Handle = func(ev eui.UIEvent) {
				if ev.Type == eui.EventRadioSelected {
					name = nameCopy
					passHash = hashCopy
					pass = ""
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
			cCopy := c
			trashEvents.Handle = func(ev eui.UIEvent) {
				if ev.Type == eui.EventClick {
					confirmRemoveCharacter(cCopy)
				}
			}
			row.AddItem(trash)
			charactersList.AddItem(row)
		}
	}
	// Preserve window position while contents change size
	// Restore prior scroll position to keep the user's place.
	charactersList.Scroll = prevScroll
	// Keep UI fresh after potential content changes.
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
	passInput, passEvents := eui.NewInput()
	passInput.Label = "Password"
	passInput.TextPtr = &addCharPass
	passInput.HideText = true
	passInput.Size = eui.Point{X: 200, Y: 24}
	addCharPassInput = passInput
	addCharPassPrev = addCharPass
	passEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventInputChanged {
			checkCapsWarning(&addCharPassPrev, addCharPass, addCharPassWarn)
		}
	}
	flow.AddItem(passInput)

	addCharPassWarn, _ = eui.NewText()
	addCharPassWarn.TextColor = eui.NewColor(255, 0, 0, 255)
	addCharPassWarn.Size = eui.Point{X: 200, Y: 24}
	addCharPassWarn.FontSize = 12
	flow.AddItem(addCharPassWarn)

	rememberCB, rememberEvents := eui.NewCheckbox()
	rememberCB.Text = "Remember Password"
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
	addCharWin.DefaultButton = addBtn
	addEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			h := md5.Sum([]byte(addCharPass))
			hash := hex.EncodeToString(h[:])
			if !addCharRemember {
				hash = ""
			}
			// Check for existing character names case-insensitively
			exists := false
			for i := range characters {
				if strings.EqualFold(characters[i].Name, addCharName) {
					// Preserve canonical case from the stored character
					addCharName = characters[i].Name
					characters[i].passHash = hash
					characters[i].DontRemember = !addCharRemember
					exists = true
					break
				}
			}
			if !exists {
				characters = append(characters, Character{Name: addCharName, passHash: hash, DontRemember: !addCharRemember})
			}
			saveCharacters()
			// Update selection to the newly added character
			name = addCharName
			passHash = hash
			pass = ""
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
			addCharPassPrev = ""
			clearCapsWarnings()
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

func makePasswordWindow() {
	if passWin != nil {
		return
	}
	passWin = eui.NewWindow()
	passWin.Title = "Enter Password"
	passWin.Closable = false
	passWin.Resizable = false
	passWin.AutoSize = true
	passWin.Movable = true

	flow := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL}

	input, passEvents := eui.NewInput()
	input.Label = "Password"
	input.TextPtr = &pass
	input.HideText = true
	input.Size = eui.Point{X: 200, Y: 24}
	passInput = input
	passPrev = pass
	passEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventInputChanged {
			checkCapsWarning(&passPrev, pass, passWarn)
		}
	}
	flow.AddItem(input)

	passWarn, _ = eui.NewText()
	passWarn.TextColor = eui.NewColor(255, 0, 0, 255)
	passWarn.Size = eui.Point{X: 200, Y: 24}
	passWarn.FontSize = 12
	flow.AddItem(passWarn)

	passRememberCB, rememberEvents := eui.NewCheckbox()
	passRememberCB.Text = "Remember Password"
	passRememberCB.Size = eui.Point{X: 200, Y: 24}
	passRememberCB.Checked = passRemember
	rememberEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			passRemember = ev.Checked
		}
	}
	flow.AddItem(passRememberCB)

	btnFlow := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_HORIZONTAL}

	cancelBtn, cancelEvents := eui.NewButton()
	cancelBtn.Text = "Cancel"
	cancelBtn.Size = eui.Point{X: 96, Y: 24}
	cancelEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			pass = ""
			passPrev = ""
			clearCapsWarnings()
			passWin.Close()
		}
	}
	btnFlow.AddItem(cancelBtn)

	okBtn, okEvents := eui.NewButton()
	okBtn.Text = "Connect"
	okBtn.Size = eui.Point{X: 96, Y: 24}
	okEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			if pass == "" {
				makeErrorWindow("Error: Login: password is empty")
				return
			}
			if name != "" {
				if passRemember {
					h := md5.Sum([]byte(pass))
					hash := hex.EncodeToString(h[:])
					passHash = hash
					setCharacterPassHash(name, hash, true)
					pass = ""
				} else {
					passHash = ""
					setCharacterPassHash(name, "", false)
				}
			}
			passWin.Close()
			startLogin()
		}
	}
	btnFlow.AddItem(okBtn)

	flow.AddItem(btnFlow)

	passWin.AddItem(flow)
	passWin.AddWindow(false)
}

func showPrecachePopup(onDone func()) {
	if precacheWin != nil {
		go func() {
			for !assetsPrecached {
				time.Sleep(100 * time.Millisecond)
			}
			onDone()
		}()
		return
	}
	var msg string
	switch {
	case gs.PrecacheImages && gs.PrecacheSounds:
		msg = "Preloading images and sounds..."
	case gs.PrecacheImages:
		msg = "Preloading images..."
	case gs.PrecacheSounds:
		msg = "Preloading sounds..."
	}
	pb, _ := eui.NewProgressBar()
	pb.Size = eui.Point{X: 300, Y: 14}
	pb.MinValue = 0
	pb.MaxValue = 1
	pb.Value = 0
	eui.SetProgressIndeterminate(pb, true)
	precacheWin = showPopup("Preloading", msg, nil, pb)
	precacheProgress = func(done, total int) {
		if total > 0 {
			eui.SetProgressIndeterminate(pb, false)
			pb.MinValue = 0
			pb.MaxValue = float32(total)
			pb.Value = float32(done)
		} else {
			eui.SetProgressIndeterminate(pb, true)
		}
		pb.Dirty = true
		if precacheWin != nil {
			precacheWin.Refresh()
		}
	}
	go func(win *eui.WindowData) {
		for !assetsPrecached {
			time.Sleep(100 * time.Millisecond)
		}
		win.Close()
		precacheWin = nil
		precacheProgress = nil
		onDone()
	}(precacheWin)
}

func startLogin() {
	if (gs.PrecacheSounds || gs.PrecacheImages) && !assetsPrecached {
		showPrecachePopup(startLogin)
		return
	}
	if status.Version > clVersion {
		clVersion = status.Version
	}

	loginWin.Close()
	showConnectDialog(fmt.Sprintf("Connecting to %s...", host))
	go func() {
		ctx, cancel := context.WithCancel(gameCtx)
		loginMu.Lock()
		loginCancel = cancel
		loginMu.Unlock()
		if err := login(ctx, clVersion); err != nil {
			closeConnectDialog()
			logError("login: %v", err)
			pass = ""
			// Bring login forward first so the popup stays on top
			loginWin.MarkOpen()
			updateCharacterButtons()
			makeErrorWindow("Error: Login: " + err.Error())
			return
		}
		closeConnectDialog()
	}()
}

func ensureConnectDialog() {
	if connectWin != nil {
		return
	}

	connectWin = eui.NewWindow()
	connectWin.Title = "Connecting"
	connectWin.Closable = false
	connectWin.Resizable = false
	connectWin.AutoSize = true
	connectWin.Movable = true
	connectWin.SetZone(eui.HZoneCenter, eui.VZoneMiddleTop)
	connectWin.Padding = 8
	connectWin.BorderPad = 4

	flow := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL}

	status, _ := eui.NewText()
	status.FontSize = 13
	status.Size = eui.Point{X: 360, Y: 24}
	status.Text = ""
	connectStatusText = status
	flow.AddItem(status)

	pb, _ := eui.NewProgressBar()
	pb.Size = eui.Point{X: 360, Y: 14}
	pb.MinValue = 0
	pb.MaxValue = 1
	pb.Value = 0
	eui.SetProgressIndeterminate(pb, true)
	flow.AddItem(pb)

	btnRow := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_HORIZONTAL, Alignment: eui.ALIGN_RIGHT}
	btnRow.Size = eui.Point{X: 360, Y: 28}
	cancelBtn, cancelEvents := eui.NewButton()
	cancelBtn.Text = "Cancel"
	cancelBtn.Size = eui.Point{X: 100, Y: 24}
	cancelEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			handleDisconnect()
			closeConnectDialog()
		}
	}
	btnRow.AddItem(cancelBtn)
	flow.AddItem(btnRow)

	connectWin.AddItem(flow)
	connectWin.AddWindow(false)
}

func showConnectDialog(initial string) {
	ensureConnectDialog()
	updateConnectDialog(initial)
	if connectWin != nil {
		connectWin.MarkOpen()
		connectWin.Refresh()
	}
}

func updateConnectDialog(msg string) {
	if connectStatusText != nil {
		connectStatusText.Text = msg
		connectStatusText.Dirty = true
	}
	if connectWin != nil {
		connectWin.Refresh()
	}
}

func closeConnectDialog() {
	if connectWin != nil {
		connectWin.Close()
	}
	connectWin = nil
	connectStatusText = nil
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
	// Set the login window opacity
	loginWin.Opacity = 0.9
	// Increase title font size for "Login" by 2pt
	loginWin.SetTitleSize(loginWin.GetRawTitleSize() + 2)
	loginWin.SetZone(eui.HZoneCenter, eui.VZoneMiddleTop)
	loginFlow := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL}
	// Characters list lives in its own flow and is scrollable.
	// Use a fixed height so the window doesn't grow unbounded.
	charactersList = &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL}
	charactersList.Scrollable = true
	charactersList.Fixed = true
	charactersList.Size = eui.Point{X: charWinWidth, Y: 300}

	/*
		manBtn, manBtnEvents := eui.NewButton(&eui.ItemData{Text: "Manage account", Size: eui.Point{X: 200, Y: 24}})
		manBtnEvents.Handle = func(ev eui.UIEvent) {
			if ev.Type == eui.EventClick {
				//Add manage account window here
			}
		}
		loginFlow.AddItem(manBtn)
	*/

	connBtn, connEvents := eui.NewButton()
	connBtn.Text = "Connect"
	connBtn.Size = eui.Point{X: charWinWidth, Y: 48}
	connBtn.Padding = 10
	connBtn.FontSize = 24
	loginWin.DefaultButton = connBtn
	// Keep a handle so we can enable/disable it dynamically.
	connEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			if name == "" {
				// No character selected: instruct the user to pick one first.
				makeErrorWindow("Please select a character to connect with first.")
				return
			}
			if passHash == "" && pass == "" {
				passRemember = true
				for i := range characters {
					if characters[i].Name == name {
						passRemember = !characters[i].DontRemember
						break
					}
				}
				if passWin == nil {
					makePasswordWindow()
				}
				if passRememberCB != nil {
					passRememberCB.Checked = passRemember
					passRememberCB.Dirty = true
				}
				pass = ""
				if passInput != nil {
					passInput.Text = ""
					passInput.Dirty = true
				}
				passWin.MarkOpenNear(ev.Item)
				return
			}
			gs.LastCharacter = name
			saveSettings()
			startLogin()
			updateCharacterButtons()
		}
	}

	demoBtn, demoEvents := eui.NewButton()
	demoBtn.Text = "Try the demo"
	demoBtn.SetTooltip("Connect with a random demo character")
	demoBtn.Size = eui.Point{X: charWinWidth, Y: 24}
	demoEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			go func() {
				n, err := fetchRandomDemoCharacter(clVersion)
				if err != nil {
					logError("demo: %v", err)
					loginWin.MarkOpen()
					makeErrorWindow("Error: Demo: " + err.Error())
					return
				}
				name = n
				passHash = ""
				pass = "demo"
				startLogin()
			}()
		}
	}

	addBtn, addEvents := eui.NewButton()
	addBtn.Text = "Add Character"
	addBtn.Size = eui.Point{X: charWinWidth, Y: 24}
	addEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			addCharName = ""
			addCharPass = ""
			addCharPassPrev = ""
			clearCapsWarnings()
			addCharRemember = true
			loginWin.Close()
			addCharWin.MarkOpenNear(ev.Item)
		}
	}

	openBtn, openEvents := eui.NewButton()
	openBtn.Text = "Play movie file"
	openBtn.SetTooltip("Open and play a .clmov recording")
	openBtn.Size = eui.Point{X: charWinWidth, Y: 24}
	openEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			filename, err := pickMovieFile()
			if err != nil {
				if errors.Is(err, errMovieDialogCancelled) {
					return
				}
				logError("open clMov: %v", err)
				// Keep popup on top of login
				makeErrorWindow("Error: Open clMov: " + err.Error())
				return
			}
			if filename == "" {
				return
			}
			clmov = filename
			loginWin.Close()
			go func() {
				drawStateEncrypted = false
				frames, err := parseMovie(filename, clVersion)
				if err != nil {
					logError("parse movie: %v", err)
					clmov = ""
					loginWin.MarkOpen()
					makeErrorWindow("Error: Open clMov: " + err.Error())
					return
				}
				playerName = extractMoviePlayerName(frames)
				applyEnabledScripts()
				ctx, cancel := context.WithCancel(gameCtx)
				mp := newMoviePlayer(frames, clMovFPS, cancel)
				mp.makePlaybackWindow()
				run := func() { go mp.run(ctx) }
				if (gs.PrecacheSounds || gs.PrecacheImages) && !assetsPrecached {
					showPrecachePopup(run)
				} else {
					run()
				}
			}()
		}
	}

	quitBttn, quitEvn := eui.NewButton()
	quitBttn.Text = "Quit"
	// Increase Quit button font size by 2pt
	quitBttn.FontSize = 24
	// Double the height of the Quit button
	quitBttn.Size = eui.Point{X: charWinWidth, Y: 48}
	quitEvn.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			confirmQuit()
		}
	}

	verFlow := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_HORIZONTAL, Size: eui.Point{X: 260, Y: 24}}
	verLabel, _ := eui.NewText()
	verLabel.Text = fmt.Sprintf("goThoom test %4d", appVersion)
	verLabel.FontSize = 14
	verLabel.Size = eui.Point{X: 357, Y: 24}
	verFlow.AddItem(verLabel)

	changeBtn, changeEvents := eui.NewButton()
	changeBtn.Text = "Changelog"
	changeBtn.SetTooltip("View recent changes")
	changeBtn.Size = eui.Point{X: 70, Y: 24}
	changeBtn.FontSize = 10
	changeEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			makeChangelogWindow()
			if changelogWin != nil {
				changelogWin.MarkOpenNear(ev.Item)
			}
		}
	}
	verFlow.AddItem(changeBtn)

	aboutBtn, aboutEvents := eui.NewButton()
	aboutBtn.Text = "About"
	aboutBtn.Size = eui.Point{X: 60, Y: 24}
	aboutBtn.FontSize = 10
	aboutEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			openAboutWindow(ev.Item)
		}
	}
	verFlow.AddItem(aboutBtn)

	loginFlow.AddItem(connBtn)
	loginFlow.AddItem(demoBtn)
	label, _ := eui.NewText()
	label.Text = ""
	label.FontSize = 15
	label.Size = eui.Point{X: 1, Y: 25}
	loginFlow.AddItem(label)
	loginFlow.AddItem(charactersList)
	label, _ = eui.NewText()
	label.Text = ""
	label.FontSize = 15
	label.Size = eui.Point{X: 1, Y: 25}
	loginFlow.AddItem(label)
	loginFlow.AddItem(addBtn)
	loginFlow.AddItem(openBtn)
	// Add a small spacer between Play movie file and Quit
	spacer, _ := eui.NewText()
	spacer.Text = ""
	spacer.Size = eui.Point{X: 1, Y: 16}
	loginFlow.AddItem(spacer)
	loginFlow.AddItem(quitBttn)
	loginFlow.AddItem(verFlow)

	loginWin.AddItem(loginFlow)
	loginWin.AddWindow(false)
}

func makeChangelogWindow() {
	if changelogWin == nil {
		changelogWin, changelogList, _ = makeTextWindow("Changelog", eui.HZoneCenter, eui.VZoneMiddleTop, false)
		changelogWin.OnResize = updateChangelogWindow
		flow := changelogWin.Contents[0]

		navFlow := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_HORIZONTAL, Fixed: true, Alignment: eui.ALIGN_RIGHT}
		navFlow.Size = eui.Point{Y: 24}
		flow.AddItem(navFlow)

		prevBtn, prevEvents := eui.NewButton()
		prevBtn.Text = "<"
		prevBtn.Size = eui.Point{X: 24, Y: 24}
		prevEvents.Handle = func(ev eui.UIEvent) {
			if ev.Type == eui.EventClick {
				if loadChangelogAt(changelogVersionIdx - 1) {
					updateChangelogWindow()
				}
			}
		}
		navFlow.AddItem(prevBtn)
		changelogPrevBtn = prevBtn

		nextBtn, nextEvents := eui.NewButton()
		nextBtn.Text = ">"
		nextBtn.Size = eui.Point{X: 24, Y: 24}
		nextEvents.Handle = func(ev eui.UIEvent) {
			if ev.Type == eui.EventClick {
				if loadChangelogAt(changelogVersionIdx + 1) {
					updateChangelogWindow()
				}
			}
		}
		navFlow.AddItem(nextBtn)
		changelogNextBtn = nextBtn
	}
	if changelogList != nil {
		updateChangelogWindow()
	}
	changelogWin.MarkOpen()
}

func updateChangelogWindow() {
	lines := strings.Split(changelog, "\n")
	header := fmt.Sprintf("goThoom test %d", appVersion)
	lines = append([]string{header, ""}, lines...)
	updateTextWindow(changelogWin, changelogList, nil, lines, 14, "", monoFaceSource)
	if changelogPrevBtn != nil {
		changelogPrevBtn.Disabled = changelogVersionIdx <= 0
		changelogPrevBtn.Dirty = true
	}
	if changelogNextBtn != nil {
		changelogNextBtn.Disabled = changelogVersionIdx >= len(changelogVersions)-1
		changelogNextBtn.Dirty = true
	}
	changelogWin.Refresh()
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

var SettingsLock sync.Mutex

func makeSettingsWindow() {
	if settingsWin != nil {
		return
	}
	settingsWin = eui.NewWindow()
	settingsWin.Title = fmt.Sprintf("Settings -- goThoom test %d", appVersion)
	settingsWin.Closable = true
	settingsWin.Resizable = false
	settingsWin.AutoSize = true
	settingsWin.Movable = true

	// Split settings into three panes: basic (left), appearance (center) and advanced (right)
	var panelWidth float32 = 270
	outer := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_HORIZONTAL}
	left := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL}
	left.Size = eui.Point{X: panelWidth, Y: 10}
	center := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL}
	center.Size = eui.Point{X: panelWidth, Y: 10}
	right := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL}
	right.Size = eui.Point{X: panelWidth, Y: 10}

	label, _ := eui.NewText()
	label.Text = "\nWindow Behavior:"
	label.FontSize = 15
	label.Size = eui.Point{X: panelWidth, Y: 50}
	applyBoldFace(label)
	left.AddItem(label)

	/*
				tilingCB, tilingEvents := eui.NewCheckbox()
				tilingCB.Text = "Tiling window mode (buggy)"
				tilingCB.Size = eui.Point{X: panelWidth, Y: 24}
				tilingCB.Checked = gs.WindowTiling
				tilingCB.SetTooltip("Prevent windows from overlapping")
				tilingEvents.Handle = func(ev eui.UIEvent) {
					if ev.Type == eui.EventCheckboxChanged {
						gs.WindowTiling = ev.Checked
						eui.SetWindowTiling(ev.Checked)
						settingsDirty = true
					}
				}
				right.AddItem(tilingCB)

		               snapCB, snapEvents := eui.NewCheckbox()
		               snapCB.Text = "Window snapping"
		               snapCB.Size = eui.Point{X: panelWidth, Y: 24}
		               snapCB.Checked = gs.WindowSnapping
		               snapCB.SetTooltip("Snap windows to edges and others")
				snapEvents.Handle = func(ev eui.UIEvent) {
					if ev.Type == eui.EventCheckboxChanged {
						gs.WindowSnapping = ev.Checked
						eui.SetWindowSnapping(ev.Checked)
						settingsDirty = true
					}
				}
				right.AddItem(snapCB)
	*/

	if showUIScale {
		// Screen size settings in-place (moved from separate window)
		uiScaleSlider, uiScaleEvents := eui.NewSlider()
		uiScaleSlider.Label = "UI Scaling"
		uiScaleSlider.MinValue = 0.75
		uiScaleSlider.MaxValue = 4
		uiScaleSlider.Value = float32(gs.UIScale)
		pendingUIScale := gs.UIScale
		uiScaleEvents.Handle = func(ev eui.UIEvent) {
			if ev.Type == eui.EventSliderChanged {
				pendingUIScale = float64(ev.Value)
			}
		}

		uiScaleApplyBtn, uiScaleApplyEvents := eui.NewButton()
		uiScaleApplyBtn.Text = "Apply"
		uiScaleApplyBtn.Size = eui.Point{X: 48, Y: 24}
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
		uiScaleSlider.Size = eui.Point{X: panelWidth - uiScaleApplyBtn.Size.X - 10, Y: 24}
		uiScaleRow.AddItem(uiScaleSlider)
		uiScaleRow.AddItem(uiScaleApplyBtn)
		left.AddItem(uiScaleRow)
	}

	fullscreenCB, fullscreenEvents := eui.NewCheckbox()
	fullscreenCB.Text = "Fullscreen (F12)"
	fullscreenCB.Size = eui.Point{X: panelWidth, Y: 24}
	fullscreenCB.Checked = gs.Fullscreen
	fullscreenEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			SettingsLock.Lock()
			defer SettingsLock.Unlock()

			gs.Fullscreen = ev.Checked
			ebiten.SetFullscreen(gs.Fullscreen)
			ebiten.SetWindowFloating(gs.Fullscreen || gs.AlwaysOnTop)
			settingsDirty = true
		}
	}
	left.AddItem(fullscreenCB)

	styleDD, styleEvents := eui.NewDropdown()
	styleDD.Label = "Style Theme"
	if opts, err := eui.ListStyles(); err == nil {
		styleDD.Options = opts
		cur := eui.CurrentStyleName()
		for i, n := range opts {
			if n == cur {
				styleDD.Selected = i
				break
			}
		}
	}
	styleDD.Size = eui.Point{X: panelWidth, Y: 24}
	styleEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventDropdownSelected {
			SettingsLock.Lock()
			defer SettingsLock.Unlock()

			name := styleDD.Options[ev.Index]
			if err := eui.LoadStyle(name); err == nil {
				gs.Style = name
				settingsDirty = true
				settingsWin.Refresh()
			}
		}
	}

	var accentWheel *eui.ItemData

	themeDD, themeEvents := eui.NewDropdown()
	themeDD.Label = "Color Theme"
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
	themeDD.Size = eui.Point{X: panelWidth, Y: 24}
	themeEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventDropdownSelected {
			SettingsLock.Lock()
			defer SettingsLock.Unlock()

			name := themeDD.Options[ev.Index]
			if err := eui.LoadTheme(name); err == nil {
				gs.Theme = name
				gs.Style = eui.CurrentStyleName()
				for i, n := range styleDD.Options {
					if n == gs.Style {
						styleDD.Selected = i
						break
					}
				}
				settingsDirty = true
				settingsWin.Refresh()
				// Theme may change accent mapping; rebuild dependent windows immediately.
				updateInventoryWindow()
				updatePlayersWindow()
				updateDimmedScreenBG()
				if accentWheel != nil {
					var ac eui.Color
					_ = ac.UnmarshalJSON([]byte("\"accent\""))
					accentWheel.WheelColor = ac
				}
			}
		}
	}

	accentWheel, accentEvents := eui.NewColorWheel()
	accentWheel.Size = eui.Point{X: panelWidth, Y: 40}
	var ac eui.Color
	_ = ac.UnmarshalJSON([]byte("\"accent\""))
	accentWheel.WheelColor = ac
	accentEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventColorChanged {
			// Rebuild windows that cache accent into item colors so they update immediately.
			settingsWin.Refresh()
			updateInventoryWindow()
			updatePlayersWindow()
		}
	}

	left.AddItem(themeDD)
	left.AddItem(styleDD)
	accLabel, _ := eui.NewText()
	accLabel.Text = "Accent Color"
	accLabel.FontSize = 12
	accLabel.Size = eui.Point{X: panelWidth, Y: 20}
	left.AddItem(accLabel)
	left.AddItem(accentWheel)

	label, _ = eui.NewText()
	label.Text = "\nControls:"
	label.FontSize = 15
	label.Size = eui.Point{X: panelWidth, Y: 50}
	applyBoldFace(label)
	left.AddItem(label)

	toggle, toggleEvents := eui.NewCheckbox()
	toggle.Text = "Click-to-toggle movement"
	toggle.Size = eui.Point{X: panelWidth, Y: 24}
	toggle.Checked = gs.ClickToToggle
	toggle.SetTooltip("Click once to start walking, click again to stop.")
	toggleEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			SettingsLock.Lock()
			defer SettingsLock.Unlock()

			gs.ClickToToggle = ev.Checked
			if !gs.ClickToToggle {
				walkToggled = false
			}
			settingsDirty = true
		}
	}
	left.AddItem(toggle)

	label, _ = eui.NewText()
	label.Text = "\nQuality Options:"
	label.FontSize = 15
	label.Size = eui.Point{X: panelWidth, Y: 50}
	applyBoldFace(label)
	left.AddItem(label)

	qualityPresetDD, qpEvents := eui.NewDropdown()
	qualityPresetDD.Options = []string{"Classic", "Low", "Medium", "High", "Custom"}
	qualityPresetDD.Size = eui.Point{X: panelWidth, Y: 24}
	qualityPresetDD.Selected = detectQualityPreset()
	qualityPresetDD.FontSize = 12
	qpEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventDropdownSelected {
			switch ev.Index {
			case 0:
				applyQualityPreset("Classic")
			case 1:
				applyQualityPreset("Low")
			case 2:
				applyQualityPreset("Medium")
			case 3:
				applyQualityPreset("High")
			}
			qualityPresetDD.Selected = detectQualityPreset()
		}
	}
	left.AddItem(qualityPresetDD)

	qualityBtn, qualityEvents := eui.NewButton()
	qualityBtn.Text = "Quality Settings"
	qualityBtn.SetTooltip("Open detailed quality options")
	qualityBtn.Size = eui.Point{X: panelWidth, Y: 24}
	qualityEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			SettingsLock.Lock()
			defer SettingsLock.Unlock()

			qualityWin.ToggleNear(ev.Item)
		}
	}
	left.AddItem(qualityBtn)

	label, _ = eui.NewText()
	label.Text = "\nChat:"
	label.FontSize = 15
	label.Size = eui.Point{X: panelWidth, Y: 50}
	applyBoldFace(label)
	left.AddItem(label)

	inputOpenCB, inputOpenEvents := eui.NewCheckbox()
	inputOpenCB.Text = "Input bar always open"
	inputOpenCB.Size = eui.Point{X: panelWidth, Y: 24}
	inputOpenCB.Checked = gs.InputBarAlwaysOpen
	inputOpenCB.SetTooltip("Keep console input active after sending")
	inputOpenEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			SettingsLock.Lock()
			gs.InputBarAlwaysOpen = ev.Checked
			SettingsLock.Unlock()
			if gs.InputBarAlwaysOpen {
				inputActive = true
			} else {
				inputActive = false
				inputText = inputText[:0]
				inputPos = 0
				historyPos = len(inputHistory)
			}
			updateConsoleWindow()
			if consoleWin != nil {
				consoleWin.Refresh()
			}
			settingsDirty = true
		}
	}
	left.AddItem(inputOpenCB)

	bubbleMsgCB, bubbleMsgEvents := eui.NewCheckbox()
	bubbleMsgCB.Text = "Combine chat + console"
	bubbleMsgCB.Size = eui.Point{X: panelWidth, Y: 24}
	bubbleMsgCB.Checked = gs.MessagesToConsole
	bubbleMsgEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			SettingsLock.Lock()
			defer SettingsLock.Unlock()

			gs.MessagesToConsole = ev.Checked
			settingsDirty = true
			if ev.Checked {
				if chatWin != nil {
					chatWin.Close()
				}
			}
		}
	}
	left.AddItem(bubbleMsgCB)

	chatTSCB, chatTSEvents := eui.NewCheckbox()
	chatTSCB.Text = "Chat timestamps"
	chatTSCB.Size = eui.Point{X: panelWidth, Y: 24}
	chatTSCB.Checked = gs.ChatTimestamps
	chatTSEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			SettingsLock.Lock()
			defer SettingsLock.Unlock()

			gs.ChatTimestamps = ev.Checked
			settingsDirty = true
			updateChatWindow()
		}
	}
	left.AddItem(chatTSCB)

	consoleTSCB, consoleTSEvents := eui.NewCheckbox()
	consoleTSCB.Text = "Console timestamps"
	consoleTSCB.Size = eui.Point{X: panelWidth, Y: 24}
	consoleTSCB.Checked = gs.ConsoleTimestamps
	consoleTSEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			SettingsLock.Lock()
			defer SettingsLock.Unlock()

			gs.ConsoleTimestamps = ev.Checked
			settingsDirty = true
			updateConsoleWindow()
		}
	}
	left.AddItem(consoleTSCB)

	notifCB, notifEvents := eui.NewCheckbox()
	notifCB.Text = "Game Notifications"
	notifCB.Size = eui.Point{X: panelWidth, Y: 24}
	notifCB.Checked = gs.Notifications
	notifCB.SetTooltip("Show in-game notifications")
	notifEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			SettingsLock.Lock()
			gs.Notifications = ev.Checked
			SettingsLock.Unlock()
			settingsDirty = true
			if !ev.Checked {
				clearNotifications()
			}
		}
	}
	left.AddItem(notifCB)

	notifBtn, notifBtnEvents := eui.NewButton()
	notifBtn.Text = "Notification Settings"
	notifBtn.Size = eui.Point{X: panelWidth, Y: 24}
	notifBtnEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			SettingsLock.Lock()
			defer SettingsLock.Unlock()

			notificationsWin.ToggleNear(ev.Item)
		}
	}
	left.AddItem(notifBtn)

	label, _ = eui.NewText()
	label.Text = "\nStatus Bar Options:"
	label.FontSize = 15
	label.Size = eui.Point{X: panelWidth, Y: 50}
	applyBoldFace(label)
	right.AddItem(label)

	placements := []struct {
		name  string
		value BarPlacement
	}{
		{"Along Bottom", BarPlacementBottom},
		{"Grouped Lower Left", BarPlacementLowerLeft},
		{"Grouped Lower Right", BarPlacementLowerRight},
		{"Grouped Upper Right", BarPlacementUpperRight},
	}
	for _, p := range placements {
		p := p
		radio, radioEvents := eui.NewRadio()
		radio.Text = p.name
		radio.RadioGroup = "status-bar-placement"
		radio.Size = eui.Point{X: panelWidth, Y: 24}
		radio.Checked = gs.BarPlacement == p.value
		radioEvents.Handle = func(ev eui.UIEvent) {
			if ev.Type == eui.EventRadioSelected {
				SettingsLock.Lock()
				defer SettingsLock.Unlock()

				gs.BarPlacement = p.value
				settingsDirty = true
			}
		}
		right.AddItem(radio)
	}

	barColorCB, barColorEvents := eui.NewCheckbox()
	barColorCB.Text = "Color bars by value"
	barColorCB.Size = eui.Point{X: panelWidth, Y: 24}
	barColorCB.Checked = gs.BarColorByValue
	barColorEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			gs.BarColorByValue = ev.Checked
			settingsDirty = true
		}
	}
	right.AddItem(barColorCB)

	label, _ = eui.NewText()
	label.Text = "\nOpacity Settings:"
	label.FontSize = 15
	label.Size = eui.Point{X: panelWidth, Y: 50}
	applyBoldFace(label)
	right.AddItem(label)

	maxNightSlider, maxNightEvents := eui.NewSlider()
	maxNightSlider.Label = "Max Night Level"
	maxNightSlider.MinValue = 0
	maxNightSlider.MaxValue = 100
	maxNightSlider.IntOnly = true
	maxNightSlider.Value = float32(gs.MaxNightLevel)
	maxNightSlider.Size = eui.Point{X: panelWidth - 10, Y: 24}
	maxNightEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventSliderChanged {
			gs.MaxNightLevel = int(ev.Value)
			settingsDirty = true
		}
	}
	right.AddItem(maxNightSlider)

	nameBgSlider, nameBgEvents := eui.NewSlider()
	nameBgSlider.Label = "Name Background Opacity"
	nameBgSlider.MinValue = 0
	nameBgSlider.MaxValue = 1
	nameBgSlider.Value = float32(gs.NameBgOpacity)
	nameBgSlider.Size = eui.Point{X: panelWidth - 10, Y: 24}
	nameBgEvents.Handle = func(ev eui.UIEvent) {

		if ev.Type == eui.EventSliderChanged {
			SettingsLock.Lock()
			defer SettingsLock.Unlock()

			gs.NameBgOpacity = float64(ev.Value)
			killNameTagCache()
			settingsDirty = true
		}
	}
	right.AddItem(nameBgSlider)

	nameBorderCB, nameBorderEvents := eui.NewCheckbox()
	nameBorderCB.Text = "Name Tag Label Colors"
	nameBorderCB.Size = eui.Point{X: panelWidth - 10, Y: 24}
	nameBorderCB.Checked = gs.NameTagLabelColors
	nameBorderCB.SetTooltip("Show player label colors on name tag borders")
	nameBorderEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			SettingsLock.Lock()
			defer SettingsLock.Unlock()

			gs.NameTagLabelColors = ev.Checked
			killNameTagCache()
			settingsDirty = true
		}
	}
	right.AddItem(nameBorderCB)

	// Name-tags hover-only toggle
	nameHoverCB, nameHoverEvents := eui.NewCheckbox()
	nameHoverCB.Text = "Show name-tags only on hover"
	nameHoverCB.Size = eui.Point{X: panelWidth - 10, Y: 24}
	nameHoverCB.Checked = gs.NameTagsOnHoverOnly
	nameHoverCB.SetTooltip("Hide name-tags unless the cursor is over a character")
	nameHoverEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			SettingsLock.Lock()
			defer SettingsLock.Unlock()

			gs.NameTagsOnHoverOnly = ev.Checked
			settingsDirty = true
		}
	}
	right.AddItem(nameHoverCB)

	bubbleOpSlider, bubbleOpEvents := eui.NewSlider()
	bubbleOpSlider.Label = "Bubble Opacity"
	bubbleOpSlider.MinValue = 0
	bubbleOpSlider.MaxValue = 1
	bubbleOpSlider.Value = float32(gs.BubbleOpacity)
	bubbleOpSlider.Size = eui.Point{X: panelWidth - 10, Y: 24}
	bubbleOpEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventSliderChanged {
			gs.BubbleOpacity = float64(ev.Value)
			settingsDirty = true
		}
	}
	right.AddItem(bubbleOpSlider)

	bubbleBaseLifeSlider, bubbleBaseLifeEvents := eui.NewSlider()
	bubbleBaseLifeSlider.Label = "Base Bubble Life (s)"
	bubbleBaseLifeSlider.MinValue = 1
	bubbleBaseLifeSlider.MaxValue = 5
	bubbleBaseLifeSlider.Value = float32(gs.BubbleBaseLife)
	bubbleBaseLifeSlider.Size = eui.Point{X: panelWidth - 10, Y: 24}
	bubbleBaseLifeEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventSliderChanged {
			gs.BubbleBaseLife = float64(ev.Value)
			settingsDirty = true
		}
	}
	right.AddItem(bubbleBaseLifeSlider)

	// Life added per word in a bubble
	bubblePerWordSlider, bubblePerWordEvents := eui.NewSlider()
	bubblePerWordSlider.Label = "Bubble Life per Word (s)"
	bubblePerWordSlider.MinValue = 0
	bubblePerWordSlider.MaxValue = 2
	bubblePerWordSlider.Value = float32(gs.BubbleLifePerWord)
	bubblePerWordSlider.Size = eui.Point{X: panelWidth - 10, Y: 24}
	bubblePerWordEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventSliderChanged {
			gs.BubbleLifePerWord = float64(ev.Value)
			settingsDirty = true
		}
	}
	right.AddItem(bubblePerWordSlider)

	// Bubble visual scale (not font size)
	bubbleScaleSlider, bubbleScaleEvents := eui.NewSlider()
	bubbleScaleSlider.Label = "Bubble Scale"
	bubbleScaleSlider.MinValue = 1.0
	bubbleScaleSlider.MaxValue = 8.0
	bubbleScaleSlider.Value = float32(gs.BubbleScale)
	bubbleScaleSlider.Size = eui.Point{X: panelWidth - 10, Y: 24}
	bubbleScaleEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventSliderChanged {
			gs.BubbleScale = float64(ev.Value)
			settingsDirty = true
		}
	}
	right.AddItem(bubbleScaleSlider)

	barOpacitySlider, barOpacityEvents := eui.NewSlider()
	barOpacitySlider.Label = "Status bar opacity"
	barOpacitySlider.MinValue = 0.1
	barOpacitySlider.MaxValue = 1.0
	barOpacitySlider.Value = float32(gs.BarOpacity)
	barOpacitySlider.Size = eui.Point{X: panelWidth - 10, Y: 24}
	barOpacityEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventSliderChanged {
			SettingsLock.Lock()
			defer SettingsLock.Unlock()

			gs.BarOpacity = float64(ev.Value)
			settingsDirty = true
		}
	}
	right.AddItem(barOpacitySlider)

	advancedBtn, advancedEvents := eui.NewButton()
	advancedBtn.Text = "Advanced Settings"
	advancedBtn.Size = eui.Point{X: panelWidth, Y: 24}
	advancedBtn.SetTooltip("Open additional settings and tools")
	advancedEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			SettingsLock.Lock()
			defer SettingsLock.Unlock()

			makeAdvancedSettingsWindow()
			advancedWin.ToggleNear(ev.Item)
		}
	}
	right.AddItem(advancedBtn)

	label, _ = eui.NewText()
	label.Text = "\nGetting Started:"
	label.FontSize = 15
	label.Size = eui.Point{X: panelWidth, Y: 50}
	applyBoldFace(label)
	center.AddItem(label)

	setupBtn, setupEvents := eui.NewButton()
	setupBtn.Text = "Setup Wizard"
	setupBtn.SetTooltip("Review common controls and graphics settings")
	setupBtn.Size = eui.Point{X: panelWidth, Y: 40}
	setupBtn.FontSize = 15
	setupBtn.Color = eui.ColorDarkOrange
	setupBtn.HoverColor = eui.ColorOrange
	setupBtn.TextColor = eui.ColorWhite
	setupBtn.ForceTextColor = true
	setupEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			if settingsWin != nil {
				settingsWin.Close()
			}
			openSetupWizard(true)
		}
	}
	center.AddItem(setupBtn)

	label, _ = eui.NewText()
	label.Text = "\nText Sizes:"
	label.FontSize = 15
	label.Size = eui.Point{X: panelWidth, Y: 50}
	applyBoldFace(label)
	center.AddItem(label)

	labelFontSlider, labelFontEvents := eui.NewSlider()
	labelFontSlider.Label = "Name Font Size"
	labelFontSlider.MinValue = 5
	labelFontSlider.MaxValue = 48
	labelFontSlider.Value = float32(gs.MainFontSize)
	labelFontSlider.Size = eui.Point{X: panelWidth - 10, Y: 24}
	labelFontEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventSliderChanged {
			SettingsLock.Lock()
			defer SettingsLock.Unlock()

			gs.MainFontSize = float64(ev.Value)
			initFont()
			settingsDirty = true
		}
	}
	center.AddItem(labelFontSlider)

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
	invFontSlider.Size = eui.Point{X: panelWidth - 10, Y: 24}
	invFontEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventSliderChanged {
			SettingsLock.Lock()
			defer SettingsLock.Unlock()

			gs.InventoryFontSize = float64(ev.Value)
			settingsDirty = true
			updateInventoryWindow()
		}
	}
	center.AddItem(invFontSlider)

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
	plFontSlider.Size = eui.Point{X: panelWidth - 10, Y: 24}
	plFontEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventSliderChanged {
			SettingsLock.Lock()
			defer SettingsLock.Unlock()

			gs.PlayersFontSize = float64(ev.Value)
			settingsDirty = true
			updatePlayersWindow()
			if playersWin != nil {
				playersWin.Refresh()
			}
		}
	}
	center.AddItem(plFontSlider)

	consoleFontSlider, consoleFontEvents := eui.NewSlider()
	consoleFontSlider.Label = "Console Font Size"
	consoleFontSlider.MinValue = 4
	consoleFontSlider.MaxValue = 48
	consoleFontSlider.Value = float32(gs.ConsoleFontSize)
	consoleFontSlider.Size = eui.Point{X: panelWidth - 10, Y: 24}
	consoleFontEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventSliderChanged {
			SettingsLock.Lock()
			defer SettingsLock.Unlock()

			gs.ConsoleFontSize = float64(ev.Value)
			updateConsoleWindow()
			if consoleWin != nil {
				consoleWin.Refresh()
			}
			settingsDirty = true
		}
	}
	center.AddItem(consoleFontSlider)

	chatWindowFontSlider, chatWindowFontEvents := eui.NewSlider()
	chatWindowFontSlider.Label = "Chat Window Font Size"
	chatWindowFontSlider.MinValue = 4
	chatWindowFontSlider.MaxValue = 48
	chatWindowFontSlider.Value = float32(gs.ChatFontSize)
	chatWindowFontSlider.Size = eui.Point{X: panelWidth - 10, Y: 24}
	chatWindowFontEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventSliderChanged {
			SettingsLock.Lock()
			defer SettingsLock.Unlock()

			gs.ChatFontSize = float64(ev.Value)
			updateChatWindow()
			if chatWin != nil {
				chatWin.Refresh()
			}
			settingsDirty = true
		}
	}
	center.AddItem(chatWindowFontSlider)

	chatFontSlider, chatFontEvents := eui.NewSlider()
	chatFontSlider.Label = "Chat Bubble Font Size"
	chatFontSlider.MinValue = 4
	chatFontSlider.MaxValue = 48
	chatFontSlider.Value = float32(gs.BubbleFontSize)
	chatFontSlider.Size = eui.Point{X: panelWidth - 10, Y: 24}
	chatFontEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventSliderChanged {
			gs.BubbleFontSize = float64(ev.Value)
			initFont()
			settingsDirty = true
		}
	}
	center.AddItem(chatFontSlider)

	label, _ = eui.NewText()
	label.Text = "\nAudio:"
	label.FontSize = 15
	label.Size = eui.Point{X: panelWidth, Y: 50}
	applyBoldFace(label)
	center.AddItem(label)

	ttsSpeedSlider, ttsSpeedEvents := eui.NewSlider()
	ttsSpeedSlider.Label = "TTS Speed"
	ttsSpeedSlider.MinValue = 0.5
	ttsSpeedSlider.MaxValue = 2.0
	ttsSpeedSlider.Value = float32(gs.ChatTTSSpeed)
	ttsSpeedSlider.Size = eui.Point{X: panelWidth - 10, Y: 24}
	ttsSpeedEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventSliderChanged {
			SettingsLock.Lock()
			gs.ChatTTSSpeed = float64(ev.Value)
			SettingsLock.Unlock()
			settingsDirty = true
		}
	}
	center.AddItem(ttsSpeedSlider)

	outer.AddItem(left)
	outer.AddItem(center)
	outer.AddItem(right)
	settingsWin.AddItem(outer)
	settingsWin.AddWindow(false)
}

// resetAllSettings restores gs to defaults, reapplies, and refreshes windows.
func resetAllSettings() {
	gs = gsdef
	setHighQualityResamplingEnabled(gs.HighQualityResampling)
	clampWindowSettings()
	applySettings()
	updateGameWindowSize()
	saveSettings()
	settingsDirty = false

	// Close existing windows so they can be recreated in their default state.
	if inventoryWin != nil {
		inventoryWin.Close()
		inventoryWin = nil
	}
	if playersWin != nil {
		playersWin.Close()
		playersWin = nil
	}
	if consoleWin != nil {
		consoleWin.Close()
		consoleWin = nil
	}
	if chatWin != nil {
		chatWin.Close()
		chatWin = nil
	}
	if advancedWin != nil {
		advancedWin.Close()
		advancedWin = nil
	}

	// Recreate windows according to default settings.
	if gs.InventoryWindow.Open {
		makeInventoryWindow()
	}
	if gs.PlayersWindow.Open {
		makePlayersWindow()
	}
	if gs.MessagesWindow.Open {
		makeConsoleWindow()
	}
	if gs.ChatWindow.Open {
		_ = makeChatWindow()
	}

	restoreWindowSettings()

	if inventoryWin != nil {
		updateInventoryWindow()
		inventoryWin.Refresh()
	}
	if playersWin != nil {
		updatePlayersWindow()
		playersWin.Refresh()
	}
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
	if bubbleWin != nil {
		bubbleWin.Refresh()
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

// showPopup creates a simple modal-like popup with optional extra items, a message and buttons.
func showPopup(title, message string, buttons []popupButton, extras ...*eui.ItemData) *eui.WindowData {
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
	// Optional extra items (e.g., images) shown above the message
	for _, ex := range extras {
		if ex != nil {
			flow.AddItem(ex)
		}
	}
	if message != "" {
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
	}

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
		"Are you sure you would like to quit?",
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

// showShaderDisablePrompt suggests turning off shaders when performance is poor.
func showShaderDisablePrompt() {
	if shaderWarnWin != nil {
		return
	}
	shaderWarnWin = eui.NewWindow()
	shaderWarnWin.Title = "Low FPS Detected"
	shaderWarnWin.Closable = false
	shaderWarnWin.Resizable = false
	shaderWarnWin.AutoSize = true
	shaderWarnWin.Movable = true
	shaderWarnWin.NoScroll = true
	shaderWarnWin.SetZone(eui.HZoneRight, eui.VZoneTop)

	flow := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL}

	msg, _ := eui.NewText()
	msg.Text = "FPS has been under 50 for a while. Disabling shaders may help."
	msg.FontSize = 12
	msg.Size = eui.Point{X: 600, Y: 36}
	flow.AddItem(msg)

	shaderWarnDontShowCB, _ = eui.NewCheckbox()
	shaderWarnDontShowCB.Text = "Don't show again"
	shaderWarnDontShowCB.Size = eui.Point{X: 280, Y: 24}
	flow.AddItem(shaderWarnDontShowCB)

	btnRow := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_HORIZONTAL, Fixed: true, Alignment: eui.ALIGN_RIGHT}
	btnRow.Size = eui.Point{X: 280, Y: 28}

	cancelBtn, cancelEv := eui.NewButton()
	cancelBtn.Text = "Cancel"
	cancelBtn.Size = eui.Point{X: 80, Y: 24}
	cancelEv.Handle = func(ev eui.UIEvent) {
		if ev.Type != eui.EventClick {
			return
		}
		if shaderWarnDontShowCB != nil && shaderWarnDontShowCB.Checked {
			gs.PromptDisableShaders = false
			settingsDirty = true
			saveSettings()
		}
		shaderWarnWin.Close()
	}
	btnRow.AddItem(cancelBtn)

	disableBtn, disableEv := eui.NewButton()
	disableBtn.Text = "Disable Shaders"
	disableBtn.Size = eui.Point{X: 120, Y: 24}
	disableEv.Handle = func(ev eui.UIEvent) {
		if ev.Type != eui.EventClick {
			return
		}
		if shaderWarnDontShowCB != nil && shaderWarnDontShowCB.Checked {
			gs.PromptDisableShaders = false
		}
		gs.ShaderLighting = false
		settingsDirty = true
		applySettings()
		saveSettings()
		shaderWarnWin.Close()
	}
	btnRow.AddItem(disableBtn)

	flow.AddItem(btnRow)

	shaderWarnWin.AddItem(flow)
	shaderWarnWin.AddWindow(true)
	shaderWarnWin.MarkOpen()
}

// confirmRemoveCharacter prompts before deleting a saved character.
func confirmRemoveCharacter(c Character) {
	row := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_HORIZONTAL}

	profItem, _ := eui.NewImageItem(32, 32)
	profItem.Margin = 4
	profItem.Border = 0
	profItem.Filled = false
	if pid := professionPictID(c.Profession); pid != 0 {
		if img := loadImage(pid); img != nil {
			profItem.Image = img
			profItem.ImageName = "prof:cl:" + fmt.Sprint(pid)
		}
	}
	row.AddItem(profItem)

	avItem, _ := eui.NewImageItem(32, 32)
	avItem.Margin = 4
	avItem.Border = 0
	avItem.Filled = false
	if c.PictID != 0 {
		if m := loadMobileFrame(c.PictID, 0, c.Colors); m != nil {
			avItem.Image = m
		} else if im := loadImage(c.PictID); im != nil {
			avItem.Image = im
		}
	}
	row.AddItem(avItem)

	showPopup(
		"Remove Password",
		fmt.Sprintf("Are you sure you want to remove saved password for %s?", c.Name),
		[]popupButton{
			{Text: "Cancel"},
			{Text: "Yes, remove it", Color: &eui.ColorDarkRed, HoverColor: &eui.ColorRed, Action: func() {
				removeCharacter(c.Name)
				if name == c.Name {
					name = ""
					passHash = ""
					pass = ""
				}
				updateCharacterButtons()
				if loginWin != nil {
					loginWin.Refresh()
				}
			}},
		},
		row,
	)
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
	if qualityWin.Theme != nil {
		bg := qualityWin.Theme.Window.BGColor
		bg.A = 0xff
		qualityWin.BGColor = bg
		titleBG := qualityWin.Theme.Window.TitleBGColor
		titleBG.A = 0xff
		qualityWin.TitleBGColor = titleBG
	}
	// Render directly each frame so tall layouts do not reuse partially stale
	// cached backing images, which were causing the window background to fade
	// to a semi-transparent look below the first few rows.
	qualityWin.NoCache = true

	// Split settings into three panes: basic (left), appearance (center) and advanced (right)
	var panelWidth float32 = 270
	outer := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_HORIZONTAL}
	left := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL}
	left.Size = eui.Point{X: panelWidth, Y: 10}
	center := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL}
	center.Size = eui.Point{X: panelWidth, Y: 10}

	label, _ := eui.NewText()
	label.Text = "\nGPU Options:"
	label.FontSize = 15
	label.Size = eui.Point{X: width, Y: 50}
	applyBoldFace(label)
	left.AddItem(label)

	renderScale, renderScaleEvents := eui.NewSlider()
	renderScale.Label = "Upscale game amount (sharpness)"
	renderScale.MinValue = 1
	renderScale.MaxValue = 4
	renderScale.IntOnly = true
	if gs.GameScale < 1 {
		gs.GameScale = 1
	}
	if gs.GameScale > 4 {
		gs.GameScale = 4
	}

	renderScale.Value = float32(math.Round(gs.GameScale))
	renderScale.Size = eui.Point{X: width - 10, Y: 24}
	renderScale.SetTooltip("Game render resolution (1x - 4x). Higher will be shaper on higher-res displays.")
	renderScaleEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventSliderChanged {
			prevUpscale := gs.SpriteUpscale
			v := math.Round(float64(ev.Value))
			if v < 1 {
				v = 1
			}
			if v > 10 {
				v = 10
			}
			gs.GameScale = v
			gs.SpriteUpscale = spriteUpscaleFactor()
			if gs.SpriteUpscale != prevUpscale {
				clearCaches()
			}
			renderScale.Value = float32(v)
			settingsDirty = true
			initFont()
			if gameWin != nil {
				gameWin.Refresh()
			}
		}
	}
	left.AddItem(renderScale)

	uCB, upscaleFilterEvents := eui.NewCheckbox()
	upscaleFilterCB = uCB
	upscaleFilterCB.Text = "Artwork upscale filter"
	upscaleFilterCB.Size = eui.Point{X: width, Y: 24}
	upscaleFilterCB.Checked = gs.SpriteUpscaleFilter
	upscaleFilterCB.SetTooltip("Toggle scale-aware filtering when enlarging sprites")
	upscaleFilterEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			if gs.SpriteUpscaleFilter != ev.Checked {
				gs.SpriteUpscaleFilter = ev.Checked
				clearCaches()
				settingsDirty = true
				if gameWin != nil {
					gameWin.Refresh()
				}
			}
		}
	}
	left.AddItem(upscaleFilterCB)

	ppCB, pixelPerfectEvents := eui.NewCheckbox()
	pixelPerfectCB := ppCB
	pixelPerfectCB.Text = "Pixel-art scaling"
	pixelPerfectCB.Size = eui.Point{X: width, Y: 24}
	pixelPerfectCB.Checked = gs.PixelArtScaling
	pixelPerfectCB.SetTooltip("Keep crisp pixels when scaling")
	pixelPerfectEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			if gs.PixelArtScaling != ev.Checked {
				gs.PixelArtScaling = ev.Checked
				settingsDirty = true
				if gameWin != nil {
					gameWin.Refresh()
				}
			}
		}
	}
	left.AddItem(pixelPerfectCB)

	/*
		                                showFPSCB, showFPSEvents := eui.NewCheckbox()
		                                showFPSCB.Text = "Show FPS + UPS"
						showFPSCB.Size = eui.Point{X: width, Y: 24}
						showFPSCB.Checked = gs.ShowFPS
						showFPSCB.SetTooltip("Display frames per second, and updates per second")
						showFPSEvents.Handle = func(ev eui.UIEvent) {
							if ev.Type == eui.EventCheckboxChanged {
								gs.ShowFPS = ev.Checked
								settingsDirty = true
							}
						}
						flow.AddItem(showFPSCB)
	*/

	psCB, precacheSoundEvents := eui.NewCheckbox()
	precacheSoundCB = psCB
	precacheSoundCB.Text = "Precache Sounds"
	precacheSoundCB.Size = eui.Point{X: width, Y: 24}
	precacheSoundCB.Checked = gs.PrecacheSounds
	precacheSoundCB.SetTooltip("Load and pre-process all sounds, uses RAM but runs smoother (~300MB)")
	precacheSoundEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			gs.PrecacheSounds = ev.Checked
			if ev.Checked {
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
	left.AddItem(precacheSoundCB)

	piCB, precacheImageEvents := eui.NewCheckbox()
	precacheImageCB = piCB
	precacheImageCB.Text = "Precache Images"
	precacheImageCB.Size = eui.Point{X: width, Y: 24}
	precacheImageCB.Checked = gs.PrecacheImages
	precacheImageCB.SetTooltip("Load and pre-process all images, more RAM but runs smoother (<2GB)")
	precacheImageEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			gs.PrecacheImages = ev.Checked
			if ev.Checked {
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
	left.AddItem(precacheImageCB)

	pcCB, potatoEvents := eui.NewCheckbox()
	potatoCB = pcCB
	potatoCB.Text = "Potato GPU (low VRAM)"
	potatoCB.SetTooltip("Work-around for GPUs that only support 4096x4096 size sprites")
	potatoCB.Size = eui.Point{X: width, Y: 24}
	potatoCB.Checked = gs.PotatoGPU
	potatoEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			gs.PotatoGPU = ev.Checked
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
	left.AddItem(potatoCB)

	vsyncCB, vsyncEvents := eui.NewCheckbox()
	vsyncCB.Text = "VSync - Limit FPS"
	vsyncCB.Size = eui.Point{X: width, Y: 24}
	vsyncCB.Checked = gs.VSync
	vsyncCB.SetTooltip("Limit framerate to monitor Hz. OFF can improve speed")
	vsyncEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			gs.VSync = ev.Checked
			ebiten.SetVsyncEnabled(gs.VSync)
			settingsDirty = true
		}
	}
	left.AddItem(vsyncCB)

	// Shader lighting toggle in the Quality window
	shaderQualityCB, shaderQualityEv := eui.NewCheckbox()
	shaderLightingCB = shaderQualityCB
	shaderQualityCB.Text = "Shader Lighting Effects"
	shaderQualityCB.Size = eui.Point{X: width, Y: 24}
	shaderQualityCB.Checked = gs.ShaderLighting
	shaderQualityCB.SetTooltip("Enable shader-based lighting (disabled in Low/Ultra-Low presets)")
	shaderQualityEv.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			gs.ShaderLighting = ev.Checked
			settingsDirty = true
			if qualityPresetDD != nil {
				qualityPresetDD.Selected = detectQualityPreset()
			}
			if shaderLightSlider != nil {
				shaderLightSlider.Disabled = !ev.Checked
			}
			if shaderGlowSlider != nil {
				shaderGlowSlider.Disabled = !ev.Checked
			}
			if debugWin != nil {
				debugWin.Refresh()
			}
		}
	}
	left.AddItem(shaderQualityCB)

	sLS, shaderLightEvents := eui.NewSlider()
	shaderLightSlider = sLS
	shaderLightSlider.Label = "Light Strength"
	shaderLightSlider.MinValue = 0.01
	shaderLightSlider.MaxValue = 5000
	shaderLightSlider.IntOnly = true
	shaderLightSlider.Value = float32(gs.ShaderLightStrength * 100)
	shaderLightSlider.Size = eui.Point{X: width - 10, Y: 24}
	shaderLightSlider.Disabled = !gs.ShaderLighting
	shaderLightSlider.SetTooltip("Adjust intensity of shader lighting")
	shaderLightEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventSliderChanged {
			gs.ShaderLightStrength = float64(ev.Value / 100)
			settingsDirty = true
			if debugWin != nil {
				debugWin.Refresh()
			}
		}
	}
	left.AddItem(shaderLightSlider)

	sGS, shaderGlowEvents := eui.NewSlider()
	shaderGlowSlider = sGS
	shaderGlowSlider.Label = "Glow Strength"
	shaderGlowSlider.MinValue = 0.01
	shaderGlowSlider.MaxValue = 500
	shaderGlowSlider.IntOnly = true
	shaderGlowSlider.Value = float32(gs.ShaderGlowStrength * 100)
	shaderGlowSlider.Size = eui.Point{X: width - 10, Y: 24}
	shaderGlowSlider.Disabled = !gs.ShaderLighting
	shaderGlowSlider.SetTooltip("Adjust strength of glow halos")
	shaderGlowEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventSliderChanged {
			gs.ShaderGlowStrength = float64(ev.Value / 100)
			settingsDirty = true
			if debugWin != nil {
				debugWin.Refresh()
			}
		}
	}
	left.AddItem(shaderGlowSlider)

	label, _ = eui.NewText()
	label.Text = "\nSprite Gamma Correction:"
	label.FontSize = 15
	label.Size = eui.Point{X: width, Y: 50}
	applyBoldFace(label)
	left.AddItem(label)

	gcCB, gammaEvents := eui.NewCheckbox()
	gammaCorrectionCB = gcCB
	gammaCorrectionCB.Text = "Enable Sprite Gamma Correction"
	gammaCorrectionCB.Size = eui.Point{X: width, Y: 24}
	gammaCorrectionCB.Checked = gs.SpriteGammaCorrection
	gammaCorrectionCB.SetTooltip("Apply gamma compensation while decoding sprites")
	gammaEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			if gs.SpriteGammaCorrection != ev.Checked {
				gs.SpriteGammaCorrection = ev.Checked
				if spriteGammaSlider != nil {
					spriteGammaSlider.Disabled = !ev.Checked
				}
				if monitorGammaSlider != nil {
					monitorGammaSlider.Disabled = !ev.Checked
				}
				if clImages != nil {
					clImages.SetGammaCorrection(gs.SpriteGammaCorrection, gs.SpriteGamma, gs.MonitorGamma)
				}
				clearCaches()
				settingsDirty = true
				if qualityWin != nil {
					qualityWin.Refresh()
				}
			}
		}
	}
	left.AddItem(gammaCorrectionCB)

	sgSlider, spriteGammaEvents := eui.NewSlider()
	spriteGammaSlider = sgSlider
	spriteGammaSlider.Label = "Sprite Gamma"
	spriteGammaSlider.MinValue = float32(gammaOptions[0])
	spriteGammaSlider.MaxValue = float32(gammaOptions[len(gammaOptions)-1])
	spriteGammaSlider.Value = float32(gs.SpriteGamma)
	spriteGammaSlider.Size = eui.Point{X: width - 10, Y: 24}
	spriteGammaSlider.Disabled = !gs.SpriteGammaCorrection
	spriteGammaSlider.SetTooltip("Old Classic Macintosh OS used a gamma of 1.8, and most modern systems use 2.2 or sometimes 2.4.")
	spriteGammaEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventSliderChanged {
			target := normalizeGamma(float64(ev.Value), gs.SpriteGamma)
			if math.Abs(float64(spriteGammaSlider.Value)-target) > 0.0001 {
				spriteGammaSlider.Value = float32(target)
			}
			if math.Abs(gs.SpriteGamma-target) > 0.0001 {
				gs.SpriteGamma = target
				if clImages != nil {
					clImages.SetGammaCorrection(gs.SpriteGammaCorrection, gs.SpriteGamma, gs.MonitorGamma)
				}
				if gs.SpriteGammaCorrection {
					clearCaches()
				}
				settingsDirty = true
			}
		}
	}
	left.AddItem(spriteGammaSlider)

	mgSlider, monitorGammaEvents := eui.NewSlider()
	monitorGammaSlider = mgSlider
	monitorGammaSlider.Label = "Monitor Gamma"
	monitorGammaSlider.MinValue = float32(gammaOptions[0])
	monitorGammaSlider.MaxValue = float32(gammaOptions[len(gammaOptions)-1])
	monitorGammaSlider.Value = float32(gs.MonitorGamma)
	monitorGammaSlider.Size = eui.Point{X: width - 10, Y: 24}
	monitorGammaSlider.Disabled = !gs.SpriteGammaCorrection
	monitorGammaSlider.SetTooltip("Target display gamma to compensate towards")
	monitorGammaEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventSliderChanged {
			target := normalizeGamma(float64(ev.Value), gs.MonitorGamma)
			if math.Abs(float64(monitorGammaSlider.Value)-target) > 0.0001 {
				monitorGammaSlider.Value = float32(target)
			}
			if math.Abs(gs.MonitorGamma-target) > 0.0001 {
				gs.MonitorGamma = target
				if clImages != nil {
					clImages.SetGammaCorrection(gs.SpriteGammaCorrection, gs.SpriteGamma, gs.MonitorGamma)
				}
				if gs.SpriteGammaCorrection {
					clearCaches()
				}
				settingsDirty = true
			}
		}
	}
	left.AddItem(monitorGammaSlider)

	// (moved) Background behavior options are placed under Audio/Notifications

	label, _ = eui.NewText()
	label.Text = "\nImage denoising:"
	label.FontSize = 15
	label.Size = eui.Point{X: width, Y: 50}
	applyBoldFace(label)
	left.AddItem(label)

	dCB, denoiseEvents := eui.NewCheckbox()
	denoiseCB = dCB
	denoiseCB.Text = "Blend Image Dithering"
	denoiseCB.Size = eui.Point{X: width, Y: 24}
	denoiseCB.Checked = gs.DenoiseImages
	denoiseCB.SetTooltip("Attempts to blend image dithering to recover color information")
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
	left.AddItem(denoiseCB)

	denoiseSharpSlider, denoiseSharpEvents := eui.NewSlider()
	denoiseSharpSlider.Label = "Sharpness"
	denoiseSharpSlider.MinValue = 0
	denoiseSharpSlider.MaxValue = 100
	denoiseSharpSlider.Value = float32(gs.DenoiseSharpness * 5)
	denoiseSharpSlider.Size = eui.Point{X: width - 10, Y: 24}
	denoiseSharpSlider.SetTooltip("High is bias for not losing fine details")
	denoiseSharpEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventSliderChanged {
			gs.DenoiseSharpness = float64(ev.Value / 5)
			if clImages != nil {
				clImages.DenoiseSharpness = gs.DenoiseSharpness
			}
			clearCaches()
			settingsDirty = true
		}
	}
	left.AddItem(denoiseSharpSlider)

	denoiseAmtSlider, denoiseAmtEvents := eui.NewSlider()
	denoiseAmtSlider.Label = "Denoise strength"
	denoiseAmtSlider.MinValue = 0
	denoiseAmtSlider.MaxValue = 50
	denoiseAmtSlider.Value = float32(gs.DenoiseAmount * 100)
	denoiseAmtSlider.Size = eui.Point{X: width - 10, Y: 24}
	denoiseAmtSlider.SetTooltip("How strongly to blend dithered areas")
	denoiseAmtEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventSliderChanged {
			gs.DenoiseAmount = float64(ev.Value / 100)
			if clImages != nil {
				clImages.DenoiseAmount = gs.DenoiseAmount
			}
			clearCaches()
			settingsDirty = true
		}
	}
	left.AddItem(denoiseAmtSlider)

	label, _ = eui.NewText()
	label.Text = "\nMotion Smoothing Options:"
	label.FontSize = 15
	label.Size = eui.Point{X: width, Y: 50}
	applyBoldFace(label)
	center.AddItem(label)

	mCB, motionEvents := eui.NewCheckbox()
	motionCB = mCB
	motionCB.Text = "Smooth Motion"
	motionCB.Size = eui.Point{X: width, Y: 24}
	motionCB.Checked = gs.MotionSmoothing
	motionCB.SetTooltip("Interpolate camera and mobile movement")
	motionEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			gs.MotionSmoothing = ev.Checked
			settingsDirty = true
		}
	}
	center.AddItem(motionCB)

	// Object pinning: make small effect sprites follow mobiles smoothly
	pinCB, pinEvents := eui.NewCheckbox()
	pinCB.Text = "Object Effect Pinning"
	pinCB.Size = eui.Point{X: width, Y: 24}
	pinCB.Checked = gs.ObjectPinning
	pinCB.SetTooltip("Objects or effects on mobiles are motion smoothed")
	pinEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			gs.ObjectPinning = ev.Checked
			settingsDirty = true
		}
	}
	center.AddItem(pinCB)

	/*
		nsCB, noSmoothEvents := eui.NewCheckbox()
		noSmoothCB = nsCB
		noSmoothCB.Text = "Smooth moving objects,glitchy WIP"
		noSmoothCB.Size = eui.Point{X: width, Y: 24}
		noSmoothCB.Checked = gs.smoothMoving
		noSmoothCB.SetTooltip("Smooth moving objects that are not 'mobiles' such as chains, clouds, etc")
		noSmoothEvents.Handle = func(ev eui.UIEvent) {
			if ev.Type == eui.EventCheckboxChanged {
				gs.smoothMoving = ev.Checked
				settingsDirty = true
			}
		}
		center.AddItem(noSmoothCB)
	*/

	label, _ = eui.NewText()
	label.Text = "\nAnimation Blending Options:"
	label.FontSize = 15
	label.Size = eui.Point{X: width, Y: 50}
	applyBoldFace(label)
	center.AddItem(label)

	aCB, animEvents := eui.NewCheckbox()
	animCB = aCB
	animCB.Text = "Mobile Animation Blending"
	animCB.Size = eui.Point{X: width, Y: 24}
	animCB.Checked = gs.BlendMobiles
	animCB.SetTooltip("Gives appearance of more frames of animation at cost of latency.")
	animEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			gs.BlendMobiles = ev.Checked
			settingsDirty = true
			mobileBlendCache = map[mobileBlendKey]*ebiten.Image{}
		}
	}
	center.AddItem(animCB)

	pCB, pictBlendEvents := eui.NewCheckbox()
	pictBlendCB = pCB
	pictBlendCB.Text = "World Animation Blending"
	pictBlendCB.Size = eui.Point{X: width, Y: 24}
	pictBlendCB.Checked = gs.BlendPicts
	pictBlendCB.SetTooltip("Gives appearance of more frames of animation for water, grass, etc")
	pictBlendEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			gs.BlendPicts = ev.Checked
			settingsDirty = true
			pictBlendCache = map[pictBlendKey]*ebiten.Image{}
		}
	}
	center.AddItem(pictBlendCB)

	mobileBlendSlider, mobileBlendEvents := eui.NewSlider()
	mobileBlendSlider.Label = "Mobile Animation Blend Amount"
	mobileBlendSlider.MinValue = 0.1
	mobileBlendSlider.MaxValue = 1.0
	mobileBlendSlider.Value = float32(gs.MobileBlendAmount)
	mobileBlendSlider.Size = eui.Point{X: width - 10, Y: 24}
	mobileBlendSlider.SetTooltip("Generally looks best at 0.25-0.5, increases latency")
	mobileBlendEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventSliderChanged {
			gs.MobileBlendAmount = float64(ev.Value)
			settingsDirty = true
		}
	}
	center.AddItem(mobileBlendSlider)

	blendSlider, blendEvents := eui.NewSlider()
	blendSlider.Label = "World Animation Blending Strength"
	blendSlider.MinValue = 0.1
	blendSlider.MaxValue = 1.0
	blendSlider.Value = float32(gs.BlendAmount)
	blendSlider.Size = eui.Point{X: width - 10, Y: 24}
	blendSlider.SetTooltip("This looks amazing at max (1.0)")
	blendEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventSliderChanged {
			gs.BlendAmount = float64(ev.Value)
			settingsDirty = true
		}
	}
	center.AddItem(blendSlider)

	mobileFramesSlider, mobileFramesEvents := eui.NewSlider()
	mobileFramesSlider.Label = "Mobile Animation Blend Frames"
	mobileFramesSlider.MinValue = 3
	mobileFramesSlider.MaxValue = 30
	mobileFramesSlider.Value = float32(gs.MobileBlendFrames)
	mobileFramesSlider.Size = eui.Point{X: width - 10, Y: 24}
	mobileFramesSlider.IntOnly = true
	mobileFramesSlider.SetTooltip("Number of blending steps. 10 blend frames = ~60fps")
	mobileFramesEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventSliderChanged {
			gs.MobileBlendFrames = int(ev.Value)
			settingsDirty = true
		}
	}
	center.AddItem(mobileFramesSlider)

	pictFramesSlider, pictFramesEvents := eui.NewSlider()
	pictFramesSlider.Label = "World Animation Blend Frames"
	pictFramesSlider.MinValue = 3
	pictFramesSlider.MaxValue = 30
	pictFramesSlider.Value = float32(gs.PictBlendFrames)
	pictFramesSlider.Size = eui.Point{X: width - 10, Y: 24}
	pictFramesSlider.IntOnly = true
	pictFramesSlider.SetTooltip("Number of blending steps. 10 blend frames = ~60fps")
	pictFramesEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventSliderChanged {
			gs.PictBlendFrames = int(ev.Value)
			settingsDirty = true
		}
	}
	center.AddItem(pictFramesSlider)

	outer.AddItem(left)
	outer.AddItem(center)
	qualityWin.AddItem(outer)
	qualityWin.AddWindow(false)
}

func makeNotificationsWindow() {
	if notificationsWin != nil {
		return
	}
	var width float32 = 250
	notificationsWin = eui.NewWindow()
	notificationsWin.Title = "Notification Settings"
	notificationsWin.Closable = true
	notificationsWin.Resizable = false
	notificationsWin.AutoSize = true
	notificationsWin.Movable = true
	notificationsWin.SetZone(eui.HZoneCenterLeft, eui.VZoneMiddleTop)

	flow := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL}

	addCB := func(label string, val *bool) {
		cb, events := eui.NewCheckbox()
		cb.Text = label
		cb.Size = eui.Point{X: width, Y: 24}
		cb.Checked = *val
		events.Handle = func(ev eui.UIEvent) {
			if ev.Type == eui.EventCheckboxChanged {
				*val = ev.Checked
				settingsDirty = true
				if val == &gs.NotificationBeep {
					updateSoundVolume()
				}
			}
		}
		flow.AddItem(cb)
	}

	// Background notifications while unfocused
	addCB("Notify when in background", &gs.NotifyWhenBackground)

	addCB("Fallen", &gs.NotifyFallen)
	addCB("Not fallen", &gs.NotifyNotFallen)
	addCB("Shares", &gs.NotifyShares)
	addCB("Friend online", &gs.NotifyFriendOnline)
	addCB("Text copied", &gs.NotifyCopyText)
	addCB("Beep", &gs.NotificationBeep)

	durSlider, durEvents := eui.NewSlider()
	durSlider.Label = "Display Duration (sec)"
	durSlider.MinValue = 1
	durSlider.MaxValue = 30
	durSlider.Value = float32(gs.NotificationDuration)
	durSlider.Size = eui.Point{X: width - 10, Y: 24}
	durEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventSliderChanged {
			gs.NotificationDuration = float64(ev.Value)
			settingsDirty = true
		}
	}
	flow.AddItem(durSlider)

	// Test desktop notification button
	testBtn, testEv := eui.NewButton()
	testBtn.Text = "Send Test Notification"
	testBtn.Size = eui.Point{X: width, Y: 24}
	testEv.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			notifyDesktop("goThoom", "Background notifications test")
		}
	}
	flow.AddItem(testBtn)

	notificationsWin.AddItem(flow)
	notificationsWin.AddWindow(false)
}

func makeAdvancedSettingsWindow() {
	if advancedWin != nil {
		return
	}
	const columnWidth float32 = 260

	advancedWin = eui.NewWindow()
	advancedWin.Title = "Advanced Settings"
	advancedWin.Closable = true
	advancedWin.Resizable = false
	advancedWin.AutoSize = true
	advancedWin.Movable = true
	advancedWin.SetZone(eui.HZoneCenterLeft, eui.VZoneMiddleTop)

	addSectionLabel := func(col *eui.ItemData, text string) {
		label, _ := eui.NewText()
		label.Text = text
		label.FontSize = 15
		label.Size = eui.Point{X: columnWidth, Y: 40}
		applyBoldFace(label)
		col.AddItem(label)
	}

	columns := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_HORIZONTAL}

	toolsCol := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL}
	toolsCol.Size = eui.Point{X: columnWidth, Y: 10}
	interfaceCol := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL}
	interfaceCol.Size = eui.Point{X: columnWidth, Y: 10}
	chatCol := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL}
	chatCol.Size = eui.Point{X: columnWidth, Y: 10}
	systemCol := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL}
	systemCol.Size = eui.Point{X: columnWidth, Y: 10}

	// Tools
	addSectionLabel(toolsCol, "Tools")

	debugBtn, debugEvents := eui.NewButton()
	debugBtn.Text = "Debug Settings"
	debugBtn.Size = eui.Point{X: columnWidth, Y: 24}
	debugEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			SettingsLock.Lock()
			defer SettingsLock.Unlock()
			debugWin.ToggleNear(ev.Item)
		}
	}
	toolsCol.AddItem(debugBtn)

	dlBtn, dlEvents := eui.NewButton()
	dlBtn.Text = "Download Files"
	dlBtn.Size = eui.Point{X: columnWidth, Y: 24}
	dlBtn.SetTooltip("Download missing or optional files")
	dlEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			SettingsLock.Lock()
			defer SettingsLock.Unlock()

			if s, err := checkDataFiles(clVersion); err == nil {
				status = s
			}
			if downloadWin != nil {
				downloadWin.Close()
				downloadWin = nil
			}
			makeDownloadsWindow()
			downloadWin.MarkOpen()
		}
	}
	toolsCol.AddItem(dlBtn)

	resetBtn, resetEv := eui.NewButton()
	resetBtn.Text = "Reset All Settings"
	resetBtn.Size = eui.Point{X: columnWidth, Y: 24}
	resetBtn.Color = eui.ColorDarkRed
	resetBtn.HoverColor = eui.ColorRed
	resetBtn.SetTooltip("Restore defaults and reapply")
	resetEv.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			SettingsLock.Lock()
			defer SettingsLock.Unlock()
			confirmResetSettings()
		}
	}
	toolsCol.AddItem(resetBtn)

	addSectionLabel(toolsCol, "Automation")

	scriptKillCB, scriptKillEvents := eui.NewCheckbox()
	scriptKillCB.Text = "Auto-kill spammy scripts"
	scriptKillCB.Size = eui.Point{X: columnWidth, Y: 24}
	scriptKillCB.Checked = gs.ScriptSpamKill
	scriptKillCB.SetTooltip("Stop scripts that send too many lines")
	scriptKillEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			SettingsLock.Lock()
			gs.ScriptSpamKill = ev.Checked
			SettingsLock.Unlock()
			settingsDirty = true
		}
	}
	toolsCol.AddItem(scriptKillCB)

	autoRecCB, autoRecEvents := eui.NewCheckbox()
	autoRecCB.Text = "Auto-record sessions"
	autoRecCB.Size = eui.Point{X: columnWidth, Y: 24}
	autoRecCB.Checked = gs.AutoRecord
	autoRecCB.SetTooltip("Start recording on login and stop on logout")
	autoRecEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			gs.AutoRecord = ev.Checked
			settingsDirty = true
		}
	}
	toolsCol.AddItem(autoRecCB)

	// Interface column
	addSectionLabel(interfaceCol, "Interface")

	splashCB, splashEvents := eui.NewCheckbox()
	splashCB.Text = "Show Clan Lord splash image"
	splashCB.Size = eui.Point{X: columnWidth, Y: 24}
	splashCB.Checked = gs.ShowClanLordSplashImage
	splashCB.SetTooltip("Use CL_Images picture #4 for the splash screen")
	splashEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			gs.ShowClanLordSplashImage = ev.Checked
			settingsDirty = true
			prepareClassicSplash()
		}
	}
	interfaceCol.AddItem(splashCB)

	alwaysTopCB, alwaysTopEvents := eui.NewCheckbox()
	alwaysTopCB.Text = "Always on top"
	alwaysTopCB.Size = eui.Point{X: columnWidth, Y: 24}
	alwaysTopCB.Checked = gs.AlwaysOnTop
	alwaysTopEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			SettingsLock.Lock()
			defer SettingsLock.Unlock()

			gs.AlwaysOnTop = ev.Checked
			ebiten.SetWindowFloating(gs.Fullscreen || gs.AlwaysOnTop)
			settingsDirty = true
		}
	}
	interfaceCol.AddItem(alwaysTopCB)

	midMove, midMoveEvents := eui.NewCheckbox()
	midMove.Text = "Middle-click moves windows"
	midMove.Size = eui.Point{X: columnWidth, Y: 24}
	midMove.Checked = gs.MiddleClickMoveWindow
	midMove.SetTooltip("Drag windows using the middle mouse button")
	midMoveEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			SettingsLock.Lock()
			gs.MiddleClickMoveWindow = ev.Checked
			eui.SetMiddleClickMove(ev.Checked)
			SettingsLock.Unlock()
			settingsDirty = true
		}
	}
	interfaceCol.AddItem(midMove)

	pinLocCB, pinLocEvents := eui.NewCheckbox()
	pinLocCB.Text = "Show pin-to locations"
	pinLocCB.Size = eui.Point{X: columnWidth, Y: 24}
	pinLocCB.Checked = gs.ShowPinToLocations
	pinLocCB.SetTooltip("Show pin affordances on floating windows")
	pinLocEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			SettingsLock.Lock()
			gs.ShowPinToLocations = ev.Checked
			SettingsLock.Unlock()
			eui.SetShowPinLocations(ev.Checked)
			settingsDirty = true
		}
	}
	interfaceCol.AddItem(pinLocCB)

	keySpeedSlider, keySpeedEvents := eui.NewSlider()
	keySpeedSlider.Label = "Keyboard Walk Speed"
	keySpeedSlider.MinValue = 0.1
	keySpeedSlider.MaxValue = 1.0
	keySpeedSlider.Value = float32(gs.KBWalkSpeed)
	keySpeedSlider.Size = eui.Point{X: columnWidth - 10, Y: 24}
	keySpeedEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventSliderChanged {
			SettingsLock.Lock()
			defer SettingsLock.Unlock()
			gs.KBWalkSpeed = float64(ev.Value)
			settingsDirty = true
		}
	}
	interfaceCol.AddItem(keySpeedSlider)

	joystickBtn, joystickEvents := eui.NewButton()
	joystickBtn.Text = "Gamepad"
	joystickBtn.Size = eui.Point{X: columnWidth, Y: 24}
	joystickEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			SettingsLock.Lock()
			defer SettingsLock.Unlock()
			joystickWin.ToggleNear(ev.Item)
		}
	}
	interfaceCol.AddItem(joystickBtn)

	addSectionLabel(interfaceCol, "Visual Tweaks")

	fadePicsCB, fadePicsEvents := eui.NewCheckbox()
	fadePicsCB.Text = "Fade objects obscuring mobiles"
	fadePicsCB.Size = eui.Point{X: columnWidth, Y: 24}
	fadePicsCB.Checked = gs.FadeObscuringPictures
	fadePicsEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			gs.FadeObscuringPictures = ev.Checked
			settingsDirty = true
		}
	}
	interfaceCol.AddItem(fadePicsCB)

	obscureSlider, obscureEvents := eui.NewSlider()
	obscureSlider.Label = "Obscuring object opacity"
	obscureSlider.MinValue = 0.25
	obscureSlider.MaxValue = 0.7
	obscureSlider.Value = float32(gs.ObscuringPictureOpacity)
	obscureSlider.Size = eui.Point{X: columnWidth - 10, Y: 24}
	obscureEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventSliderChanged {
			gs.ObscuringPictureOpacity = float64(ev.Value)
			settingsDirty = true
		}
	}
	interfaceCol.AddItem(obscureSlider)

	// Chat & TTS column
	addSectionLabel(chatCol, "Chat & TTS")

	tsFormatInput, tsFormatEvents := eui.NewInput()
	tsFormatInput.Label = "Timestamp format"
	tsFormatInput.Text = gs.TimestampFormat
	tsFormatInput.TextPtr = &gs.TimestampFormat
	tsFormatInput.Size = eui.Point{X: columnWidth, Y: 24}
	tsFormatInput.SetTooltip("mo,day,hour,min,sec,yr:01,02,03...")
	tsFormatEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventInputChanged {
			SettingsLock.Lock()
			gs.TimestampFormat = ev.Text
			SettingsLock.Unlock()
			settingsDirty = true
			updateChatWindow()
			updateConsoleWindow()
		}
	}
	chatCol.AddItem(tsFormatInput)

	bubbleBtn, bubbleEvents := eui.NewButton()
	bubbleBtn.Text = "Message Bubbles"
	bubbleBtn.Size = eui.Point{X: columnWidth, Y: 24}
	bubbleEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			SettingsLock.Lock()
			defer SettingsLock.Unlock()
			bubbleWin.ToggleNear(ev.Item)
		}
	}
	chatCol.AddItem(bubbleBtn)

	voiceDD, voiceEvents := eui.NewDropdown()
	voiceDD.Label = "TTS Voice"
	if voices, err := listPiperVoices(); err == nil {
		voiceDD.Options = voices
		for i, v := range voices {
			if v == gs.ChatTTSVoice {
				voiceDD.Selected = i
				break
			}
		}
	}
	voiceDD.Action = func() {
		if !voiceDD.Open {
			return
		}
		if voices, err := listPiperVoices(); err == nil {
			voiceDD.Options = voices
			sel := 0
			for i, v := range voices {
				if v == gs.ChatTTSVoice {
					sel = i
					break
				}
			}
			voiceDD.Selected = sel
			if gs.ChatTTSVoice != voices[sel] {
				SettingsLock.Lock()
				gs.ChatTTSVoice = voices[sel]
				SettingsLock.Unlock()
				settingsDirty = true
			}
		}
	}
	voiceDD.Size = eui.Point{X: columnWidth, Y: 24}
	voiceEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventDropdownSelected {
			SettingsLock.Lock()
			gs.ChatTTSVoice = voiceDD.Options[ev.Index]
			SettingsLock.Unlock()
			settingsDirty = true
			piperModel = ""
			piperConfig = ""
			stopAllTTS()
		}
	}
	chatCol.AddItem(voiceDD)

	ttsTestInput, ttsTestEvents := eui.NewInput()
	ttsTestInput.Text = ttsTestPhrase
	ttsTestInput.TextPtr = &ttsTestPhrase
	ttsTestInput.Size = eui.Point{X: columnWidth, Y: 24}
	ttsTestEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventInputChanged {
			ttsTestPhrase = ev.Text
		}
	}
	chatCol.AddItem(ttsTestInput)

	ttsTestBtn, ttsTestBtnEvents := eui.NewButton()
	ttsTestBtn.Text = "Test TTS"
	ttsTestBtn.Size = eui.Point{X: columnWidth, Y: 24}
	ttsTestBtnEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			if !gs.ChatTTS {
				gs.ChatTTS = true
				settingsDirty = true
				if ttsMixCB != nil {
					ttsMixCB.Checked = true
				}
				if ttsMixSlider != nil {
					ttsMixSlider.Disabled = false
				}
				updateSoundVolume()
			}
			go playChatTTS(chatTTSCtx, ttsTestPhrase)
		}
	}
	chatCol.AddItem(ttsTestBtn)

	ttsEditBtn, ttsEditEvents := eui.NewButton()
	ttsEditBtn.Text = "Edit TTS corrections"
	ttsEditBtn.Size = eui.Point{X: columnWidth, Y: 24}
	ttsEditEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			open.Run(dataDirPath)
		}
	}
	chatCol.AddItem(ttsEditBtn)

	// System column (audio, network, performance)
	addSectionLabel(chatCol, "Audio")

	throttleCB, throttleEvents := eui.NewCheckbox()
	throttleSoundCB = throttleCB
	throttleSoundCB.Text = "Throttle Sounds"
	throttleSoundCB.Size = eui.Point{X: columnWidth, Y: 24}
	throttleSoundCB.Checked = gs.ThrottleSounds
	throttleSoundCB.SetTooltip("Prevent same sound from playing every tick")
	throttleEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			gs.ThrottleSounds = ev.Checked
			clearCaches()
			settingsDirty = true
		}
	}
	chatCol.AddItem(throttleSoundCB)

	enhancementCB, enhancementEvents := eui.NewCheckbox()
	soundEnhanceCB = enhancementCB
	enhancementCB.Text = "Audio enhancement for sound effects"
	enhancementCB.Size = eui.Point{X: columnWidth, Y: 24}
	enhancementCB.Checked = gs.SoundEnhancement
	enhancementCB.SetTooltip("Stereo width, ambience, and tone polish for in-game sounds")
	enhancementStrengthSlider, enhancementStrengthEvents := eui.NewSlider()
	soundEnhanceSlider = enhancementStrengthSlider
	enhancementStrengthSlider.Label = "Enhancement Strength"
	enhancementStrengthSlider.MinValue = 0.1
	enhancementStrengthSlider.MaxValue = 10
	enhancementStrengthSlider.Value = float32(gs.SoundEnhancementAmount)
	enhancementStrengthSlider.Size = eui.Point{X: columnWidth - 10, Y: 24}
	enhancementStrengthSlider.Disabled = !gs.SoundEnhancement
	enhancementStrengthSlider.SetTooltip("0.1 is subtle, 10 is very pronounced")
	enhancementEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			gs.SoundEnhancement = ev.Checked
			enhancementStrengthSlider.Disabled = !ev.Checked
			settingsDirty = true
		}
	}
	chatCol.AddItem(enhancementCB)

	enhancementStrengthEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventSliderChanged {
			gs.SoundEnhancementAmount = clampSoundEnhancementAmount(float64(ev.Value))
			settingsDirty = true
		}
	}
	chatCol.AddItem(enhancementStrengthSlider)

	resampleCB, resampleEvents := eui.NewCheckbox()
	resampleAudioCB = resampleCB
	resampleCB.Text = "High quality resampling"
	resampleCB.Size = eui.Point{X: columnWidth, Y: 24}
	resampleCB.Checked = gs.HighQualityResampling
	resampleCB.SetTooltip("Lanczos resampling and dithering for cleaner audio (uses more CPU)")
	resampleEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			gs.HighQualityResampling = ev.Checked
			setHighQualityResamplingEnabled(ev.Checked)
			clearCaches()
			settingsDirty = true
		}
	}
	chatCol.AddItem(resampleCB)

	musicEnhancementCB, musicEnhancementEvents := eui.NewCheckbox()
	musicEnhanceCB = musicEnhancementCB
	musicEnhancementCB.Text = "Audio enhancement for music"
	musicEnhancementCB.Size = eui.Point{X: columnWidth, Y: 24}
	musicEnhancementCB.Checked = gs.MusicEnhancement
	musicEnhancementCB.SetTooltip("Add space and ambience to background music")
	musicEnhancementEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			gs.MusicEnhancement = ev.Checked
			settingsDirty = true
		}
	}
	chatCol.AddItem(musicEnhancementCB)

	addSectionLabel(systemCol, "Network")

	altNetCB, altNetEvents := eui.NewCheckbox()
	altNetCB.Text = "Alt Networking"
	altNetCB.Size = eui.Point{X: columnWidth, Y: 24}
	altNetCB.Checked = gs.AltNetMode
	altNetCB.SetTooltip("Send input after a delay following server packets")
	altNetEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			gs.AltNetMode = ev.Checked
			settingsDirty = true
		}
	}
	systemCol.AddItem(altNetCB)

	netDelaySlider, netDelayEvents := eui.NewSlider()
	netDelaySlider.Label = "Net Delay (ms)"
	netDelaySlider.MinValue = 0
	netDelaySlider.MaxValue = 190
	netDelaySlider.Value = float32(gs.AltNetDelay)
	netDelaySlider.Size = eui.Point{X: columnWidth - 10, Y: 24}
	netDelayEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventSliderChanged {
			gs.AltNetDelay = int(ev.Value)
			settingsDirty = true
		}
	}
	systemCol.AddItem(netDelaySlider)

	serverInput, serverEvents := eui.NewInput()
	serverInput.Label = "Server address"
	serverInput.Text = gs.ServerAddress
	serverInput.TextPtr = &gs.ServerAddress
	serverInput.Size = eui.Point{X: columnWidth, Y: 24}
	serverInput.SetTooltip("Hostname and port used for the primary server")
	serverEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventInputChanged {
			SettingsLock.Lock()
			gs.ServerAddress = strings.TrimSpace(ev.Text)
			SettingsLock.Unlock()
			settingsDirty = true
			applyServerAddressSetting()
		}
	}
	systemCol.AddItem(serverInput)

	pingLabel, _ := eui.NewText()
	pingLabel.Text = ""
	pingLabel.Size = eui.Point{X: columnWidth, Y: 24}
	pingLabel.FontSize = 10
	systemCol.AddItem(pingLabel)

	pingBtn, pingEvents := eui.NewButton()
	pingBtn.Text = "Ping Server"
	pingBtn.Size = eui.Point{X: columnWidth, Y: 24}
	pingEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			SettingsLock.Lock()
			connected := tcpConn != nil
			SettingsLock.Unlock()
			if !connected {
				pingLabel.Text = "not connected to server"
				pingLabel.Dirty = true
				advancedWin.Refresh()
				return
			}
			pingLabel.Text = "Pinging..."
			pingLabel.Dirty = true
			advancedWin.Refresh()
			go func() {
				worst := time.Duration(0)
				for i := 0; i < 5; i++ {
					rtt := pingServer()
					if rtt > worst {
						worst = rtt
					}
					if i < 4 {
						time.Sleep(200 * time.Millisecond)
					}
				}
				pingLabel.Text = fmt.Sprintf("Ping: %d ms", worst.Milliseconds())
				pingLabel.Dirty = true
				advancedWin.Refresh()
			}()
		}
	}
	systemCol.AddItem(pingBtn)

	addSectionLabel(systemCol, "Performance")

	psBGCB, psBGEvents := eui.NewCheckbox()
	psBGCB.Text = "Power save in background"
	psBGCB.Size = eui.Point{X: columnWidth, Y: 24}
	psBGCB.Checked = gs.PowerSaveBackground
	psBGCB.SetTooltip("Reduce FPS when window is unfocused")
	psBGEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			SettingsLock.Lock()
			gs.PowerSaveBackground = ev.Checked
			SettingsLock.Unlock()
			settingsDirty = true
		}
	}
	systemCol.AddItem(psBGCB)

	psAlwaysCB, psAlwaysEvents := eui.NewCheckbox()
	psAlwaysCB.Text = "Always power save"
	psAlwaysCB.Size = eui.Point{X: columnWidth, Y: 24}
	psAlwaysCB.Checked = gs.PowerSaveAlways
	psAlwaysCB.SetTooltip("Limit FPS even when focused (useful on laptops)")
	psAlwaysEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			SettingsLock.Lock()
			gs.PowerSaveAlways = ev.Checked
			SettingsLock.Unlock()
			settingsDirty = true
		}
	}
	systemCol.AddItem(psAlwaysCB)

	psFPSSlider, psFPSEvents := eui.NewSlider()
	psFPSSlider.Label = "Power-save FPS"
	psFPSSlider.MinValue = 1
	psFPSSlider.MaxValue = 60
	psFPSSlider.IntOnly = true
	if gs.PowerSaveFPS < 1 {
		gs.PowerSaveFPS = 1
	}
	if gs.PowerSaveFPS > 60 {
		gs.PowerSaveFPS = 60
	}
	psFPSSlider.Value = float32(gs.PowerSaveFPS)
	psFPSSlider.Size = eui.Point{X: columnWidth - 10, Y: 24}
	psFPSSlider.SetTooltip("Target FPS when power saving is active")
	psFPSEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventSliderChanged {
			SettingsLock.Lock()
			v := int(ev.Value)
			if v < 1 {
				v = 1
			}
			if v > 60 {
				v = 60
			}
			gs.PowerSaveFPS = v
			SettingsLock.Unlock()
			psFPSSlider.Value = float32(v)
			settingsDirty = true
		}
	}
	systemCol.AddItem(psFPSSlider)

	columns.AddItem(toolsCol)
	columns.AddItem(interfaceCol)
	columns.AddItem(chatCol)
	columns.AddItem(systemCol)

	advancedWin.AddItem(columns)
	advancedWin.AddWindow(false)
}

func makeBubbleWindow() {
	if bubbleWin != nil {
		return
	}
	var width float32 = 250
	bubbleWin = eui.NewWindow()
	bubbleWin.Title = "Bubble Settings"
	bubbleWin.Closable = true
	bubbleWin.Resizable = false
	bubbleWin.AutoSize = true
	bubbleWin.Movable = true
	bubbleWin.SetZone(eui.HZoneCenterLeft, eui.VZoneMiddleTop)

	flow := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL}

	// Quick toggle for message bubbles in Chat & Audio
	bubblesQuickCB, bubblesQuickEvents := eui.NewCheckbox()
	bubblesQuickCB.Text = "Message Bubbles"
	bubblesQuickCB.Size = eui.Point{X: width, Y: 24}
	bubblesQuickCB.Checked = gs.SpeechBubbles
	bubblesQuickCB.SetTooltip("Show speech bubbles in game")
	bubblesQuickEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			gs.SpeechBubbles = ev.Checked
			settingsDirty = true
		}
	}
	flow.AddItem(bubblesQuickCB)

	addBubbleCB := func(label string, val *bool) {
		cb, events := eui.NewCheckbox()
		cb.Text = label
		cb.Size = eui.Point{X: width, Y: 24}
		cb.Checked = *val
		events.Handle = func(ev eui.UIEvent) {
			if ev.Type == eui.EventCheckboxChanged {
				*val = ev.Checked
				settingsDirty = true
			}
		}
		flow.AddItem(cb)
	}

	addBubbleCB("Normal", &gs.BubbleNormal)
	addBubbleCB("Whisper", &gs.BubbleWhisper)
	addBubbleCB("Yell", &gs.BubbleYell)
	addBubbleCB("Thought", &gs.BubbleThought)
	addBubbleCB("Real Action", &gs.BubbleRealAction)
	addBubbleCB("Monster", &gs.BubbleMonster)
	addBubbleCB("Player Action", &gs.BubblePlayerAction)
	addBubbleCB("Ponder", &gs.BubblePonder)
	addBubbleCB("Narrate", &gs.BubbleNarrate)
	addBubbleCB("Self", &gs.BubbleSelf)
	addBubbleCB("Other Players", &gs.BubbleOtherPlayers)
	addBubbleCB("Monsters", &gs.BubbleMonsters)
	addBubbleCB("Narration", &gs.BubbleNarration)

	bubbleWin.AddItem(flow)
	bubbleWin.AddWindow(false)
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

	recordStatsCB, recordStatsEvents := eui.NewCheckbox()
	recordStatsCB.Text = "Record Asset Stats"
	recordStatsCB.Size = eui.Point{X: width, Y: 24}
	recordStatsCB.Checked = gs.recordAssetStats
	recordStatsCB.SetTooltip("Writes stats.json with number of times image-id is loaded")
	recordStatsEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			gs.recordAssetStats = ev.Checked
			settingsDirty = true
		}
	}
	debugFlow.AddItem(recordStatsCB)

	hideMoveCB, hideMoveEvents := eui.NewCheckbox()
	hideMoveCB.Text = "Hide Moving Objects"
	hideMoveCB.SetTooltip("Helpful for screenshots")
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
	hideMobCB.SetTooltip("Helpful for screenshots")
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
	planesCB.SetTooltip("Shows plane (layer) number on each sprite")
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
	pictIDCB.SetTooltip("Shows picture ID on each sprite")
	pictIDCB.Size = eui.Point{X: width, Y: 24}
	pictIDCB.Checked = gs.pictIDDebug
	pictIDEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			gs.pictIDDebug = ev.Checked
			settingsDirty = true
		}
	}
	debugFlow.AddItem(pictIDCB)

	scriptOutCB, scriptOutEvents := eui.NewCheckbox()
	scriptOutCB.Text = "Always show script output"
	scriptOutCB.Size = eui.Point{X: width, Y: 24}
	scriptOutCB.Checked = gs.scriptOutputDebug
	scriptOutEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			gs.scriptOutputDebug = ev.Checked
			settingsDirty = true
		}
	}
	debugFlow.AddItem(scriptOutCB)

	// Add a small "Reload" button beside the shader checkbox for hot-reload.
	reloadBtn, reloadEv := eui.NewButton()
	reloadBtn.Text = "Reload Shaders"
	reloadBtn.Size = eui.Point{X: 160, Y: 24}
	reloadBtn.SetTooltip("Recompile the lighting shader from data/shaders/light.kage")
	reloadEv.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			if err := ReloadLightingShader(); err != nil {
				consoleMessage("Shader reload failed:" + err.Error())
			} else {
				consoleMessage("Shader reloaded.")
			}
		}
	}

	shaderRow := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_HORIZONTAL, Fixed: true}
	shaderRow.AddItem(reloadBtn)
	debugFlow.AddItem(shaderRow)

	// Force Night dropdown in Debug: Auto/Day/25/50/75/100
	forceNightDD, forceNightEv := eui.NewDropdown()
	forceNightDD.Label = "Force Night"
	forceNightDD.Options = []string{"Auto", "Day (0%)", "25%", "50%", "75%", "Night (100%)"}
	// Map gs.ForceNightLevel to option index
	switch gs.forceNightLevel {
	case -1:
		forceNightDD.Selected = 0
	case 0:
		forceNightDD.Selected = 1
	case 25:
		forceNightDD.Selected = 2
	case 50:
		forceNightDD.Selected = 3
	case 75:
		forceNightDD.Selected = 4
	case 100:
		forceNightDD.Selected = 5
	default:
		forceNightDD.Selected = 0
	}
	forceNightDD.Size = eui.Point{X: width, Y: 24}
	forceNightEv.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventDropdownSelected {
			switch ev.Index {
			case 0:
				gs.forceNightLevel = -1
			case 1:
				gs.forceNightLevel = 0
			case 2:
				gs.forceNightLevel = 25
			case 3:
				gs.forceNightLevel = 50
			case 4:
				gs.forceNightLevel = 75
			case 5:
				gs.forceNightLevel = 100
			}
			settingsDirty = true
		}
	}
	debugFlow.AddItem(forceNightDD)

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

	scaledFrameCacheLabel, _ = eui.NewText()
	scaledFrameCacheLabel.Text = ""
	scaledFrameCacheLabel.Size = eui.Point{X: width, Y: 24}
	scaledFrameCacheLabel.FontSize = 10
	debugFlow.AddItem(scaledFrameCacheLabel)

	mobileCacheLabel, _ = eui.NewText()
	mobileCacheLabel.Text = ""
	mobileCacheLabel.Size = eui.Point{X: width, Y: 24}
	mobileCacheLabel.FontSize = 10
	debugFlow.AddItem(mobileCacheLabel)

	scaledMobileCacheLabel, _ = eui.NewText()
	scaledMobileCacheLabel.Text = ""
	scaledMobileCacheLabel.Size = eui.Point{X: width, Y: 24}
	scaledMobileCacheLabel.FontSize = 10
	debugFlow.AddItem(scaledMobileCacheLabel)

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
	clearCacheBtn.SetTooltip("Clear cached assets")
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

	stats := imageCacheStats()
	soundCount, soundBytes := soundCacheStats()

	if sheetCacheLabel != nil {
		sheetCacheLabel.Text = fmt.Sprintf("Sprite Sheets: %d (%s)", stats.sheetCount, humanize.Bytes(uint64(stats.sheetBytes)))
		sheetCacheLabel.Dirty = true
	}
	if frameCacheLabel != nil {
		frameCacheLabel.Text = fmt.Sprintf("Animation Frames: %d (%s)", stats.frameCount, humanize.Bytes(uint64(stats.frameBytes)))
		frameCacheLabel.Dirty = true
	}
	if scaledFrameCacheLabel != nil {
		scaledFrameCacheLabel.Text = fmt.Sprintf("Upscaled Frames: %d (%s)", stats.scaledFrameCount, humanize.Bytes(uint64(stats.scaledFrameBytes)))
		scaledFrameCacheLabel.Dirty = true
	}
	if mobileCacheLabel != nil {
		mobileCacheLabel.Text = fmt.Sprintf("Mobile Animation Frames: %d (%s)", stats.mobileCount, humanize.Bytes(uint64(stats.mobileBytes)))
		mobileCacheLabel.Dirty = true
	}
	if scaledMobileCacheLabel != nil {
		scaledMobileCacheLabel.Text = fmt.Sprintf("Upscaled Mobile Frames: %d (%s)", stats.scaledMobileCount, humanize.Bytes(uint64(stats.scaledMobileBytes)))
		scaledMobileCacheLabel.Dirty = true
	}
	if mobileBlendLabel != nil {
		mobileBlendLabel.Text = fmt.Sprintf("Mobile Blend Frames: %d (%s)", stats.mobileBlendCount, humanize.Bytes(uint64(stats.mobileBlendBytes)))
		mobileBlendLabel.Dirty = true
	}
	if pictBlendLabel != nil {
		pictBlendLabel.Text = fmt.Sprintf("World Blend Frames: %d (%s)", stats.pictBlendCount, humanize.Bytes(uint64(stats.pictBlendBytes)))
		pictBlendLabel.Dirty = true
	}
	if soundCacheLabel != nil {
		soundCacheLabel.Text = fmt.Sprintf("Sounds: %d (%s)", soundCount, humanize.Bytes(uint64(soundBytes)))
		soundCacheLabel.Dirty = true
	}
	if totalCacheLabel != nil {
		total := stats.sheetBytes + stats.frameBytes + stats.scaledFrameBytes + stats.mobileBytes + stats.scaledMobileBytes + stats.mobileBlendBytes + stats.pictBlendBytes + soundBytes
		totalCacheLabel.Text = fmt.Sprintf("Total: %s", humanize.Bytes(uint64(total)))
		totalCacheLabel.Dirty = true
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
				if chatWin == nil {
					_ = makeChatWindow()
				}
				if chatWin != nil {
					chatWin.MarkOpenNear(ev.Item)
				}
			} else if chatWin != nil {
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

	helpBox, helpBoxEvents := eui.NewCheckbox()
	windowsHelpCB = helpBox
	helpBox.Text = "Help"
	helpBox.Size = eui.Point{X: 128, Y: 24}
	helpBox.Checked = helpWin != nil && helpWin.IsOpen()
	helpBoxEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			if ev.Checked {
				openHelpWindow(ev.Item)
			} else {
				helpWin.Close()
			}
		}
	}
	flow.AddItem(helpBox)

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
	playersWin.Searchable = true
	playersWin.OnSearch = func(s string) { searchTextWindow(playersWin, playersList, s) }
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
