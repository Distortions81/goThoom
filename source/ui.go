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

	text "github.com/hajimehoshi/ebiten/v2/text/v2"
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
var advancedWin *eui.WindowData
var tileLayoutWin *eui.WindowData
var settingsToolbarPlacementDD *eui.ItemData
var settingsCombineMessagesCB *eui.ItemData
var connectWin *eui.WindowData
var connectStatusText *eui.ItemData
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

// newConfigurationSection keeps configuration windows visually consistent.
// Each section owns its controls, with enough space above the heading to make
// neighboring groups easy to scan without adding decorative UI machinery.
func newConfigurationSection(title string, width float32) *eui.ItemData {
	section := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL}
	section.Size = eui.Point{X: width, Y: 10}

	spacer, _ := eui.NewText()
	spacer.Size = eui.Point{X: width, Y: 10}
	section.AddItem(spacer)

	heading, _ := eui.NewText()
	heading.Text = title
	heading.FontSize = 15
	heading.Size = eui.Point{X: width, Y: 30}
	applyBoldFace(heading)
	section.AddItem(heading)
	return section
}

func newConfigurationSubheading(title string, width float32) *eui.ItemData {
	heading, _ := eui.NewText()
	heading.Text = title
	heading.FontSize = 12
	heading.Size = eui.Point{X: width, Y: 24}
	applyBoldFace(heading)
	return heading
}

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
)

var (
	sheetCacheLabel        *eui.ItemData
	frameCacheLabel        *eui.ItemData
	scaledFrameCacheLabel  *eui.ItemData
	mobileCacheLabel       *eui.ItemData
	scaledMobileCacheLabel *eui.ItemData
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
	soundEnhanceCB           *eui.ItemData
	musicEnhanceCB           *eui.ItemData
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

	row1 = &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_HORIZONTAL}
	row2 = &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_HORIZONTAL}
	menu = &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL}

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
	shotBtn.Text = "Snapshot"
	shotBtn.SetTooltip("Save the visible game view as a PNG in the user data folder's Screenshots directory.")
	shotBtn.Size = eui.Point{X: buttonWidth, Y: buttonHeight}
	shotBtn.FontSize = toolFontSize
	shotEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			takeScreenshot()
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
	scriptsButtons = buttonsBottom
	root.AddItem(buttonsBottom)

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
	root := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL}
	newScriptWin.AddItem(root)
	intro, _ := eui.NewText()
	intro.Text = "Choose a starting point:"
	intro.Size = eui.Point{X: 360, Y: 24}
	root.AddItem(intro)
	for _, template := range newScriptTemplates {
		template := template
		row := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_HORIZONTAL}
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
		row := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_HORIZONTAL}
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
			row := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_HORIZONTAL}
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
	scriptsButtons.Size = eui.Point{X: clientW, Y: 24}
	scriptsList.Size = eui.Point{X: clientW, Y: max(float32(24), clientH-scriptsManagerRowHeight-24)}
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
	scriptInfoWin = showPopup("Script Info", "", []popupButton{{Text: "Close", Action: func() { scriptInfoWin = nil }}}, scriptDetails)
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

	actions := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_HORIZONTAL}
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
		t.Size = eui.Point{X: 400, Y: 16}
		scriptDebugList.AddItem(t)
	}
	if debugWin != nil {
		debugWin.Refresh()
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
// quality preset or the detailed Settings window changes these values.
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
		if soundEnhanceCB != nil {
			soundEnhanceCB.Checked = ev.Checked
			soundEnhanceCB.Dirty = true
		}
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
		if musicEnhanceCB != nil {
			musicEnhanceCB.Checked = ev.Checked
			musicEnhanceCB.Dirty = true
		}
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
	muteUnfocusCB.SetTooltip("Mute audio when unfocused.")
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
	var buttonWidth float32 = 80
	if docked {
		buttonWidth = 68
	}

	controls := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_HORIZONTAL}
	if hands := toolbarHandsSource(); hands != nil {
		w, h := hands.Bounds().Dx(), hands.Bounds().Dy()
		handsRow := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_HORIZONTAL}
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
	bounds := toolbarHandsImage.Bounds()
	middle := bounds.Dx() / 2
	leftHand := toolbarHandsImage.SubImage(image.Rect(0, 0, middle, bounds.Dy())).(*ebiten.Image)
	rightHand := toolbarHandsImage.SubImage(image.Rect(middle, 0, bounds.Dx(), bounds.Dy())).(*ebiten.Image)
	rightID, leftID := equippedItemPicts()
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
	refreshToolbar()
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

	btnFlow := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_HORIZONTAL}
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
	charWinWidth          = 500
	freeDemoCharacterName = "-Demo Character-"
	freeDemoSelection     = "\x00free-demo"
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
		radio.Size = eui.Point{X: 374, Y: 48}
		radio.FontSize = 20
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
	editCharWin.Title = "Edit Character"
	editCharWin.Closable = false
	editCharWin.Resizable = false
	editCharWin.AutoSize = true
	editCharWin.Movable = true

	flow := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL}

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
	rememberCB.SetTooltip("Store this character's password for future logins.")
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

	btnFlow := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_HORIZONTAL}
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
		demoLoginActive = true
		loginMu.Unlock()
		dispatchMainThread(func() { startLoginWithDemoCandidates(demoCandidates) })
	}()
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
	centerLoginWindow()
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
	setMaterialButtonIcon(connBtn, "login")
	connBtn.Size = eui.Point{X: charWinWidth, Y: 48}
	connBtn.Padding = 10
	connBtn.FontSize = 24
	connBtn.Outlined = true
	connBtn.Border = 2
	connBtn.OutlineColor = eui.ColorGreen
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
	addBtn.Size = eui.Point{X: 164, Y: 24}
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
	editBtn.Size = eui.Point{X: 164, Y: 24}
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
	deleteBtn.Size = eui.Point{X: 164, Y: 24}
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

	characterActions := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_HORIZONTAL}
	characterActions.Size = eui.Point{X: charWinWidth, Y: 24}
	characterActions.AddItem(addBtn)
	characterActions.AddItem(editBtn)
	characterActions.AddItem(deleteBtn)

	openBtn, openEvents := eui.NewButton()
	openBtn.Text = "Play movie file [clMov]"
	setMaterialButtonIcon(openBtn, "movie")
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
	// Increase Quit button font size by 2pt
	quitBttn.FontSize = 24
	// Double the height of the Quit button
	quitBttn.Size = eui.Point{X: charWinWidth, Y: 48}
	quitBttn.Outlined = true
	quitBttn.Border = 2
	quitBttn.OutlineColor = eui.ColorRed
	quitEvn.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			confirmQuit()
		}
	}

	verFlow := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_HORIZONTAL, Size: eui.Point{X: 260, Y: 24}}
	verLabel, _ := eui.NewText()
	verLabel.Text = fmt.Sprintf("goThoom test %4d", appVersion)
	verLabel.FontSize = 14
	verLabel.Size = eui.Point{X: 330, Y: 24}
	verFlow.AddItem(verLabel)

	changeBtn, changeEvents := eui.NewButton()
	changeBtn.Text = "Changelog"
	setMaterialButtonIcon(changeBtn, "history")
	changeBtn.SetTooltip("View recent changes")
	changeBtn.Size = eui.Point{X: 97, Y: 24}
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
	setMaterialButtonIcon(aboutBtn, "info")
	aboutBtn.Size = eui.Point{X: 60, Y: 24}
	aboutBtn.FontSize = 10
	aboutEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			openAboutWindow(ev.Item)
		}
	}
	verFlow.AddItem(aboutBtn)

	addLoginSpacer := func(height float32) {
		spacer, _ := eui.NewText()
		spacer.Size = eui.Point{X: charWinWidth, Y: height}
		loginFlow.AddItem(spacer)
	}

	loginFlow.AddItem(quitBttn)
	addLoginSpacer(8)
	loginFlow.AddItem(openBtn)
	addLoginSpacer(10)
	characterEditLabel, _ := eui.NewText()
	characterEditLabel.Text = "Edit Characters:"
	characterEditLabel.FontSize = 15
	characterEditLabel.Size = eui.Point{X: charWinWidth, Y: 25}
	loginFlow.AddItem(characterEditLabel)
	loginFlow.AddItem(characterActions)
	addLoginSpacer(12)
	characterListLabel, _ := eui.NewText()
	characterListLabel.Text = "Character list:"
	characterListLabel.FontSize = 15
	characterListLabel.Size = eui.Point{X: charWinWidth, Y: 25}
	loginFlow.AddItem(characterListLabel)
	loginFlow.AddItem(charactersList)
	addLoginSpacer(10)
	loginFlow.AddItem(connBtn)
	addLoginSpacer(8)
	loginFlow.AddItem(verFlow)

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
	updateTextWindow(changelogWin, changelogList, nil, lines, 14, "", monoFaceSource, false)
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
	settingsWin.SetRefreshInterval(100 * time.Millisecond)

	// Use four balanced panes so the window fits comfortably on a 1920x1080
	// desktop without making the user scroll through one overly tall column.
	var panelWidth float32 = 270
	outer := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_HORIZONTAL}
	left := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL}
	left.Size = eui.Point{X: panelWidth, Y: 10}
	centerLeft := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL}
	centerLeft.Size = eui.Point{X: panelWidth, Y: 10}
	centerRight := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL}
	centerRight.Size = eui.Point{X: panelWidth, Y: 10}
	right := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL}
	right.Size = eui.Point{X: panelWidth, Y: 10}

	windowSection := newConfigurationSection("Window & Display", panelWidth)
	appearanceSection := newConfigurationSection("Appearance", panelWidth)
	controlsSection := newConfigurationSection("Controls", panelWidth)
	qualitySection := newConfigurationSection("Graphics Quality", panelWidth)
	left.AddItem(windowSection)
	left.AddItem(appearanceSection)
	left.AddItem(controlsSection)
	left.AddItem(qualitySection)

	gettingStartedSection := newConfigurationSection("Getting Started", panelWidth)
	textSizeSection := newConfigurationSection("Text Sizes", panelWidth)
	chatSection := newConfigurationSection("Chat & Messages", panelWidth)
	centerLeft.AddItem(gettingStartedSection)
	centerLeft.AddItem(textSizeSection)
	centerLeft.AddItem(chatSection)

	statusSection := newConfigurationSection("Status Bars", panelWidth)
	visibilitySection := newConfigurationSection("World Visibility", panelWidth)
	nameSection := newConfigurationSection("Character Names", panelWidth)
	centerRight.AddItem(statusSection)
	centerRight.AddItem(visibilitySection)
	centerRight.AddItem(nameSection)

	bubbleSection := newConfigurationSection("Speech Bubbles", panelWidth)
	notificationsSection := newConfigurationSection("Notifications", panelWidth)
	ttsSection := newConfigurationSection("Text to Speech", panelWidth)
	moreSection := newConfigurationSection("More Settings", panelWidth)
	right.AddItem(bubbleSection)
	right.AddItem(notificationsSection)
	right.AddItem(ttsSection)
	right.AddItem(moreSection)

	resetWindowsBtn, resetWindowsEvents := eui.NewButton()
	resetWindowsBtn.Text = "Reset Windows"
	setMaterialButtonIcon(resetWindowsBtn, "restart_alt")
	resetWindowsBtn.Size = eui.Point{X: panelWidth, Y: 24}
	resetWindowsBtn.SetTooltip("Restore the default window layout.")
	resetWindowsEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			confirmResetWindows()
		}
	}

	tiledModeCB, tiledModeEvents := eui.NewCheckbox()
	tiledModeCB.Text = "Tiled window mode"
	tiledModeCB.Size = eui.Point{X: panelWidth, Y: 24}
	tiledModeCB.Checked = gs.TiledWindows
	tiledModeCB.SetTooltip("Arrange the Game, Inventory, Players, Console, and Chat windows as one tiled workspace.")
	tiledModeEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			gs.TiledWindows = ev.Checked
			applyTiledWorkspaceLayout()
		}
	}
	windowSection.AddItem(tiledModeCB)

	tiledLayoutBtn, tiledLayoutEvents := eui.NewButton()
	tiledLayoutBtn.Text = "Tiled Layout"
	setMaterialButtonIcon(tiledLayoutBtn, "dashboard_customize")
	tiledLayoutBtn.Size = eui.Point{X: (panelWidth - 8) / 2, Y: 24}
	tiledLayoutBtn.SetTooltip("Open panel ordering, game placement, and combined-message controls.")
	tiledLayoutEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			makeTileLayoutWindow()
			tileLayoutWin.ToggleNear(ev.Item)
		}
	}

	toolbarPlacementDD, toolbarPlacementEvents := eui.NewDropdown()
	settingsToolbarPlacementDD = toolbarPlacementDD
	toolbarPlacementDD.Label = "Toolbar Placement"
	toolbarPlacementDD.Options = []string{"Inside Inventory", "Inside Players"}
	if !gs.TiledWindows {
		toolbarPlacementDD.Options = append(toolbarPlacementDD.Options, "Floating Window")
	}
	toolbarPlacementDD.Selected = int(gs.ToolbarPlacement)
	toolbarPlacementDD.Size = eui.Point{X: panelWidth, Y: 24}
	toolbarPlacementDD.SetTooltip("Dock in Inventory or Players. Floating is unavailable while tiled mode is on.")
	toolbarPlacementEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventDropdownSelected {
			placeToolbar(ToolbarPlacement(ev.Index), true)
		}
	}
	windowSection.AddItem(toolbarPlacementDD)

	toolbarInfoCB, toolbarInfoEvents := eui.NewCheckbox()
	toolbarInfoCB.Text = "Toolbar Info"
	toolbarInfoCB.Size = eui.Point{X: (panelWidth - 8) / 2, Y: 24}
	toolbarInfoCB.Checked = gs.ToolbarInfoBar
	toolbarInfoCB.SetTooltip("Show network and performance stats.")
	toolbarInfoEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			gs.ToolbarInfoBar = ev.Checked
			placeToolbar(gs.ToolbarPlacement, true)
		}
	}
	layoutToolsRow := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_HORIZONTAL}
	layoutToolsRow.AddItem(tiledLayoutBtn)
	layoutToolsRow.AddItem(toolbarInfoCB)
	windowSection.AddItem(layoutToolsRow)

	// UI scale is always available: users need a direct recovery path if a
	// display's DPI report is unusual. Retina/HiDPI scaling is applied on top
	// of this preference automatically.
	uiScaleSlider, uiScaleEvents := eui.NewSlider()
	uiScaleSlider.Label = "UI Scale"
	uiScaleSlider.MinValue = 0.75
	uiScaleSlider.MaxValue = 4
	uiScaleSlider.Value = float32(gs.UIScale)
	uiScaleSlider.SetTooltip("Base UI size. Retina and other HiDPI displays are scaled automatically.")
	pendingUIScale := gs.UIScale
	uiScaleEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventSliderChanged {
			pendingUIScale = float64(ev.Value)
		}
	}

	uiScaleApplyBtn, uiScaleApplyEvents := eui.NewButton()
	uiScaleApplyBtn.Text = "Apply"
	setMaterialButtonIcon(uiScaleApplyBtn, "check_circle")
	uiScaleApplyBtn.Size = eui.Point{X: 64, Y: 24}
	uiScaleApplyEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			gs.UIScale = pendingUIScale
			eui.SetUserUIScale(float32(gs.UIScale))
			updateGameWindowSize()
			settingsDirty = true
		}
	}

	// Place the slider and button on the same row.
	uiScaleRow := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_HORIZONTAL}
	uiScaleSlider.Size = eui.Point{X: panelWidth - uiScaleApplyBtn.Size.X - 10, Y: 24}
	uiScaleRow.AddItem(uiScaleSlider)
	uiScaleRow.AddItem(uiScaleApplyBtn)
	windowSection.AddItem(uiScaleRow)

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
	windowSection.AddItem(fullscreenCB)

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
				refreshMessageTextWindows()
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

	appearanceSection.AddItem(themeDD)
	appearanceSection.AddItem(styleDD)
	accLabel, _ := eui.NewText()
	accLabel.Text = "Accent Color"
	accLabel.FontSize = 12
	accLabel.Size = eui.Point{X: panelWidth, Y: 20}
	appearanceSection.AddItem(accLabel)
	appearanceSection.AddItem(accentWheel)

	toggle, toggleEvents := eui.NewCheckbox()
	toggle.Text = "Click-to-toggle movement"
	toggle.Size = eui.Point{X: panelWidth, Y: 24}
	toggle.Checked = gs.ClickToToggle
	toggle.SetTooltip("Click once to keep walking toward the pointer; click again to stop.")
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
	controlsSection.AddItem(toggle)

	qualityPresetDD, qpEvents := eui.NewDropdown()
	qualityPresetDD.Options = []string{"Lowest", "Low", "Medium", "High", "Ultra", "Custom"}
	qualityPresetDD.Size = eui.Point{X: panelWidth, Y: 24}
	qualityPresetDD.Selected = detectQualityPreset()
	qualityPresetDD.FontSize = 12
	qualityPresetDD.SetTooltip("Choose one of five cumulative quality tiers; Custom preserves individual choices.")
	qpEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventDropdownSelected {
			switch ev.Index {
			case 0:
				applyQualityPreset("Lowest")
			case 1:
				applyQualityPreset("Low")
			case 2:
				applyQualityPreset("Medium")
			case 3:
				applyQualityPreset("High")
			case 4:
				applyQualityPreset("Ultra")
			}
			qualityPresetDD.Selected = detectQualityPreset()
		}
	}
	qualitySection.AddItem(qualityPresetDD)

	qualityBtn, qualityEvents := eui.NewButton()
	qualityBtn.Text = "Quality Settings"
	setMaterialButtonIcon(qualityBtn, "tune")
	qualityBtn.SetTooltip("Tune artwork scaling, shadows, lighting, motion, and GPU costs individually.")
	qualityBtn.Size = eui.Point{X: panelWidth, Y: 24}
	qualityEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			SettingsLock.Lock()
			defer SettingsLock.Unlock()

			qualityWin.ToggleNear(ev.Item)
		}
	}
	qualitySection.AddItem(qualityBtn)

	inputOpenCB, inputOpenEvents := eui.NewCheckbox()
	inputOpenCB.Text = "Input bar always open"
	inputOpenCB.Size = eui.Point{X: panelWidth, Y: 24}
	inputOpenCB.Checked = gs.InputBarAlwaysOpen
	inputOpenCB.SetTooltip("Close for WASD and extra hotkeys.")
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
	controlsSection.AddItem(inputOpenCB)

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
	chatSection.AddItem(chatTSCB)

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
	chatSection.AddItem(consoleTSCB)

	notifCB, notifEvents := eui.NewCheckbox()
	notifCB.Text = "Game Notifications"
	notifCB.Size = eui.Point{X: panelWidth, Y: 24}
	notifCB.Checked = gs.Notifications
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
	notificationsSection.AddItem(notifCB)

	notifBtn, notifBtnEvents := eui.NewButton()
	notifBtn.Text = "Notification Settings"
	setMaterialButtonIcon(notifBtn, "notifications")
	notifBtn.Size = eui.Point{X: panelWidth, Y: 24}
	notifBtnEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			SettingsLock.Lock()
			defer SettingsLock.Unlock()

			notificationsWin.ToggleNear(ev.Item)
		}
	}
	notificationsSection.AddItem(notifBtn)

	textColorsBtn, textColorsEvents := eui.NewButton()
	textColorsBtn.Text = "Text Colors"
	setMaterialButtonIcon(textColorsBtn, "palette")
	textColorsBtn.Size = eui.Point{X: panelWidth, Y: 24}
	textColorsEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			makeTextColorsWindow()
			textColorsWin.ToggleNear(ev.Item)
		}
	}
	chatSection.AddItem(textColorsBtn)

	alternateRowsCB, alternateRowsEvents := eui.NewCheckbox()
	alternateRowsCB.Text = "Alternate row backgrounds"
	alternateRowsCB.Size = eui.Point{X: panelWidth, Y: 24}
	alternateRowsCB.Checked = gs.AlternateRowBackgrounds
	alternateRowsEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type != eui.EventCheckboxChanged {
			return
		}
		SettingsLock.Lock()
		gs.AlternateRowBackgrounds = ev.Checked
		SettingsLock.Unlock()
		settingsDirty = true
		updateConsoleWindow()
		updateChatWindow()
		updateInventoryWindow()
		updatePlayersWindow()
	}
	appearanceSection.AddItem(alternateRowsCB)

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
		statusSection.AddItem(radio)
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
	statusSection.AddItem(barColorCB)

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
	visibilitySection.AddItem(maxNightSlider)

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
	nameSection.AddItem(nameBgSlider)

	darkBubblesAndNamesCB, darkBubblesAndNamesEvents := eui.NewCheckbox()
	darkBubblesAndNamesCB.Text = "Dark Mode Names/Bubbles"
	darkBubblesAndNamesCB.Size = eui.Point{X: panelWidth - 10, Y: 24}
	darkBubblesAndNamesCB.Checked = gs.DarkBubblesAndNames
	darkBubblesAndNamesCB.SetTooltip("Uses dark backgrounds and light text for speech bubbles and name tags.")
	darkBubblesAndNamesEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			SettingsLock.Lock()
			defer SettingsLock.Unlock()

			gs.DarkBubblesAndNames = ev.Checked
			killNameTagCache()
			settingsDirty = true
		}
	}
	bubbleSection.AddItem(darkBubblesAndNamesCB)

	nameBorderCB, nameBorderEvents := eui.NewCheckbox()
	nameBorderCB.Text = "Name Tag Label Colors"
	nameBorderCB.Size = eui.Point{X: panelWidth - 10, Y: 24}
	nameBorderCB.Checked = gs.NameTagLabelColors
	nameBorderCB.SetTooltip("Color name-tag borders by label.")
	nameBorderEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			SettingsLock.Lock()
			defer SettingsLock.Unlock()

			gs.NameTagLabelColors = ev.Checked
			killNameTagCache()
			settingsDirty = true
		}
	}
	nameSection.AddItem(nameBorderCB)

	healthBarStyleDD, healthBarStyleEvents := eui.NewDropdown()
	healthBarStyleDD.Label = "Player Health Display"
	healthBarStyleDD.Options = []string{"Color bar", "Classic name color"}
	healthBarStyleDD.Selected = 0
	if !gs.NameHealthBarModern {
		healthBarStyleDD.Selected = 1
	}
	healthBarStyleDD.Size = eui.Point{X: panelWidth - 10, Y: 24}
	healthBarStyleDD.SetTooltip("Color bar keeps names stable; Classic changes name color as health falls.")
	healthBarStyleEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventDropdownSelected {
			SettingsLock.Lock()
			defer SettingsLock.Unlock()

			gs.NameHealthBarModern = ev.Index == 0
			killNameTagCache()
			settingsDirty = true
		}
	}
	nameSection.AddItem(healthBarStyleDD)

	healthBarPositionDD, healthBarPositionEvents := eui.NewDropdown()
	healthBarPositionDD.Label = "Color Bar Position"
	healthBarPositionDD.Options = []string{"Above name", "Below name"}
	healthBarPositionDD.Selected = 0
	if !gs.NameHealthBarAbove {
		healthBarPositionDD.Selected = 1
	}
	healthBarPositionDD.Size = eui.Point{X: panelWidth - 10, Y: 24}
	healthBarPositionEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventDropdownSelected {
			SettingsLock.Lock()
			defer SettingsLock.Unlock()

			gs.NameHealthBarAbove = ev.Index == 0
			killNameTagCache()
			settingsDirty = true
		}
	}
	nameSection.AddItem(healthBarPositionDD)

	healthBarThicknessSlider, healthBarThicknessEvents := eui.NewSlider()
	healthBarThicknessSlider.Label = "Modern Bar Thickness"
	healthBarThicknessSlider.MinValue = 1
	healthBarThicknessSlider.MaxValue = 8
	healthBarThicknessSlider.IntOnly = true
	healthBarThicknessSlider.Value = float32(gs.NameHealthBarThickness)
	healthBarThicknessSlider.Size = eui.Point{X: panelWidth - 10, Y: 24}
	healthBarThicknessEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventSliderChanged {
			SettingsLock.Lock()
			defer SettingsLock.Unlock()

			gs.NameHealthBarThickness = int(ev.Value)
			killNameTagCache()
			settingsDirty = true
		}
	}
	nameSection.AddItem(healthBarThicknessSlider)

	hideSelfNameCB, hideSelfNameEvents := eui.NewCheckbox()
	hideSelfNameCB.Text = "Hide My Name Tag"
	hideSelfNameCB.Size = eui.Point{X: panelWidth - 10, Y: 24}
	hideSelfNameCB.Checked = gs.HideSelfNameTag
	hideSelfNameEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			SettingsLock.Lock()
			defer SettingsLock.Unlock()

			gs.HideSelfNameTag = ev.Checked
			killNameTagCache()
			settingsDirty = true
		}
	}
	nameSection.AddItem(hideSelfNameCB)

	// Name-tags hover-only toggle
	nameHoverCB, nameHoverEvents := eui.NewCheckbox()
	nameHoverCB.Text = "Show name-tags only on hover"
	nameHoverCB.Size = eui.Point{X: panelWidth - 10, Y: 24}
	nameHoverCB.Checked = gs.NameTagsOnHoverOnly
	nameHoverEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			SettingsLock.Lock()
			defer SettingsLock.Unlock()

			gs.NameTagsOnHoverOnly = ev.Checked
			clearNameTagHoverReveals()
			settingsDirty = true
		}
	}
	nameSection.AddItem(nameHoverCB)

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
	bubbleSection.AddItem(bubbleOpSlider)

	var refreshBubbleLifetimeControls func()
	bubbleLifetimeDD, bubbleLifetimeEvents := eui.NewDropdown()
	bubbleLifetimeDD.Label = "Bubble Lifetime"
	bubbleLifetimeDD.Options = []string{BubbleLifetimeModern, BubbleLifetimeClassic}
	bubbleLifetimeDD.Selected = 0
	if normalizeBubbleLifetimeMode(gs.BubbleLifetimeMode) == BubbleLifetimeClassic {
		bubbleLifetimeDD.Selected = 1
	}
	bubbleLifetimeDD.Size = eui.Point{X: panelWidth, Y: 24}
	bubbleLifetimeDD.SetTooltip("Classic: fixed 8 seconds. Modern: base seconds plus seconds per word.")
	bubbleLifetimeEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventDropdownSelected && ev.Index >= 0 && ev.Index < len(bubbleLifetimeDD.Options) {
			gs.BubbleLifetimeMode = bubbleLifetimeDD.Options[ev.Index]
			if refreshBubbleLifetimeControls != nil {
				refreshBubbleLifetimeControls()
			}
			settingsDirty = true
		}
	}
	bubbleSection.AddItem(bubbleLifetimeDD)

	bubbleBaseLifeSlider, bubbleBaseLifeEvents := eui.NewSlider()
	bubbleBaseLifeSlider.Label = "Modern Base Life (s)"
	bubbleBaseLifeSlider.MinValue = 1
	bubbleBaseLifeSlider.MaxValue = 5
	bubbleBaseLifeSlider.Value = float32(gs.BubbleBaseLife)
	bubbleBaseLifeSlider.Size = eui.Point{X: panelWidth - 10, Y: 24}
	bubbleBaseLifeSlider.SetTooltip("Modern mode starts with this many seconds (default 2).")
	bubbleBaseLifeEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventSliderChanged {
			gs.BubbleBaseLife = float64(ev.Value)
			settingsDirty = true
		}
	}
	bubbleSection.AddItem(bubbleBaseLifeSlider)

	// Life added per word in a bubble
	bubblePerWordSlider, bubblePerWordEvents := eui.NewSlider()
	bubblePerWordSlider.Label = "Modern Life per Word (s)"
	bubblePerWordSlider.MinValue = 0
	bubblePerWordSlider.MaxValue = 2
	bubblePerWordSlider.Value = float32(gs.BubbleLifePerWord)
	bubblePerWordSlider.Size = eui.Point{X: panelWidth - 10, Y: 24}
	bubblePerWordSlider.SetTooltip("Modern mode adds this many seconds per word (default 1).")
	bubblePerWordEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventSliderChanged {
			gs.BubbleLifePerWord = float64(ev.Value)
			settingsDirty = true
		}
	}
	bubbleSection.AddItem(bubblePerWordSlider)

	refreshBubbleLifetimeControls = func() {
		disabled := normalizeBubbleLifetimeMode(gs.BubbleLifetimeMode) == BubbleLifetimeClassic
		bubbleBaseLifeSlider.Disabled = disabled
		bubblePerWordSlider.Disabled = disabled
	}
	refreshBubbleLifetimeControls()

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
	bubbleSection.AddItem(bubbleScaleSlider)

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
	statusSection.AddItem(barOpacitySlider)

	advancedBtn, advancedEvents := eui.NewButton()
	advancedBtn.Text = "Advanced Settings"
	setMaterialButtonIcon(advancedBtn, "tune")
	advancedBtn.Size = eui.Point{X: panelWidth, Y: 24}
	advancedEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			SettingsLock.Lock()
			defer SettingsLock.Unlock()

			makeAdvancedSettingsWindow()
			advancedWin.ToggleNear(ev.Item)
		}
	}
	moreSection.AddItem(advancedBtn)

	// Keep the Window & Display pane balanced: the UI Scale row belongs there,
	// while layout recovery is an infrequent action that fits naturally with
	// the other additional tools.
	moreSection.AddItem(resetWindowsBtn)

	setupBtn, setupEvents := eui.NewButton()
	setupBtn.Text = "Setup Wizard"
	setMaterialButtonIcon(setupBtn, "auto_fix_high")
	setupBtn.SetTooltip("Reopen guided graphics, layout, controls, and audio setup without resetting choices.")
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
	gettingStartedSection.AddItem(setupBtn)

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
	textSizeSection.AddItem(labelFontSlider)

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
	textSizeSection.AddItem(invFontSlider)

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
	textSizeSection.AddItem(plFontSlider)

	recentPlayersCB, recentPlayersEvents := eui.NewCheckbox()
	recentPlayersCB.Text = "Show recently on-screen group"
	recentPlayersCB.Size = eui.Point{X: panelWidth, Y: 24}
	recentPlayersCB.Checked = gs.ShowRecentPlayers
	recentPlayersEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			gs.ShowRecentPlayers = ev.Checked
			playersDirty = true
			settingsDirty = true
		}
	}
	textSizeSection.AddItem(recentPlayersCB)

	clanPlayersCB, clanPlayersEvents := eui.NewCheckbox()
	clanPlayersCB.Text = "Group clan members together"
	clanPlayersCB.Size = eui.Point{X: panelWidth, Y: 24}
	clanPlayersCB.Checked = gs.GroupClanMembers
	clanPlayersEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			gs.GroupClanMembers = ev.Checked
			playersDirty = true
			settingsDirty = true
		}
	}
	textSizeSection.AddItem(clanPlayersCB)

	shareIconsCB, shareIconsEvents := eui.NewCheckbox()
	shareIconsCB.Text = "Show sharing icons in Players list"
	shareIconsCB.Size = eui.Point{X: panelWidth, Y: 24}
	shareIconsCB.Checked = gs.PlayerShareIcons
	shareIconsEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			gs.PlayerShareIcons = ev.Checked
			playersDirty = true
			settingsDirty = true
		}
	}
	textSizeSection.AddItem(shareIconsCB)

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
	textSizeSection.AddItem(consoleFontSlider)

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
	textSizeSection.AddItem(chatWindowFontSlider)

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
	textSizeSection.AddItem(chatFontSlider)

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
	ttsSection.AddItem(ttsSpeedSlider)

	outer.AddItem(left)
	outer.AddItem(centerLeft)
	outer.AddItem(centerRight)
	outer.AddItem(right)
	settingsWin.AddItem(outer)
	settingsWin.AddWindow(false)
}

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
	if advancedWin != nil {
		advancedWin.Close()
		advancedWin = nil
	}
	if textColorsWin != nil {
		textColorsWin.Close()
		textColorsWin = nil
		textColorWheelsDark = nil
		textColorWheelsLight = nil
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
	if qualityWin != nil {
		refreshShaderEffectControls()
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

// popupButton defines a button in a popup dialog.
type popupButton struct {
	Text       string
	Width      float32
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
	win.NoScroll = true
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
		txt.SelectableText = true
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
		if b.Width > 0 {
			btn.Size.X = b.Width
		}
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

func confirmResetWindows() {
	showPopup(
		"Confirm Reset Windows",
		"Reset window positions, sizes, visibility, and pinned locations to defaults?",
		[]popupButton{
			{Text: "Cancel"},
			{Text: "Reset", Color: &eui.ColorDarkRed, HoverColor: &eui.ColorRed, Action: resetWindows},
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

	flow := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL}

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
		"Delete Character",
		fmt.Sprintf("Are you sure you want to delete %s?", c.Name),
		[]popupButton{
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
	qualityWin.SetRefreshInterval(100 * time.Millisecond)
	qualityWin.SetZone(eui.HZoneCenterLeft, eui.VZoneMiddleTop)
	// Use three short columns so the larger shader group does not make this
	// window unnecessarily tall.
	var panelWidth float32 = 270
	outer := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_HORIZONTAL}
	left := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL}
	left.Size = eui.Point{X: panelWidth, Y: 10}
	center := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL}
	center.Size = eui.Point{X: panelWidth, Y: 10}
	right := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL}
	right.Size = eui.Point{X: panelWidth, Y: 10}

	artworkSection := newConfigurationSection("Artwork Scaling", width)
	occlusionSection := newConfigurationSection("Foreground Occlusion", width)
	performanceSection := newConfigurationSection("GPU & Performance", width)
	left.AddItem(artworkSection)
	left.AddItem(occlusionSection)
	left.AddItem(performanceSection)

	shadowSection := newConfigurationSection("Shadows", width)
	motionSection := newConfigurationSection("Motion Smoothing", width)
	gammaSection := newConfigurationSection("Sprite Gamma", width)
	denoiseSection := newConfigurationSection("Dither Cleanup", width)
	center.AddItem(shadowSection)
	center.AddItem(motionSection)
	center.AddItem(gammaSection)
	center.AddItem(denoiseSection)

	shaderSection := newConfigurationSection("Shader Effects", width)
	right.AddItem(shaderSection)

	masterShaders, masterShaderEvents := eui.NewCheckbox()
	shadersEnabledCB = masterShaders
	shadersEnabledCB.Text = "Enable Shader Effects"
	shadersEnabledCB.Size = eui.Point{X: width, Y: 24}
	shadersEnabledCB.Checked = gs.ShadersEnabled
	shadersEnabledCB.SetTooltip("Enable custom shader effects. Individual choices are preserved while disabled.")
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
	shaderSection.AddItem(shadersEnabledCB)

	renderScale, renderScaleEvents := eui.NewSlider()
	qualityRenderScaleSlider = renderScale
	renderScale.Label = "Max Upscale"
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
	renderScale.SetTooltip("Caps artwork upscale at 2x-4x; fitted textures may use less to avoid wasted GPU memory.")
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

	psCB, precacheSoundEvents := eui.NewCheckbox()
	precacheSoundCB = psCB
	precacheSoundCB.Text = "Precache Sounds"
	precacheSoundCB.Size = eui.Point{X: width, Y: 24}
	precacheSoundCB.Checked = gs.PrecacheSounds
	precacheSoundCB.SetTooltip("Uses roughly 300 MB more RAM to avoid first-play sound decoding delays.")
	precacheSoundEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			gs.PrecacheSounds = ev.Checked
			if ev.Checked {
				if noCacheCB != nil {
					noCacheCB.Checked = false
				}
				go precacheSounds()
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
	performanceSection.AddItem(precacheSoundCB)

	activityIndicatorsCB, activityIndicatorEvents := eui.NewCheckbox()
	activityIndicatorsCB.Text = "Show asset activity dots"
	activityIndicatorsCB.Size = eui.Point{X: width, Y: 24}
	activityIndicatorsCB.Checked = gs.AssetActivityIndicators
	activityIndicatorsCB.SetTooltip("Show activity dots in the game view's lower-right: green for artwork decoding and processing, amber for audio decoding, and red for GPU artwork uploads.")
	activityIndicatorEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			gs.AssetActivityIndicators = ev.Checked
			setClientActivityIndicatorsEnabled(ev.Checked)
			settingsDirty = true
		}
	}
	performanceSection.AddItem(activityIndicatorsCB)

	var shadowDarknessSlider *eui.ItemData
	pcCB, potatoEvents := eui.NewCheckbox()
	potatoCB = pcCB
	potatoCB.Text = "Potato GPU (4096px Limit)"
	potatoCB.SetTooltip("Use standalone textures instead of a large shared atlas for GPUs limited to 4096x4096 textures. Artwork resolution and upscaling are unchanged.")
	potatoCB.Size = eui.Point{X: width, Y: 24}
	potatoCB.Checked = gs.PotatoGPU
	potatoEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			gs.PotatoGPU = ev.Checked
			applySettings()
			clearCaches()
			settingsDirty = true
		}
	}
	performanceSection.AddItem(potatoCB)

	vsyncCB, vsyncEvents := eui.NewCheckbox()
	vsyncCB.Text = "VSync - Limit FPS"
	vsyncCB.Size = eui.Point{X: width, Y: 24}
	vsyncCB.Checked = gs.VSync
	vsyncCB.SetTooltip("Sync frame rate to the display.")
	vsyncEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			gs.VSync = ev.Checked
			applyVSyncSetting()
			settingsDirty = true
		}
	}
	performanceSection.AddItem(vsyncCB)

	wsCB, windowShadowsEvents := eui.NewCheckbox()
	windowShadowsCB = wsCB
	windowShadowsCB.Text = "Window Shadows"
	windowShadowsCB.Size = eui.Point{X: width, Y: 24}
	windowShadowsCB.Checked = gs.WindowShadows
	windowShadowsEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			gs.WindowShadows = ev.Checked
			eui.SetWindowShadows(gs.WindowShadows)
			settingsDirty = true
		}
	}
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

	shaderSection.AddItem(newConfigurationSubheading("Lighting", width))
	shaderQualityCB, shaderQualityEv := eui.NewCheckbox()
	shaderLightingCB = shaderQualityCB
	shaderQualityCB.Text = "Shader Lighting Effects"
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
	shaderSection.AddItem(shaderQualityCB)

	mobileConeCB, mobileConeEvents := eui.NewCheckbox()
	mobileLightConeShadowsCB = mobileConeCB
	mobileConeCB.Text = "Mobile light-cone shadows"
	mobileConeCB.Size = eui.Point{X: width, Y: 24}
	mobileConeCB.Checked = gs.MobileLightConeShadows
	mobileConeCB.SetTooltip("Let mobiles cast experimental soft cone shadows from nearby shader lights.")
	mobileConeEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			gs.MobileLightConeShadows = ev.Checked
			settingsDirty = true
		}
	}
	shaderSection.AddItem(mobileConeCB)

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
	shaderSection.AddItem(flameFlickerCB)

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
	shaderSection.AddItem(flameFlickerSlider)

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
	shaderSection.AddItem(shaderLightSlider)

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
	shaderSection.AddItem(shaderGlowSlider)

	shaderSection.AddItem(newConfigurationSubheading("Magic Effects", width))
	replacementCB, replacementEffectsEvents := eui.NewCheckbox()
	replacementEffectsCB = replacementCB
	replacementEffectsCB.Text = "Replacement Effects"
	replacementEffectsCB.Size = eui.Point{X: width, Y: 24}
	replacementEffectsCB.Checked = gs.ReplacementEffects
	replacementEffectsCB.SetTooltip("Use procedural magic effects.")
	replacementEffectsEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			gs.ReplacementEffects = ev.Checked
			settingsDirty = true
		}
	}
	shaderSection.AddItem(replacementEffectsCB)

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
				if qualityWin != nil {
					qualityWin.Refresh()
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
	denoiseCB.SetTooltip("Smooth palette dithering.")
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
	denoiseSharpSlider.SetTooltip("Preserve more fine detail.")
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
	motionCB.SetTooltip("Smooth camera and mobiles.")
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
	smallMovingPicturesCB.Text = "Interpolate small moving sprites"
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

	shaderSection.AddItem(newConfigurationSubheading("Frame Blending", width))
	aCB, animEvents := eui.NewCheckbox()
	animCB = aCB
	animCB.Text = "Mobile Animation Blending"
	animCB.Size = eui.Point{X: width, Y: 24}
	animCB.Checked = gs.BlendMobiles
	animEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			gs.BlendMobiles = ev.Checked
			refreshShaderEffectControls()
			settingsDirty = true
		}
	}
	shaderSection.AddItem(animCB)

	pCB, pictBlendEvents := eui.NewCheckbox()
	pictBlendCB = pCB
	pictBlendCB.Text = "World Animation Blending"
	pictBlendCB.Size = eui.Point{X: width, Y: 24}
	pictBlendCB.Checked = gs.BlendPicts
	pictBlendCB.SetTooltip("Blend scenery animation frames.")
	pictBlendEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			gs.BlendPicts = ev.Checked
			refreshShaderEffectControls()
			settingsDirty = true
		}
	}
	shaderSection.AddItem(pictBlendCB)

	mobileSlider, mobileBlendEvents := eui.NewSlider()
	mobileBlendSlider = mobileSlider
	mobileBlendSlider.Label = "Mobile Animation Blend Duration"
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
	shaderSection.AddItem(mobileBlendSlider)

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
	shaderSection.AddItem(worldBlendSlider)

	refreshShaderEffectControls()

	outer.AddItem(left)
	outer.AddItem(center)
	outer.AddItem(right)
	qualityWin.AddItem(outer)
	qualityWin.AddWindow(false)
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
	tileLayoutWin.Title = "Tiled Window Layout"
	tileLayoutWin.Closable = true
	tileLayoutWin.Resizable = false
	tileLayoutWin.AutoSize = true
	tileLayoutWin.Movable = true
	tileLayoutWin.SetZone(eui.HZoneCenterLeft, eui.VZoneMiddleTop)

	flow := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL}
	workspace := newConfigurationSection("Workspace", width)
	arrangement := newConfigurationSection("Arrangement", width)
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
	notificationsWin.Title = "Notification Settings"
	notificationsWin.Closable = true
	notificationsWin.Resizable = false
	notificationsWin.AutoSize = true
	notificationsWin.Movable = true
	notificationsWin.SetZone(eui.HZoneCenterLeft, eui.VZoneMiddleTop)

	flow := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL}
	eventsSection := newConfigurationSection("Notify About", width)
	deliverySection := newConfigurationSection("Display & Sound", width)
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
				if val == &gs.NotificationBeep {
					updateSoundVolume()
				}
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
	addCB(deliverySection, "Beep", &gs.NotificationBeep)

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
		spacer, _ := eui.NewText()
		spacer.Size = eui.Point{X: columnWidth, Y: 10}
		col.AddItem(spacer)

		label, _ := eui.NewText()
		label.Text = text
		label.FontSize = 15
		label.Size = eui.Point{X: columnWidth, Y: 30}
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
	setMaterialButtonIcon(debugBtn, "bug_report")
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
	setMaterialButtonIcon(dlBtn, "download")
	dlBtn.Size = eui.Point{X: columnWidth, Y: 24}
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

	dataFolderBtn, dataFolderEvents := eui.NewButton()
	dataFolderBtn.Text = "Open User Data Folder"
	setMaterialButtonIcon(dataFolderBtn, "folder_open")
	dataFolderBtn.Size = eui.Point{X: columnWidth, Y: 24}
	dataFolderBtn.SetTooltip("Open the persistent folder containing settings, characters, scripts, recordings, themes, and downloaded assets.")
	dataFolderEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			if err := open.Run(dataDirPath); err != nil {
				consoleMessage("open user data folder: " + err.Error())
			}
		}
	}
	toolsCol.AddItem(dataFolderBtn)

	diagnosticsFolderBtn, diagnosticsFolderEvents := eui.NewButton()
	diagnosticsFolderBtn.Text = "Open Diagnostics Folder"
	setMaterialButtonIcon(diagnosticsFolderBtn, "folder_open")
	diagnosticsFolderBtn.Size = eui.Point{X: columnWidth, Y: 24}
	diagnosticsFolderBtn.SetTooltip("Open the folder containing the current and rotated diagnostic logs for bug reports.")
	diagnosticsFolderEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			path := diagnosticsLogDir()
			if err := os.MkdirAll(path, 0o755); err != nil {
				logError("create diagnostics folder: %v", err)
				return
			}
			if err := open.Run(path); err != nil {
				logError("open diagnostics folder: %v", err)
			}
		}
	}
	toolsCol.AddItem(diagnosticsFolderBtn)

	resetBtn, resetEv := eui.NewButton()
	resetBtn.Text = "Reset All Settings"
	setMaterialButtonIcon(resetBtn, "restart_alt")
	resetBtn.Size = eui.Point{X: columnWidth, Y: 24}
	resetBtn.Color = eui.ColorDarkRed
	resetBtn.HoverColor = eui.ColorRed
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
	scriptKillCB.SetTooltip("Stop scripts that spam output.")
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
	autoRecCB.SetTooltip("Record each login session.")
	autoRecEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			gs.AutoRecord = ev.Checked
			settingsDirty = true
		}
	}
	toolsCol.AddItem(autoRecCB)

	// Interface column
	addSectionLabel(interfaceCol, "Interface")

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
	setMaterialButtonIcon(joystickBtn, "sports_esports")
	joystickBtn.Size = eui.Point{X: columnWidth, Y: 24}
	joystickEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			SettingsLock.Lock()
			defer SettingsLock.Unlock()
			joystickWin.ToggleNear(ev.Item)
		}
	}
	interfaceCol.AddItem(joystickBtn)

	addSectionLabel(interfaceCol, "Rendering")

	coordsCB, coordsEvents := eui.NewCheckbox()
	coordsCB.Text = "Floating-point sprite coordinates"
	coordsCB.Size = eui.Point{X: columnWidth, Y: 24}
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
	interfaceCol.AddItem(coordsCB)

	// Chat & TTS column
	addSectionLabel(chatCol, "Chat & TTS")

	ttsEnabledCB, ttsEnabledEvents := eui.NewCheckbox()
	ttsEnabledCB.Text = "Enable chat TTS"
	ttsEnabledCB.Size = eui.Point{X: columnWidth, Y: 24}
	ttsEnabledCB.Checked = gs.ChatTTS
	ttsEnabledCB.SetTooltip("Speak eligible chat messages.")
	ttsEnabledEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type != eui.EventCheckboxChanged {
			return
		}
		if !ev.Checked {
			disableTTS()
			return
		}
		gs.ChatTTS = true
		if ttsMixCB != nil {
			ttsMixCB.Checked = true
			ttsMixCB.Dirty = true
		}
		if ttsMixSlider != nil {
			ttsMixSlider.Disabled = false
			ttsMixSlider.Dirty = true
		}
		settingsDirty = true
		updateSoundVolume()
	}
	chatCol.AddItem(ttsEnabledCB)

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
	setMaterialButtonIcon(bubbleBtn, "chat")
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
	setMaterialButtonIcon(ttsTestBtn, "volume_up")
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
	setMaterialButtonIcon(ttsEditBtn, "edit")
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
	throttleSoundCB.Text = "Throttle Repeated Sounds"
	throttleSoundCB.Size = eui.Point{X: columnWidth, Y: 24}
	throttleSoundCB.Checked = gs.ThrottleSounds
	throttleSoundCB.SetTooltip("Suppress the same effect when it repeats in adjacent server updates.")
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
	enhancementCB.SetTooltip("Adds stereo width, ambience, and tone polish to newly played game sounds.")
	enhancementEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			gs.SoundEnhancement = ev.Checked
			refreshMixerEnhancementControls()
			settingsDirty = true
		}
	}
	chatCol.AddItem(enhancementCB)

	resampleCB, resampleEvents := eui.NewCheckbox()
	resampleAudioCB = resampleCB
	resampleCB.Text = "High quality resampling"
	resampleCB.Size = eui.Point{X: columnWidth, Y: 24}
	resampleCB.Checked = gs.HighQualityResampling
	resampleCB.SetTooltip("Uses Lanczos resampling and dithering for cleaner audio at higher CPU cost.")
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
	musicEnhancementCB.SetTooltip("Adds space and ambience to newly started bard music.")
	musicEnhancementEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			gs.MusicEnhancement = ev.Checked
			refreshMixerEnhancementControls()
			settingsDirty = true
		}
	}
	chatCol.AddItem(musicEnhancementCB)

	addSectionLabel(systemCol, "Network")

	altNetCB, altNetEvents := eui.NewCheckbox()
	advancedPNACheckbox = altNetCB
	altNetCB.Text = "Network Latency & Server Phase Timing (NLSPT)"
	altNetCB.Size = eui.Point{X: columnWidth, Y: 24}
	altNetCB.Checked = gs.AltNetMode
	altNetCB.SetTooltip("Learns the server frame phase and sends fresh input shortly before its next processing window. Packet loss temporarily restores original timing.")
	altNetEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			setPNAEnabled(ev.Checked)
		}
	}
	systemCol.AddItem(altNetCB)

	pnaSafetySlider, pnaSafetyEvents := eui.NewSlider()
	pnaSafetySlider.Label = "NLSPT safety (%)"
	pnaSafetySlider.MinValue = 0
	pnaSafetySlider.MaxValue = 50
	pnaSafetySlider.Value = float32(networkAdjustmentSafetyPercent.Load())
	pnaSafetySlider.Size = eui.Point{X: columnWidth - 10, Y: 24}
	pnaSafetySlider.SetTooltip("Minimum lead as a percentage of one server frame, with frame jitter added separately. Session-only; resets to 10% when goThoom starts.")
	pnaSafetyEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventSliderChanged {
			networkAdjustmentSafetyPercent.Store(int64(ev.Value))
		}
	}
	systemCol.AddItem(pnaSafetySlider)

	serverInput, serverEvents := eui.NewInput()
	serverInput.Label = "Server address"
	serverInput.Text = gs.ServerAddress
	serverInput.TextPtr = &gs.ServerAddress
	serverInput.Size = eui.Point{X: columnWidth, Y: 24}
	serverInput.SetTooltip("Set the primary server address.")
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

	timingLabel, _ := eui.NewText()
	timingLabel.Text = ""
	timingLabel.Size = eui.Point{X: columnWidth, Y: 24}
	timingLabel.FontSize = 10
	systemCol.AddItem(timingLabel)

	timingBtn, timingEvents := eui.NewButton()
	timingBtn.Text = "Show Network Timing"
	setMaterialButtonIcon(timingBtn, "query_stats")
	timingBtn.Size = eui.Point{X: columnWidth, Y: 24}
	timingEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			loginMu.Lock()
			connected := tcpConn != nil
			loginMu.Unlock()
			if !connected {
				timingLabel.Text = "not connected to server"
				timingLabel.Dirty = true
				advancedWin.Refresh()
				return
			}
			reply, jitter := networkTimingSnapshot()
			if reply == 0 {
				timingLabel.Text = fmt.Sprintf("Cmd reply: waiting   Frame p95: %s", formatToolbarLatency(jitter))
			} else {
				timingLabel.Text = fmt.Sprintf("Cmd reply: %s   Frame p95: %s", formatToolbarLatency(reply), formatToolbarLatency(jitter))
			}
			timingLabel.Dirty = true
			advancedWin.Refresh()
		}
	}
	systemCol.AddItem(timingBtn)

	addSectionLabel(systemCol, "Performance")

	batchArtworkCB, batchArtworkEvents := eui.NewCheckbox()
	batchArtworkCB.Text = "Batch room artwork loading"
	batchArtworkCB.Size = eui.Point{X: columnWidth, Y: 24}
	batchArtworkCB.Checked = gs.BatchArtworkLoading
	batchArtworkCB.SetTooltip("Load missing room artwork in one pause instead of spreading first-use work across frames.")
	batchArtworkEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			gs.BatchArtworkLoading = ev.Checked
			settingsDirty = true
		}
	}
	systemCol.AddItem(batchArtworkCB)

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
	psAlwaysCB.SetTooltip("Limit FPS while focused.")
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
	psFPSSlider.SetTooltip("Set power-saving FPS.")
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
	displaySection := newConfigurationSection("Display", width)
	bubbleTypesSection := newConfigurationSection("Message Types", width)
	bubbleSourcesSection := newConfigurationSection("Show For", width)
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
	animatedBubblesCB.SetTooltip("Animate special speech bubbles.")
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
	debugWin.Title = "Debug Settings"
	debugWin.Closable = true
	debugWin.Resizable = false
	debugWin.AutoSize = true
	debugWin.Movable = true
	debugWin.SetZone(eui.HZoneCenterLeft, eui.VZoneMiddleTop)

	debugFlow := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL}
	diagnosticsSection := newConfigurationSection("Diagnostics", width)
	sceneSection := newConfigurationSection("Scene Overrides", width)
	shaderSection := newConfigurationSection("Shader Tools", width)
	cacheSection := newConfigurationSection("Cache Statistics", width)
	scriptSection := newConfigurationSection("Script Events", width)
	debugFlow.AddItem(diagnosticsSection)
	debugFlow.AddItem(sceneSection)
	debugFlow.AddItem(shaderSection)
	debugFlow.AddItem(scriptSection)
	debugFlow.AddItem(cacheSection)

	debugCB, debugEvents := eui.NewCheckbox()
	debugCB.Text = "Record script events"
	debugCB.Size = eui.Point{X: width, Y: 24}
	debugCB.Checked = gs.scriptEventDebug
	debugEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			gs.scriptEventDebug = ev.Checked
			settingsDirty = true
			scriptDebugList.Invisible = !ev.Checked
			if ev.Checked {
				refreshscriptDebug()
			} else {
				debugWin.Refresh()
			}
		}
	}
	scriptSection.AddItem(debugCB)

	scriptDebugList = &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL, Scrollable: true, Fixed: true}
	scriptDebugList.Size = eui.Point{X: width, Y: 120}
	scriptDebugList.Invisible = !gs.scriptEventDebug
	scriptSection.AddItem(scriptDebugList)

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
	hideMoveCB.SetTooltip("Helpful for screenshots")
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
	hideMobCB.SetTooltip("Helpful for screenshots")
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
	pictIDCB.SetTooltip("Shows picture ID on each sprite")
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
	reloadBtn.SetTooltip("Reload effect shaders.")
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

	cacheLabel, _ := eui.NewText()
	cacheLabel.Text = "Caches:"
	cacheLabel.Size = eui.Point{X: width, Y: 24}
	cacheLabel.FontSize = 10
	cacheSection.AddItem(cacheLabel)

	sheetCacheLabel, _ = eui.NewText()
	sheetCacheLabel.Text = ""
	sheetCacheLabel.Size = eui.Point{X: width, Y: 24}
	sheetCacheLabel.FontSize = 10
	cacheSection.AddItem(sheetCacheLabel)

	frameCacheLabel, _ = eui.NewText()
	frameCacheLabel.Text = ""
	frameCacheLabel.Size = eui.Point{X: width, Y: 24}
	frameCacheLabel.FontSize = 10
	cacheSection.AddItem(frameCacheLabel)

	scaledFrameCacheLabel, _ = eui.NewText()
	scaledFrameCacheLabel.Text = ""
	scaledFrameCacheLabel.Size = eui.Point{X: width, Y: 24}
	scaledFrameCacheLabel.FontSize = 10
	cacheSection.AddItem(scaledFrameCacheLabel)

	mobileCacheLabel, _ = eui.NewText()
	mobileCacheLabel.Text = ""
	mobileCacheLabel.Size = eui.Point{X: width, Y: 24}
	mobileCacheLabel.FontSize = 10
	cacheSection.AddItem(mobileCacheLabel)

	scaledMobileCacheLabel, _ = eui.NewText()
	scaledMobileCacheLabel.Text = ""
	scaledMobileCacheLabel.Size = eui.Point{X: width, Y: 24}
	scaledMobileCacheLabel.FontSize = 10
	cacheSection.AddItem(scaledMobileCacheLabel)

	soundCacheLabel, _ = eui.NewText()
	soundCacheLabel.Text = ""
	soundCacheLabel.Size = eui.Point{X: width, Y: 24}
	soundCacheLabel.FontSize = 10
	cacheSection.AddItem(soundCacheLabel)

	clearCacheBtn, clearCacheEvents := eui.NewButton()
	clearCacheBtn.Text = "Clear All Caches"
	setMaterialButtonIcon(clearCacheBtn, "delete_sweep")
	clearCacheBtn.Size = eui.Point{X: width, Y: 24}
	clearCacheEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			clearCaches()
			updateDebugStats()
		}
	}
	cacheSection.AddItem(clearCacheBtn)
	totalCacheLabel, _ = eui.NewText()
	totalCacheLabel.Text = ""
	totalCacheLabel.Size = eui.Point{X: width, Y: 24}
	totalCacheLabel.FontSize = 10
	cacheSection.AddItem(totalCacheLabel)

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
	if soundCacheLabel != nil {
		soundCacheLabel.Text = fmt.Sprintf("Sounds: %d (%s)", soundCount, humanize.Bytes(uint64(soundBytes)))
		soundCacheLabel.Dirty = true
	}
	if totalCacheLabel != nil {
		total := stats.sheetBytes + stats.frameBytes + stats.scaledFrameBytes + stats.mobileBytes + stats.scaledMobileBytes + soundBytes
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
	// Use the common text window scaffold to get an inner scrollable list
	// and consistent padding/behavior with Inventory/Chat windows.
	playersWin, playersList, _ = makeTextWindow("Players", eui.HZoneRight, eui.VZoneTop, false)
	playersWin.Searchable = true
	playersWin.OnSearch = searchPlayersWindow
	// Refresh contents on resize so word-wrapping and row sizing stay correct.
	playersWin.OnResize = func() { updatePlayersWindow() }
	updatePlayersWindow()
}
