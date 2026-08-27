package main

import (
	"fmt"
	"math"
	"strings"
	"time"

	"gothoom/eui"

	text "github.com/hajimehoshi/ebiten/v2/text/v2"
)

const (
	setupWizardPageCount             = 9
	setupWizardGraphicsBenchmarkPage = 1
	setupWizardTopGap                = 24
)

var (
	setupWizardWin  *eui.WindowData
	setupWizardRoot *eui.ItemData
	setupWizardPage int

	setupWizardPreviewActive bool
	setupWizardPreviewLogin  bool
	setupWizardPreviewCrypt  bool
	setupWizardPreviewBubble bool
	setupWizardPreviewNight  movieNightState

	setupWizardGraphicsRecommendation string
	setupWizardGraphicsTested         bool
	setupWizardGraphicsPending        bool
	setupWizardGraphicsStarted        time.Time
	setupWizardGraphicsFPSSum         float64
	setupWizardGraphicsFPSCount       int
	setupWizardVSyncBypass            bool
)

func shouldShowSetupWizard(configLoaded bool, completedVersion, currentVersion int) bool {
	return !configLoaded || completedVersion < currentVersion
}

func openSetupWizard(force bool) {
	if !force && !shouldShowSetupWizard(settingsLoaded, gs.SetupWizardVersion, appVersion) {
		return
	}
	setupWizardPage = 0
	setupWizardScenePage = -1
	setupWizardSceneStarted = time.Time{}
	setupWizardGraphicsRecommendation = ""
	setupWizardGraphicsTested = false
	setupWizardGraphicsPending = false
	setupWizardGraphicsStarted = time.Time{}
	setupWizardGraphicsFPSSum = 0
	setupWizardGraphicsFPSCount = 0
	setupWizardVSyncBypass = false
	if setupWizardWin == nil {
		setupWizardWin = eui.NewWindow()
		setupWizardWin.Closable = false
		setupWizardWin.Resizable = false
		setupWizardWin.AutoSize = true
		setupWizardWin.Movable = true
		setupWizardWin.SetRefreshInterval(100 * time.Millisecond)
		setupWizardWin.Padding = 10
		setupWizardWin.BorderPad = 4
		setupWizardWin.SetZone(eui.HZoneLeft, eui.VZoneTop)
		setupWizardWin.AddWindow(false)
		setupWizardWin.ClearZone()
		_ = setupWizardWin.SetPos(eui.Point{X: 0, Y: setupWizardTopGap})
	}
	rebuildSetupWizard()
	setupWizardWin.MarkOpen()
	// Assets can finish loading after startup settings were first applied.
	// Reapply everything before the preview uses the normal game renderer.
	applySettings()
	startSetupWizardPreview()
}

func startSetupWizardPreview() {
	if setupWizardPreviewActive || tcpConn != nil || clmov != "" || playingMovie || pcapPath != "" || fake || clImages == nil {
		return
	}
	setupWizardPreviewLogin = loginWin != nil && loginWin.IsOpen()
	setupWizardPreviewCrypt = drawStateEncrypted
	setupWizardPreviewBubble = blockBubbles
	setupWizardPreviewNight = captureMovieNightState()
	setupWizardPreviewActive = true
	// The synthetic visibility scene supplies its own deterministic bubble.
	blockBubbles = false
	drawStateEncrypted = false
	if loginWin != nil {
		loginWin.Close()
	}
}

func stopSetupWizardPreview() {
	if !setupWizardPreviewActive {
		return
	}
	setupWizardPreviewActive = false
	blockBubbles = setupWizardPreviewBubble
	drawStateEncrypted = setupWizardPreviewCrypt
	restoreMovieNightState(setupWizardPreviewNight)
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
	if shouldStartSetupWizardGraphicsDetection(setupWizardPage, setupWizardGraphicsPending, setupWizardGraphicsTested) {
		startSetupWizardGraphicsDetection()
	}
	selectSetupWizardSceneForPage(setupWizardPage)
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
		buildSetupGraphicsPage(root)
	case 2:
		buildSetupInterfacePage(root)
	case 3:
		buildSetupControlsPage(root)
	case 4:
		buildSetupMotionPage(root)
	case 5:
		buildSetupShadowsPage(root)
	case 6:
		buildSetupNightLightingPage(root)
	case 7:
		buildSetupAudioPage(root)
	case 8:
		buildSetupFinishPage(root)
	}

	root.AddItem(setupWizardNavigation())
	if setupWizardGraphicsPending {
		setSetupWizardDisabled(root, true)
		detail := setupWizardText("Testing Full Quality with the running game for five seconds. The wizard will unlock when detection finishes.", 11, 620)
		heading := setupWizardHeading("Auto-adjusting performance…")
		root.PrependItem(detail)
		root.PrependItem(heading)
	}
	if setupWizardRoot == nil {
		setupWizardWin.AddItem(root)
	} else {
		setupWizardWin.ReplaceItem(0, root)
	}
	setupWizardRoot = root
	setupWizardWin.Refresh()
}

func shouldStartSetupWizardGraphicsDetection(page int, pending, tested bool) bool {
	return page == setupWizardGraphicsBenchmarkPage && !pending && !tested
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
		"We will review graphics, interface layout, controls, motion, shadows, night lighting, and audio. You can skip at any point.",
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

func buildSetupInterfacePage(root *eui.ItemData) {
	root.AddItem(setupWizardHeading("Interface and readability"))
	root.AddItem(setupWizardText("Set up the main layout and the information drawn over the game. Detailed sizing, opacity, and window controls remain in Settings.", 11, 620))

	toolbar, toolbarEvents := eui.NewDropdown()
	toolbar.Label = "Toolbar placement"
	toolbar.Options = []string{"Inside Inventory", "Inside Players"}
	if !gs.TiledWindows {
		toolbar.Options = append(toolbar.Options, "Floating Window")
	}
	toolbar.Selected = int(gs.ToolbarPlacement)
	toolbar.Size = eui.Point{X: 320, Y: 24}
	toolbarEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventDropdownSelected && ev.Index >= int(ToolbarInInventory) && ev.Index < len(toolbar.Options) {
			placeToolbar(ToolbarPlacement(ev.Index), true)
		}
	}
	root.AddItem(toolbar)
	root.AddItem(setupWizardCheckbox("Show toolbar info bar", "Show FPS, packet loss, ping, and jitter below the toolbar when it is docked.", gs.ToolbarInfoBar, func(checked bool) {
		gs.ToolbarInfoBar = checked
		placeToolbar(gs.ToolbarPlacement, true)
	}))

	root.AddItem(setupWizardCheckbox("Tiled window mode", "Arrange the main windows as one tiled workspace.", gs.TiledWindows, func(checked bool) {
		gs.TiledWindows = checked
		applyTiledWorkspaceLayout()
	}))

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

	healthDisplay, healthDisplayEvents := eui.NewDropdown()
	healthDisplay.Label = "Player health display"
	healthDisplay.Options = []string{"Color bar", "Classic name color"}
	if !gs.NameHealthBarModern {
		healthDisplay.Selected = 1
	}
	healthDisplay.Size = eui.Point{X: 320, Y: 24}
	healthDisplayEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventDropdownSelected && ev.Index >= 0 && ev.Index <= 1 {
			gs.NameHealthBarModern = ev.Index == 0
			killNameTagCache()
			settingsDirty = true
		}
	}
	root.AddItem(healthDisplay)

	root.AddItem(setupWizardCheckbox("Dark mode names/bubbles", "Use dark backgrounds with light text for speech bubbles and character names.", gs.DarkBubblesAndNames, func(checked bool) {
		gs.DarkBubblesAndNames = checked
		killNameTagCache()
		settingsDirty = true
	}))
	root.AddItem(setupWizardCheckbox("Speech bubbles", "Show spoken text over characters as well as in chat.", gs.SpeechBubbles, func(checked bool) {
		gs.SpeechBubbles = checked
		settingsDirty = true
	}))
	root.AddItem(setupWizardCheckbox("Fade obscuring objects", "Fade foreground artwork when it covers a character.", gs.FadeObscuringPictures, func(checked bool) {
		gs.FadeObscuringPictures = checked
		settingsDirty = true
	}))
}

func buildSetupGraphicsPage(root *eui.ItemData) {
	if gs.GameScale < 2 {
		gs.GameScale = 2
		gs.SpriteUpscale = spriteUpscaleFactor()
		clearCaches()
		initFont()
		settingsDirty = true
	}
	root.AddItem(setupWizardHeading("Graphics and performance"))
	root.AddItem(setupWizardText(
		"Your current graphics choices are selected below. Adjust only what you want while watching the real renderer behind this window.",
		11, 620,
	))
	root.AddItem(setupWizardText("Watch the movie behind this window while changing these options; it uses the real game renderer at your current game-window scale.", 10, 620))

	graphicsTest, graphicsTestEvents := eui.NewButton()
	graphicsTest.Text = "Rerun Graphics Detection"
	graphicsTest.Size = eui.Point{X: 240, Y: 24}
	graphicsTest.Disabled = isWASM
	graphicsTest.SetTooltip("Measure FPS and choose a preset.")
	graphicsRecommendation := setupWizardText(setupWizardGraphicsRecommendation, 10, 350)
	graphicsRecommendation.Size.Y = 24
	graphicsTestEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type != eui.EventClick {
			return
		}
		startSetupWizardGraphicsDetection()
		rebuildSetupWizard()
	}
	graphicsTestRow := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_HORIZONTAL, Fixed: true}
	graphicsTestRow.Size = eui.Point{X: 620, Y: 28}
	graphicsTestRow.AddItem(graphicsTest)
	graphicsTestRow.AddItem(graphicsRecommendation)
	root.AddItem(graphicsTestRow)

	graphicsMode, graphicsModeEvents := eui.NewDropdown()
	graphicsMode.Label = "Graphics performance mode"
	graphicsMode.Options = []string{"iGPU Graphics", "Full Quality"}
	graphicsMode.Selected = 1
	if igpuGraphicsPresetApplied() {
		graphicsMode.Selected = 0
	}
	graphicsMode.Size = eui.Point{X: 320, Y: 24}
	graphicsMode.SetTooltip("Choose the graphics mode.")
	graphicsModeEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type != eui.EventDropdownSelected || ev.Index < 0 || ev.Index > 1 {
			return
		}
		preset := "Full Graphics"
		if ev.Index == 0 {
			preset = "iGPU Graphics"
		}
		applyQualityPreset(preset)
		rebuildSetupWizard()
	}
	root.AddItem(graphicsMode)

	root.AddItem(setupWizardSlider("Max Upscale", "Sets the maximum artwork upscale from 2x to 4x. Actual texture resolution is capped to twice its fitted on-screen size.", 2, 4, float32(math.Round(gs.GameScale)), true, func(value float32) {
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
		"Smooths irregular nearby palette colors while preserving pixel-art edges, lines, and isolated details.",
		gs.DenoiseImages,
		func(checked bool) {
			gs.DenoiseImages = checked
			if clImages != nil {
				clImages.SetDenoise(gs.DenoiseImages, gs.DenoiseSharpness, gs.DenoiseAmount)
			}
			if denoiseCB != nil {
				denoiseCB.Checked = checked
			}
			clearCaches()
			markQualityCustom()
		},
	))
	upscaleStyle, upscaleEvents := eui.NewDropdown()
	upscaleStyle.Label = "Artwork upscale style"
	upscaleStyle.Options = artworkUpscaleModeNames
	upscaleStyle.Selected = artworkUpscaleMode()
	upscaleStyle.Size = eui.Point{X: 320, Y: 24}
	upscaleStyle.SetTooltip("Choose edge smoothing.")
	upscaleEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type != eui.EventDropdownSelected || ev.Index < artworkUpscaleOff || ev.Index > artworkUpscaleUltraSmooth {
			return
		}
		if artworkUpscaleMode() == ev.Index {
			return
		}
		setArtworkUpscaleMode(ev.Index)
		if upscaleModeDD != nil {
			upscaleModeDD.Selected = artworkUpscaleMode()
		}
		clearCaches()
		markQualityCustom()
	}
	root.AddItem(upscaleStyle)
	wizardVSync := setupWizardCheckbox("VSync", "VSync is temporarily bypassed only during the five-second graphics benchmark. Your saved setting applies normally afterward.", effectiveVSyncEnabled(), func(checked bool) {
		gs.VSync = checked
		applyVSyncSetting()
		settingsDirty = true
	})
	setSetupWizardDisabled(wizardVSync, setupWizardVSyncBypass)
	root.AddItem(wizardVSync)
}

func updateSetupWizardGraphicsDetection() {
	if !setupWizardGraphicsPending {
		return
	}
	now := time.Now()
	if setupWizardGraphicsStarted.IsZero() {
		setupWizardGraphicsStarted = now
		return
	}
	// Give the preview and uncapped presentation a second to settle, then
	// observe the complete renderer for the remaining four seconds.
	if now.Sub(setupWizardGraphicsStarted) >= time.Second {
		result, err := runGraphicsBenchmark()
		if err == nil {
			setupWizardGraphicsFPSSum += result.ActualFPS
			setupWizardGraphicsFPSCount++
		}
	}
	if now.Sub(setupWizardGraphicsStarted) < 5*time.Second {
		return
	}

	setupWizardGraphicsPending = false
	setupWizardGraphicsTested = true
	if setupWizardGraphicsFPSCount == 0 {
		setupWizardGraphicsRecommendation = "Detection failed"
	} else {
		fps := setupWizardGraphicsFPSSum / float64(setupWizardGraphicsFPSCount)
		applySetupWizardGraphicsRecommendation(graphicsBenchmarkResult{
			ActualFPS:     fps,
			RecommendIGPU: recommendIGPUGraphics(fps),
		})
	}
	setupWizardVSyncBypass = false
	applyVSyncSetting()
	rebuildSetupWizard()
}

func startSetupWizardGraphicsDetection() {
	if isWASM {
		return
	}
	// Always measure the same workload. Testing whatever preset happened to
	// be active would make results incomparable and could hide a slow GPU.
	applyQualityPreset("Full Graphics")
	setupWizardVSyncBypass = true
	applyVSyncSetting()
	setupWizardGraphicsPending = true
	setupWizardGraphicsTested = false
	setupWizardGraphicsStarted = time.Time{}
	setupWizardGraphicsFPSSum = 0
	setupWizardGraphicsFPSCount = 0
	setupWizardGraphicsRecommendation = ""
}

func applySetupWizardGraphicsRecommendation(result graphicsBenchmarkResult) {
	setupWizardGraphicsRecommendation = fmt.Sprintf("%s (%.0f FPS)", graphicsBenchmarkRecommendedLabel(result), result.ActualFPS)
	applyQualityPreset(graphicsBenchmarkRecommendedPreset(result))
}

func igpuGraphicsPresetApplied() bool {
	return gs.MotionSmoothing && !gs.BlendMobiles && !gs.BlendPicts && !gs.ShaderLighting &&
		gs.GameScale == 2 && !gs.DenoiseImages &&
		!gs.WindowShadows && !gs.CharacterShadows && !gs.AnimatedChatBubbles &&
		artworkUpscaleMode() == artworkUpscaleBalanced
}

func buildSetupMotionPage(root *eui.ItemData) {
	root.AddItem(setupWizardHeading("Motion and animation"))
	root.AddItem(setupWizardText("Smooth movement reduces visible stepping between server updates. Animation blending softens sprite-frame changes and can use more GPU.", 11, 620))
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
	addFrameBlendOption(setupWizardCheckbox("World animation blending", "Blends animated water, grass, and other world artwork.", gs.BlendPicts, func(checked bool) {
		gs.BlendPicts = checked
		clearCaches()
		markQualityCustom()
	}))
}

func buildSetupShadowsPage(root *eui.ItemData) {
	root.AddItem(setupWizardHeading("Daylight and character shadows"))
	root.AddItem(setupWizardText(
		"Use the daylight riverbank to compare character shadows, including the guide shading the nearby apprentice.",
		11, 620,
	))

	var shadowOptions []*eui.ItemData
	root.AddItem(setupWizardCheckbox("Character shadows", "Show shadows beneath characters and creatures.", gs.CharacterShadows, func(checked bool) {
		gs.CharacterShadows = checked
		for _, option := range shadowOptions {
			setSetupWizardDisabled(option, !checked)
		}
		markQualityCustom()
		if setupWizardWin != nil {
			setupWizardWin.Refresh()
		}
	}))
	addShadowOption := func(option *eui.ItemData) {
		option = setupWizardSubOption(option)
		setSetupWizardDisabled(option, !gs.CharacterShadows)
		shadowOptions = append(shadowOptions, option)
		root.AddItem(option)
	}
	addShadowOption(setupWizardCheckbox("Accurate character shadows", "Keep overlapping shadows from becoming too dark.", gs.DetailedCharacterShadows, func(checked bool) {
		gs.DetailedCharacterShadows = checked
		settingsDirty = true
	}))
	addShadowOption(setupWizardCheckbox("Mobiles receive sun shadows", "Darken a mobile standing in another mobile's shadow.", gs.MobilesReceiveSunShadows, func(checked bool) {
		gs.MobilesReceiveSunShadows = checked
		settingsDirty = true
	}))
	root.AddItem(setupWizardCheckbox("Sprite gamma correction", "Adjust classic artwork for modern displays.", gs.SpriteGammaCorrection, func(checked bool) {
		gs.SpriteGammaCorrection = checked
		if gammaCorrectionCB != nil {
			gammaCorrectionCB.Checked = checked
		}
		applySetupWizardGamma()
	}))
}

func buildSetupNightLightingPage(root *eui.ItemData) {
	root.AddItem(setupWizardHeading("Night lighting"))
	root.AddItem(setupWizardText(
		"This preview stays at full night so darkness, lantern light, glow, and flicker can be compared directly.",
		11, 620,
	))

	root.AddItem(setupWizardSlider("Maximum night darkness", "Limit how dark the world may become.", 0, 100, float32(gs.MaxNightLevel), true, func(value float32) {
		gs.MaxNightLevel = int(value)
		settingsDirty = true
	}))

	var shaderOptions []*eui.ItemData
	root.AddItem(setupWizardCheckbox("Shader lighting effects", "Use colored lights, glow, and light cones.", gs.ShaderLighting, func(checked bool) {
		gs.ShaderLighting = checked
		for _, option := range shaderOptions {
			setSetupWizardDisabled(option, !checked)
		}
		if shaderLightingCB != nil {
			shaderLightingCB.Checked = checked
		}
		if shaderLightSlider != nil {
			shaderLightSlider.Disabled = !checked
		}
		if shaderGlowSlider != nil {
			shaderGlowSlider.Disabled = !checked
		}
		if flameFlickerCB != nil {
			flameFlickerCB.Disabled = !checked
		}
		if flameFlickerSlider != nil {
			flameFlickerSlider.Disabled = !checked || !gs.FlameLightFlicker
		}
		markQualityCustom()
		if setupWizardWin != nil {
			setupWizardWin.Refresh()
		}
	}))
	addShaderOption := func(option *eui.ItemData) {
		option = setupWizardSubOption(option)
		setSetupWizardDisabled(option, !gs.ShaderLighting)
		shaderOptions = append(shaderOptions, option)
		root.AddItem(option)
	}
	addShaderOption(setupWizardSlider("Light strength", "Set lantern and colored-light intensity.", 0, 200, float32(gs.ShaderLightStrength*100), true, func(value float32) {
		gs.ShaderLightStrength = float64(value / 100)
		if shaderLightSlider != nil {
			shaderLightSlider.Value = value
			shaderLightSlider.Dirty = true
		}
		settingsDirty = true
	}))
	addShaderOption(setupWizardSlider("Glow strength", "Set the halo around light sources.", 0, 200, float32(gs.ShaderGlowStrength*100), true, func(value float32) {
		gs.ShaderGlowStrength = float64(value / 100)
		if shaderGlowSlider != nil {
			shaderGlowSlider.Value = value
			shaderGlowSlider.Dirty = true
		}
		settingsDirty = true
	}))
	addShaderOption(setupWizardCheckbox("Flame light flicker", "Animate lantern-light intensity.", gs.FlameLightFlicker, func(checked bool) {
		gs.FlameLightFlicker = checked
		if flameFlickerCB != nil {
			flameFlickerCB.Checked = checked
		}
		if flameFlickerSlider != nil {
			flameFlickerSlider.Disabled = !checked || !gs.ShaderLighting
		}
		settingsDirty = true
	}))
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
		"Choose audio enhancements and whether to spend extra memory preparing sounds before play. Detailed volume controls remain in Settings.",
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
	root.AddItem(setupWizardRecommendedCheckbox("Precache sounds", "Warm game sounds in the background to reduce first-use delay. This uses roughly 300 MB more RAM.", gs.PrecacheSounds, defaultPrecacheSounds, func(checked bool) {
		gs.PrecacheSounds = checked
		if checked && !soundsPrecached.Load() {
			go precacheSounds()
		}
		settingsDirty = true
	}))
}

func buildSetupFinishPage(root *eui.ItemData) {
	root.AddItem(setupWizardHeading("Ready for the next hunt"))

	movement := "hold-to-walk"
	if gs.ClickToToggle {
		movement = "click-to-toggle"
	}
	root.AddItem(setupWizardText(fmt.Sprintf(
		"Movement: %s\nMax upscale: %.0fx\nToolbar: %s\nSmooth motion: %s\nMaximum night: %d%%\nShader lighting: %s",
		movement,
		gs.GameScale,
		setupWizardToolbarPlacementName(gs.ToolbarPlacement),
		onOff(gs.MotionSmoothing),
		gs.MaxNightLevel,
		onOff(gs.ShaderLighting),
	), 12, 620))
	root.AddItem(setupWizardText(
		"Finish saves these choices and records this goThoom release as reviewed. Reopen this tour any time with the Setup Wizard button in Settings; the full Settings and Quality windows contain the finer controls.",
		12, 620,
	))
}

func setupWizardNavigation() *eui.ItemData {
	row := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_HORIZONTAL, Fixed: true, Alignment: eui.ALIGN_RIGHT}
	row.Size = eui.Point{X: 620, Y: 30}

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
	skip.Text = "Skip Tour"
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
		next.Text = "Start"
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
	setupWizardVSyncBypass = false
	applyVSyncSetting()
	rebuildSettingsAfterSetupWizard()
	// The preview normally restores Login itself, but it cannot do so when the
	// preview was unavailable. Rebuild cached character images and always return
	// an offline player to Login, including the first-run download path.
	refreshLoginAfterAssetsAvailable()
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

func setupWizardRecommendedCheckbox(label, explanation string, checked, recommended bool, changed func(bool)) *eui.ItemData {
	flow := setupWizardCheckbox(label, explanation, checked, changed)
	if !recommended || len(flow.Contents) == 0 {
		return flow
	}

	checkbox := flow.Contents[0]
	flow.RemoveItem(checkbox)
	checkbox.Size.X = 480
	row := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_HORIZONTAL, Fixed: true}
	row.Size = eui.Point{X: 610, Y: 24}
	row.AddItem(checkbox)

	pill := setupWizardText("  Recommended", 9, 104)
	pill.Size.Y = 20
	pill.Position.Y = 2
	pill.Filled = true
	pill.Fillet = 10
	pill.Color = eui.ColorDarkGreen
	pill.TextColor = eui.ColorWhite
	pill.ForceTextColor = true
	row.AddItem(pill)
	flow.PrependItem(row)
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
	if item == nil {
		return
	}
	item.Disabled = disabled
	for _, child := range item.Contents {
		setSetupWizardDisabled(child, disabled)
	}
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

func setupWizardToolbarPlacementName(placement ToolbarPlacement) string {
	switch placement {
	case ToolbarInPlayers:
		return "inside Players"
	case ToolbarFloating:
		return "floating window"
	default:
		return "inside Inventory"
	}
}
