package main

import (
	"context"
	_ "embed"
	"fmt"
	"math"
	"strings"
	"time"

	"gothoom/eui"

	"github.com/hajimehoshi/ebiten/v2"
	text "github.com/hajimehoshi/ebiten/v2/text/v2"
	"github.com/pkg/browser"
)

const (
	setupWizardPageCount = 9
	setupWizardTopGap    = 24
	setupWizardKoFiURL   = "https://ko-fi.com/distortions"
)

//go:embed data/setup-wizard/Distortions__2026-08-21-17-35-52.clMov.zip
var setupWizardMovieZip []byte

var (
	setupWizardWin  *eui.WindowData
	setupWizardRoot *eui.ItemData
	setupWizardPage int

	setupWizardPreview       *moviePlayer
	setupWizardPreviewCancel context.CancelFunc
	setupWizardPreviewDone   chan struct{}
	setupWizardPreviewActive bool
	setupWizardPreviewLogin  bool
	setupWizardPreviewPlayer string
	setupWizardPreviewCrypt  bool
	setupWizardPreviewBubble bool
	setupWizardPreviewRev    int32
)

func shouldShowSetupWizard(configLoaded bool, completedVersion, currentVersion int) bool {
	return !configLoaded || completedVersion < currentVersion
}

func openSetupWizard(force bool) {
	if !force && !shouldShowSetupWizard(settingsLoaded, gs.SetupWizardVersion, appVersion) {
		return
	}
	setupWizardPage = 0
	if setupWizardWin == nil {
		setupWizardWin = eui.NewWindow()
		setupWizardWin.Closable = false
		setupWizardWin.Resizable = false
		setupWizardWin.AutoSize = true
		setupWizardWin.Movable = true
		setupWizardWin.Padding = 10
		setupWizardWin.BorderPad = 4
		setupWizardWin.SetZone(eui.HZoneLeft, eui.VZoneTop)
		setupWizardWin.AddWindow(false)
		setupWizardWin.ClearZone()
		_ = setupWizardWin.SetPos(eui.Point{X: 0, Y: setupWizardTopGap})
	}
	rebuildSetupWizard()
	setupWizardWin.MarkOpen()
	startSetupWizardPreview()
}

func startSetupWizardPreview() {
	if setupWizardPreviewActive || tcpConn != nil || clmov != "" || playingMovie || pcapPath != "" || fake || clImages == nil || gameCtx == nil {
		return
	}
	previousRevision := movieRevision
	frames, err := parseMovieZipBytes(setupWizardMovieZip, clVersion)
	if err != nil || len(frames) == 0 {
		movieRevision = previousRevision
		if err != nil {
			logError("setup wizard movie: %v", err)
		}
		return
	}

	previewCtx, cancel := context.WithCancel(gameCtx)
	setupWizardPreviewCancel = cancel
	setupWizardPreviewDone = make(chan struct{})
	setupWizardPreviewLogin = loginWin != nil && loginWin.IsOpen()
	setupWizardPreviewPlayer = playerName
	setupWizardPreviewCrypt = drawStateEncrypted
	setupWizardPreviewBubble = blockBubbles
	setupWizardPreviewRev = previousRevision
	setupWizardPreviewActive = true
	blockBubbles = true
	drawStateEncrypted = false
	playerName = extractMoviePlayerName(frames)
	if loginWin != nil {
		loginWin.Close()
	}

	mp := newMoviePlayer(frames, clMovFPS, cancel)
	mp.repeat = true
	setupWizardPreview = mp
	go func() {
		mp.run(previewCtx)
		close(setupWizardPreviewDone)
	}()
}

func stopSetupWizardPreview() {
	if !setupWizardPreviewActive {
		return
	}
	setupWizardPreviewActive = false
	if setupWizardPreview != nil {
		setupWizardPreview.playing = false
		if setupWizardPreview.ticker != nil {
			setupWizardPreview.ticker.Stop()
		}
	}
	if setupWizardPreviewCancel != nil {
		setupWizardPreviewCancel()
	}
	if setupWizardPreviewDone != nil {
		select {
		case <-setupWizardPreviewDone:
		case <-time.After(250 * time.Millisecond):
		}
	}
	stopAllSounds()
	stopAllTTS()
	stopAllMusic()
	playingMovie = false
	movieMode = false
	blockBubbles = setupWizardPreviewBubble
	playerName = setupWizardPreviewPlayer
	drawStateEncrypted = setupWizardPreviewCrypt
	movieRevision = setupWizardPreviewRev
	setupWizardPreview = nil
	setupWizardPreviewCancel = nil
	setupWizardPreviewDone = nil
	resetDrawState()
	// Movie messages can populate the Players window. Restore its persisted
	// contents without allowing preview-only names to be saved.
	playersMu.Lock()
	players = make(map[string]*Player)
	playersMu.Unlock()
	loadPlayersPersist()
	updatePlayersWindow()
	playersPersistDirty = false
	playersDirty = false
	updateRecordButton()
	if setupWizardPreviewLogin && loginWin != nil {
		loginWin.MarkOpen()
	}
}

func rebuildSetupWizard() {
	if setupWizardWin == nil {
		return
	}
	if setupWizardPage < 0 {
		setupWizardPage = 0
	}
	if setupWizardPage >= setupWizardPageCount {
		setupWizardPage = setupWizardPageCount - 1
	}
	setupWizardWin.Scroll = eui.Point{}

	setupWizardWin.Title = fmt.Sprintf("goThoom %d Setup", appVersion)
	root := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL}
	root.Size = eui.Point{X: 620, Y: 10}

	step := setupWizardText(fmt.Sprintf("Step %d of %d", setupWizardPage+1, setupWizardPageCount), 10, 620)
	root.AddItem(step)

	switch setupWizardPage {
	case 0:
		buildSetupWelcomePage(root)
	case 1:
		buildSetupControlsPage(root)
	case 2:
		buildSetupStatusPage(root)
	case 3:
		buildSetupVisibilityPage(root)
	case 4:
		buildSetupGraphicsPage(root)
	case 5:
		buildSetupMotionPage(root)
	case 6:
		buildSetupLightingPage(root)
	case 7:
		buildSetupAudioPage(root)
	case 8:
		buildSetupFinishPage(root)
	}

	root.AddItem(setupWizardNavigation())
	if setupWizardRoot == nil {
		setupWizardWin.AddItem(root)
	} else {
		setupWizardWin.ReplaceItem(0, root)
	}
	setupWizardRoot = root
	setupWizardWin.Refresh()
}

func buildSetupWelcomePage(root *eui.ItemData) {
	root.AddItem(setupWizardHeading("Welcome to Puddleby"))
	root.AddItem(setupWizardText(
		fmt.Sprintf("Welcome to goThoom version %d. This short tour appears on first run and once after each client release, so useful new options do not stay hidden.", appVersion),
		12, 620,
	))
	root.AddItem(setupWizardText(
		"Every control starts from your current setting, and changes appear immediately in the offline preview. All options remain available later from Settings and Quality.",
		12, 620,
	))
	root.AddItem(setupWizardText(
		"We will review controls, interface visibility, graphics, and performance. You can skip at any point.",
		12, 620,
	))
}

func buildSetupControlsPage(root *eui.ItemData) {
	root.AddItem(setupWizardHeading("Controls"))
	root.AddItem(setupWizardText("Choose the interaction style that feels natural. Keyboard walking and the normal title-bar window drag continue to work with either choice.", 12, 620))

	root.AddItem(setupWizardCheckbox(
		"Click-to-toggle movement",
		"Off: hold the left mouse button to walk. On: click once to start walking toward the pointer and click again to stop.",
		gs.ClickToToggle,
		func(checked bool) {
			gs.ClickToToggle = checked
			if !checked {
				walkToggled = false
			}
			settingsDirty = true
		},
	))
	root.AddItem(setupWizardCheckbox(
		"Middle-click moves windows",
		"Lets you drag a goThoom window from anywhere inside it with the middle mouse button, in addition to dragging its title bar.",
		gs.MiddleClickMoveWindow,
		func(checked bool) {
			gs.MiddleClickMoveWindow = checked
			eui.SetMiddleClickMove(checked)
			settingsDirty = true
		},
	))
	root.AddItem(setupWizardCheckbox(
		"Keep the input bar open",
		"Keeps chat/command input active after sending. Leave it off to use WASD walking and a larger range of hotkeys whenever you are not chatting.",
		gs.InputBarAlwaysOpen,
		func(checked bool) {
			gs.InputBarAlwaysOpen = checked
			if checked {
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
		},
	))
}

func buildSetupStatusPage(root *eui.ItemData) {
	root.AddItem(setupWizardHeading("Status bars"))
	root.AddItem(setupWizardText("Choose where health, spirit, and balance appear and how strongly they stand out over the game.", 11, 620))

	placement, events := eui.NewDropdown()
	placement.Label = "Status bar placement"
	placement.Options = []string{"Along bottom", "Grouped lower left", "Grouped lower right", "Grouped upper right"}
	placement.Selected = int(gs.BarPlacement)
	placement.Size = eui.Point{X: 320, Y: 24}
	events.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventDropdownSelected && ev.Index >= 0 && ev.Index <= int(BarPlacementUpperRight) {
			gs.BarPlacement = BarPlacement(ev.Index)
			settingsDirty = true
		}
	}
	root.AddItem(placement)

	root.AddItem(setupWizardCheckbox(
		"Color bars by value",
		"Gradually changes bar color as a value rises or falls, making low health and spirit easier to notice.",
		gs.BarColorByValue,
		func(checked bool) {
			gs.BarColorByValue = checked
			settingsDirty = true
		},
	))
	root.AddItem(setupWizardSlider("Status bar opacity", "Lower values keep more of the world visible behind the bars.", 0.1, 1, float32(gs.BarOpacity), false, func(value float32) {
		gs.BarOpacity = float64(value)
		settingsDirty = true
	}))
}

func buildSetupVisibilityPage(root *eui.ItemData) {
	root.AddItem(setupWizardHeading("Bubbles, names, and visibility"))
	root.AddItem(setupWizardCheckbox("Speech bubbles", "Show spoken text over characters as well as in chat.", gs.SpeechBubbles, func(checked bool) {
		gs.SpeechBubbles = checked
		settingsDirty = true
	}))
	root.AddItem(setupWizardSlider("Bubble opacity", "Adjust how strongly speech bubbles cover the world.", 0, 1, float32(gs.BubbleOpacity), false, func(value float32) {
		gs.BubbleOpacity = float64(value)
		settingsDirty = true
	}))
	root.AddItem(setupWizardSlider("Bubble scale", "Changes bubble size without changing the chat-window font.", 1, 8, float32(gs.BubbleScale), false, func(value float32) {
		gs.BubbleScale = float64(value)
		settingsDirty = true
	}))
	root.AddItem(setupWizardCheckbox("Dark bubbles and names", "Use dark backgrounds with light text for speech bubbles and character names.", gs.DarkBubblesAndNames, func(checked bool) {
		gs.DarkBubblesAndNames = checked
		killNameTagCache()
		settingsDirty = true
	}))
	root.AddItem(setupWizardCheckbox("Hide my name tag", "Do not show a name tag over your own character.", gs.HideSelfNameTag, func(checked bool) {
		gs.HideSelfNameTag = checked
		killNameTagCache()
		settingsDirty = true
	}))
	root.AddItem(setupWizardCheckbox("Name tags only on hover", "Hide character names until the pointer is over that character.", gs.NameTagsOnHoverOnly, func(checked bool) {
		gs.NameTagsOnHoverOnly = checked
		settingsDirty = true
	}))
	root.AddItem(setupWizardCheckbox("Name-tag label colors", "Use saved player label colors around name tags.", gs.NameTagLabelColors, func(checked bool) {
		gs.NameTagLabelColors = checked
		killNameTagCache()
		settingsDirty = true
	}))
	root.AddItem(setupWizardSlider("Name background opacity", "Controls the dark backing behind character names.", 0, 1, float32(gs.NameBgOpacity), false, func(value float32) {
		gs.NameBgOpacity = float64(value)
		killNameTagCache()
		settingsDirty = true
	}))
	root.AddItem(setupWizardCheckbox("Fade obscuring objects", "Fade foreground artwork when it covers a character.", gs.FadeObscuringPictures, func(checked bool) {
		gs.FadeObscuringPictures = checked
		settingsDirty = true
	}))
	root.AddItem(setupWizardSlider("Obscuring-object opacity", "Sets how transparent a faded foreground object becomes.", 0, 1, float32(gs.ObscuringPictureOpacity), false, func(value float32) {
		gs.ObscuringPictureOpacity = float64(value)
		settingsDirty = true
	}))
}

func buildSetupGraphicsPage(root *eui.ItemData) {
	root.AddItem(setupWizardHeading("Graphics and comfort"))
	root.AddItem(setupWizardText(
		"Your current graphics choices are selected below. Adjust only what you want while watching the real renderer behind this window.",
		11, 620,
	))
	root.AddItem(setupWizardText("Watch the movie behind this window while changing these options; it uses the real game renderer at your current game-window scale.", 10, 620))

	root.AddItem(setupWizardSlider("Upscale game amount", "Renders the game at 1x to 4x resolution. Higher values improve sharpness on high-resolution displays but use more GPU.", 1, 4, float32(math.Round(gs.GameScale)), true, func(value float32) {
		previousUpscale := gs.SpriteUpscale
		gs.GameScale = math.Round(float64(value))
		gs.SpriteUpscale = spriteUpscaleFactor()
		if gs.SpriteUpscale != previousUpscale {
			clearCaches()
		}
		initFont()
		if gameWin != nil {
			gameWin.Refresh()
		}
		settingsDirty = true
	}))

	root.AddItem(setupWizardCheckbox(
		"Blend image dithering",
		"Blends nearby, similar palette colors to recover shades that dithering suggested on older displays.",
		gs.DenoiseImages,
		func(checked bool) {
			gs.DenoiseImages = checked
			if clImages != nil {
				clImages.Denoise = checked
			}
			if denoiseCB != nil {
				denoiseCB.Checked = checked
			}
			clearCaches()
			markQualityCustom()
		},
	))
	root.AddItem(setupWizardCheckbox(
		"Artwork upscale filter",
		"Uses scale-aware filtering when sprites are enlarged. Turn it off if you prefer harder pixel edges.",
		gs.SpriteUpscaleFilter,
		func(checked bool) {
			gs.SpriteUpscaleFilter = checked
			if upscaleFilterCB != nil {
				upscaleFilterCB.Checked = checked
			}
			clearCaches()
			markQualityCustom()
		},
	))
	root.AddItem(setupWizardCheckbox("VSync", "Limits presentation to the monitor refresh rate to prevent tearing. Turning it off can improve speed on some systems.", gs.VSync, func(checked bool) {
		gs.VSync = checked
		ebiten.SetVsyncEnabled(checked)
		settingsDirty = true
	}))
	root.AddItem(setupWizardCheckbox("Precache images", "Loads game artwork before play for fewer pauses, using up to about 2 GB of additional RAM.", gs.PrecacheImages, func(checked bool) {
		gs.PrecacheImages = checked
		if checked && !assetsPrecached {
			go precacheAssets()
		}
		settingsDirty = true
	}))
	root.AddItem(setupWizardCheckbox("Precache sounds", "Loads and prepares game sounds before play for fewer pauses, using roughly 300 MB more RAM.", gs.PrecacheSounds, func(checked bool) {
		gs.PrecacheSounds = checked
		if checked && !assetsPrecached {
			go precacheAssets()
		}
		settingsDirty = true
	}))
}

func buildSetupMotionPage(root *eui.ItemData) {
	root.AddItem(setupWizardHeading("Motion smoothing"))
	root.AddItem(setupWizardText("Position smoothing moves the camera and characters smoothly between server updates. Animation blending is optional, requires position smoothing, and controls how sprite frames transition.", 11, 620))
	var frameBlendOptions []*eui.ItemData
	root.AddItem(setupWizardCheckbox("Smooth movement positions", "Interpolates only camera and character positions between server frames.", gs.MotionSmoothing, func(checked bool) {
		gs.MotionSmoothing = checked
		for _, option := range frameBlendOptions {
			setSetupWizardDisabled(option, !checked)
		}
		markQualityCustom()
		if setupWizardWin != nil {
			setupWizardWin.Refresh()
		}
	}))
	animationHeading := setupWizardText("Animation blending", 14, 600)
	animationHeading.Position.X = 20
	applyBoldFace(animationHeading)
	root.AddItem(animationHeading)
	addFrameBlendOption := func(option *eui.ItemData) {
		option = setupWizardSubOption(option)
		setSetupWizardDisabled(option, !gs.MotionSmoothing)
		frameBlendOptions = append(frameBlendOptions, option)
		root.AddItem(option)
	}
	addFrameBlendOption(setupWizardCheckbox("Character animation blending", "Blends character animation frames instead of switching abruptly between them.", gs.BlendMobiles, func(checked bool) {
		gs.BlendMobiles = checked
		clearCaches()
		markQualityCustom()
	}))
	addFrameBlendOption(setupWizardSlider("Character blend amount", "Higher values blend more strongly but retain the prior frame longer.", 0.1, 1, float32(gs.MobileBlendAmount), false, func(value float32) {
		gs.MobileBlendAmount = float64(value)
		settingsDirty = true
	}))
	addFrameBlendOption(setupWizardCheckbox("World animation blending", "Blends animated water, grass, and other world artwork.", gs.BlendPicts, func(checked bool) {
		gs.BlendPicts = checked
		clearCaches()
		markQualityCustom()
	}))
	addFrameBlendOption(setupWizardSlider("World blend amount", "Controls how strongly world animation frames blend together.", 0.1, 1, float32(gs.BlendAmount), false, func(value float32) {
		gs.BlendAmount = float64(value)
		settingsDirty = true
	}))
}

func buildSetupLightingPage(root *eui.ItemData) {
	root.AddItem(setupWizardHeading("Lighting and gamma"))
	root.AddItem(setupWizardText(
		"Shader lighting adds smoother night darkening, colored light, and compact glow around light sources. It looks richer, but uses more GPU than classic lighting.",
		11, 620,
	))

	root.AddItem(setupWizardCheckbox(
		"Shader lighting effects",
		"Enable the enhanced lighting path. Light and glow strength remain adjustable later in Quality settings.",
		gs.ShaderLighting,
		func(checked bool) {
			gs.ShaderLighting = checked
			if shaderLightingCB != nil {
				shaderLightingCB.Checked = checked
			}
			if shaderLightSlider != nil {
				shaderLightSlider.Disabled = !checked
			}
			if shaderGlowSlider != nil {
				shaderGlowSlider.Disabled = !checked
			}
			markQualityCustom()
		},
	))
	root.AddItem(setupWizardCheckbox("Sprite gamma correction", "Compensates classic Macintosh artwork for a modern display. Disable it if the artwork looks washed out or too dark.", gs.SpriteGammaCorrection, func(checked bool) {
		gs.SpriteGammaCorrection = checked
		applySetupWizardGamma()
	}))
	root.AddItem(setupWizardGammaSlider("Original artwork gamma", "Clan Lord artwork was authored around Macintosh gamma 1.8.", &gs.SpriteGamma))
	root.AddItem(setupWizardGammaSlider("Monitor gamma", "Most modern displays use gamma 2.2; some use 2.4.", &gs.MonitorGamma))
}

func applySetupWizardGamma() {
	if clImages != nil {
		clImages.SetGammaCorrection(gs.SpriteGammaCorrection, gs.SpriteGamma, gs.MonitorGamma)
	}
	clearCaches()
	settingsDirty = true
}

func buildSetupAudioPage(root *eui.ItemData) {
	root.AddItem(setupWizardHeading("Audio and music"))
	root.AddItem(setupWizardText(
		"These options improve sound quality at the cost of additional CPU. You can change them later in Settings.",
		12, 620,
	))
	root.AddItem(setupWizardCheckbox(
		"Enhance sound effects",
		"Adds stereo width, ambience, and tone polish to in-game sounds.",
		gs.SoundEnhancement,
		func(checked bool) {
			gs.SoundEnhancement = checked
			settingsDirty = true
		},
	))
	root.AddItem(setupWizardCheckbox(
		"High quality audio resampling",
		"Uses Lanczos resampling and dithering for cleaner audio, with higher CPU use.",
		gs.HighQualityResampling,
		func(checked bool) {
			gs.HighQualityResampling = checked
			setHighQualityResamplingEnabled(checked)
			clearCaches()
			settingsDirty = true
		},
	))
	root.AddItem(setupWizardCheckbox(
		"Enhance music",
		"Adds space and ambience to background music.",
		gs.MusicEnhancement,
		func(checked bool) {
			gs.MusicEnhancement = checked
			settingsDirty = true
		},
	))
}

func buildSetupFinishPage(root *eui.ItemData) {
	root.AddItem(setupWizardHeading("Ready for the next hunt"))

	movement := "hold-to-walk"
	if gs.ClickToToggle {
		movement = "click-to-toggle"
	}
	root.AddItem(setupWizardText(fmt.Sprintf(
		"Movement: %s\nGame rendering: %.0fx\nDithering blend: %s\nSmooth motion: %s",
		movement,
		gs.GameScale,
		onOff(gs.DenoiseImages),
		onOff(gs.MotionSmoothing),
	), 12, 620))
	root.AddItem(setupWizardText(
		"Finish saves these choices and records this goThoom release as reviewed. Reopen this tour any time with the Setup Wizard button in Settings; the full Settings and Quality windows contain the finer controls.",
		12, 620,
	))
}

func setupWizardNavigation() *eui.ItemData {
	row := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_HORIZONTAL, Fixed: true, Alignment: eui.ALIGN_RIGHT}
	row.Size = eui.Point{X: 620, Y: 30}

	fund, fundEvents := eui.NewButton()
	fund.Text = "Fund development on Ko-fi"
	fund.Size = eui.Point{X: 185, Y: 24}
	fundEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			browser.OpenURL(setupWizardKoFiURL)
		}
	}
	row.AddItem(fund)

	if setupWizardPage > 0 {
		back, backEvents := eui.NewButton()
		back.Text = "Back"
		back.Size = eui.Point{X: 90, Y: 24}
		backEvents.Handle = func(ev eui.UIEvent) {
			if ev.Type == eui.EventClick {
				setupWizardPage--
				rebuildSetupWizard()
			}
		}
		row.AddItem(back)
	} else {
		spacer, _ := eui.NewText()
		spacer.Size = eui.Point{X: 90, Y: 24}
		spacer.Fixed = true
		row.AddItem(spacer)
	}

	skip, skipEvents := eui.NewButton()
	skip.Text = "Skip"
	skip.Size = eui.Point{X: 150, Y: 24}
	skipEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			completeSetupWizard()
		}
	}
	row.AddItem(skip)

	next, nextEvents := eui.NewButton()
	next.Size = eui.Point{X: 100, Y: 24}
	if setupWizardPage == 0 {
		next.Text = "Setup Wizard"
		next.Size.X = 130
		next.Color = eui.ColorDarkOrange
		next.HoverColor = eui.ColorOrange
		next.TextColor = eui.ColorWhite
		next.ForceTextColor = true
	} else if setupWizardPage == setupWizardPageCount-1 {
		next.Text = "Finish"
	} else {
		next.Text = "Next"
	}
	nextEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type != eui.EventClick {
			return
		}
		if setupWizardPage == setupWizardPageCount-1 {
			completeSetupWizard()
			return
		}
		setupWizardPage++
		rebuildSetupWizard()
	}
	row.AddItem(next)
	if setupWizardWin != nil {
		setupWizardWin.DefaultButton = next
	}
	return row
}

func completeSetupWizard() {
	stopSetupWizardPreview()
	if appVersion > gs.SetupWizardVersion {
		gs.SetupWizardVersion = appVersion
	}
	settingsDirty = true
	saveSettings()
	if setupWizardWin != nil {
		setupWizardWin.Close()
	}
	// The preview normally restores Login itself, but it cannot do so when the
	// preview was unavailable (such as on a fresh install before assets load).
	// Completing the wizard should always return an offline player to Login.
	if tcpConn == nil && clmov == "" && pcapPath == "" && !fake && loginWin != nil {
		loginWin.MarkOpen()
	}
	rebuildSettingsAfterSetupWizard()
}

func rebuildSettingsAfterSetupWizard() {
	if settingsWin != nil {
		wasOpen := settingsWin.IsOpen()
		settingsWin.RemoveWindow()
		settingsWin = nil
		makeSettingsWindow()
		if wasOpen {
			settingsWin.MarkOpen()
		}
	}
	if advancedWin != nil {
		wasOpen := advancedWin.IsOpen()
		advancedWin.RemoveWindow()
		advancedWin = nil
		makeAdvancedSettingsWindow()
		if wasOpen {
			advancedWin.MarkOpen()
		}
	}
	if qualityWin != nil {
		wasOpen := qualityWin.IsOpen()
		qualityWin.RemoveWindow()
		qualityWin = nil
		makeQualityWindow()
		if wasOpen {
			qualityWin.MarkOpen()
		}
	}
}

func markQualityCustom() {
	settingsDirty = true
	if qualityPresetDD != nil {
		qualityPresetDD.Selected = detectQualityPreset()
	}
	if qualityWin != nil {
		qualityWin.Refresh()
	}
}

func setupWizardCheckbox(label, explanation string, checked bool, changed func(bool)) *eui.ItemData {
	flow := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL}
	flow.Size = eui.Point{X: 620, Y: 10}
	checkbox, events := eui.NewCheckbox()
	checkbox.Text = label
	checkbox.Checked = checked
	checkbox.Size = eui.Point{X: 610, Y: 24}
	events.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged && changed != nil {
			changed(ev.Checked)
		}
	}
	flow.AddItem(checkbox)
	description := setupWizardText(explanation, 10, 590)
	description.Position.X = 20
	flow.AddItem(description)
	return flow
}

func setupWizardSlider(label, explanation string, minValue, maxValue, value float32, intOnly bool, changed func(float32)) *eui.ItemData {
	flow := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL}
	flow.Size = eui.Point{X: 620, Y: 10}
	slider, events := eui.NewSlider()
	slider.Label = label
	slider.MinValue = minValue
	slider.MaxValue = maxValue
	slider.Value = value
	slider.IntOnly = intOnly
	slider.Size = eui.Point{X: 600, Y: 24}
	events.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventSliderChanged && changed != nil {
			changed(ev.Value)
		}
	}
	flow.AddItem(slider)
	description := setupWizardText(explanation, 10, 590)
	description.Position.X = 20
	flow.AddItem(description)
	return flow
}

func setupWizardSubOption(item *eui.ItemData) *eui.ItemData {
	item.Position.X += 20
	if item.Size.X > 20 {
		item.Size.X -= 20
	}
	for _, child := range item.Contents {
		if child.Size.X > 20 {
			child.Size.X -= 20
		}
	}
	return item
}

func setSetupWizardDisabled(item *eui.ItemData, disabled bool) {
	for _, child := range item.Contents {
		child.Disabled = disabled
	}
}

func setupWizardGammaSlider(label, explanation string, target *float64) *eui.ItemData {
	flow := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL}
	flow.Size = eui.Point{X: 620, Y: 10}
	slider, events := eui.NewSlider()
	slider.Label = label
	slider.MinValue = float32(gammaOptions[0])
	slider.MaxValue = float32(gammaOptions[len(gammaOptions)-1])
	slider.Value = float32(*target)
	slider.Size = eui.Point{X: 600, Y: 24}
	events.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventSliderChanged {
			*target = normalizeGamma(float64(ev.Value), *target)
			ev.Item.Value = float32(*target)
			ev.Item.Dirty = true
			applySetupWizardGamma()
		}
	}
	flow.AddItem(slider)
	description := setupWizardText(explanation, 10, 590)
	description.Position.X = 20
	flow.AddItem(description)
	return flow
}

func setupWizardHeading(title string) *eui.ItemData {
	heading := setupWizardText(title, 18, 620)
	applyBoldFace(heading)
	return heading
}

func setupWizardText(body string, fontSize, width float32) *eui.ItemData {
	item, _ := eui.NewText()
	fontSize *= 1.1
	item.FontSize = fontSize
	faceSize := float64(fontSize*eui.UIScale() + 2)
	if src := eui.FontSource(); src != nil {
		item.Face = &text.GoTextFace{Source: src, Size: faceSize}
	} else {
		item.Face = &text.GoTextFace{Size: faceSize}
	}
	_, lines := wrapText(body, item.Face, float64(width*eui.UIScale()))
	if len(lines) == 0 {
		lines = []string{""}
	}
	item.Text = strings.Join(lines, "\n")
	metrics := item.Face.Metrics()
	lineHeight := math.Ceil(metrics.HAscent + metrics.HDescent + 2)
	item.Size = eui.Point{X: width, Y: float32(lineHeight*float64(len(lines)))/eui.UIScale() + 4}
	item.Fixed = true
	return item
}

func onOff(enabled bool) string {
	if enabled {
		return "on"
	}
	return "off"
}
