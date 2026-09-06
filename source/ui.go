package main

import (
	"bytes"
	"context"
	_ "embed"
	"errors"
	"fmt"
	"image"
	"image/png"
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
	clipboard "golang.design/x/clipboard"

	"gothoom/climg"
)

const cval = 1000

const userManualURL = "https://gothoom.m45sci.xyz/help"

var (
	TOP_RIGHT = eui.Point{X: cval, Y: 0}
	TOP_LEFT  = eui.Point{X: 0, Y: 0}

	BOTTOM_LEFT  = eui.Point{X: 0, Y: cval}
	BOTTOM_RIGHT = eui.Point{X: cval, Y: cval}
)

var loginWin *eui.WindowData
var downloadWin *eui.WindowData
var charactersList *eui.ItemData
var tileLayoutWin *eui.WindowData
var settingsToolbarPlacementDD *eui.ItemData
var settingsCombineMessagesCB *eui.ItemData
var connectWin *eui.WindowData
var connectStatusText *eui.ItemData
var loginConnectButton *eui.ItemData
var loginServerDropdown *eui.ItemData
var serverListWin *eui.WindowData
var serverListContents *eui.ItemData
var serverListAddress string
var demoCharacterWin *eui.WindowData
var demoCharacterList *eui.ItemData
var demoCharacterSelection string
var addCharWin *eui.WindowData
var addCharName string
var addCharPass string
var addCharRemember bool
var addCharProfile bool
var addCharProfileCB *eui.ItemData
var editCharWin *eui.WindowData
var editCharName string
var editCharPass string
var editCharPassInput *eui.ItemData
var editCharPassWarn *eui.ItemData
var editCharPassPrev string
var editCharRemember bool
var editCharRememberCB *eui.ItemData
var editCharProfile bool
var editCharProfileCB *eui.ItemData
var editCharBtn *eui.ItemData
var deleteCharBtn *eui.ItemData
var passWin *eui.WindowData
var passInput *eui.ItemData
var passWarn *eui.ItemData
var passPrev string
var passRemember bool
var passRememberCB *eui.ItemData

var changelogWin *eui.WindowData

func refreshShaderEffectControls() {
	masterDisabled := !gs.ShadersEnabled
	if shadersEnabledCB != nil {
		shadersEnabledCB.Checked = gs.ShadersEnabled
	}
	if upscaleModeDD != nil {
		upscaleModeDD.Selected = artworkUpscaleMode()
		upscaleModeDD.Disabled = false
	}
	if shaderLightingCB != nil {
		shaderLightingCB.Checked = gs.ShaderLighting
		shaderLightingCB.Disabled = masterDisabled
	}
	lightingDisabled := masterDisabled || !gs.ShaderLighting
	if mobileLightConeShadowsCB != nil {
		mobileLightConeShadowsCB.Checked = gs.MobileLightConeShadows
		mobileLightConeShadowsCB.Disabled = lightingDisabled
	}
	if fasterCharacterShadowsCB != nil {
		fasterCharacterShadowsCB.Checked = gs.FasterCharacterShadows
		fasterCharacterShadowsCB.Disabled = masterDisabled || !gs.CharacterShadows
	}
	if shaderLightSlider != nil {
		shaderLightSlider.Disabled = lightingDisabled
	}
	if shaderGlowSlider != nil {
		shaderGlowSlider.Disabled = lightingDisabled
	}
	if flameFlickerCB != nil {
		flameFlickerCB.Checked = gs.FlameLightFlicker
		flameFlickerCB.Disabled = lightingDisabled
	}
	if flameFlickerSlider != nil {
		flameFlickerSlider.Disabled = lightingDisabled || !gs.FlameLightFlicker
	}
	if replacementEffectsCB != nil {
		replacementEffectsCB.Checked = gs.ReplacementEffects
		replacementEffectsCB.Disabled = masterDisabled
	}
	if smallMovingPicturesCB != nil {
		smallMovingPicturesCB.Checked = gs.InterpolateSmallMovingPictures
		smallMovingPicturesCB.Disabled = !gs.MotionSmoothing
	}
	frameBlendDisabled := masterDisabled || !gs.MotionSmoothing
	if animCB != nil {
		animCB.Checked = gs.BlendMobiles
		animCB.Disabled = frameBlendDisabled
	}
	if pictBlendCB != nil {
		pictBlendCB.Checked = gs.BlendPicts
		pictBlendCB.Disabled = frameBlendDisabled
	}
	if mobileBlendSlider != nil {
		mobileBlendSlider.Disabled = frameBlendDisabled || !gs.BlendMobiles
	}
	if worldBlendSlider != nil {
		worldBlendSlider.Disabled = frameBlendDisabled || !gs.BlendPicts
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
var newScriptWin *eui.WindowData
var scriptsRoot *eui.ItemData
var scriptsHeader *eui.ItemData
var scriptsList *eui.ItemData
var scriptsButtons *eui.ItemData
var scriptDetails *eui.ItemData
var scriptInfoWin *eui.WindowData
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
var toolbarRoot *eui.ItemData
var toolbarStatsText *eui.ItemData
var toolbarStatsOnce sync.Once
var shaderWarnWin *eui.WindowData
var shaderWarnDontShowCB *eui.ItemData

//go:embed data/images/hands.png
var toolbarHandsPNG []byte

var (
	toolbarHandsOnce      sync.Once
	toolbarHandsSrc       image.Image
	toolbarHandsImage     *ebiten.Image
	leftHandImg           *eui.ItemData
	rightHandImg          *eui.ItemData
	toolbarLeftComposite  *ebiten.Image
	toolbarRightComposite *ebiten.Image
	toolbarHandsRendered  bool
	toolbarHandsRightID   uint16
	toolbarHandsLeftID    uint16
	toolbarHandsTargetL   *eui.ItemData
	toolbarHandsTargetR   *eui.ItemData
	toolbarHandsSourceGPU *ebiten.Image
)

var (
	sheetCacheLabel        *eui.ItemData
	frameCacheLabel        *eui.ItemData
	scaledFrameCacheLabel  *eui.ItemData
	mobileCacheLabel       *eui.ItemData
	scaledMobileCacheLabel *eui.ItemData
	spriteSlotCacheLabel   *eui.ItemData
	renderPoolCacheLabel   *eui.ItemData
	soundCacheLabel        *eui.ItemData
	totalCacheLabel        *eui.ItemData

	recordBtn                *eui.ItemData
	recordPath               string
	qualityPresetDD          *eui.ItemData
	qualityRenderScaleSlider *eui.ItemData
	fadeObscuringCB          *eui.ItemData
	characterShadowsCB       *eui.ItemData
	mobileSunShadowsCB       *eui.ItemData
	characterShadowSlider    *eui.ItemData
	shaderLightSlider        *eui.ItemData
	shaderGlowSlider         *eui.ItemData
	flameFlickerCB           *eui.ItemData
	flameFlickerSlider       *eui.ItemData
	gammaCorrectionCB        *eui.ItemData
	spriteGammaSlider        *eui.ItemData
	monitorGammaSlider       *eui.ItemData
	denoiseCB                *eui.ItemData
	motionCB                 *eui.ItemData
	smallMovingPicturesCB    *eui.ItemData
	animCB                   *eui.ItemData
	pictBlendCB              *eui.ItemData
	shadersEnabledCB         *eui.ItemData
	shaderLightingCB         *eui.ItemData
	mobileLightConeShadowsCB *eui.ItemData
	fasterCharacterShadowsCB *eui.ItemData
	upscaleModeDD            *eui.ItemData
	replacementEffectsCB     *eui.ItemData
	mobileBlendSlider        *eui.ItemData
	worldBlendSlider         *eui.ItemData
	throttleSoundCB          *eui.ItemData
	resampleAudioCB          *eui.ItemData
	precacheSoundCB          *eui.ItemData
	noCacheCB                *eui.ItemData
	potatoCB                 *eui.ItemData
	windowShadowsCB          *eui.ItemData
	volumeSlider             *eui.ItemData
	muteBtn                  *eui.ItemData
	mixerWin                 *eui.WindowData
	gameMixSlider            *eui.ItemData
	musicMixSlider           *eui.ItemData
	ttsMixSlider             *eui.ItemData
	notifMixSlider           *eui.ItemData
	soundEnhanceMixCB        *eui.ItemData
	soundEnhanceSlider       *eui.ItemData
	musicEnhanceMixCB        *eui.ItemData
	musicEnhanceSlider       *eui.ItemData
	mixMuteBtn               *eui.ItemData
	musicMixCB               *eui.ItemData
	ttsMixCB                 *eui.ItemData
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
	if editCharPassWarn != nil {
		editCharPassWarn.Text = ""
		editCharPassWarn.Dirty = true
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
				enqueueCommand("/be-who")
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

	eui.SetUserUIScale(float32(gs.UIScale))

	makeGameWindow()
	makeDownloadsWindow()
	makeLoginWindow()
	makeAddCharacterWindow()
	makeEditCharacterWindow()
	makeChatWindow()
	makeConsoleWindow()
	makeSettingsWindow()
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
	makeKeybindingsWindow()
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
	if clmov == "" && pcapPath == "" && !fake && clImages != nil && currentCLSoundsArchive() != nil && !status.NeedImages && !status.NeedSounds && shouldShowSetupWizard(settingsLoaded, gs.SetupWizardVersion, appVersion) {
		openSetupWizard(false)
	}
}

func buildToolbar(toolFontSize, buttonWidth, buttonHeight float32) *eui.ItemData {
	var row1, row2, menu *eui.ItemData

	row1 = eui.NewRow()
	row2 = eui.NewRow()
	menu = eui.NewColumn()

	winBtn, winEvents := eui.NewButton()
	winBtn.Text = "Windows"
	setMaterialButtonIcon(winBtn, "window")
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
	setMaterialButtonIcon(btn, "settings")
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
	setMaterialButtonIcon(actionsBtn, "bolt")
	actionsBtn.SetTooltip("Open hotkeys, shortcuts, keybindings, scripts, macros, and saved data.")
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
			"Keybindings",
			"Scripts",
			"Legacy Macros",
			"Saved Data",
		}
		icons := materialMenuIcons(
			"keyboard_command_key",
			"shortcut",
			"keyboard",
			"code",
			"terminal",
			"save",
		)
		eui.ShowContextMenuWithIcons(options, icons, r.X0, r.Y1, func(i int) {
			switch i {
			case 0:
				hotkeysWin.ToggleNear(actionsBtn)
			case 1:
				refreshShortcutsList()
				shortcutsWin.ToggleNear(actionsBtn)
			case 2:
				refreshKeybindingsList()
				keybindingsWin.ToggleNear(actionsBtn)
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
	setMaterialButtonIcon(recordBtn, "fiber_manual_record")
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
	setMaterialButtonIcon(helpBtn, "help")
	helpBtn.SetTooltip("Open the goThoom user manual in your browser.")
	helpBtn.Size = eui.Point{X: buttonWidth, Y: buttonHeight}
	helpBtn.FontSize = toolFontSize
	helpEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			if err := open.Run(userManualURL); err != nil {
				consoleMessage("open user manual: " + err.Error())
			}
		}
	}
	row2.AddItem(helpBtn)

	shotBtn, shotEvents := eui.NewButton()
	shotBtn.Text = "Snap"
	setMaterialButtonIcon(shotBtn, "photo_camera")
	shotBtn.SetTooltip("Open snapshot options to choose a filename, capture area, and image format.")
	shotBtn.Size = eui.Point{X: buttonWidth, Y: buttonHeight}
	shotBtn.FontSize = toolFontSize
	shotEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			showSnapshotWindow()
		}
	}
	row2.AddItem(shotBtn)

	paletteBtn, paletteEvents := eui.NewButton()
	paletteBtn.Text = "Palette"
	setMaterialButtonIcon(paletteBtn, "search")
	paletteBtn.SetTooltip("Search settings, windows, scripts, player actions, and commands (Ctrl+Shift+P).")
	paletteBtn.Size = eui.Point{X: buttonWidth, Y: buttonHeight}
	paletteBtn.FontSize = toolFontSize
	paletteEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			toggleCommandPalette()
		}
	}
	row2.AddItem(paletteBtn)

	mixBtn, mixEvents := eui.NewButton()
	mixBtn.Text = "Audio"
	setMaterialButtonIcon(mixBtn, "volume_up")
	mixBtn.SetTooltip("Adjust game, music, speech, notification, and enhancement levels.")
	mixBtn.Size = eui.Point{X: buttonWidth, Y: buttonHeight}
	mixBtn.FontSize = toolFontSize
	mixEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			mixerWin.ToggleNear(ev.Item)
		}
	}
	row1.AddItem(mixBtn)

	statsBtn, statsEvents := eui.NewButton()
	statsBtn.Text = "Stats"
	setMaterialButtonIcon(statsBtn, "query_stats")
	statsBtn.SetTooltip("Show live network, frame-rate, and cache statistics.")
	statsBtn.Size = eui.Point{X: buttonWidth, Y: buttonHeight}
	statsBtn.FontSize = toolFontSize
	statsEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			makeStatsWindow()
			statsWin.ToggleNear(ev.Item)
			lastStatsRender = time.Time{}
			updateStatsWindow(time.Now())
		}
	}
	row1.AddItem(statsBtn)

	exitBtn, exitEvents := eui.NewButton()
	exitBtn.Text = "Logout"
	setMaterialButtonIcon(exitBtn, "logout")
	exitBtn.SetTooltip("Disconnect and return to the login screen.")
	exitBtn.Size = eui.Point{X: buttonWidth, Y: buttonHeight}
	exitBtn.FontSize = toolFontSize
	exitBtn.Color = eui.ColorDarkRed
	exitEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			confirmExitSession()
		}
	}
	row2.AddItem(exitBtn)

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
	scriptsWin.Resizable = true
	scriptsWin.NoScroll = true
	scriptsWin.Size = eui.Point{X: scriptsManagerListWidth, Y: 640}
	scriptsWin.Movable = true
	scriptsWin.SetZone(eui.HZoneCenterLeft, eui.VZoneMiddleTop)

	root := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL, Fixed: true}
	scriptsRoot = root
	scriptsWin.AddItem(root)

	listHeader := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_HORIZONTAL, Fixed: true}
	scriptsHeader = listHeader
	for _, column := range []struct {
		label string
		width float32
	}{
		{label: "", width: scriptsManagerInfoSize},
		{label: "Player", width: scriptsManagerCheckSize},
		{label: "Global", width: scriptsManagerCheckSize},
		{label: "Script", width: scriptsManagerNameWidth},
	} {
		text, _ := eui.NewText()
		text.Text = column.label
		text.FontSize = 9
		text.Size = eui.Point{X: column.width, Y: scriptsManagerRowHeight}
		listHeader.AddItem(text)
	}
	root.AddItem(listHeader)

	list := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL, Scrollable: true, Fixed: true}
	list.Size = eui.Point{X: scriptsManagerListWidth, Y: scriptsManagerPaneHeight}
	scriptsList = list
	root.AddItem(list)

	buttonsBottom := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_HORIZONTAL, Fixed: true}
	scriptsButtons = &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL, Fixed: true}
	root.AddItem(scriptsButtons)
	scriptsButtons.AddItem(buttonsBottom)

	refreshBtn, rh := eui.NewButton()
	refreshBtn.Text = "Refresh"
	setMaterialButtonIcon(refreshBtn, "refresh")
	refreshBtn.SetTooltip("Rescan scripts and reload list")
	refreshBtn.Size = eui.Point{X: 80, Y: 24}
	rh.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			rescanscripts()
		}
	}
	buttonsBottom.AddItem(refreshBtn)

	openBtn, oh := eui.NewButton()
	openBtn.Text = "Open scripts folder"
	setMaterialButtonIcon(openBtn, "folder_open")
	// Label already clear; no tooltip.
	openBtn.Size = eui.Point{X: 160, Y: 24}
	oh.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			open.Run(userScriptsDir())
		}
	}
	buttonsBottom.AddItem(openBtn)

	eventsBtn, eventsHandler := eui.NewButton()
	eventsBtn.Text = "Script Events"
	eventsBtn.Size = eui.Point{X: 120, Y: 24}
	eventsHandler.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			makeScriptEventsWindow()
			scriptEventsWin.ToggleNear(ev.Item)
		}
	}
	buttonsBottom.AddItem(eventsBtn)

	scriptKillCB, scriptKillEvents := eui.NewCheckbox()
	scriptKillCB.Text = "Auto-kill spammy scripts"
	scriptKillCB.Size = eui.Point{X: 240, Y: 24}
	scriptKillCB.Checked = gs.ScriptSpamKill
	scriptKillEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			SettingsLock.Lock()
			gs.ScriptSpamKill = ev.Checked
			SettingsLock.Unlock()
			settingsDirty = true
		}
	}
	scriptsButtons.AddItem(scriptKillCB)

	scriptsWin.OnResize = refreshscriptsWindow
	scriptsWin.AddWindow(false)
	refreshscriptsWindow()
}

const (
	scriptsManagerCheckSize  = 32
	scriptsManagerInfoSize   = 28
	scriptsManagerNameWidth  = 400
	scriptsManagerInfoWidth  = 600
	scriptsManagerListWidth  = 640
	scriptsManagerPaneHeight = 420
	scriptsManagerRowHeight  = 32
)

type newScriptTemplate struct {
	name        string
	filename    string
	description string
	imports     string
	body        string
}

var newScriptTemplates = []newScriptTemplate{
	{
		name: "Command", filename: "my_command", description: "Adds a /hello command.",
		imports: `import "gt2"`,
		body: `func Init() {
	gt2.Command("hello", func(args string) {
		gt2.Print("Hello " + args)
	})
}
`,
	},
	{
		name: "Hotkey / Click", filename: "my_click", description: "Handles Shift-click and consumes it.",
		imports: `import "gt2"`,
		body: `func Init() {
	gt2.Bind("Shift-LeftClick", func(event gt2.InputEvent) {
		event.Consume()
		gt2.Print("Shift-clicked")
	})
}
`,
	},
	{
		name: "Chat Event", filename: "my_chat_event", description: "Responds when chat contains hello.",
		imports: `import "gt2"`,
		body: `func Init() {
	gt2.OnChat(gt2.ChatFilter{Contains: "hello"}, func(event gt2.ChatEvent) {
		gt2.Print(event.Speaker + " said: " + event.Message)
	})
}
`,
	},
	{
		name: "Equipment Sequence", filename: "my_equipment_sequence", description: "Temporarily equips an item while running actions.",
		imports: `import (
	"gt2"
	"time"
)`,
		body: `func Init() {
	gt2.Command("sequence", func(string) {
		gt2.WithEquipment("item name", func() {
			gt2.Send("/action")
			gt2.Wait(time.Second)
			gt2.Send("/action")
		})
	})
}
`,
	},
}

func openNewScriptWindow() {
	if newScriptWin != nil {
		newScriptWin.MarkOpen()
		return
	}
	newScriptWin = eui.NewWindow()
	newScriptWin.Title = "New Script"
	newScriptWin.Closable = true
	newScriptWin.Resizable = false
	newScriptWin.AutoSize = true
	newScriptWin.Movable = true
	newScriptWin.OnClose = func() { newScriptWin = nil }
	newScriptWin.SetZone(eui.HZoneCenterLeft, eui.VZoneMiddleTop)
	root := eui.NewColumn()
	newScriptWin.AddItem(root)
	intro, _ := eui.NewText()
	intro.Text = "Choose a starting point:"
	intro.Size = eui.Point{X: 360, Y: 24}
	root.AddItem(intro)
	for _, template := range newScriptTemplates {
		template := template
		row := eui.NewRow()
		button, events := eui.NewButton()
		button.Text = template.name
		button.Size = eui.Point{X: 140, Y: 24}
		events.Handle = func(event eui.UIEvent) {
			if event.Type != eui.EventClick {
				return
			}
			path, err := createScriptFromTemplate(userScriptsDir(), template)
			if err != nil {
				consoleMessage("[script] create: " + err.Error())
				return
			}
			rescanscripts()
			if err := open.Run(path); err != nil {
				consoleMessage("[script] open file: " + err.Error())
			}
			newScriptWin.Close()
			newScriptWin = nil
		}
		row.AddItem(button)
		description, _ := eui.NewText()
		description.Text = template.description
		description.Size = eui.Point{X: 320, Y: 24}
		row.AddItem(description)
		root.AddItem(row)
	}
	newScriptWin.AddWindow(false)
}

func createScriptFromTemplate(dir string, template newScriptTemplate) (string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	for number := 1; ; number++ {
		base := template.filename
		if number > 1 {
			base += "_" + strconv.Itoa(number)
		}
		path := filepath.Join(dir, base+".go")
		file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if os.IsExist(err) {
			continue
		}
		if err != nil {
			return "", err
		}
		displayName := template.name
		if number > 1 {
			displayName += " " + strconv.Itoa(number)
		}
		source := "package main\n\n" + template.imports + "\n\n" +
			"const scriptID = \"" + normalizeScriptID(base) + "\"\n" +
			"const scriptName = \"" + displayName + "\"\n" +
			"const scriptDescription = \"" + template.description + "\"\n" +
			"const scriptAPIVersion = 2\n\n" + template.body
		if _, err := file.WriteString(source); err != nil {
			_ = file.Close()
			_ = os.Remove(path)
			return "", err
		}
		if err := file.Close(); err != nil {
			_ = os.Remove(path)
			return "", err
		}
		return path, nil
	}
}

func refreshscriptsWindow() {
	if scriptsList == nil || scriptsWin == nil {
		return
	}
	savedScroll := scriptsList.Scroll
	layoutScriptsWindow()
	checkSize := eui.Point{X: scriptsManagerCheckSize, Y: scriptsManagerRowHeight}
	scriptSize := eui.Point{X: scriptsManagerNameWidth, Y: scriptsManagerRowHeight}

	scriptsList.Contents = scriptsList.Contents[:0]

	type entry struct {
		owner        string
		name         string
		cat          string
		sub          string
		invalid      bool
		disabled     bool
		errorText    string
		reloadFailed bool
	}
	scriptMu.RLock()
	cats := make(map[string][]entry)
	for o, n := range scriptDisplayNames {
		cats[scriptCategories[o]] = append(cats[scriptCategories[o]], entry{
			owner:        o,
			name:         n,
			cat:          scriptCategories[o],
			sub:          scriptSubCategories[o],
			invalid:      scriptInvalid[o],
			disabled:     scriptDisabled[o],
			errorText:    scriptErrors[o],
			reloadFailed: scriptReloadFailed[o],
		})
	}
	scriptMu.RUnlock()
	var catList []string
	for c := range cats {
		catList = append(catList, c)
	}
	sort.Strings(catList)
	for _, cat := range catList {
		row := eui.NewRow()
		infoSpacer := &eui.ItemData{ItemType: eui.ITEM_TEXT, Size: eui.Point{X: scriptsManagerInfoSize, Y: scriptsManagerRowHeight}, Fixed: true}
		spacer1 := &eui.ItemData{ItemType: eui.ITEM_TEXT, Size: checkSize, Fixed: true}
		spacer2 := &eui.ItemData{ItemType: eui.ITEM_TEXT, Size: checkSize, Fixed: true}
		row.AddItem(infoSpacer)
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
			row := eui.NewRow()
			charCB, charEvents := eui.NewCheckbox()
			charCB.Size = checkSize
			allCB, allEvents := eui.NewCheckbox()
			allCB.Size = checkSize
			// Consider LastCharacter before login so the per-character
			// checkbox reflects the saved preference.
			effChar := effectiveCharacterName()
			label := e.name
			if e.sub != "" {
				label += " [" + e.sub + "]"
			}
			label += " — " + scriptStatusLabel(e.disabled, e.invalid, e.errorText, e.reloadFailed)
			owner := e.owner
			scriptMu.RLock()
			scope := scriptEnabledFor[owner]
			scriptMu.RUnlock()
			charCB.Checked = effChar != "" && scope.Chars != nil && scope.Chars[effChar]
			charCB.Disabled = e.invalid || effChar == ""
			allCB.Checked = scope.All
			allCB.Disabled = e.invalid
			infoBtn, infoEvents := eui.NewButton()
			infoBtn.Text = "i"
			setMaterialIconOnly(infoBtn, "info", "i")
			infoBtn.Size = eui.Point{X: scriptsManagerInfoSize, Y: 24}
			infoBtn.SetTooltip("Show script details and actions.")
			infoEvents.Handle = func(ev eui.UIEvent) {
				if ev.Type == eui.EventClick {
					selectscript(owner)
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
			row.AddItem(infoBtn)
			row.AddItem(charCB)
			row.AddItem(allCB)
			nameTxt, _ := eui.NewText()
			nameTxt.Text = label
			nameTxt.FontSize = 12
			nameTxt.Size = scriptSize
			nameTxt.Disabled = e.invalid
			row.AddItem(nameTxt)

			if !e.invalid {
				reloadBtn, rh := eui.NewButton()
				reloadBtn.Text = "Reload"
				setMaterialIconOnly(reloadBtn, "restart_alt", "Reload")
				reloadBtn.SetTooltip("Restart this script if enabled")
				reloadBtn.Size = eui.Point{X: 32, Y: 24}
				rh.Handle = func(ev eui.UIEvent) {
					if ev.Type == eui.EventClick {
						scriptMu.RLock()
						enabled := !scriptDisabled[owner]
						scriptMu.RUnlock()
						if enabled {
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
					setMaterialButtonIcon(cfgBtn, "settings")
					cfgBtn.Size = eui.Point{X: 82, Y: 24}
					ch.Handle = func(ev eui.UIEvent) {
						if ev.Type == eui.EventClick {
							openscriptConfigWindow(owner)
						}
					}
					row.AddItem(cfgBtn)
				}
			}
			scriptsList.AddItem(row)
		}
	}
	// Adding the first replacement row temporarily makes the list shorter than
	// its viewport, which causes EUI to clamp its scroll to zero. Restore the
	// offset after every row is present; Refresh will clamp it only if the final
	// list is genuinely shorter.
	scriptsList.Scroll = savedScroll
	if scriptsWin != nil {
		scriptsWin.Refresh()
	}
}

func layoutScriptsWindow() {
	scale := eui.UIScale()
	if scriptsWin.NoScale {
		scale = 1
	}
	clientW := scriptsWin.GetSize().X/scale - 2*(scriptsWin.Padding+scriptsWin.BorderPad)
	clientH := (scriptsWin.GetSize().Y-scriptsWin.GetTitleSize())/scale - 2*(scriptsWin.Padding+scriptsWin.BorderPad)
	clientW = max(0, clientW)
	clientH = max(0, clientH)

	scriptsRoot.Size = eui.Point{X: clientW, Y: clientH}
	scriptsHeader.Size = eui.Point{X: clientW, Y: scriptsManagerRowHeight}
	scriptsButtons.Size = eui.Point{X: clientW}
	footerHeight := scriptsButtons.GetSize().Y / scale
	scriptsList.Size = eui.Point{X: clientW, Y: max(float32(24), clientH-scriptsManagerRowHeight-footerHeight)}
}

func selectscript(owner string) {
	if owner == "" {
		return
	}
	selectedscript = owner
	if scriptInfoWin != nil {
		scriptInfoWin.Close()
	}
	scriptDetails = &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL, Scrollable: true, Fixed: true}
	scriptDetails.Size = eui.Point{X: scriptsManagerInfoWidth, Y: scriptsManagerPaneHeight}
	refreshscriptDetails()
	scriptInfoWin = eui.ShowPopup("Script Info", "", []eui.PopupButton{{Text: "Close", Action: func() { scriptInfoWin = nil }}}, scriptDetails)
}

func refreshscriptDetails() {
	infoSize := eui.Point{X: scriptsManagerInfoWidth, Y: 24}
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
	description := scriptDescriptions[owner]
	apiVersion := scriptAPIVersions[owner]
	path := scriptPaths[owner]
	openPath := path
	if info, ok := scriptPackages[owner]; ok && info.assets != nil && info.assets.zipped {
		openPath = info.container
	}
	disabled := scriptDisabled[owner]
	invalid := scriptInvalid[owner]
	errorText := scriptErrors[owner]
	validationResult := scriptValidationResults[owner]
	reloadFailed := scriptReloadFailed[owner]
	scriptMu.RUnlock()

	status := scriptStatusLabel(disabled, invalid, errorText, reloadFailed)

	line := func(s string) {
		item, _ := eui.NewText()
		item.Text = s
		item.FontSize = 12
		item.Size = infoSize
		scriptDetails.AddItem(item)
	}

	line("Name: " + name)
	line("Author: " + author)
	line("Path: " + path)
	line("Description: " + valueOrNone(description))
	line("API version: " + strconv.Itoa(apiVersion))
	catLabel := cat
	if sub != "" {
		if catLabel != "" {
			catLabel += " / "
		}
		catLabel += sub
	}
	line("Category: " + catLabel)
	line("Status: " + status)
	line("Error: " + valueOrNone(errorText))
	line("Validation: " + valueOrNone(validationResult))

	commands, bindings, events, timers, settings := scriptRegistrationSummary(owner)
	addScriptDetailList(line, "Commands", commands)
	addScriptDetailList(line, "Bindings", bindings)
	addScriptDetailList(line, "Events", events)
	if timers == 0 {
		line("Timers: none")
	} else {
		line("Timers: " + strconv.Itoa(timers))
	}
	addScriptDetailList(line, "Settings", settings)

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

	actions := eui.NewRow()
	button := func(label string, disabled bool, action func()) {
		item, events := eui.NewButton()
		item.Text = label
		item.Size = eui.Point{X: 84, Y: 24}
		item.Disabled = disabled
		events.Handle = func(event eui.UIEvent) {
			if event.Type == eui.EventClick && !item.Disabled {
				action()
			}
		}
		actions.AddItem(item)
	}
	button("Copy Error", errorText == "", func() {
		_, _ = clipboard.Write(context.Background(), clipboard.FmtText, []byte(errorText))
	})
	button("Open File", openPath == "", func() {
		if err := open.Run(openPath); err != nil {
			consoleMessage("[script] open file: " + err.Error())
		}
	})
	button("Open Folder", openPath == "", func() {
		if err := open.Run(filepath.Dir(openPath)); err != nil {
			consoleMessage("[script] open folder: " + err.Error())
		}
	})
	button("Validate", path == "", func() {
		result := "Passed"
		if err := validateScriptFile(owner, path); err != nil {
			result = err.Error()
			consoleMessage("[script] validation failed: " + result)
		} else {
			consoleMessage("[script] validation passed: " + name)
		}
		scriptMu.Lock()
		scriptValidationResults[owner] = result
		scriptMu.Unlock()
		refreshscriptDetails()
	})
	button("Reload", disabled || invalid, func() { enablescript(owner) })
	button("Stop", disabled, func() { clearscriptScope(owner) })
	scriptDetails.AddItem(actions)

	if scriptInfoWin != nil {
		scriptInfoWin.Refresh()
	}
}

func valueOrNone(value string) string {
	if strings.TrimSpace(value) == "" {
		return "none"
	}
	return value
}

func scriptStatusLabel(disabled, invalid bool, errorText string, reloadFailed bool) string {
	if reloadFailed && !disabled {
		return "Reload Failed (old version still running)"
	}
	if errorText != "" && disabled {
		return "Stopped After Error"
	}
	if invalid {
		return "Stopped After Error"
	}
	if disabled {
		return "Disabled"
	}
	return "Running"
}

func addScriptDetailList(line func(string), label string, values []string) {
	if len(values) == 0 {
		line(label + ": none")
		return
	}
	line(label + ": " + strings.Join(values, ", "))
}

func scriptRegistrationSummary(owner string) (commands, bindings, events []string, timers int, settings []string) {
	scriptMu.RLock()
	for command, commandOwner := range scriptCommandOwners {
		if commandOwner == owner {
			commands = append(commands, "/"+command)
		}
	}
	timers = len(scriptRepeats[owner])
	scriptMu.RUnlock()

	hotkeysMu.RLock()
	for _, hotkey := range hotkeys {
		if hotkey.Script == owner {
			bindings = append(bindings, hotkey.Combo)
		}
	}
	hotkeysMu.RUnlock()

	chatHandlersMu.RLock()
	for _, handler := range scriptStructuredChatHandlers {
		if handler.owner == owner {
			events = append(events, "chat")
		}
	}
	for _, handler := range scriptServerMessageHandlers {
		if handler.owner == owner {
			events = append(events, "server message")
		}
	}
	for _, handler := range scriptLifecycleHandlers {
		if handler.owner == owner {
			events = append(events, handler.kind)
		}
	}
	for _, handler := range scriptChangeHandlers {
		if handler.owner == owner {
			events = append(events, "change: "+handler.kind)
		}
	}
	chatHandlersMu.RUnlock()

	scriptConfigMu.RLock()
	for _, entry := range scriptConfigEntries[owner] {
		settings = append(settings, entry.Label+" ("+entry.Key+")")
	}
	scriptConfigMu.RUnlock()
	sort.Strings(commands)
	sort.Strings(bindings)
	sort.Strings(events)
	sort.Strings(settings)
	return commands, bindings, events, timers, settings
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
		t.Size = eui.Point{X: 0, Y: 0}
		scriptDebugList.AddItem(t)
	}
	if scriptEventsWin != nil {
		scriptEventsWin.Refresh()
	}
}

func openscriptConfigWindow(owner string) {
	scriptConfigMu.RLock()
	entries := append([]scriptConfigEntry(nil), scriptConfigEntries[owner]...)
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
	scriptConfigWin.ShowTooltipIndicators = true
	scriptConfigWin.Title = "Configure: " + name
	scriptConfigWin.Closable = true
	scriptConfigWin.Resizable = false
	scriptConfigWin.AutoSize = true
	scriptConfigWin.Movable = true
	scriptConfigWin.SetZone(eui.HZoneCenterLeft, eui.VZoneMiddleTop)

	root := eui.NewColumn()
	scriptConfigWin.AddItem(root)

	for _, ce := range entries {
		row := eui.NewRow()
		lbl, _ := eui.NewText()
		lbl.Text = ce.Label
		if ce.Help != "" {
			lbl.SetTooltip(ce.Help)
		}
		lbl.FontSize = 12
		lbl.Size = eui.Point{X: 120, Y: 24}
		row.AddItem(lbl)

		switch ce.Type {
		case "int", "float":
			s, events := eui.NewSlider()
			if ce.Max > ce.Min {
				s.MinValue = float32(ce.Min)
				s.MaxValue = float32(ce.Max)
			} else {
				s.MinValue = 0
				s.MaxValue = 100
			}
			if ce.Type == "int" {
				s.IntOnly = true
				if value, ok := ce.Value.(int); ok {
					s.Value = float32(value)
				}
			} else if value, ok := ce.Value.(float64); ok {
				s.Value = float32(value)
			}
			s.Size = eui.Point{X: 120, Y: 24}
			key := ce.Key
			typ := ce.Type
			events.Handle = func(ev eui.UIEvent) {
				if ev.Type != eui.EventSliderChanged {
					return
				}
				value := float64(ev.Value)
				if ce.Step > 0 {
					value = math.Round(value/ce.Step) * ce.Step
				}
				if typ == "int" {
					scriptSetConfigValue(owner, key, int(value))
				} else {
					scriptSetConfigValue(owner, key, value)
				}
			}
			row.AddItem(s)
		case "bool":
			cb, events := eui.NewCheckbox()
			cb.Checked, _ = ce.Value.(bool)
			cb.Size = eui.Point{X: 24, Y: 24}
			key := ce.Key
			events.Handle = func(ev eui.UIEvent) {
				if ev.Type == eui.EventCheckboxChanged {
					scriptSetConfigValue(owner, key, ev.Checked)
				}
			}
			row.AddItem(cb)
		case "text", "key":
			inp, events := eui.NewInput()
			inp.Text, _ = ce.Value.(string)
			inp.Size = eui.Point{X: 120, Y: 24}
			key := ce.Key
			events.Handle = func(ev eui.UIEvent) {
				if ev.Type == eui.EventInputChanged {
					scriptSetConfigValue(owner, key, ev.Text)
				}
			}
			row.AddItem(inp)
		case "choice":
			dd, events := eui.NewDropdown()
			dd.Options = append([]string(nil), ce.Choices...)
			current, _ := ce.Value.(string)
			for i, option := range dd.Options {
				if option == current {
					dd.Selected = i
					break
				}
			}
			dd.Size = eui.Point{X: 120, Y: 24}
			key := ce.Key
			events.Handle = func(ev eui.UIEvent) {
				if ev.Type == eui.EventDropdownSelected && ev.Index >= 0 && ev.Index < len(ev.Item.Options) {
					scriptSetConfigValue(owner, key, ev.Item.Options[ev.Index])
				}
			}
			row.AddItem(dd)
		case "item":
			dd, events := eui.NewDropdown()
			current, _ := ce.Value.(string)
			seen := map[string]bool{}
			if current != "" {
				dd.Options = append(dd.Options, current)
				seen[current] = true
			}
			for _, item := range getInventory() {
				if item.Name != "" && !seen[item.Name] {
					dd.Options = append(dd.Options, item.Name)
					seen[item.Name] = true
				}
			}
			sort.Strings(dd.Options)
			for i, option := range dd.Options {
				if option == current {
					dd.Selected = i
					break
				}
			}
			dd.Size = eui.Point{X: 120, Y: 24}
			key := ce.Key
			events.Handle = func(ev eui.UIEvent) {
				if ev.Type == eui.EventDropdownSelected && ev.Index >= 0 && ev.Index < len(ev.Item.Options) {
					scriptSetConfigValue(owner, key, ev.Item.Options[ev.Index])
				}
			}
			row.AddItem(dd)
		default:
			t, _ := eui.NewText()
			t.Text = ce.Type
			t.FontSize = 12
			t.Size = eui.Point{X: 120, Y: 24}
			row.AddItem(t)
		}
		root.AddItem(row)
		if ce.Help != "" {
			help, _ := eui.NewText()
			help.Text = ce.Help
			help.FontSize = 10
			help.Size = eui.Point{X: 240, Y: 18}
			root.AddItem(help)
		}
	}

	scriptConfigWin.AddWindow(false)
	scriptConfigOwner = owner
}

// refreshMixerEnhancementControls keeps the mixer in sync when an audio
// quality preset or the setup wizard changes these values.
func refreshMixerEnhancementControls() {
	if soundEnhanceMixCB != nil {
		soundEnhanceMixCB.Checked = gs.SoundEnhancement
		soundEnhanceMixCB.Dirty = true
	}
	if soundEnhanceSlider != nil {
		soundEnhanceSlider.Value = float32(clampSoundEnhancementAmount(gs.SoundEnhancementAmount))
		soundEnhanceSlider.Disabled = !gs.SoundEnhancement
		soundEnhanceSlider.Dirty = true
	}
	if musicEnhanceMixCB != nil {
		musicEnhanceMixCB.Checked = gs.MusicEnhancement
		musicEnhanceMixCB.Dirty = true
	}
	if musicEnhanceSlider != nil {
		musicEnhanceSlider.Value = float32(clampMusicEnhancementAmount(gs.MusicEnhancementAmount))
		musicEnhanceSlider.Disabled = !gs.MusicEnhancement
		musicEnhanceSlider.Dirty = true
	}
}

func makeMixerWindow() {
	if mixerWin != nil {
		return
	}
	mixerWin = eui.NewWindow()
	mixerWin.ShowTooltipIndicators = true
	mixerWin.Title = "Mixer"
	mixerWin.Closable = true
	mixerWin.Resizable = false
	mixerWin.AutoSize = true
	mixerWin.Movable = true

	flow := eui.NewRow()

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
				setTTSEnabled(ev.Checked)
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

	addBigSpacer()

	// Enhancement controls stay in the mixer because they change how new sound
	// effects and bard tunes are rendered, just as the channel controls change
	// their level. The sliders preserve the existing default mix at 1.00.
	enhanceCol := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL, Size: eui.Point{X: 180, Y: 140}}
	var soundEnhanceEvents *eui.EventHandler
	soundEnhanceMixCB, soundEnhanceEvents = eui.NewCheckbox()
	soundEnhanceMixCB.Text = "Enhance sound effects"
	soundEnhanceMixCB.Size = eui.Point{X: 180, Y: 24}
	soundEnhanceMixCB.SetTooltip("Add ambience to newly played game sound effects.")
	soundEnhanceEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type != eui.EventCheckboxChanged {
			return
		}
		gs.SoundEnhancement = ev.Checked
		refreshMixerEnhancementControls()
		settingsDirty = true
	}
	enhanceCol.AddItem(soundEnhanceMixCB)

	var soundEnhanceSliderEvents *eui.EventHandler
	soundEnhanceSlider, soundEnhanceSliderEvents = eui.NewSlider()
	soundEnhanceSlider.MinValue = 0.1
	soundEnhanceSlider.MaxValue = 10
	soundEnhanceSlider.Value = float32(clampSoundEnhancementAmount(gs.SoundEnhancementAmount))
	soundEnhanceSlider.Size = eui.Point{X: 180, Y: 24}
	soundEnhanceSlider.SetTooltip("Sound effect enhancement strength. 1.00 is the normal enhanced mix.")
	soundEnhanceSliderEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type != eui.EventSliderChanged {
			return
		}
		amount := math.Round(float64(ev.Value)*100) / 100
		gs.SoundEnhancementAmount = clampSoundEnhancementAmount(amount)
		if ev.Item.Value != float32(gs.SoundEnhancementAmount) {
			ev.Item.Value = float32(gs.SoundEnhancementAmount)
			ev.Item.Dirty = true
		}
		settingsDirty = true
	}
	enhanceCol.AddItem(soundEnhanceSlider)

	var musicEnhanceEvents *eui.EventHandler
	musicEnhanceMixCB, musicEnhanceEvents = eui.NewCheckbox()
	musicEnhanceMixCB.Text = "Enhance bard music"
	musicEnhanceMixCB.Size = eui.Point{X: 180, Y: 24}
	musicEnhanceMixCB.SetTooltip("Add ambience to newly started bard music.")
	musicEnhanceEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type != eui.EventCheckboxChanged {
			return
		}
		gs.MusicEnhancement = ev.Checked
		refreshMixerEnhancementControls()
		settingsDirty = true
	}
	enhanceCol.AddItem(musicEnhanceMixCB)

	var musicEnhanceSliderEvents *eui.EventHandler
	musicEnhanceSlider, musicEnhanceSliderEvents = eui.NewSlider()
	musicEnhanceSlider.MinValue = 0.1
	musicEnhanceSlider.MaxValue = 2
	musicEnhanceSlider.Value = float32(clampMusicEnhancementAmount(gs.MusicEnhancementAmount))
	musicEnhanceSlider.Size = eui.Point{X: 180, Y: 24}
	musicEnhanceSlider.SetTooltip("Bard music ambience strength. 1.00 matches the prior enhanced sound.")
	musicEnhanceSliderEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type != eui.EventSliderChanged {
			return
		}
		amount := math.Round(float64(ev.Value)*100) / 100
		gs.MusicEnhancementAmount = clampMusicEnhancementAmount(amount)
		if ev.Item.Value != float32(gs.MusicEnhancementAmount) {
			ev.Item.Value = float32(gs.MusicEnhancementAmount)
			ev.Item.Dirty = true
		}
		settingsDirty = true
	}
	enhanceCol.AddItem(musicEnhanceSlider)
	flow.AddItem(enhanceCol)

	addBigSpacer()

	var mixMuteEvents *eui.EventHandler
	mixMuteBtn, mixMuteEvents = eui.NewButton()
	setAudioMuteButtonState(mixMuteBtn, gs.Mute)
	// Make the mute button wider to accommodate label and adjacent checkbox context
	mixMuteBtn.Size = eui.Point{X: 192, Y: 24}
	mixMuteEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			gs.Mute = !gs.Mute
			if gs.Mute {
				if volumeSlider != nil {
					volumeSlider.Value = 0
				}
				if masterMixSlider != nil {
					masterMixSlider.Value = 0
					masterMixSlider.Dirty = true
				}
				stopAllAudioPlayers()
				clearTuneQueue()
			} else {
				if volumeSlider != nil {
					volumeSlider.Value = float32(gs.MasterVolume)
				}
				if masterMixSlider != nil {
					masterMixSlider.Value = float32(gs.MasterVolume)
					masterMixSlider.Dirty = true
				}
			}
			setAudioMuteButtonState(mixMuteBtn, gs.Mute)
			setAudioMuteButtonState(muteBtn, gs.Mute)
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
	muteUnfocusEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			gs.MuteWhenUnfocused = ev.Checked
			if ev.Checked {
				if !windowIsFocused() {
					focusMuted = true
				}
			} else {
				focusMuted = false
			}
			settingsDirty = true
			updateSoundVolume()
		}
	}
	// Keep the mixer-wide controls together to the right of the channel sliders.
	muteCol := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL, Size: eui.Point{X: 192, Y: 60}}
	muteCol.AddItem(mixMuteBtn)
	muteCol.AddItem(muteUnfocusCB)
	flow.AddItem(muteCol)

	refreshMixerEnhancementControls()
	mixerWin.AddItem(flow)
}

func makeToolbar() {
	if toolbarRoot != nil {
		return
	}
	placeToolbar(gs.ToolbarPlacement, false)
	toolbarStatsOnce.Do(func() {
		go func() {
			for {
				time.Sleep(5 * time.Second)
				updateToolbarStats()
			}
		}()
	})
}

func buildToolbarRoot(docked bool) *eui.ItemData {
	var toolFontSize float32 = 10
	var buttonHeight float32 = 24
	var buttonWidth float32 = 88
	if docked {
		buttonWidth = 84
	}

	controls := eui.NewRow()
	if hands := toolbarHandsSource(); hands != nil {
		w, h := hands.Bounds().Dx(), hands.Bounds().Dy()
		handsRow := eui.NewRow()
		var leftBacking, rightBacking *ebiten.Image
		leftHandImg, leftBacking = eui.NewImageItem(w/2, h)
		rightHandImg, rightBacking = eui.NewImageItem(w-w/2, h)
		leftBacking.Deallocate()
		rightBacking.Deallocate()
		leftHandImg.Image = nil
		rightHandImg.Image = nil
		handsRow.AddItem(leftHandImg)
		handsRow.AddItem(rightHandImg)
		controls.AddItem(handsRow)
	}
	controls.AddItem(buildToolbar(toolFontSize, buttonWidth, buttonHeight))

	root := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL, Fixed: true}
	root.AddItem(controls)
	scriptRows := buildScriptToolbarRows()
	for _, row := range scriptRows {
		root.AddItem(row)
	}
	toolbarHeight := buttonHeight * 2
	if hands := toolbarHandsSource(); hands != nil {
		toolbarHeight = float32(hands.Bounds().Dy())
	}
	scriptToolbarHeight := float32(len(scriptRows)) * 32
	toolbarStatsText = nil
	if docked && gs.ToolbarInfoBar {
		root.Size.Y = toolbarHeight + 22 + scriptToolbarHeight
		toolbarStatsText, _ = eui.NewText()
		toolbarStatsText.FontSize = 10
		toolbarStatsText.Size = eui.Point{X: buttonWidth * 5, Y: 18}
		toolbarStatsText.SetTooltip("loss is recent packet loss; jit is 60-second p95 server-frame jitter; ping is the latest command reply.")
		root.AddItem(toolbarStatsText)
	} else {
		root.Size.Y = toolbarHeight + scriptToolbarHeight
	}
	updateToolbarStats()
	return root
}

func placeToolbar(placement ToolbarPlacement, dirty bool) {
	if placement < ToolbarInInventory || placement > ToolbarFloating {
		placement = ToolbarInInventory
	}
	if gs.TiledWindows && placement == ToolbarFloating {
		placement = ToolbarInInventory
	}
	var oldHost *eui.WindowData
	if toolbarRoot != nil {
		oldHost = toolbarRoot.ParentWindow
		if toolbarRoot.Parent != nil {
			toolbarRoot.Parent.RemoveItem(toolbarRoot)
		} else if hudWin != nil {
			hudWin.RemoveItem(toolbarRoot)
		}
	}
	if hudWin != nil {
		hudWin.RemoveWindow()
		hudWin = nil
	}
	if oldHost == inventoryWin {
		updateInventoryWindow()
		inventoryWin.Refresh()
	} else if oldHost == playersWin {
		updatePlayersWindow()
		playersWin.Refresh()
	}

	gs.ToolbarPlacement = placement
	refreshToolbarPlacementControl()
	toolbarRoot = buildToolbarRoot(placement != ToolbarFloating)
	switch placement {
	case ToolbarInInventory:
		if inventoryList != nil && inventoryList.Parent != nil {
			inventoryList.Parent.PrependItem(toolbarRoot)
			updateInventoryWindow()
			inventoryWin.Refresh()
		}
	case ToolbarInPlayers:
		if playersList != nil && playersList.Parent != nil {
			playersList.Parent.PrependItem(toolbarRoot)
			updatePlayersWindow()
			playersWin.Refresh()
		}
	case ToolbarFloating:
		hudWin = eui.NewWindow()
		hudWin.Title = "Toolbar"
		hudWin.Closable = false
		hudWin.Resizable = false
		hudWin.AutoSize = false
		hudWin.Size = eui.Point{X: float32(dockedToolbarMinimumWidth), Y: 49 + toolbarRoot.Size.Y}
		hudWin.Movable = true
		hudWin.NoScroll = true
		hudWin.AddItem(toolbarRoot)
		hudWin.AddWindow(false)
		applyWindowState(hudWin, &gs.ToolbarWindow)
		hudWin.MarkOpen()
	}

	updateToolbarStats()
	updateToolbarHands()
	updateRecordButton()
	if dirty {
		settingsDirty = true
	}
}

func refreshToolbarPlacementControl() {
	if settingsToolbarPlacementDD == nil {
		return
	}
	settingsToolbarPlacementDD.Options = []string{"Inside Inventory", "Inside Players"}
	if !gs.TiledWindows {
		settingsToolbarPlacementDD.Options = append(settingsToolbarPlacementDD.Options, "Floating Window")
	}
	settingsToolbarPlacementDD.Selected = int(gs.ToolbarPlacement)
	settingsToolbarPlacementDD.Dirty = true
	if settingsWin != nil {
		settingsWin.Refresh()
	}
}

// ensureToolbarAccessible prevents a docked toolbar from disappearing with the
// window that contains it. This also repairs a saved layout whose host window
// was closed when the client last exited.
func ensureToolbarAccessible() {
	if toolbarRoot == nil {
		return
	}

	var host *eui.WindowData
	switch gs.ToolbarPlacement {
	case ToolbarInInventory:
		host = inventoryWin
	case ToolbarInPlayers:
		host = playersWin
	default:
		return
	}
	if host == nil || host.IsOpen() {
		return
	}

	placeToolbar(ToolbarFloating, true)
}

func updateToolbarStats() {
	reply, jitter := networkTimingSnapshot()
	recentLoss, _, _, _ := packetLossSnapshot()
	if gs.ToolbarPlacement == ToolbarFloating && hudWin != nil {
		hudWin.Title = fmt.Sprintf("Toolbar - fps %.0f, loss %s, jit %s, ping %s",
			ebiten.ActualFPS(), formatToolbarLoss(recentLoss), formatToolbarLatency(jitter), formatToolbarLatency(reply))
		hudWin.Refresh()
		return
	}
	if toolbarStatsText == nil {
		return
	}
	toolbarStatsText.Text = fmt.Sprintf("fps %.0f, loss %s, jit %s, ping %s",
		ebiten.ActualFPS(), formatToolbarLoss(recentLoss), formatToolbarLatency(jitter), formatToolbarLatency(reply))
	toolbarStatsText.Dirty = true
	refreshToolbar()
}

func formatToolbarLatency(duration time.Duration) string {
	return fmt.Sprintf("%.1fms", float64(duration)/float64(time.Millisecond))
}

func formatToolbarLoss(loss float64) string {
	if loss == 0 {
		return "0%"
	}
	return fmt.Sprintf("%.1f%%", loss)
}

func refreshToolbar() {
	if toolbarRoot != nil && toolbarRoot.ParentWindow != nil {
		toolbarRoot.ParentWindow.Refresh()
	}
}

func toolbarHandsSource() image.Image {
	toolbarHandsOnce.Do(func() {
		img, err := png.Decode(bytes.NewReader(toolbarHandsPNG))
		if err != nil {
			logError("decode embedded toolbar hands: %v", err)
			return
		}
		toolbarHandsSrc = img
	})
	return toolbarHandsSrc
}

// loadToolbarHands defers the Ebitengine image creation until Draw, when the
// graphics context is active. initUI runs before RunGame and cannot upload
// pixel data yet.
func loadToolbarHands() {
	if toolbarHandsImage != nil || leftHandImg == nil || rightHandImg == nil {
		return
	}
	if hands := toolbarHandsSource(); hands != nil {
		toolbarHandsImage = newManagedImageFromImage(hands)
		updateToolbarHands()
	}
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
	w := max(hand.Bounds().Dx(), item.Bounds().Dx())
	h := max(hand.Bounds().Dy(), item.Bounds().Dy())
	out := newUnmanagedImage(w, h)
	opHand := overlayHandOpts
	opHand.ColorScale.Reset()
	opHand.ColorScale.ScaleAlpha(0.5)
	opHand.GeoM.Reset()
	opHand.GeoM.Translate(float64((w-hand.Bounds().Dx())/2), float64((h-hand.Bounds().Dy())/2))
	out.DrawImage(hand, opHand)
	opItem := overlayItemOpts
	opItem.ColorScale.Reset()
	opItem.GeoM.Reset()
	opItem.GeoM.Translate(float64((w-item.Bounds().Dx())/2), float64((h-item.Bounds().Dy())/2))
	out.DrawImage(item, opItem)
	return out
}

// updateToolbarHands keeps the original held-item treatment: each item is
// centered over its hand and the left-hand item is mirrored. Only the hand
// background comes from the embedded two-hand PNG.
func updateToolbarHands() {
	if toolbarHandsImage == nil || leftHandImg == nil || rightHandImg == nil {
		return
	}
	rightID, leftID := equippedItemPicts()
	if toolbarHandsRendered && toolbarHandsRightID == rightID && toolbarHandsLeftID == leftID &&
		toolbarHandsTargetL == leftHandImg && toolbarHandsTargetR == rightHandImg && toolbarHandsSourceGPU == toolbarHandsImage {
		return
	}
	bounds := toolbarHandsImage.Bounds()
	middle := bounds.Dx() / 2
	leftHand := toolbarHandsImage.SubImage(image.Rect(0, 0, middle, bounds.Dy())).(*ebiten.Image)
	rightHand := toolbarHandsImage.SubImage(image.Rect(middle, 0, bounds.Dx(), bounds.Dy())).(*ebiten.Image)
	if toolbarLeftComposite != nil {
		toolbarLeftComposite.Deallocate()
		toolbarLeftComposite = nil
	}
	if toolbarRightComposite != nil {
		toolbarRightComposite.Deallocate()
		toolbarRightComposite = nil
	}
	rightImage := rightHand
	if rightID != 0 {
		if item := loadImage(rightID); item != nil {
			toolbarRightComposite = overlayItemOnHand(rightHand, item)
			rightImage = toolbarRightComposite
		}
	}
	leftImage := leftHand
	if leftID != 0 {
		if item := loadImage(leftID); item != nil {
			mirrored := mirrorImage(item)
			toolbarLeftComposite = overlayItemOnHand(leftHand, mirrored)
			mirrored.Deallocate()
			leftImage = toolbarLeftComposite
		}
	}
	leftHandImg.Image = leftImage
	leftHandImg.Size = eui.Point{X: float32(leftImage.Bounds().Dx()), Y: float32(leftImage.Bounds().Dy())}
	leftHandImg.Dirty = true
	rightHandImg.Image = rightImage
	rightHandImg.Size = eui.Point{X: float32(rightImage.Bounds().Dx()), Y: float32(rightImage.Bounds().Dy())}
	rightHandImg.Dirty = true
	toolbarHandsRendered = true
	toolbarHandsRightID = rightID
	toolbarHandsLeftID = leftID
	toolbarHandsTargetL = leftHandImg
	toolbarHandsTargetR = rightHandImg
	toolbarHandsSourceGPU = toolbarHandsImage
	refreshToolbar()
}

func confirmExitSession() {
	if playingMovie && !setupWizardPreviewActive {
		eui.ShowPopup("Exit Movie", "Stop playback and return to login?", []eui.PopupButton{
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
		eui.ShowPopup("Exit Session", "Disconnect and return to login?", []eui.PopupButton{
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
	mr.AddStateSnapshot(snapshot, uint16(clVersion), captureMovieNightState())
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
	setMaterialButtonIcon(saveBtn, "save")
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
	retryRow := eui.NewRow()
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

func makeDownloadsWindow(preferTTS ...bool) {
	ttsRequested := len(preferTTS) > 0 && preferTTS[0]

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

	flow := eui.NewColumn()

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
	eui.ApplyBoldFace(t)
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
		eui.ApplyBoldFace(opt)
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
		sfCB.Checked = !ttsRequested
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
		pc.Checked = ttsRequested
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
		cancelRow := eui.NewRow()
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
				retryRow := eui.NewRow()
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
			var img *climg.CLImages
			var err error
			if isWASM && len(wasmCLImagesData) > 0 {
				img, err = climg.LoadBytes(wasmCLImagesData)
			} else {
				img, err = climg.Load(assetFilePath(CL_ImagesFile))
			}
			if err != nil {
				logError("failed to load CL_Images: %v", err)
				handleDownloadAssetError(flow, statusText, pb, startDownload, &startedDownload, "Failed to load CL_Images")
				return
			}

			sounds, err := loadCLSoundsArchive()
			if err != nil {
				logError("failed to load CL_Sounds: %v", err)
				handleDownloadAssetError(flow, statusText, pb, startDownload, &startedDownload, "Failed to load CL_Sounds")
				return
			}
			refreshedStatus, statusErr := checkDataFiles(clVersion)
			dispatchMainThread(func() {
				img.SetDenoise(gs.DenoiseImages, gs.DenoiseSharpness, gs.DenoiseAmount)
				img.SetGammaCorrection(gs.SpriteGammaCorrection, gs.SpriteGamma, gs.MonitorGamma)
				clImages = img
				replaceCLSoundsArchive(sounds)
				// Login and the setup preview may already have requested artwork while
				// no archive was available. Drop every derived entry before either UI
				// renders from the newly installed archive.
				clearCaches()
				// Keep the download dialog up until the small, common startup set is
				// resident. Full sound precaching, when enabled, remains background work.
				preloadStartupArtwork()
				precacheStartupSounds(false)
				if gs.PrecacheSounds && !startupLoader.precacheRun {
					startupLoader.precacheRun = true
					go precacheSounds()
				}
				markWorldStateChanged()
				// Startup prepares the Clan Lord splash after loading CL_Images.
				// Queue the same game-loop-safe rebuild after a first-run download.
				classicSplashFilterPending = gs.ShowClanLordSplashImage
				inventoryDirty = true
				playersDirty = true
				if statusErr == nil {
					dlMutex.Lock()
					status = refreshedStatus
					dlMutex.Unlock()
				}
				refreshLoginAfterAssetsAvailable()
				// Clear the callback to avoid stray updates after closing.
				downloadStatus = nil
				downloadProgress = nil
				downloadWin.Close()
				if clmov == "" && pcapPath == "" && !fake && clImages != nil && currentCLSoundsArchive() != nil && shouldShowSetupWizard(settingsLoaded, gs.SetupWizardVersion, appVersion) {
					openSetupWizard(false)
				}
			})
		}()
	}

	// Auto-start download in WASM to avoid extra click; keep window open for progress.
	if isWASM {
		startDownload()
	}

	btnFlow := eui.NewRow()
	if !isWASM {
		dlBtn, dlEvents := eui.NewButton()
		dlBtn.Text = "Download"
		setMaterialButtonIcon(dlBtn, "download")
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

const (
	charWinWidth          float32 = 480
	freeDemoCharacterName         = "-Demo Character-"
	freeDemoSelection             = "\x00free-demo"
)

var newbieBrownColors = []byte{
	135, 171, 1, 8, 15, 1, 7, 8, 1, 8,
	15, 8, 15, 7, 1, 43, 1, 43, 36, 108,
}

type loginCharacterChoice struct {
	character Character
	selection string
	demo      bool
}

func loginCharacterChoices() []loginCharacterChoice {
	choices := make([]loginCharacterChoice, 0, len(characters)+1)
	for _, character := range characters {
		choices = append(choices, loginCharacterChoice{
			character: character,
			selection: character.Name,
		})
	}
	choices = append(choices, loginCharacterChoice{
		character: Character{
			Name:   freeDemoCharacterName,
			PictID: defaultMobilePictID(genderUnknown),
			Colors: append([]byte(nil), newbieBrownColors...),
		},
		selection: freeDemoSelection,
		demo:      true,
	})
	return choices
}

func validLoginCharacterSelection(selection string) bool {
	if selection == freeDemoSelection {
		return true
	}
	for _, character := range characters {
		if character.Name == selection {
			return true
		}
	}
	return false
}

func refreshLoginAfterAssetsAvailable() {
	if loginWin == nil {
		return
	}
	if name == "" {
		// Force reselect from LastCharacter if available.
		passHash = ""
		pass = ""
	}
	// Archive installation and wizard quality changes can invalidate the image
	// items created while Login is hidden. Always rebuild, even when a saved
	// character was already selected.
	updateCharacterButtons()
	loginWin.Refresh()
	if tcpConn == nil && clmov == "" && !playingMovie && pcapPath == "" && !fake {
		loginWin.MarkOpen()
	}
}

func updateCharacterButtons() {
	if loginWin == nil {
		return
	}
	if charactersList == nil {
		return
	}
	// Preserve current scroll position while rebuilding the list
	prevScroll := charactersList.Scroll
	if name != "" && !validLoginCharacterSelection(name) {
		name = ""
		passHash = ""
		pass = ""
	}
	if name == "" {
		if gs.LastCharacter != "" {
			for _, c := range characters {
				if c.Name == gs.LastCharacter {
					name = c.Name
					passHash = c.passHash
					if stagedHash, ok := stagedPasswordHash(c.Name); ok {
						passHash = stagedHash
					}
					pass = ""
					break
				}
			}
		}
		if name == "" && len(characters) == 1 {
			name = characters[0].Name
			passHash = characters[0].passHash
			if stagedHash, ok := stagedPasswordHash(characters[0].Name); ok {
				passHash = stagedHash
			}
			pass = ""
		}
		if name == "" && len(characters) == 0 {
			name = freeDemoSelection
			passHash = ""
			pass = ""
		}
	}
	for i := range charactersList.Contents {
		charactersList.Contents[i] = nil
	}
	charactersList.Contents = charactersList.Contents[:0]

	for _, choice := range loginCharacterChoices() {
		c := choice.character
		row := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_HORIZONTAL, Position: eui.Point{Y: 4}}

		profItem, _ := eui.NewImageItem(48, 48)
		profItem.Position = eui.Point{X: 4}
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
		avItem.Position = eui.Point{X: 4}
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
		radio.Size = eui.Point{X: charWinWidth - 124, Y: 32}
		radio.Position = eui.Point{X: 8, Y: 16}
		radio.AuxSpace = 8
		radio.FontSize = 16
		radio.Checked = name == choice.selection
		selectionCopy := choice.selection
		demoCopy := choice.demo
		savedHashCopy := c.passHash
		hashCopy := savedHashCopy
		if !choice.demo {
			if stagedHash, ok := stagedPasswordHash(c.Name); ok {
				hashCopy = stagedHash
			}
		}
		if name == choice.selection {
			passHash = hashCopy
			pass = ""
		}
		radioEvents.Handle = func(ev eui.UIEvent) {
			if ev.Type == eui.EventRadioSelected {
				discardStagedPassword()
				name = selectionCopy
				passHash = savedHashCopy
				pass = ""
				if demoCopy {
					switchCharacterProfile("")
				} else {
					switchCharacterProfile(selectionCopy)
				}
				// Rebuild the list so only the selected radio is checked
				// across all rows and refresh the login UI immediately.
				updateCharacterButtons()
				if loginWin != nil {
					loginWin.Refresh()
				}
			}
		}
		row.AddItem(radio)
		charactersList.AddItem(row)
	}
	// Preserve window position while contents change size
	// Restore prior scroll position to keep the user's place.
	charactersList.Scroll = prevScroll
	for _, action := range []struct {
		button *eui.ItemData
		help   string
	}{
		{editCharBtn, "Change the selected character's password, password saving, or settings profile."},
		{deleteCharBtn, "Delete the selected saved character"},
	} {
		if action.button == nil {
			continue
		}
		action.button.Disabled = name == freeDemoSelection
		if action.button.Disabled {
			action.button.SetTooltip("The demo character is built in and cannot be edited or deleted.")
		} else {
			action.button.SetTooltip(action.help)
		}
		action.button.Dirty = true
	}
	if loginConnectButton != nil {
		loginConnectButton.Disabled = name == ""
		if loginConnectButton.Disabled {
			loginConnectButton.SetTooltip("Select a character before connecting.")
		} else {
			loginConnectButton.SetTooltip("Connect as the selected character.")
		}
		loginConnectButton.Dirty = true
	}
	// Keep UI fresh after potential content changes.
	loginWin.Refresh()
}

func makeAddCharacterWindow() {
	if addCharWin != nil {
		return
	}
	addCharWin = eui.NewWindow()
	addCharWin.ShowTooltipIndicators = true
	addCharWin.Title = "Add Character"
	addCharWin.Closable = false
	addCharWin.Resizable = false
	addCharWin.AutoSize = true
	addCharWin.Movable = true
	//addCharWin.SetZone(eui.HZoneCenterLeft, eui.VZoneMiddleTop)

	flow := eui.NewColumn()

	nameInput, _ := eui.NewInput()
	nameInput.Label = "Character"
	nameInput.TextPtr = &addCharName
	nameInput.Size = eui.Point{X: 200, Y: 24}
	addCharNameInput = nameInput
	flow.AddItem(nameInput)
	passInput, passEvents := eui.NewInput()
	passInput.Label = "Password (optional)"
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

	profileCB, profileEvents := eui.NewCheckbox()
	addCharProfileCB = profileCB
	profileCB.Text = "Keep settings separate"
	profileCB.SetTooltip("Start this character with independent windows, appearance, audio, notifications, and related settings.")
	profileCB.Size = eui.Point{X: 200, Y: 24}
	profileCB.Checked = addCharProfile
	profileEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			addCharProfile = ev.Checked
		}
	}
	flow.AddItem(profileCB)

	addBtn, addEvents := eui.NewButton()
	addBtn.Text = "Add"
	setMaterialButtonIcon(addBtn, "add")
	addBtn.Size = eui.Point{X: 200, Y: 24}
	addCharWin.DefaultButton = addBtn
	addEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			characterName := strings.TrimSpace(addCharName)
			if characterName == "" {
				makeErrorWindow("Error: Add Character: character name is empty")
				return
			}
			// Check for existing character names case-insensitively
			exists := false
			for i := range characters {
				if strings.EqualFold(characters[i].Name, characterName) {
					// Preserve canonical case from the stored character
					characterName = characters[i].Name
					exists = true
					break
				}
			}
			if !exists {
				characters = append(characters, Character{Name: characterName, DontRemember: true})
			}
			saveCharacters()
			hash := stageAddedCharacterPassword(characterName, addCharPass, addCharRemember)
			// Update selection to the newly added character
			name = characterName
			passHash = hash
			pass = ""
			setCharacterProfileEnabled(characterName, addCharProfile)
			switchCharacterProfile(characterName)
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
			addCharProfile = false
			clearPasswordInput(addCharPassInput, &addCharPass)
			addCharPassPrev = ""
			clearCapsWarnings()
			if addCharNameInput != nil {
				addCharNameInput.Text = ""
				addCharNameInput.Dirty = true
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
			clearPasswordInput(addCharPassInput, &addCharPass)
			addCharWin.Close()
			loginWin.MarkOpen()
		}
	}
	flow.AddItem(cancelBtn)

	addCharWin.AddItem(flow)
	addCharWin.AddWindow(false)
}

func stageAddedCharacterPassword(characterName, password string, remember bool) string {
	if password != "" {
		return stagePasswordUpdate(characterName, password, remember)
	}
	if hash, staged := stagedPasswordHash(characterName); staged {
		return hash
	}
	if character, exists := selectedCharacter(characterName); exists {
		return character.passHash
	}
	return ""
}

func selectedCharacter(characterName string) (Character, bool) {
	characterName = strings.TrimSpace(characterName)
	for i := range characters {
		if strings.EqualFold(characters[i].Name, characterName) {
			return characters[i], true
		}
	}
	return Character{}, false
}

func prepareEditCharacter(characterName string) error {
	character, ok := selectedCharacter(characterName)
	if !ok {
		return errors.New("select a character to edit first")
	}

	editCharName = character.Name
	editCharRemember = !character.DontRemember && character.passHash != ""
	if _, remember, staged := stagedPasswordSettings(character.Name); staged {
		editCharRemember = remember
	}
	editCharProfile = characterProfileEnabled(character.Name)
	editCharPass = ""
	editCharPassPrev = ""
	clearPasswordInput(editCharPassInput, &editCharPass)
	clearCapsWarnings()

	if editCharWin != nil {
		editCharWin.Title = "Edit Character: " + character.Name
	}
	if editCharRememberCB != nil {
		editCharRememberCB.Checked = editCharRemember
		editCharRememberCB.Dirty = true
	}
	if editCharProfileCB != nil {
		editCharProfileCB.Checked = editCharProfile
		editCharProfileCB.Text = "Keep settings separate"
		editCharProfileCB.Dirty = true
	}
	return nil
}

func makeEditCharacterWindow() {
	if editCharWin != nil {
		return
	}
	editCharWin = eui.NewWindow()
	editCharWin.ShowTooltipIndicators = true
	editCharWin.Title = "Edit Character"
	editCharWin.Closable = false
	editCharWin.Resizable = false
	editCharWin.AutoSize = true
	editCharWin.Movable = true

	flow := eui.NewColumn()

	input, inputEvents := eui.NewInput()
	input.Label = "New Password"
	input.TextPtr = &editCharPass
	input.HideText = true
	input.Size = eui.Point{X: 280, Y: 24}
	input.SetTooltip("Leave blank to keep the current password.")
	editCharPassInput = input
	inputEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventInputChanged {
			checkCapsWarning(&editCharPassPrev, editCharPass, editCharPassWarn)
		}
	}
	flow.AddItem(input)

	editCharPassWarn, _ = eui.NewText()
	editCharPassWarn.TextColor = eui.NewColor(255, 0, 0, 255)
	editCharPassWarn.Size = eui.Point{X: 280, Y: 24}
	editCharPassWarn.FontSize = 12
	flow.AddItem(editCharPassWarn)

	rememberCB, rememberEvents := eui.NewCheckbox()
	editCharRememberCB = rememberCB
	rememberCB.Text = "Save Password"
	rememberCB.Size = eui.Point{X: 280, Y: 24}
	rememberEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			editCharRemember = ev.Checked
		}
	}
	flow.AddItem(rememberCB)

	profileCB, profileEvents := eui.NewCheckbox()
	editCharProfileCB = profileCB
	profileCB.Text = "Keep settings separate"
	profileCB.SetTooltip("Give this character independent windows, appearance, audio, notifications, and related settings.")
	profileCB.Size = eui.Point{X: 280, Y: 24}
	profileEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			editCharProfile = ev.Checked
		}
	}
	flow.AddItem(profileCB)

	btnFlow := eui.NewRow()
	cancelBtn, cancelEvents := eui.NewButton()
	cancelBtn.Text = "Cancel"
	cancelBtn.Size = eui.Point{X: 136, Y: 24}
	cancelEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			clearPasswordInput(editCharPassInput, &editCharPass)
			editCharPassPrev = ""
			clearCapsWarnings()
			editCharWin.Close()
			loginWin.MarkOpen()
		}
	}
	btnFlow.AddItem(cancelBtn)

	saveBtn, saveEvents := eui.NewButton()
	saveBtn.Text = "Save"
	setMaterialButtonIcon(saveBtn, "save")
	saveBtn.Size = eui.Point{X: 136, Y: 24}
	editCharWin.DefaultButton = saveBtn
	saveEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type != eui.EventClick {
			return
		}
		hash, err := applyCharacterCredentialEdit(editCharName, editCharPass, editCharRemember)
		if err != nil {
			makeErrorWindow("Error: Edit Character: " + err.Error())
			return
		}
		setCharacterProfileEnabled(editCharName, editCharProfile)
		if strings.EqualFold(name, editCharName) {
			switchCharacterProfile(editCharName)
			passHash = hash
			pass = ""
		}
		clearPasswordInput(editCharPassInput, &editCharPass)
		editCharPassPrev = ""
		clearCapsWarnings()
		editCharWin.Close()
		loginWin.MarkOpen()
		updateCharacterButtons()
	}
	btnFlow.AddItem(saveBtn)
	flow.AddItem(btnFlow)

	editCharWin.AddItem(flow)
	editCharWin.AddWindow(false)
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

	flow := eui.NewColumn()

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

	btnFlow := eui.NewRow()

	cancelBtn, cancelEvents := eui.NewButton()
	cancelBtn.Text = "Cancel"
	cancelBtn.Size = eui.Point{X: 96, Y: 24}
	cancelEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			clearPasswordInput(passInput, &pass)
			passPrev = ""
			clearCapsWarnings()
			passWin.Close()
		}
	}
	btnFlow.AddItem(cancelBtn)

	okBtn, okEvents := eui.NewButton()
	okBtn.Text = "Connect"
	setMaterialButtonIcon(okBtn, "login")
	okBtn.Size = eui.Point{X: 96, Y: 24}
	okEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			if pass == "" {
				makeErrorWindow("Error: Login: password is empty")
				return
			}
			if name != "" {
				passHash = stagePasswordUpdate(name, pass, passRemember)
			}
			clearPasswordInput(passInput, &pass)
			passPrev = ""
			passWin.Close()
			startLogin()
		}
	}
	btnFlow.AddItem(okBtn)

	flow.AddItem(btnFlow)

	passWin.AddItem(flow)
	passWin.AddWindow(false)
}

func reserveMoviePlayback(filename string) bool {
	loginMu.Lock()
	defer loginMu.Unlock()
	if tcpConn != nil || loginInProgress || clmov != "" || playingMovie {
		return false
	}
	clmov = filename
	return true
}

func startLogin() {
	startLoginWithDemoCandidates(nil)
}

func startLoginWithDemoCandidates(demoCandidates []string) {
	loginMu.Lock()
	if loginInProgress || tcpConn != nil || clmov != "" || playingMovie {
		loginMu.Unlock()
		return
	}
	loginInProgress = true
	loginMu.Unlock()
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
		err := loginWithDemoCandidates(ctx, clVersion, demoCandidates)
		loginMu.Lock()
		loginCancel = nil
		loginInProgress = false
		connected := tcpConn != nil
		loginMu.Unlock()
		if err != nil {
			logError("login: %v", err)
			if len(demoCandidates) > 0 {
				loginMu.Lock()
				demoLoginActive = false
				loginMu.Unlock()
			}
			dispatchMainThread(func() {
				closeConnectDialog()
				discardStagedPassword()
				clearPasswordInput(passInput, &pass)
				passHash = ""
				if len(demoCandidates) > 0 {
					name = freeDemoSelection
				}
				if connected {
					return
				}
				// Bring login forward first so the popup stays on top.
				loginWin.MarkOpen()
				updateCharacterButtons()
				makeErrorWindow("Error: Login: " + err.Error())
			})
			return
		}
		dispatchMainThread(closeConnectDialog)
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

	flow := eui.NewColumn()

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

func showDemoCharacterDialog(candidates []string) {
	if len(candidates) == 0 {
		makeErrorWindow("Error: Demo: no demo characters are available.")
		loginWin.MarkOpen()
		return
	}
	if demoCharacterWin == nil {
		demoCharacterWin = eui.NewWindow()
		demoCharacterWin.Title = "Choose Demo Character"
		demoCharacterWin.Closable = false
		demoCharacterWin.Resizable = false
		demoCharacterWin.AutoSize = true
		demoCharacterWin.Movable = true

		flow := eui.NewColumn()
		prompt, _ := eui.NewText()
		prompt.Text = "Choose a demo character to play:"
		prompt.FontSize = 16
		prompt.Size = eui.Point{X: 360, Y: 28}
		flow.AddItem(prompt)

		demoCharacterList = eui.NewColumn()
		demoCharacterList.Scrollable = true
		demoCharacterList.Fixed = true
		demoCharacterList.Size = eui.Point{X: 360, Y: 320}
		flow.AddItem(demoCharacterList)

		buttons := eui.NewRow()
		cancel, cancelEvents := eui.NewButton()
		cancel.Text = "Cancel"
		cancel.Size = eui.Point{X: 96, Y: 24}
		cancelEvents.Handle = func(ev eui.UIEvent) {
			if ev.Type == eui.EventClick {
				demoCharacterWin.Close()
				loginWin.MarkOpen()
			}
		}
		buttons.AddItem(cancel)

		connect, connectEvents := eui.NewButton()
		connect.Text = "Connect"
		setMaterialButtonIcon(connect, "login")
		connect.Size = eui.Point{X: 96, Y: 24}
		connectEvents.Handle = func(ev eui.UIEvent) {
			if ev.Type != eui.EventClick || demoCharacterSelection == "" {
				return
			}
			demoCharacterWin.Close()
			loginMu.Lock()
			demoLoginActive = true
			loginMu.Unlock()
			startLoginWithDemoCandidates([]string{demoCharacterSelection})
		}
		buttons.AddItem(connect)
		demoCharacterWin.DefaultButton = connect
		flow.AddItem(buttons)

		demoCharacterWin.AddItem(flow)
		demoCharacterWin.AddWindow(false)
	}

	demoCharacterSelection = candidates[0]
	for i := range demoCharacterList.Contents {
		demoCharacterList.Contents[i] = nil
	}
	demoCharacterList.Contents = demoCharacterList.Contents[:0]
	for _, candidate := range candidates {
		radio, events := eui.NewRadio()
		radio.Text = candidate
		radio.RadioGroup = "demo-characters"
		radio.Size = eui.Point{X: 344, Y: 28}
		radio.Checked = candidate == demoCharacterSelection
		candidateCopy := candidate
		events.Handle = func(ev eui.UIEvent) {
			if ev.Type == eui.EventRadioSelected {
				demoCharacterSelection = candidateCopy
			}
		}
		demoCharacterList.AddItem(radio)
	}
	demoCharacterWin.MarkOpen()
	demoCharacterWin.Refresh()
}

func startDemoLogin() {
	loginMu.Lock()
	if demoLookupInProgress || loginInProgress || tcpConn != nil {
		loginMu.Unlock()
		return
	}
	demoLookupInProgress = true
	loginMu.Unlock()
	loginWin.Close()
	showConnectDialog("Finding an available demo character...")
	go func() {
		demoCandidates, err := fetchDemoCharacters(clVersion)
		if err != nil {
			loginMu.Lock()
			demoLookupInProgress = false
			connected := tcpConn != nil || loginInProgress
			loginMu.Unlock()
			logError("demo: %v", err)
			dispatchMainThread(func() {
				closeConnectDialog()
				if connected {
					return
				}
				loginWin.MarkOpen()
				makeErrorWindow("Error: Demo: " + err.Error())
			})
			return
		}
		loginMu.Lock()
		demoLookupInProgress = false
		loginMu.Unlock()
		dispatchMainThread(func() {
			closeConnectDialog()
			showDemoCharacterDialog(demoCandidates)
		})
	}()
}

func refreshLoginServerDropdown() {
	if loginServerDropdown == nil {
		return
	}
	addresses := serverAddresses()
	loginServerDropdown.Options = append(addresses, editServerListOption)
	loginServerDropdown.Selected = 0
	for i, address := range addresses {
		if sameServerAddress(address, gs.ServerAddress) {
			loginServerDropdown.Selected = i
			break
		}
	}
	loginServerDropdown.Dirty = true
	if loginWin != nil {
		loginWin.Refresh()
	}
}

func selectLoginServer(address string) {
	if normalized, ok := normalizeServerAddress(address); ok {
		gs.ServerAddress = normalized
		applyServerAddressSetting()
		settingsDirty = true
		refreshLoginServerDropdown()
	}
}

func refreshServerListEditor() {
	if serverListContents == nil {
		return
	}
	for i := range serverListContents.Contents {
		serverListContents.Contents[i] = nil
	}
	serverListContents.Contents = serverListContents.Contents[:0]
	for _, address := range serverAddresses() {
		row := eui.NewRow()
		label := eui.NewLabel(address)
		label.Size = eui.Point{X: 300, Y: 24}
		row.AddItem(label)
		if isBuiltInServerAddress(address) {
			builtIn := eui.NewLabel("Built-in")
			builtIn.Size = eui.Point{X: 80, Y: 24}
			row.AddItem(builtIn)
		} else {
			remove, events := eui.NewButton()
			remove.Text = "Remove"
			remove.Size = eui.Point{X: 80, Y: 24}
			addressCopy := address
			events.Handle = func(ev eui.UIEvent) {
				if ev.Type != eui.EventClick || !removeServerAddress(addressCopy) {
					return
				}
				applyServerAddressSetting()
				settingsDirty = true
				refreshLoginServerDropdown()
				refreshServerListEditor()
			}
			row.AddItem(remove)
		}
		serverListContents.AddItem(row)
	}
	if serverListWin != nil {
		serverListWin.Refresh()
	}
}

func openServerListWindow() {
	if serverListWin != nil {
		serverListWin.MarkOpen()
		refreshServerListEditor()
		return
	}
	serverListWin = eui.NewWindow()
	serverListWin.Title = "Edit Server List"
	serverListWin.Closable = true
	serverListWin.Resizable = false
	serverListWin.AutoSize = true
	serverListWin.Movable = true
	serverListWin.OnClose = func() {
		serverListWin = nil
		serverListContents = nil
	}

	flow := eui.NewColumn()
	instructions, _ := eui.NewText()
	instructions.Text = "Built-in servers are always available and cannot be removed."
	instructions.FontSize = 14
	instructions.Size = eui.Point{X: 420, Y: 28}
	flow.AddItem(instructions)

	serverListContents = eui.NewColumn()
	serverListContents.Scrollable = true
	serverListContents.Fixed = true
	serverListContents.Size = eui.Point{X: 420, Y: 224}
	flow.AddItem(serverListContents)

	addressInput, _ := eui.NewInput()
	addressInput.Label = "Server address"
	addressInput.TextPtr = &serverListAddress
	addressInput.Size = eui.Point{X: 300, Y: 24}
	flow.AddItem(addressInput)

	buttons := eui.NewRow()
	add, addEvents := eui.NewButton()
	add.Text = "Add Server"
	add.Size = eui.Point{X: 120, Y: 24}
	addEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type != eui.EventClick {
			return
		}
		if !addServerAddress(serverListAddress) {
			makeErrorWindow("Error: Server: enter a host and port, such as server.example:5010.")
			return
		}
		selectLoginServer(serverListAddress)
		serverListAddress = ""
		refreshServerListEditor()
	}
	buttons.AddItem(add)

	close, closeEvents := eui.NewButton()
	close.Text = "Close"
	close.Size = eui.Point{X: 96, Y: 24}
	closeEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			serverListWin.Close()
		}
	}
	buttons.AddItem(close)
	flow.AddItem(buttons)

	serverListWin.AddItem(flow)
	serverListWin.AddWindow(false)
	refreshServerListEditor()
	serverListWin.MarkOpen()
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
	loginWin.Padding = 12
	// Set the login window opacity
	loginWin.Opacity = 0.9
	// Increase title font size for "Login" by 2pt
	loginWin.SetTitleSize(loginWin.GetRawTitleSize() + 2)
	centerLoginWindow()
	loginFlow := eui.NewColumn()
	serverDropdown, serverEvents := eui.NewDropdown()
	loginServerDropdown = serverDropdown
	serverDropdown.Size = eui.Point{X: charWinWidth - 208, Y: 44}
	serverDropdown.SetTooltip("Choose the server to connect to, or edit the server list.")
	serverEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type != eui.EventDropdownSelected {
			return
		}
		addresses := serverAddresses()
		if ev.Index == len(addresses) {
			refreshLoginServerDropdown()
			openServerListWindow()
			return
		}
		if ev.Index >= 0 && ev.Index < len(addresses) {
			selectLoginServer(addresses[ev.Index])
		}
	}
	refreshLoginServerDropdown()
	// Characters list lives in its own flow and is scrollable.
	// Use a fixed height so the window doesn't grow unbounded.
	charactersList = eui.NewColumn()
	charactersList.Scrollable = true
	charactersList.Fixed = true
	charactersList.Size = eui.Point{X: charWinWidth, Y: 224}

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
	setMaterialButtonIcon(connBtn, "login")
	connBtn.Size = eui.Point{X: 200, Y: 44}
	connBtn.FontSize = 18
	connBtn.Outlined = true
	connBtn.Border = 2
	connBtn.OutlineColor = eui.ColorGreen
	loginConnectButton = connBtn
	loginWin.DefaultButton = connBtn
	// Keep a handle so we can enable/disable it dynamically.
	connEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			if name == "" {
				// No character selected: instruct the user to pick one first.
				makeErrorWindow("Please select a character to connect with first.")
				return
			}
			if name == freeDemoSelection {
				startDemoLogin()
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
				clearPasswordInput(passInput, &pass)
				passPrev = ""
				passWin.MarkOpenNear(ev.Item)
				return
			}
			switchCharacterProfile(name)
			startLogin()
			updateCharacterButtons()
		}
	}

	addBtn, addEvents := eui.NewButton()
	addBtn.Text = "Add"
	setMaterialButtonIcon(addBtn, "add")
	addBtn.SetTooltip("Add a saved character")
	addBtn.Size = eui.Point{X: (charWinWidth - 16) / 3, Y: 32}
	addEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			addCharName = ""
			clearPasswordInput(addCharPassInput, &addCharPass)
			addCharPassPrev = ""
			clearCapsWarnings()
			addCharRemember = true
			addCharProfile = false
			if addCharProfileCB != nil {
				addCharProfileCB.Checked = false
				addCharProfileCB.Dirty = true
			}
			loginWin.Close()
			addCharWin.MarkOpenNear(ev.Item)
		}
	}

	editBtn, editEvents := eui.NewButton()
	editCharBtn = editBtn
	editBtn.Text = "Edit"
	setMaterialButtonIcon(editBtn, "edit")
	editBtn.SetTooltip("Change the selected character's password, password saving, or settings profile.")
	editBtn.Size = eui.Point{X: (charWinWidth - 16) / 3, Y: 32}
	editEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type != eui.EventClick {
			return
		}
		if _, ok := selectedCharacter(name); !ok {
			makeErrorWindow("Select a saved character to edit.")
			return
		}
		if editCharWin == nil {
			makeEditCharacterWindow()
		}
		if err := prepareEditCharacter(name); err != nil {
			makeErrorWindow("Error: Edit Character: " + err.Error())
			return
		}
		loginWin.Close()
		editCharWin.MarkOpenNear(ev.Item)
	}

	deleteBtn, deleteEvents := eui.NewButton()
	deleteCharBtn = deleteBtn
	deleteBtn.Text = "Delete"
	setMaterialButtonIcon(deleteBtn, "delete")
	deleteBtn.SetTooltip("Delete the selected saved character")
	deleteBtn.Size = eui.Point{X: (charWinWidth - 16) / 3, Y: 32}
	deleteBtn.Color = eui.ColorDarkRed
	deleteBtn.HoverColor = eui.ColorRed
	deleteEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type != eui.EventClick {
			return
		}
		character, ok := selectedCharacter(name)
		if !ok {
			makeErrorWindow("Select a saved character to delete.")
			return
		}
		confirmRemoveCharacter(character)
	}

	characterActions := eui.NewRow()
	characterActions.Size = eui.Point{X: charWinWidth, Y: 32}
	addBtn.Position = eui.Point{}
	editBtn.Position = eui.Point{X: 8}
	deleteBtn.Position = eui.Point{X: 8}
	characterActions.AddItem(addBtn)
	characterActions.AddItem(editBtn)
	characterActions.AddItem(deleteBtn)

	openBtn, openEvents := eui.NewButton()
	openBtn.Text = "Play movie file [clMov]"
	setMaterialButtonIcon(openBtn, "movie")
	openBtn.SetTooltip("Open and play a .clmov recording")
	openBtn.Size = eui.Point{X: 248, Y: 32}
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
			if !reserveMoviePlayback(filename) {
				makeErrorWindow("Disconnect from the server before playing a movie.")
				return
			}
			loginWin.Close()
			go func() {
				drawStateEncrypted = false
				frames, err := parseMovie(filename, clVersion)
				if err != nil {
					logError("parse movie: %v", err)
					clmov = ""
					dispatchMainThread(func() {
						loginWin.MarkOpen()
						makeErrorWindow("Error: Open clMov: " + err.Error())
					})
					return
				}
				playerName = extractMoviePlayerName(frames)
				applyEnabledScripts()
				ctx, cancel := context.WithCancel(gameCtx)
				var mp *moviePlayer
				if !dispatchMainThreadAndWait(ctx, func() {
					updateGameWindowTitle()
					mp = newMoviePlayer(frames, clMovFPS, cancel)
					mp.makePlaybackWindow()
				}) {
					cancel()
					return
				}
				go mp.run(ctx)
			}()
		}
	}

	quitBttn, quitEvn := eui.NewButton()
	quitBttn.Text = "Quit"
	setMaterialButtonIcon(quitBttn, "power_settings_new")
	quitBttn.FontSize = 14
	quitBttn.Size = eui.Point{X: 112, Y: 32}
	quitBttn.Outlined = true
	quitBttn.Border = 2
	quitBttn.OutlineColor = eui.ColorRed
	quitEvn.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			confirmQuit()
		}
	}

	verFlow := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_HORIZONTAL, Size: eui.Point{X: charWinWidth, Y: 28}}
	verLabel, _ := eui.NewText()
	verLabel.Text = fmt.Sprintf("goThoom test %4d", appVersion)
	verLabel.FontSize = 12
	verLabel.Size = eui.Point{X: charWinWidth - 216, Y: 28}
	verLabel.Position = eui.Point{Y: 4}
	verFlow.AddItem(verLabel)

	changeBtn, changeEvents := eui.NewButton()
	changeBtn.Text = "Changelog"
	setMaterialButtonIcon(changeBtn, "history")
	changeBtn.SetTooltip("View recent changes")
	changeBtn.Size = eui.Point{X: 120, Y: 28}
	changeBtn.FontSize = 12
	changeBtn.Position = eui.Point{X: 8}
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
	setMaterialButtonIcon(aboutBtn, "info")
	aboutBtn.Size = eui.Point{X: 80, Y: 28}
	aboutBtn.FontSize = 12
	aboutBtn.Position = eui.Point{X: 8}
	aboutEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			openAboutWindow(ev.Item)
		}
	}
	verFlow.AddItem(aboutBtn)

	addLoginSpacer := func(height float32) {
		spacer := &eui.ItemData{ItemType: eui.ITEM_TEXT}
		spacer.Size = eui.Point{X: charWinWidth, Y: height}
		loginFlow.AddItem(spacer)
	}

	utilityRow := eui.NewRow()
	openBtn.Position = eui.Point{}
	quitBttn.Position = eui.Point{X: charWinWidth - openBtn.Size.X - quitBttn.Size.X}
	utilityRow.AddItem(openBtn)
	utilityRow.AddItem(quitBttn)
	loginFlow.AddItem(utilityRow)
	addLoginSpacer(12)
	characterEditLabel, _ := eui.NewText()
	characterEditLabel.Text = "Edit Characters:"
	characterEditLabel.FontSize = 13
	eui.ApplyBoldFace(characterEditLabel)
	characterEditLabel.Size = eui.Point{X: charWinWidth, Y: 28}
	loginFlow.AddItem(characterEditLabel)
	loginFlow.AddItem(characterActions)
	addLoginSpacer(12)
	characterListLabel, _ := eui.NewText()
	characterListLabel.Text = "Character list:"
	characterListLabel.FontSize = 13
	eui.ApplyBoldFace(characterListLabel)
	characterListLabel.Size = eui.Point{X: charWinWidth, Y: 28}
	loginFlow.AddItem(characterListLabel)
	loginFlow.AddItem(charactersList)
	addLoginSpacer(12)
	connectRow := eui.NewRow()
	connBtn.Position = eui.Point{}
	serverDropdown.Position = eui.Point{X: 8}
	connectRow.AddItem(connBtn)
	connectRow.AddItem(serverDropdown)
	loginFlow.AddItem(connectRow)
	addLoginSpacer(8)
	loginFlow.AddItem(verFlow)

	// Root controls share one left edge; horizontal rows own their internal gaps.
	for _, item := range loginFlow.Contents {
		item.Position = eui.Point{}
	}
	loginWin.AddItem(loginFlow)
	loginWin.AddWindow(false)
}

func centerLoginWindow() {
	if loginWin != nil {
		loginWin.SetZone(eui.HZoneCenter, eui.VZoneCenter)
	}
}

func makeChangelogWindow() {
	if changelogWin == nil {
		changelogWin, changelogList, _ = eui.NewTextWindow("Changelog", eui.HZoneCenter, eui.VZoneMiddleTop, false)
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
	updateTextWindow(changelogWin, changelogList, nil, lines, 14, "", monoFaceSource, false, &changelogTextWrapCache)
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
	eui.ShowPopup("Error", body, []eui.PopupButton{{Text: "OK"}})
}

var SettingsLock sync.Mutex

// resetAllSettings restores gs to defaults, reapplies, and refreshes windows.
func resetAllSettings() {
	gs = gsdef
	ensureMessageTextColors()
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
	if filePathsWin != nil {
		filePathsWin.Close()
		filePathsWin = nil
		filePathsCopyCB = nil
		filePathsStatus = nil
		filePathsDisplays = nil
	}
	if textColorsWin != nil {
		textColorsWin.Close()
		textColorsWin = nil
		textColorSwatchesDark = nil
		textColorSwatchesLight = nil
		themeTextColorOverrideCB = nil
		classicMessageColorsCB = nil
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
	placeToolbar(gs.ToolbarPlacement, false)

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
	if settingsWin != nil {
		refreshShaderEffectControls()
		settingsWin.Refresh()
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

// resetWindows restores the default window layout without changing other settings.
func resetWindows() {
	resetSavedWindowSettings()
	clampWindowSettings()

	if gameWin != nil {
		gameWin.MarkOpen()
	}

	forgetWindow := func(win **eui.WindowData) {
		if *win != nil {
			(*win).RemoveWindow()
			*win = nil
		}
	}
	forgetWindow(&inventoryWin)
	forgetWindow(&playersWin)
	forgetWindow(&consoleWin)
	forgetWindow(&chatWin)

	makeInventoryWindow()
	makePlayersWindow()
	makeConsoleWindow()
	_ = makeChatWindow()
	placeToolbar(gs.ToolbarPlacement, false)
	applyManagedWindowLayout()
	windowsRestored = true

	// Save the newly-created default geometry and zones immediately.
	syncWindowSettings()
	saveSettings()
	settingsDirty = false
}

func confirmResetSettings() {
	// Use a red confirm button to indicate a destructive action
	eui.ShowPopup(
		"Confirm Reset",
		"Reset all settings to defaults? This cannot be undone.",
		[]eui.PopupButton{
			{Text: "Cancel"},
			{Text: "Reset", Color: &eui.ColorDarkRed, HoverColor: &eui.ColorRed, Action: func() { resetAllSettings() }},
		},
	)
}

func confirmResetWindows() {
	eui.ShowPopup(
		"Confirm Reset Windows",
		"Reset window positions, sizes, visibility, and pinned locations to defaults?",
		[]eui.PopupButton{
			{Text: "Cancel"},
			{Text: "Reset", Color: &eui.ColorDarkRed, HoverColor: &eui.ColorRed, Action: resetWindows},
		},
	)
}

func confirmQuit() {
	eui.ShowPopup(
		"Confirm Quit",
		"Are you sure you would like to quit?",
		[]eui.PopupButton{
			{Text: "Cancel"},
			{Text: "Quit", Color: &eui.ColorDarkRed, HoverColor: &eui.ColorRed, Action: func() {
				saveCharacters()
				saveSettings()
				exitApplication(0, "user confirmed Quit")
			}},
		},
	)
}

// showShaderDisablePrompt suggests the complete lowest-resource preset when
// sustained real-world rendering performance is poor.
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

	flow := eui.NewColumn()

	msg, _ := eui.NewText()
	msg.Text = "FPS has been under 50 for a while. The Lowest quality preset may provide smoother rendering."
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
	disableBtn.Text = "Use Lowest Preset"
	disableBtn.Size = eui.Point{X: 140, Y: 24}
	disableEv.Handle = func(ev eui.UIEvent) {
		if ev.Type != eui.EventClick {
			return
		}
		if shaderWarnDontShowCB != nil && shaderWarnDontShowCB.Checked {
			gs.PromptDisableShaders = false
		}
		applyQualityPreset("Lowest")
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
	row := eui.NewRow()

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

	eui.ShowPopup(
		"Delete Character",
		fmt.Sprintf("Are you sure you want to delete %s?", c.Name),
		[]eui.PopupButton{
			{Text: "Cancel"},
			{Text: "Delete Character", Color: &eui.ColorDarkRed, HoverColor: &eui.ColorRed, Action: func() {
				discardStagedPassword()
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

const artworkScaleGuidance = "Suggested starting points:\n2x: up to 1080p (default)\n3x: 1440p\n4x: 4K / large game views"

func newArtworkScaleGuidance(width float32) *eui.ItemData {
	info, _ := eui.NewText()
	info.Text = artworkScaleGuidance
	info.FontSize = 10
	info.Size = eui.Point{X: width, Y: 90}
	return info
}

func addQualityColumns(page *eui.ItemData, width float32, groups ...[]*eui.ItemData) {
	row := eui.NewRow()
	for i, sections := range groups {
		if i > 0 {
			row.AddItem(&eui.ItemData{ItemType: eui.ITEM_TEXT, Size: eui.Point{X: 12, Y: 1}})
		}
		column := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL, Size: eui.Point{X: width, Y: 10}}
		for _, section := range sections {
			column.AddItem(section)
		}
		row.AddItem(column)
	}
	page.AddItem(row)
}

func newGraphicsPerformanceOptions() *eui.ItemData {
	const pageWidth = settingsPanelWidth
	const width = (pageWidth - 20) / 2
	outer := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL, ActiveOutline: true}
	artworkPage := newSettingsPage("Artwork", pageWidth)
	motionPage := newSettingsPage("Motion", pageWidth)
	shaderPage := newSettingsPage("Lighting & Effects", pageWidth)

	artworkSection := eui.NewSection("Artwork Scaling", width)
	occlusionSection := eui.NewSection("Foreground Occlusion", width)
	gammaSection := eui.NewSection("Sprite Gamma", width)
	denoiseSection := eui.NewSection("Dither Cleanup", width)
	shadowSection := eui.NewSection("Shadows", width)
	motionSection := eui.NewSection("Motion Smoothing", width)
	shaderSection := eui.NewSection("Lighting & Effects", pageWidth)
	addQualityColumns(artworkPage, width, []*eui.ItemData{artworkSection, occlusionSection}, []*eui.ItemData{gammaSection, denoiseSection})
	shaderPage.AddItem(shaderSection)
	outer.Tabs = []*eui.ItemData{artworkPage, motionPage, shaderPage}

	masterShaders, masterShaderEvents := eui.NewCheckbox()
	shadersEnabledCB = masterShaders
	shadersEnabledCB.Text = "Enhanced visual effects"
	shadersEnabledCB.Size = eui.Point{X: pageWidth, Y: 24}
	shadersEnabledCB.Checked = gs.ShadersEnabled
	shadersEnabledCB.SetTooltip("Controls lighting, animation blending, replacement effects, and faster shadows. Individual choices are preserved while disabled.")
	masterShaderEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type != eui.EventCheckboxChanged {
			return
		}
		gs.ShadersEnabled = ev.Checked
		settingsDirty = true
		refreshShaderEffectControls()
		if qualityPresetDD != nil {
			qualityPresetDD.Selected = detectQualityPreset()
		}
		if gameWin != nil {
			gameWin.Refresh()
		}
		if debugWin != nil {
			debugWin.Refresh()
		}
	}
	lightingSection := eui.NewColumn()
	animationSection := eui.NewSection("Animation Blending", width)
	addQualityColumns(shaderSection, width, []*eui.ItemData{lightingSection}, []*eui.ItemData{shadowSection})
	addQualityColumns(motionPage, width, []*eui.ItemData{motionSection}, []*eui.ItemData{animationSection})
	experimentalSection := eui.NewColumn()
	experimentalSection.AddItem(eui.NewSubheading("Experimental", pageWidth))
	shaderSection.AddItem(experimentalSection)

	renderScale, renderScaleEvents := eui.NewSlider()
	qualityRenderScaleSlider = renderScale
	renderScale.Label = "Artwork scale override"
	renderScale.MinValue = 2
	renderScale.MaxValue = 4
	renderScale.IntOnly = true
	if gs.GameScale < 2 {
		gs.GameScale = 2
	}
	if gs.GameScale > 4 {
		gs.GameScale = 4
	}

	renderScale.Value = float32(math.Round(gs.GameScale))
	renderScale.Size = eui.Point{X: width - 10, Y: 24}
	renderScale.SetTooltip("Fixed artwork texture scale. Default: 2x. Resizing the game window does not change it. Higher values use more GPU memory.")
	renderScaleEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventSliderChanged {
			prevUpscale := gs.SpriteUpscale
			v := math.Round(float64(ev.Value))
			if v < 2 {
				v = 2
			}
			if v > 4 {
				v = 4
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
	artworkSection.AddItem(renderScale)
	artworkSection.AddItem(newArtworkScaleGuidance(width))

	uDD, upscaleModeEvents := eui.NewDropdown()
	upscaleModeDD = uDD
	upscaleModeDD.Label = "Artwork upscale style"
	upscaleModeDD.Options = artworkUpscaleModeNames
	upscaleModeDD.Selected = artworkUpscaleMode()
	upscaleModeDD.Size = eui.Point{X: width, Y: 24}
	upscaleModeDD.SetTooltip("Crisp preserves hard edges; smoother modes blend more neighboring pixels.")
	upscaleModeEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventDropdownSelected && ev.Index >= artworkUpscaleOff && ev.Index <= artworkUpscaleUltraSmooth {
			if artworkUpscaleMode() != ev.Index {
				setArtworkUpscaleMode(ev.Index)
				clearCaches()
				settingsDirty = true
				if gameWin != nil {
					gameWin.Refresh()
				}
			}
		}
	}
	artworkSection.AddItem(upscaleModeDD)

	ppCB, pixelPerfectEvents := eui.NewCheckbox()
	pixelPerfectCB := ppCB
	pixelPerfectCB.Text = "Pixel-art scaling"
	pixelPerfectCB.Size = eui.Point{X: width, Y: 24}
	pixelPerfectCB.Checked = gs.PixelArtScaling
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
	artworkSection.AddItem(pixelPerfectCB)

	fadePicsCB, fadePicsEvents := eui.NewCheckbox()
	fadeObscuringCB = fadePicsCB
	fadePicsCB.Text = "Fade objects obscuring mobiles"
	fadePicsCB.Size = eui.Point{X: width, Y: 24}
	fadePicsCB.Checked = gs.FadeObscuringPictures
	fadePicsEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			gs.FadeObscuringPictures = ev.Checked
			settingsDirty = true
		}
	}
	occlusionSection.AddItem(fadePicsCB)

	obscureSlider, obscureEvents := eui.NewSlider()
	obscureSlider.Label = "Obscuring Object Opacity"
	obscureSlider.MinValue = 0.25
	obscureSlider.MaxValue = 0.7
	obscureSlider.Value = float32(gs.ObscuringPictureOpacity)
	obscureSlider.Size = eui.Point{X: width - 10, Y: 24}
	obscureSlider.SetTooltip("Lower values make covering artwork more transparent while it is faded.")
	obscureEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventSliderChanged {
			gs.ObscuringPictureOpacity = float64(ev.Value)
			settingsDirty = true
		}
	}
	occlusionSection.AddItem(obscureSlider)

	/*
		                                showFPSCB, showFPSEvents := eui.NewCheckbox()
		                                showFPSCB.Text = "Show FPS + UPS"
						showFPSCB.Size = eui.Point{X: width, Y: 24}
						showFPSCB.Checked = gs.ShowFPS
						showFPSCB.SetTooltip("Show FPS and update rate.")
						showFPSEvents.Handle = func(ev eui.UIEvent) {
							if ev.Type == eui.EventCheckboxChanged {
								gs.ShowFPS = ev.Checked
								settingsDirty = true
							}
						}
						flow.AddItem(showFPSCB)
	*/

	var shadowDarknessSlider *eui.ItemData

	windowShadowsCB = newWindowShadowsCheckbox(width)
	shadowSection.AddItem(windowShadowsCB)

	characterShadowItem, characterShadowsEvents := eui.NewCheckbox()
	characterShadowsCB = characterShadowItem
	characterShadowsCB.Text = "Character Shadows"
	characterShadowsCB.Size = eui.Point{X: width, Y: 24}
	characterShadowsCB.Checked = gs.CharacterShadows
	characterShadowsEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			gs.CharacterShadows = ev.Checked
			if shadowDarknessSlider != nil {
				shadowDarknessSlider.Disabled = !ev.Checked
			}
			if mobileSunShadowsCB != nil {
				mobileSunShadowsCB.Disabled = !ev.Checked
			}
			refreshShaderEffectControls()
			settingsDirty = true
		}
	}
	shadowSection.AddItem(characterShadowsCB)

	shadowDarknessSlider, shadowDarknessEvents := eui.NewSlider()
	characterShadowSlider = shadowDarknessSlider
	shadowDarknessSlider.Label = "Character Shadow Darkness"
	shadowDarknessSlider.MinValue = 1
	shadowDarknessSlider.MaxValue = 200
	shadowDarknessSlider.IntOnly = true
	shadowDarknessSlider.Value = float32(gs.CharacterShadowDarkness * 100)
	shadowDarknessSlider.Size = eui.Point{X: width - 10, Y: 24}
	shadowDarknessSlider.Disabled = !gs.CharacterShadows
	shadowDarknessEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventSliderChanged {
			gs.CharacterShadowDarkness = float64(ev.Value / 100)
			settingsDirty = true
		}
	}
	shadowSection.AddItem(shadowDarknessSlider)

	fasterShadowCB, fasterShadowEvents := eui.NewCheckbox()
	fasterCharacterShadowsCB = fasterShadowCB
	fasterShadowCB.Text = "Faster Character Shadows"
	fasterShadowCB.Size = eui.Point{X: width, Y: 24}
	fasterShadowCB.Checked = gs.FasterCharacterShadows
	fasterShadowCB.Disabled = !gs.ShadersEnabled || !gs.CharacterShadows
	fasterShadowCB.SetTooltip("Uses one cheaper shadow pass, but may shade foreground artwork that should cover a shadow.")
	fasterShadowEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			gs.FasterCharacterShadows = ev.Checked
			settingsDirty = true
		}
	}
	shadowSection.AddItem(fasterShadowCB)

	mobileSunShadowItem, mobileSunShadowsEvents := eui.NewCheckbox()
	mobileSunShadowsCB = mobileSunShadowItem
	mobileSunShadowsCB.Text = "Characters Receive Sun Shadows"
	mobileSunShadowsCB.Size = eui.Point{X: width, Y: 24}
	mobileSunShadowsCB.Checked = gs.MobilesReceiveSunShadows
	mobileSunShadowsCB.Disabled = !gs.CharacterShadows
	mobileSunShadowsCB.SetTooltip("Shade characters in proportion to how much another character's projected shadow covers them.")
	mobileSunShadowsEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			gs.MobilesReceiveSunShadows = ev.Checked
			settingsDirty = true
		}
	}
	shadowSection.AddItem(mobileSunShadowsCB)

	lightingSection.AddItem(eui.NewSubheading("Lighting", width))
	shaderQualityCB, shaderQualityEv := eui.NewCheckbox()
	shaderLightingCB = shaderQualityCB
	shaderQualityCB.Text = "Lighting Effects"
	shaderQualityCB.Size = eui.Point{X: width, Y: 24}
	shaderQualityCB.Checked = gs.ShaderLighting
	shaderQualityEv.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			gs.ShaderLighting = ev.Checked
			settingsDirty = true
			if qualityPresetDD != nil {
				qualityPresetDD.Selected = detectQualityPreset()
			}
			refreshShaderEffectControls()
			if debugWin != nil {
				debugWin.Refresh()
			}
		}
	}
	lightingSection.AddItem(shaderQualityCB)

	mobileConeCB, mobileConeEvents := eui.NewCheckbox()
	mobileLightConeShadowsCB = mobileConeCB
	mobileConeCB.Text = "Mobile light-cone shadows (experimental)"
	mobileConeCB.Size = eui.Point{X: pageWidth, Y: 24}
	mobileConeCB.Checked = gs.MobileLightConeShadows
	mobileConeCB.SetTooltip("Let mobiles cast experimental soft cone shadows from nearby lights.")
	mobileConeEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			gs.MobileLightConeShadows = ev.Checked
			settingsDirty = true
		}
	}
	experimentalSection.AddItem(mobileConeCB)

	flameCB, flameEvents := eui.NewCheckbox()
	flameFlickerCB = flameCB
	flameFlickerCB.Text = "Flame Light Flicker"
	flameFlickerCB.Size = eui.Point{X: width, Y: 24}
	flameFlickerCB.Checked = gs.FlameLightFlicker
	flameEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			gs.FlameLightFlicker = ev.Checked
			if flameFlickerSlider != nil {
				refreshShaderEffectControls()
			}
			settingsDirty = true
		}
	}
	lightingSection.AddItem(flameFlickerCB)

	flameSlider, flameSliderEvents := eui.NewSlider()
	flameFlickerSlider = flameSlider
	flameFlickerSlider.Label = "Flame Flicker Strength"
	flameFlickerSlider.MinValue = 0
	flameFlickerSlider.MaxValue = 200
	flameFlickerSlider.IntOnly = true
	flameFlickerSlider.Value = float32(gs.FlameFlickerStrength * 100)
	flameFlickerSlider.Size = eui.Point{X: width - 10, Y: 24}
	flameSliderEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventSliderChanged {
			gs.FlameFlickerStrength = float64(ev.Value / 100)
			settingsDirty = true
		}
	}
	lightingSection.AddItem(flameFlickerSlider)

	sLS, shaderLightEvents := eui.NewSlider()
	shaderLightSlider = sLS
	shaderLightSlider.Label = "Light Strength"
	shaderLightSlider.MinValue = 0.01
	shaderLightSlider.MaxValue = 5000
	shaderLightSlider.IntOnly = true
	shaderLightSlider.Value = float32(gs.ShaderLightStrength * 100)
	shaderLightSlider.Size = eui.Point{X: width - 10, Y: 24}
	shaderLightEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventSliderChanged {
			gs.ShaderLightStrength = float64(ev.Value / 100)
			settingsDirty = true
			if debugWin != nil {
				debugWin.Refresh()
			}
		}
	}
	lightingSection.AddItem(shaderLightSlider)

	sGS, shaderGlowEvents := eui.NewSlider()
	shaderGlowSlider = sGS
	shaderGlowSlider.Label = "Glow Strength"
	shaderGlowSlider.MinValue = 0.01
	shaderGlowSlider.MaxValue = 500
	shaderGlowSlider.IntOnly = true
	shaderGlowSlider.Value = float32(gs.ShaderGlowStrength * 100)
	shaderGlowSlider.Size = eui.Point{X: width - 10, Y: 24}
	shaderGlowEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventSliderChanged {
			gs.ShaderGlowStrength = float64(ev.Value / 100)
			settingsDirty = true
			if debugWin != nil {
				debugWin.Refresh()
			}
		}
	}
	lightingSection.AddItem(shaderGlowSlider)

	replacementCB, replacementEffectsEvents := eui.NewCheckbox()
	replacementEffectsCB = replacementCB
	replacementEffectsCB.Text = "Replacement Effects (experimental)"
	replacementEffectsCB.Size = eui.Point{X: pageWidth, Y: 24}
	replacementEffectsCB.Checked = gs.ReplacementEffects
	replacementEffectsCB.SetTooltip("Use procedural magic effects.")
	replacementEffectsEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			gs.ReplacementEffects = ev.Checked
			settingsDirty = true
		}
	}
	experimentalSection.AddItem(replacementEffectsCB)

	gcCB, gammaEvents := eui.NewCheckbox()
	gammaCorrectionCB = gcCB
	gammaCorrectionCB.Text = "Enable Sprite Gamma Correction"
	gammaCorrectionCB.Size = eui.Point{X: width, Y: 24}
	gammaCorrectionCB.Checked = gs.SpriteGammaCorrection
	gammaCorrectionCB.SetTooltip("Compensates classic artwork for modern displays; disable if colors look washed out.")
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
				if settingsWin != nil {
					settingsWin.Refresh()
				}
			}
		}
	}
	gammaSection.AddItem(gammaCorrectionCB)

	sgSlider, spriteGammaEvents := eui.NewSlider()
	spriteGammaSlider = sgSlider
	spriteGammaSlider.Label = "Sprite Gamma"
	spriteGammaSlider.MinValue = float32(gammaOptions[0])
	spriteGammaSlider.MaxValue = float32(gammaOptions[len(gammaOptions)-1])
	spriteGammaSlider.Value = float32(gs.SpriteGamma)
	spriteGammaSlider.Size = eui.Point{X: width - 10, Y: 24}
	spriteGammaSlider.Disabled = !gs.SpriteGammaCorrection
	spriteGammaSlider.SetTooltip("Describes how brightness is encoded in the original sprite artwork.")
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
	gammaSection.AddItem(spriteGammaSlider)

	mgSlider, monitorGammaEvents := eui.NewSlider()
	monitorGammaSlider = mgSlider
	monitorGammaSlider.Label = "Monitor Gamma"
	monitorGammaSlider.MinValue = float32(gammaOptions[0])
	monitorGammaSlider.MaxValue = float32(gammaOptions[len(gammaOptions)-1])
	monitorGammaSlider.Value = float32(gs.MonitorGamma)
	monitorGammaSlider.Size = eui.Point{X: width - 10, Y: 24}
	monitorGammaSlider.Disabled = !gs.SpriteGammaCorrection
	monitorGammaSlider.SetTooltip("Describes the target display response used when correcting sprite brightness.")
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
	gammaSection.AddItem(monitorGammaSlider)

	// (moved) Background behavior options are placed under Audio/Notifications

	dCB, denoiseEvents := eui.NewCheckbox()
	denoiseCB = dCB
	denoiseCB.Text = "Blend Image Dithering"
	denoiseCB.Size = eui.Point{X: width, Y: 24}
	denoiseCB.Checked = gs.DenoiseImages
	denoiseEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			gs.DenoiseImages = ev.Checked
			if clImages != nil {
				clImages.SetDenoise(gs.DenoiseImages, gs.DenoiseSharpness, gs.DenoiseAmount)
			}
			clearCaches()
			settingsDirty = true
		}
	}
	denoiseSection.AddItem(denoiseCB)

	denoiseSharpSlider, denoiseSharpEvents := eui.NewSlider()
	denoiseSharpSlider.Label = "Sharpness"
	denoiseSharpSlider.MinValue = 0
	denoiseSharpSlider.MaxValue = 100
	denoiseSharpSlider.Value = float32(gs.DenoiseSharpness * 5)
	denoiseSharpSlider.Size = eui.Point{X: width - 10, Y: 24}
	denoiseSharpEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventSliderChanged {
			gs.DenoiseSharpness = float64(ev.Value / 5)
			if clImages != nil {
				clImages.SetDenoise(gs.DenoiseImages, gs.DenoiseSharpness, gs.DenoiseAmount)
			}
			clearCaches()
			settingsDirty = true
		}
	}
	denoiseSection.AddItem(denoiseSharpSlider)

	denoiseAmtSlider, denoiseAmtEvents := eui.NewSlider()
	denoiseAmtSlider.Label = "Denoise strength"
	denoiseAmtSlider.MinValue = 0
	denoiseAmtSlider.MaxValue = 50
	denoiseAmtSlider.Value = float32(gs.DenoiseAmount * 100)
	denoiseAmtSlider.Size = eui.Point{X: width - 10, Y: 24}
	denoiseAmtEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventSliderChanged {
			gs.DenoiseAmount = float64(ev.Value / 100)
			if clImages != nil {
				clImages.SetDenoise(gs.DenoiseImages, gs.DenoiseSharpness, gs.DenoiseAmount)
			}
			clearCaches()
			settingsDirty = true
		}
	}
	denoiseSection.AddItem(denoiseAmtSlider)

	mCB, motionEvents := eui.NewCheckbox()
	motionCB = mCB
	motionCB.Text = "Smooth Motion"
	motionCB.Size = eui.Point{X: width, Y: 24}
	motionCB.Checked = gs.MotionSmoothing
	motionEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			gs.MotionSmoothing = ev.Checked
			refreshShaderEffectControls()
			settingsDirty = true
		}
	}
	motionSection.AddItem(motionCB)

	smallMovingCB, smallMovingEvents := eui.NewCheckbox()
	smallMovingPicturesCB = smallMovingCB
	smallMovingPicturesCB.Text = "Smooth small moving objects"
	smallMovingPicturesCB.Size = eui.Point{X: width, Y: 24}
	smallMovingPicturesCB.Checked = gs.InterpolateSmallMovingPictures
	smallMovingPicturesCB.Disabled = !gs.MotionSmoothing
	smallMovingPicturesCB.SetTooltip("Attempt to smooth independently moving picture sprites whose visible area is up to 35x35 pixels and moves no more than 64 pixels between updates, such as coins. Requires Smooth Motion.")
	smallMovingEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			gs.InterpolateSmallMovingPictures = ev.Checked
			settingsDirty = true
		}
	}
	motionSection.AddItem(smallMovingPicturesCB)

	coordsCB, coordsEvents := eui.NewCheckbox()
	coordsCB.Text = "Subpixel movement"
	coordsCB.Size = eui.Point{X: width, Y: 24}
	coordsCB.Checked = gs.FloatingPointSpriteCoords
	coordsCB.SetTooltip("Keeps interpolated sprites at subpixel screen positions for smoother movement. This can cause shimmering on some artwork; turn it off to floor sprites to whole pixels.")
	coordsEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			SettingsLock.Lock()
			gs.FloatingPointSpriteCoords = ev.Checked
			SettingsLock.Unlock()
			settingsDirty = true
		}
	}
	motionSection.AddItem(coordsCB)

	aCB, animEvents := eui.NewCheckbox()
	animCB = aCB
	animCB.Text = "Character Animation Blending"
	animCB.Size = eui.Point{X: width, Y: 24}
	animCB.Checked = gs.BlendMobiles
	animEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			gs.BlendMobiles = ev.Checked
			refreshShaderEffectControls()
			settingsDirty = true
		}
	}
	animationSection.AddItem(animCB)

	pCB, pictBlendEvents := eui.NewCheckbox()
	pictBlendCB = pCB
	pictBlendCB.Text = "World Animation Blending"
	pictBlendCB.Size = eui.Point{X: width, Y: 24}
	pictBlendCB.Checked = gs.BlendPicts
	pictBlendEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			gs.BlendPicts = ev.Checked
			refreshShaderEffectControls()
			settingsDirty = true
		}
	}
	animationSection.AddItem(pictBlendCB)

	mobileSlider, mobileBlendEvents := eui.NewSlider()
	mobileBlendSlider = mobileSlider
	mobileBlendSlider.Label = "Character Animation Blend Duration"
	mobileBlendSlider.MinValue = 0.1
	mobileBlendSlider.MaxValue = 1.0
	mobileBlendSlider.Value = float32(gs.MobileBlendAmount)
	mobileBlendSlider.Size = eui.Point{X: width - 10, Y: 24}
	mobileBlendSlider.SetTooltip("Set how much of the animation interval is spent blending frames.")
	mobileBlendEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventSliderChanged {
			gs.MobileBlendAmount = float64(ev.Value)
			settingsDirty = true
		}
	}
	animationSection.AddItem(mobileBlendSlider)

	worldSlider, blendEvents := eui.NewSlider()
	worldBlendSlider = worldSlider
	worldBlendSlider.Label = "World Animation Blend Duration"
	worldBlendSlider.MinValue = 0.1
	worldBlendSlider.MaxValue = 1.0
	worldBlendSlider.Value = float32(gs.BlendAmount)
	worldBlendSlider.Size = eui.Point{X: width - 10, Y: 24}
	worldBlendSlider.SetTooltip("Set how much of the animation interval is spent blending frames.")
	blendEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventSliderChanged {
			gs.BlendAmount = float64(ev.Value)
			settingsDirty = true
		}
	}
	animationSection.AddItem(worldBlendSlider)

	refreshShaderEffectControls()

	addPerformancePages(outer)
	return outer
}

func applyTiledWorkspaceLayout() {
	clampTiledLayoutSettings()
	if gs.TiledWindows && gs.ToolbarPlacement == ToolbarFloating {
		placeToolbar(ToolbarInInventory, true)
	}
	refreshToolbarPlacementControl()
	if gs.TiledWindows && gs.TiledLayout == TiledLayoutSide && !gs.MessagesToConsole {
		// The alternate workspace has one shared messages pane beneath its
		// top lists, so selecting it also combines chat and console output.
		gs.MessagesToConsole = true
	}
	if settingsCombineMessagesCB != nil {
		settingsCombineMessagesCB.Checked = gs.MessagesToConsole
		settingsCombineMessagesCB.Dirty = true
		if settingsWin != nil {
			settingsWin.Refresh()
		}
	}
	if gs.MessagesToConsole {
		if chatWin != nil {
			chatWin.Close()
		}
		gs.ChatWindow.Open = false
	} else {
		gs.ChatWindow.Open = true
		if chatWin == nil {
			_ = makeChatWindow()
		} else {
			chatWin.MarkOpen()
		}
	}
	applyManagedWindowLayout()
	if inventoryWin != nil {
		updateInventoryWindow()
	}
	if playersWin != nil {
		updatePlayersWindow()
	}
	if consoleWin != nil {
		updateConsoleWindow()
	}
	if chatWin != nil {
		updateChatWindow()
	}
	settingsDirty = true
}

func makeTileLayoutWindow() {
	if tileLayoutWin != nil {
		return
	}
	const width float32 = 310
	tileLayoutWin = eui.NewWindow()
	tileLayoutWin.ShowTooltipIndicators = true
	tileLayoutWin.Title = "Tiled Window Layout"
	tileLayoutWin.Closable = true
	tileLayoutWin.Resizable = false
	tileLayoutWin.AutoSize = true
	tileLayoutWin.Movable = true
	tileLayoutWin.SetZone(eui.HZoneCenterLeft, eui.VZoneMiddleTop)

	flow := eui.NewColumn()
	workspace := eui.NewSection("Workspace", width)
	arrangement := eui.NewSection("Arrangement", width)
	flow.AddItem(workspace)
	flow.AddItem(arrangement)

	tiledCB, tiledEvents := eui.NewCheckbox()
	tiledCB.Text = "Use tiled window layout"
	tiledCB.Size = eui.Point{X: width, Y: 24}
	tiledCB.Checked = gs.TiledWindows
	tiledCB.SetTooltip("Keep the main windows aligned as a workspace. Turn this off for freeform windows.")
	tiledEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			gs.TiledWindows = ev.Checked
			applyTiledWorkspaceLayout()
		}
	}
	workspace.AddItem(tiledCB)

	keepGameLargeCB, keepGameLargeEvents := eui.NewCheckbox()
	keepGameLargeCB.Text = "Keep game window large"
	keepGameLargeCB.Size = eui.Point{X: width, Y: 24}
	keepGameLargeCB.Checked = gs.TiledKeepGameLarge
	keepGameLargeCB.SetTooltip("Keep the centered game pane at its largest square size; drag a divider to reposition it.")
	keepGameLargeEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			gs.TiledKeepGameLarge = ev.Checked
			applyTiledWorkspaceLayout()
		}
	}
	workspace.AddItem(keepGameLargeCB)

	combineMessagesCB, combineMessagesEvents := eui.NewCheckbox()
	settingsCombineMessagesCB = combineMessagesCB
	combineMessagesCB.Text = "Combine chat + console"
	combineMessagesCB.Size = eui.Point{X: width, Y: 24}
	combineMessagesCB.Checked = gs.MessagesToConsole
	combineMessagesCB.SetTooltip("Show chat and console output together in the Console tile, and hide the separate Chat tile.")
	combineMessagesEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			gs.MessagesToConsole = ev.Checked
			applyTiledWorkspaceLayout()
		}
	}
	workspace.AddItem(combineMessagesCB)

	var gameSideDD *eui.ItemData
	layoutDD, layoutEvents := eui.NewDropdown()
	layoutDD.Label = "Layout"
	layoutDD.Options = []string{"Game centered", "Game on a side"}
	layoutDD.Selected = int(gs.TiledLayout)
	layoutDD.Size = eui.Point{X: width, Y: 24}
	layoutDD.SetTooltip("Centered uses two side columns; side puts the game beside a shared panel.")
	layoutEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventDropdownSelected {
			gs.TiledLayout = TiledLayout(ev.Index)
			applyTiledWorkspaceLayout()
			if gameSideDD != nil {
				gameSideDD.Disabled = gs.TiledLayout != TiledLayoutSide
				gameSideDD.Dirty = true
				tileLayoutWin.Refresh()
			}
		}
	}
	arrangement.AddItem(layoutDD)

	topDD, topEvents := eui.NewDropdown()
	topDD.Label = "Inventory / Players"
	topDD.Options = []string{"Inventory left, Players right", "Players left, Inventory right"}
	if !gs.TiledInventoryLeft {
		topDD.Selected = 1
	}
	topDD.Size = eui.Point{X: width, Y: 24}
	topDD.SetTooltip("Sets list order in the side columns or the upper shared panel.")
	topEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventDropdownSelected {
			gs.TiledInventoryLeft = ev.Index == 0
			applyTiledWorkspaceLayout()
		}
	}
	arrangement.AddItem(topDD)

	bottomDD, bottomEvents := eui.NewDropdown()
	bottomDD.Label = "Console / Chat"
	bottomDD.Options = []string{"Console left, Chat right", "Chat left, Console right"}
	if !gs.TiledConsoleLeft {
		bottomDD.Selected = 1
	}
	bottomDD.Size = eui.Point{X: width, Y: 24}
	bottomDD.SetTooltip("Sets message sides in centered layout; ignored when messages are combined.")
	bottomEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventDropdownSelected {
			gs.TiledConsoleLeft = ev.Index == 0
			applyTiledWorkspaceLayout()
		}
	}
	arrangement.AddItem(bottomDD)

	gameSideDD, gameSideEvents := eui.NewDropdown()
	gameSideDD.Label = "Alternate game side"
	gameSideDD.Options = []string{"Game left", "Game right"}
	if !gs.TiledGameLeft {
		gameSideDD.Selected = 1
	}
	gameSideDD.Disabled = gs.TiledLayout != TiledLayoutSide
	gameSideDD.Size = eui.Point{X: width, Y: 24}
	gameSideDD.SetTooltip("Chooses which edge holds the game when Game on a side is selected.")
	gameSideEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventDropdownSelected {
			gs.TiledGameLeft = ev.Index == 0
			applyTiledWorkspaceLayout()
		}
	}
	arrangement.AddItem(gameSideDD)

	tileLayoutWin.AddItem(flow)
	tileLayoutWin.AddWindow(false)
}

func makeNotificationsWindow() {
	if notificationsWin != nil {
		return
	}
	var width float32 = 250
	notificationsWin = eui.NewWindow()
	notificationsWin.ShowTooltipIndicators = true
	notificationsWin.Title = "Notification Settings"
	notificationsWin.Closable = true
	notificationsWin.Resizable = false
	notificationsWin.AutoSize = true
	notificationsWin.Movable = true
	notificationsWin.SetZone(eui.HZoneCenterLeft, eui.VZoneMiddleTop)

	flow := eui.NewColumn()
	eventsSection := eui.NewSection("Notify About", width)
	deliverySection := eui.NewSection("Display", width)
	flow.AddItem(eventsSection)
	flow.AddItem(deliverySection)

	addCB := func(section *eui.ItemData, label string, val *bool) {
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
		section.AddItem(cb)
	}

	// Background notifications while unfocused
	addCB(eventsSection, "Notify when in background", &gs.NotifyWhenBackground)
	addCB(eventsSection, "Fallen", &gs.NotifyFallen)
	addCB(eventsSection, "Not fallen", &gs.NotifyNotFallen)
	addCB(eventsSection, "Shares", &gs.NotifyShares)
	addCB(eventsSection, "Friend online", &gs.NotifyFriendOnline)
	addCB(eventsSection, "Text copied", &gs.NotifyCopyText)

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
	deliverySection.AddItem(durSlider)

	// Test desktop notification button
	testBtn, testEv := eui.NewButton()
	testBtn.Text = "Send Test Notification"
	setMaterialButtonIcon(testBtn, "notifications")
	testBtn.Size = eui.Point{X: width, Y: 24}
	testEv.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			notifyDesktop("goThoom", "Background notifications test")
		}
	}
	deliverySection.AddItem(testBtn)

	notificationsWin.AddItem(flow)
	notificationsWin.AddWindow(false)
}

func makeBubbleWindow() {
	if bubbleWin != nil {
		return
	}
	var width float32 = 250
	bubbleWin = eui.NewWindow()
	bubbleWin.ShowTooltipIndicators = true
	bubbleWin.Title = "Bubble Settings"
	bubbleWin.Closable = true
	bubbleWin.Resizable = false
	bubbleWin.AutoSize = true
	bubbleWin.Movable = true
	bubbleWin.SetZone(eui.HZoneCenterLeft, eui.VZoneMiddleTop)

	flow := eui.NewColumn()
	displaySection := eui.NewSection("Display", width)
	bubbleTypesSection := eui.NewSection("Message Types", width)
	bubbleSourcesSection := eui.NewSection("Show For", width)
	flow.AddItem(displaySection)
	flow.AddItem(bubbleTypesSection)
	flow.AddItem(bubbleSourcesSection)

	// Quick toggle for message bubbles in Chat & Audio
	bubblesQuickCB, bubblesQuickEvents := eui.NewCheckbox()
	bubblesQuickCB.Text = "Message Bubbles"
	bubblesQuickCB.Size = eui.Point{X: width, Y: 24}
	bubblesQuickCB.Checked = gs.SpeechBubbles
	bubblesQuickEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			gs.SpeechBubbles = ev.Checked
			settingsDirty = true
		}
	}
	displaySection.AddItem(bubblesQuickCB)

	animatedBubblesCB, animatedBubblesEvents := eui.NewCheckbox()
	animatedBubblesCB.Text = "Animated Chat Bubbles"
	animatedBubblesCB.Size = eui.Point{X: width, Y: 24}
	animatedBubblesCB.Checked = gs.AnimatedChatBubbles
	animatedBubblesEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			gs.AnimatedChatBubbles = ev.Checked
			settingsDirty = true
		}
	}
	displaySection.AddItem(animatedBubblesCB)

	avoidBubbleOverlapCB, avoidBubbleOverlapEvents := eui.NewCheckbox()
	avoidBubbleOverlapCB.Text = "Prevent Bubble Overlap"
	avoidBubbleOverlapCB.Size = eui.Point{X: width, Y: 24}
	avoidBubbleOverlapCB.Checked = gs.AvoidBubbleOverlap
	avoidBubbleOverlapCB.SetTooltip("Move crowded chat bubbles apart while keeping their arrows anchored to the speaker.")
	avoidBubbleOverlapEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			gs.AvoidBubbleOverlap = ev.Checked
			settingsDirty = true
		}
	}
	displaySection.AddItem(avoidBubbleOverlapCB)

	addBubbleCB := func(section *eui.ItemData, label string, val *bool) {
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
		section.AddItem(cb)
	}

	addBubbleCB(bubbleTypesSection, "Normal", &gs.BubbleNormal)
	addBubbleCB(bubbleTypesSection, "Whisper", &gs.BubbleWhisper)
	addBubbleCB(bubbleTypesSection, "Yell", &gs.BubbleYell)
	addBubbleCB(bubbleTypesSection, "Thought", &gs.BubbleThought)
	addBubbleCB(bubbleTypesSection, "Real Action", &gs.BubbleRealAction)
	addBubbleCB(bubbleTypesSection, "Monster", &gs.BubbleMonster)
	addBubbleCB(bubbleTypesSection, "Player Action", &gs.BubblePlayerAction)
	addBubbleCB(bubbleTypesSection, "Ponder", &gs.BubblePonder)
	addBubbleCB(bubbleTypesSection, "Narrate", &gs.BubbleNarrate)
	addBubbleCB(bubbleSourcesSection, "Self", &gs.BubbleSelf)
	addBubbleCB(bubbleSourcesSection, "Other Players", &gs.BubbleOtherPlayers)
	addBubbleCB(bubbleSourcesSection, "Monsters", &gs.BubbleMonsters)
	addBubbleCB(bubbleSourcesSection, "Narration", &gs.BubbleNarration)

	bubbleWin.AddItem(flow)
	bubbleWin.AddWindow(false)
}

func makeDebugWindow() {
	if debugWin != nil {
		return
	}

	var width float32 = 250
	debugWin = eui.NewWindow()
	debugWin.ShowTooltipIndicators = true
	debugWin.Title = "Debug Settings"
	debugWin.Closable = true
	debugWin.Resizable = false
	debugWin.AutoSize = true
	debugWin.Movable = true
	debugWin.SetZone(eui.HZoneCenterLeft, eui.VZoneMiddleTop)

	debugFlow := eui.NewColumn()
	diagnosticsSection := eui.NewSection("Diagnostics", width)
	sceneSection := eui.NewSection("Scene Overrides", width)
	shaderSection := eui.NewSection("Shader Tools", width)
	debugFlow.AddItem(diagnosticsSection)
	debugFlow.AddItem(sceneSection)
	debugFlow.AddItem(shaderSection)

	recordStatsCB, recordStatsEvents := eui.NewCheckbox()
	recordStatsCB.Text = "Record Asset Stats"
	recordStatsCB.Size = eui.Point{X: width, Y: 24}
	recordStatsCB.Checked = gs.recordAssetStats
	recordStatsCB.SetTooltip("Write image counts to stats.json.")
	recordStatsEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			gs.recordAssetStats = ev.Checked
			settingsDirty = true
		}
	}
	diagnosticsSection.AddItem(recordStatsCB)

	hideMoveCB, hideMoveEvents := eui.NewCheckbox()
	hideMoveCB.Text = "Hide Moving Objects"
	hideMoveCB.Size = eui.Point{X: width, Y: 24}
	hideMoveCB.Checked = gs.hideMoving
	hideMoveEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			gs.hideMoving = ev.Checked
			settingsDirty = true
		}
	}
	sceneSection.AddItem(hideMoveCB)

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
	sceneSection.AddItem(hideMobCB)

	planesCB, planesEvents := eui.NewCheckbox()
	planesCB.Text = "Show image planes"
	planesCB.SetTooltip("Show sprite layer numbers.")
	planesCB.Size = eui.Point{X: width, Y: 24}
	planesCB.Checked = gs.imgPlanesDebug
	planesEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			gs.imgPlanesDebug = ev.Checked
			settingsDirty = true
		}
	}
	diagnosticsSection.AddItem(planesCB)

	pictIDCB, pictIDEvents := eui.NewCheckbox()
	pictIDCB.Text = "Show picture IDs"
	pictIDCB.Size = eui.Point{X: width, Y: 24}
	pictIDCB.Checked = gs.pictIDDebug
	pictIDEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			gs.pictIDDebug = ev.Checked
			settingsDirty = true
		}
	}
	diagnosticsSection.AddItem(pictIDCB)

	// Add a small "Reload" button beside the shader checkbox for hot-reload.
	reloadBtn, reloadEv := eui.NewButton()
	reloadBtn.Text = "Reload Shaders"
	setMaterialButtonIcon(reloadBtn, "restart_alt")
	reloadBtn.Size = eui.Point{X: 160, Y: 24}
	reloadEv.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			if err := ReloadLightingShader(); err != nil {
				consoleMessage("Shader reload failed:" + err.Error())
			} else if err := ReloadReplacementEffectsShader(); err != nil {
				consoleMessage("Shader reload failed:" + err.Error())
			} else {
				consoleMessage("Shaders reloaded.")
			}
		}
	}

	shaderRow := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_HORIZONTAL, Fixed: true}
	shaderRow.AddItem(reloadBtn)
	shaderSection.AddItem(shaderRow)

	previewEffectsBtn, previewEffectsEvents := eui.NewButton()
	previewEffectsBtn.Text = "Toggle Effects Preview"
	setMaterialButtonIcon(previewEffectsBtn, "visibility")
	previewEffectsBtn.Size = eui.Point{X: width, Y: 24}
	previewEffectsBtn.SetTooltip("Preview every replacement effect.")
	previewEffectsEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			replacementEffectsPreview = !replacementEffectsPreview
		}
	}
	shaderSection.AddItem(previewEffectsBtn)

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
	sceneSection.AddItem(forceNightDD)

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
	sceneSection.AddItem(smoothinCB)
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
	sceneSection.AddItem(pictAgainCB)

	debugWin.AddItem(debugFlow)

	debugWin.AddWindow(false)
}

// updateDebugStats refreshes the dedicated cache statistics window.
func updateDebugStats() {
	if cacheStatsWin == nil || !cacheStatsWin.IsOpen() {
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
		scaledMobileCacheLabel.Text = fmt.Sprintf("Upscaled Mobile Frames/Masks: %d (%s)", stats.scaledMobileCount, humanize.Bytes(uint64(stats.scaledMobileBytes)))
		scaledMobileCacheLabel.Dirty = true
	}
	if spriteSlotCacheLabel != nil {
		spriteSlotCacheLabel.Text = fmt.Sprintf("Sprite slots: %d (%s)\nLive: %s | Spare: %s\nReuses: %d | Evicted IDs: %d\nFirst IDs: %d | Reloads: %d\nGame frames: %d", stats.slotCount, humanize.Bytes(uint64(stats.slotBytes)), humanize.Bytes(uint64(stats.slotUsedBytes)), humanize.Bytes(uint64(stats.slotBytes-stats.slotUsedBytes)), stats.slotReuses, stats.slotEvictions, stats.slotLoads, stats.slotReloads, stats.spriteGameFrames)
		spriteSlotCacheLabel.Dirty = true
	}
	if renderPoolCacheLabel != nil {
		textPool, bodyPool, uiPool := bubbleTextTargets.Stats(), bubbleBodyTargets.Stats(), eui.RenderTargetPoolStats()
		bubbleBytes := textPool.ActiveBytes + textPool.FreeBytes + bodyPool.ActiveBytes + bodyPool.FreeBytes
		uiBytes := uiPool.ActiveBytes + uiPool.FreeBytes
		renderPoolCacheLabel.Text = fmt.Sprintf("Bubble pool: %s | UI pool: %s\nReuses: %d bubbles, %d UI\nSpare slots: %d bubbles, %d UI", humanize.Bytes(uint64(bubbleBytes)), humanize.Bytes(uint64(uiBytes)), textPool.Reuses+bodyPool.Reuses, uiPool.Reuses, textPool.Free+bodyPool.Free, uiPool.Free)
		renderPoolCacheLabel.Dirty = true
	}
	if soundCacheLabel != nil {
		soundCacheLabel.Text = fmt.Sprintf("Sounds: %d (%s)", soundCount, humanize.Bytes(uint64(soundBytes)))
		soundCacheLabel.Dirty = true
	}
	if totalCacheLabel != nil {
		total := stats.totalBytes() + int64(soundBytes)
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

	flow := eui.NewColumn()

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

	resetBtn, resetEvents := eui.NewButton()
	resetBtn.Text = "Reset Windows"
	setMaterialButtonIcon(resetBtn, "restart_alt")
	resetBtn.Size = eui.Point{X: 128, Y: 24}
	resetBtn.SetTooltip("Restore the default window layout.")
	resetEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			confirmResetWindows()
		}
	}
	flow.AddItem(resetBtn)

	windowsWin.AddItem(flow)
	windowsWin.AddWindow(false)

}

func makePlayersWindow() {
	if playersWin != nil {
		return
	}
	cachedPlayerRows = map[string]cachedPlayerRow{}
	cachedPlayerHeaders = map[string]cachedPlayerHeader{}
	playerArtworkViewport.valid = false
	renderedPlayerSelection = ""
	// Use the common text window scaffold to get an inner scrollable list
	// and consistent padding/behavior with Inventory/Chat windows.
	playersWin, playersList, _ = eui.NewTextWindow("Players", eui.HZoneRight, eui.VZoneTop, false)
	playersWin.Searchable = true
	playersWin.OnSearch = searchPlayersWindow
	playersWin.OnOpen = updatePlayersWindow
	// Refresh contents on resize so word-wrapping and row sizing stay correct.
	playersWin.OnResize = func() { updatePlayersWindow() }
	updatePlayersWindow()
}
