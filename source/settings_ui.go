package main

import (
	"fmt"
	"math"
	"os"
	"time"

	"gothoom/eui"

	"github.com/hajimehoshi/ebiten/v2"
	open "github.com/skratchdot/open-golang/open"
)

const (
	settingsControlHeight float32 = 28
	settingsPanelWidth    float32 = 690
	settingsWindowWidth   float32 = 730
	settingsWindowHeight  float32 = 700
)

func selectedSettingsTab() string {
	if settingsWin != nil && len(settingsWin.Contents) > 0 {
		flow := settingsWin.Contents[0]
		if flow.ActiveTab >= 0 && flow.ActiveTab < len(flow.Tabs) {
			return flow.Tabs[flow.ActiveTab].Name
		}
	}
	return ""
}

func selectSettingsTab(name string) {
	if settingsWin == nil || len(settingsWin.Contents) == 0 {
		return
	}
	flow := settingsWin.Contents[0]
	for index, tab := range flow.Tabs {
		if tab.Name == name {
			flow.ActiveTab = index
			flow.Scroll = eui.Point{}
			settingsWin.Refresh()
			return
		}
	}
}

func newSettingsPage(name string, width float32) *eui.ItemData {
	return &eui.ItemData{Name: name, ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL, Size: eui.Point{X: width, Y: 10}}
}

func addSettingsSection(page *eui.ItemData, title string, width float32) *eui.ItemData {
	section := eui.NewSection(title, width)
	if len(page.Contents) == 0 {
		section.Contents = section.Contents[1:]
	}
	page.AddItem(section)
	return section
}

func makeSettingsWindow() {
	if settingsWin != nil {
		return
	}
	settingsWin = eui.NewWindow()
	settingsWin.ShowTooltipIndicators = true
	settingsWin.Title = fmt.Sprintf("Settings -- goThoom test %d", appVersion)
	settingsWin.Closable = true
	settingsWin.Resizable = false
	settingsWin.Size = eui.Point{X: settingsWindowWidth, Y: settingsWindowHeight}
	settingsWin.Movable = true
	settingsWin.Padding = 12
	settingsWin.SetRefreshInterval(100 * time.Millisecond)

	const panelWidth = settingsPanelWidth
	outer := &eui.ItemData{
		ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL,
		ActiveOutline: true,
	}
	displayPage := newSettingsPage("Display", panelWidth)
	worldPage := newSettingsPage("World", panelWidth)
	textPage := newSettingsPage("Text", panelWidth)
	bubblesPage := newSettingsPage("Bubbles", panelWidth)
	audioPage := newSettingsPage("Audio", panelWidth)
	ttsPage := newSettingsPage("TTS", panelWidth)
	controlsPage := newSettingsPage("Controls", panelWidth)
	performancePage := newSettingsPage("Performance", panelWidth)
	networkPage := newSettingsPage("Network", panelWidth)
	filesPage := newSettingsPage("Files", panelWidth)
	toolsPage := newSettingsPage("Tools", panelWidth)

	const displayColumnWidth = (panelWidth - 20) / 2
	displayColumns := eui.NewRow()
	displayWindowPage := newSettingsPage("", displayColumnWidth)
	displayLayoutPage := newSettingsPage("", displayColumnWidth)
	displayColumns.AddItem(displayWindowPage)
	displayColumns.AddItem(&eui.ItemData{ItemType: eui.ITEM_TEXT, Size: eui.Point{X: 20, Y: 1}})
	displayColumns.AddItem(displayLayoutPage)
	displayPage.AddItem(displayColumns)
	windowSection := addSettingsSection(displayWindowPage, "Window & Display", displayColumnWidth)
	layoutSection := addSettingsSection(displayLayoutPage, "Windows & Toolbar", displayColumnWidth)
	windowsSection := addSettingsSection(displayLayoutPage, "Show / Hide Windows", displayColumnWidth)
	appearanceSection := addSettingsSection(displayPage, "Appearance", panelWidth)
	textSizeSection := addSettingsSection(textPage, "Text Sizes", panelWidth)
	textColumns := eui.NewRow()
	textMessagesPage := newSettingsPage("", displayColumnWidth)
	textInputPage := newSettingsPage("", displayColumnWidth)
	textColumns.AddItem(textMessagesPage)
	textColumns.AddItem(&eui.ItemData{ItemType: eui.ITEM_TEXT, Size: eui.Point{X: 20, Y: 1}})
	textColumns.AddItem(textInputPage)
	textPage.AddItem(textColumns)
	chatSection := addSettingsSection(textMessagesPage, "Chat & Messages", displayColumnWidth)
	inputSection := addSettingsSection(textInputPage, "Input Assistance", displayColumnWidth)
	const worldColumnWidth float32 = (panelWidth - 20) / 2
	worldStatus := newSettingsPage("", worldColumnWidth)
	worldNames := newSettingsPage("", worldColumnWidth)
	worldColumns := eui.NewRow()
	worldColumns.AddItem(worldStatus)
	worldColumns.AddItem(&eui.ItemData{ItemType: eui.ITEM_TEXT, Size: eui.Point{X: 20, Y: 1}})
	worldColumns.AddItem(worldNames)
	worldPage.AddItem(worldColumns)
	statusSection := addSettingsSection(worldStatus, "Status Bars", worldColumnWidth)
	visibilitySection := addSettingsSection(worldStatus, "World Visibility", worldColumnWidth)
	nameSection := addSettingsSection(worldNames, "Character Names", worldColumnWidth)
	playersSection := addSettingsSection(worldNames, "Players List", worldColumnWidth)
	bubbleSection := addSettingsSection(bubblesPage, "Speech Bubbles", panelWidth)
	audioSection := addSettingsSection(audioPage, "Sound & Music", panelWidth)
	ttsSection := addSettingsSection(ttsPage, "Text to Speech", panelWidth)
	ttsTestSection := addSettingsSection(ttsPage, "Test & Corrections", panelWidth)
	notificationsSection := addSettingsSection(audioPage, "Notifications", panelWidth)
	controlsSection := addSettingsSection(controlsPage, "Movement & Input", panelWidth)
	qualitySection := addSettingsSection(performancePage, "Graphics Quality", panelWidth)
	networkSection := addSettingsSection(networkPage, nlsptExpandedName, panelWidth)
	filesSection := addSettingsSection(filesPage, "Files & Folders", panelWidth)
	recordingSection := addSettingsSection(filesPage, "Recordings", panelWidth)
	gettingStartedSection := addSettingsSection(toolsPage, "Setup", panelWidth)
	diagnosticsSection := addSettingsSection(toolsPage, "Diagnostics", panelWidth)
	resetSection := addSettingsSection(toolsPage, "Reset", panelWidth)

	tiledModeCB, tiledModeEvents := eui.NewCheckbox()
	tiledModeCB.Text = "Tiled window mode"
	tiledModeCB.Size = eui.Point{X: displayColumnWidth, Y: settingsControlHeight}
	tiledModeCB.Checked = gs.TiledWindows
	tiledModeCB.SetTooltip("Arrange the Game, Inventory, Players, Console, and Chat windows as one tiled workspace.")
	tiledModeEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			gs.TiledWindows = ev.Checked
			applyTiledWorkspaceLayout()
		}
	}
	settingsTiledModeCB = tiledModeCB
	layoutSection.AddItem(tiledModeCB)
	settingsCombineMessagesCB = newCombineMessagesCheckbox(displayColumnWidth)
	layoutSection.AddItem(settingsCombineMessagesCB)

	tiledLayoutBtn, tiledLayoutEvents := eui.NewButton()
	tiledLayoutBtn.Text = "Tiled Layout"
	setMaterialButtonIcon(tiledLayoutBtn, "dashboard_customize")
	tiledLayoutBtn.Size = eui.Point{X: (displayColumnWidth - 8) / 2, Y: settingsControlHeight}
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
	toolbarPlacementDD.Size = eui.Point{X: displayColumnWidth, Y: settingsControlHeight}
	toolbarPlacementDD.SetTooltip("Dock in Inventory or Players. Floating is unavailable while tiled mode is on.")
	toolbarPlacementEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventDropdownSelected {
			placeToolbar(ToolbarPlacement(ev.Index), true)
		}
	}
	layoutSection.AddItem(toolbarPlacementDD)

	toolbarInfoCB, toolbarInfoEvents := eui.NewCheckbox()
	toolbarInfoCB.Text = "Toolbar Info"
	toolbarInfoCB.Size = eui.Point{X: (displayColumnWidth - 8) / 2, Y: 16}
	toolbarInfoCB.Position.Y = 10
	toolbarInfoCB.Checked = gs.ToolbarInfoBar
	toolbarInfoCB.SetTooltip("Show network and performance stats.")
	toolbarInfoEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			gs.ToolbarInfoBar = ev.Checked
			placeToolbar(gs.ToolbarPlacement, true)
		}
	}
	layoutToolsRow := eui.NewRow()
	layoutToolsRow.AddItem(tiledLayoutBtn)
	layoutToolsRow.AddItem(toolbarInfoCB)
	layoutSection.AddItem(layoutToolsRow)

	// UI scale is always available: users need a direct recovery path if a
	// display's DPI report is unusual. Retina/HiDPI scaling is applied on top
	// of this preference automatically.
	uiScaleSlider, uiScaleEvents := eui.NewSlider()
	uiScaleLabel, _ := eui.NewText()
	uiScaleLabel.Text = "UI Scale"
	uiScaleLabel.FontSize = 12
	uiScaleLabel.Size = eui.Point{X: displayColumnWidth, Y: 20}
	uiScaleSlider.MinValue = 0.75
	uiScaleSlider.MaxValue = 4
	uiScaleSlider.Value = float32(gs.UIScale)
	uiScaleLabel.SetTooltip("Base UI size. Retina and other HiDPI displays are scaled automatically.")
	pendingUIScale := gs.UIScale
	uiScaleEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventSliderChanged {
			pendingUIScale = float64(ev.Value)
		}
	}

	uiScaleApplyBtn, uiScaleApplyEvents := eui.NewButton()
	uiScaleApplyBtn.Text = "Apply"
	setMaterialButtonIcon(uiScaleApplyBtn, "check_circle")
	uiScaleApplyBtn.Size = eui.Point{X: 80, Y: settingsControlHeight}
	uiScaleApplyEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			gs.UIScale = pendingUIScale
			eui.SetUserUIScale(float32(gs.UIScale))
			updateGameWindowSize()
			settingsDirty = true
		}
	}

	// Keep the label above both controls so Apply aligns with the slider track.
	windowSection.AddItem(uiScaleLabel)
	uiScaleRow := eui.NewRow()
	uiScaleSlider.Size = eui.Point{X: displayColumnWidth - uiScaleApplyBtn.Size.X - 10, Y: settingsControlHeight}
	uiScaleRow.AddItem(uiScaleSlider)
	uiScaleRow.AddItem(uiScaleApplyBtn)
	windowSection.AddItem(uiScaleRow)

	fullscreenCB, fullscreenEvents := eui.NewCheckbox()
	fullscreenCB.Text = "Fullscreen (F12)"
	fullscreenCB.Size = eui.Point{X: displayColumnWidth, Y: settingsControlHeight}
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
	styleDD.Size = eui.Point{X: displayColumnWidth, Y: settingsControlHeight}
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

	var accentSwatch *eui.ItemData

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
	themeDD.Size = eui.Point{X: displayColumnWidth, Y: settingsControlHeight}
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
				if accentSwatch != nil {
					var ac eui.Color
					_ = ac.UnmarshalJSON([]byte("\"accent\""))
					eui.SetColorSwatch(accentSwatch, ac)
				}
			}
		}
	}

	accentSwatch = newColorSwatch("Accent Color", eui.AccentColor(), func(col eui.Color) {
		_, saturation, _, _ := col.HSVA()
		eui.SetAccentSaturation(saturation)
		eui.SetAccentColor(col)
		refreshThemePreview()
		settingsDirty = true
	})

	bindThemePreview(themeDD, true, func() {
		for i, name := range styleDD.Options {
			if name == eui.CurrentStyleName() {
				styleDD.Selected = i
				break
			}
		}
		refreshThemePreview()
		eui.SetColorSwatch(accentSwatch, eui.AccentColor())
		settingsWin.Refresh()
	})
	bindThemePreview(styleDD, false, func() { settingsWin.Refresh() })
	themeDD.SetTooltip("Hover to preview a palette. Click to keep it; move away or press Escape to restore your choice.")
	styleDD.SetTooltip("Hover to preview control shapes and spacing. Click to keep the style.")

	themeRow := eui.NewRow()
	styleDD.Position.X = 24
	themeRow.AddItem(themeDD)
	themeRow.AddItem(styleDD)
	appearanceSection.AddItem(themeRow)
	accLabel, _ := eui.NewText()
	accLabel.Text = "Accent Color"
	accLabel.FontSize = 12
	accLabel.Size = eui.Point{X: panelWidth, Y: 20}
	appearanceSection.AddItem(accLabel)
	appearanceSection.AddItem(accentSwatch)

	toggle, toggleEvents := eui.NewCheckbox()
	toggle.Text = "Click-to-toggle movement"
	toggle.Size = eui.Point{X: panelWidth, Y: settingsControlHeight}
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
	qualityPresetDD.Label = "Quality preset"
	qualityPresetDD.Options = []string{"Lowest", "Low", "Medium", "High", "Ultra", "Custom"}
	qualityPresetDD.Size = eui.Point{X: 240, Y: settingsControlHeight}
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
	performanceOptions := newGraphicsPerformanceOptions()
	qualitySection.AddItem(shadersEnabledCB)
	qualitySection.AddItem(&eui.ItemData{ItemType: eui.ITEM_FLOW, Size: eui.Point{X: panelWidth, Y: 12}, Fixed: true})

	performancePage.AddItem(performanceOptions)

	inputOpenCB, inputOpenEvents := eui.NewCheckbox()
	inputOpenCB.Text = "Input bar always open"
	inputOpenCB.Size = eui.Point{X: panelWidth, Y: settingsControlHeight}
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
			updateMessageInputWindows()
			if consoleWin != nil {
				consoleWin.Refresh()
			}
			settingsDirty = true
		}
	}
	controlsSection.AddItem(inputOpenCB)

	chatTSCB, chatTSEvents := eui.NewCheckbox()
	chatTSCB.Text = "Chat timestamps"
	chatTSCB.Size = eui.Point{X: displayColumnWidth, Y: settingsControlHeight}
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
	consoleTSCB.Size = eui.Point{X: displayColumnWidth, Y: settingsControlHeight}
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
	notifCB.Size = eui.Point{X: panelWidth, Y: settingsControlHeight}
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
	notifBtn.Size = eui.Point{X: 200, Y: settingsControlHeight}
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
	textColorsBtn.Size = eui.Point{X: 180, Y: settingsControlHeight}
	textColorsEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			makeTextColorsWindow()
			textColorsWin.ToggleNear(ev.Item)
		}
	}
	chatSection.AddItem(textColorsBtn)

	appearanceSection.AddItem(eui.NewSubheading("Alternating row colors", panelWidth))
	var alternatingRow *eui.ItemData
	for i, option := range []struct {
		name  string
		value *bool
	}{
		{"Inventory", &gs.InventoryAlternatingRowColors},
		{"Chat", &gs.ChatAlternatingRowColors},
		{"Console", &gs.ConsoleAlternatingRowColors},
		{"Players", &gs.PlayersAlternatingRowColors},
	} {
		if i%2 == 0 {
			alternatingRow = eui.NewRow()
			appearanceSection.AddItem(alternatingRow)
		}
		checkbox, events := eui.NewCheckbox()
		checkbox.Text = option.name
		checkbox.Size = eui.Point{X: (panelWidth - 8) / 2, Y: settingsControlHeight}
		checkbox.Checked = *option.value
		events.Handle = func(ev eui.UIEvent) {
			if ev.Type != eui.EventCheckboxChanged {
				return
			}
			SettingsLock.Lock()
			*option.value = ev.Checked
			SettingsLock.Unlock()
			settingsDirty = true
			refreshMessageTextWindows()
			updateInventoryWindow()
			updatePlayersWindow()
		}
		alternatingRow.AddItem(checkbox)
	}

	placements := []struct {
		name  string
		value BarPlacement
	}{
		{"Along Bottom", BarPlacementBottom},
		{"Grouped Lower Left", BarPlacementLowerLeft},
		{"Grouped Lower Right", BarPlacementLowerRight},
		{"Grouped Upper Right", BarPlacementUpperRight},
	}
	statusSection.AddItem(eui.NewSubheading("Placement", worldColumnWidth))
	for _, p := range placements {
		p := p
		radio, radioEvents := eui.NewRadio()
		radio.Text = p.name
		radio.RadioGroup = "status-bar-placement"
		radio.Size = eui.Point{X: worldColumnWidth, Y: settingsControlHeight}
		radio.Checked = gs.BarPlacement == p.value
		radioEvents.Handle = func(ev eui.UIEvent) {
			if ev.Type == eui.EventRadioSelected {
				SettingsLock.Lock()
				gs.BarPlacement = p.value
				SettingsLock.Unlock()
				placeToolbar(gs.ToolbarPlacement, false)
				settingsDirty = true
			}
		}
		statusSection.AddItem(radio)
	}
	statusSection.AddItem(&eui.ItemData{ItemType: eui.ITEM_FLOW, Size: eui.Point{X: worldColumnWidth, Y: 8}, Fixed: true})
	toolbarBars, toolbarBarsEvents := eui.NewCheckbox()
	toolbarBars.Text = "Status bars below toolbar hands"
	toolbarBars.Size = eui.Point{X: worldColumnWidth, Y: settingsControlHeight}
	toolbarBars.Checked = gs.ToolbarStatusBars
	toolbarBars.SetTooltip("Show the status bars beneath the toolbar hand slots instead of in the game view.")
	toolbarBarsEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			SettingsLock.Lock()
			gs.ToolbarStatusBars = ev.Checked
			SettingsLock.Unlock()
			placeToolbar(gs.ToolbarPlacement, false)
			settingsDirty = true
		}
	}
	statusSection.AddItem(toolbarBars)

	barStyles := []struct {
		name  string
		value BarStyle
	}{
		{"Regular", BarStyleRegular},
		{"Modern -- thin", BarStyleCompact},
		{"Hidden", BarStyleHidden},
	}
	statusSection.AddItem(eui.NewSubheading("Style", worldColumnWidth))
	for _, style := range barStyles {
		style := style
		radio, radioEvents := eui.NewRadio()
		radio.Text = style.name
		radio.RadioGroup = "status-bar-style"
		radio.Size = eui.Point{X: worldColumnWidth, Y: settingsControlHeight}
		radio.Checked = gs.BarStyle == style.value
		radioEvents.Handle = func(ev eui.UIEvent) {
			if ev.Type == eui.EventRadioSelected {
				SettingsLock.Lock()
				defer SettingsLock.Unlock()

				gs.BarStyle = style.value
				settingsDirty = true
			}
		}
		statusSection.AddItem(radio)
	}

	barColorCB, barColorEvents := eui.NewCheckbox()
	barColorCB.Text = "Color bars by value"
	barColorCB.Size = eui.Point{X: worldColumnWidth, Y: settingsControlHeight}
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
	maxNightSlider.Size = eui.Point{X: worldColumnWidth - 10, Y: settingsControlHeight}
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
	nameBgSlider.Size = eui.Point{X: worldColumnWidth - 10, Y: settingsControlHeight}
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
	darkBubblesAndNamesCB.Size = eui.Point{X: 400, Y: settingsControlHeight}
	darkBubblesAndNamesCB.Checked = gs.DarkBubblesAndNames
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
	nameBorderCB.Size = eui.Point{X: worldColumnWidth - 10, Y: settingsControlHeight}
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
	healthBarStyleDD.Size = eui.Point{X: worldColumnWidth - 10, Y: settingsControlHeight}
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
	healthBarPositionDD.Size = eui.Point{X: worldColumnWidth - 10, Y: settingsControlHeight}
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
	healthBarThicknessSlider.Size = eui.Point{X: worldColumnWidth - 10, Y: settingsControlHeight}
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
	hideSelfNameCB.Size = eui.Point{X: worldColumnWidth - 10, Y: settingsControlHeight}
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
	nameHoverCB.Size = eui.Point{X: worldColumnWidth - 10, Y: settingsControlHeight}
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
	bubbleOpSlider.Size = eui.Point{X: 400, Y: settingsControlHeight}
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
	bubbleLifetimeDD.Size = eui.Point{X: 240, Y: settingsControlHeight}
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
	bubbleBaseLifeSlider.Size = eui.Point{X: 400, Y: settingsControlHeight}
	bubbleBaseLifeSlider.SetTooltip("Modern mode starts with this many seconds (default 3).")
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
	bubblePerWordSlider.Size = eui.Point{X: 400, Y: settingsControlHeight}
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
	bubbleScaleSlider.Size = eui.Point{X: 400, Y: settingsControlHeight}
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
	barOpacitySlider.Size = eui.Point{X: worldColumnWidth - 10, Y: settingsControlHeight}
	barOpacityEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventSliderChanged {
			SettingsLock.Lock()
			defer SettingsLock.Unlock()

			gs.BarOpacity = float64(ev.Value)
			settingsDirty = true
		}
	}
	statusSection.AddItem(barOpacitySlider)

	filePathsBtn, filePathsEvents := eui.NewButton()
	filePathsBtn.Text = "File Paths"
	setMaterialButtonIcon(filePathsBtn, "folder_open")
	filePathsBtn.Size = eui.Point{X: 180, Y: settingsControlHeight}
	filePathsBtn.Disabled = isWASM
	filePathsBtn.SetTooltip("Choose alternate folders for assets and audio, logs, legacy macros, and Go scripts.")
	filePathsEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick && !isWASM {
			SettingsLock.Lock()
			defer SettingsLock.Unlock()
			makeFilePathsWindow()
			filePathsWin.ToggleNear(ev.Item)
		}
	}
	filesSection.AddItem(filePathsBtn)

	setupBtn, setupEvents := eui.NewButton()
	setupBtn.Text = "Setup Wizard"
	setMaterialButtonIcon(setupBtn, "auto_fix_high")
	setupBtn.SetTooltip("Reopen guided graphics, layout, controls, and audio setup without resetting choices.")
	setupBtn.Size = eui.Point{X: 200, Y: 40}
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
	labelFontSlider.Size = eui.Point{X: 400, Y: settingsControlHeight}
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
	invFontSlider.Size = eui.Point{X: 400, Y: settingsControlHeight}
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
	plFontSlider.Size = eui.Point{X: 400, Y: settingsControlHeight}
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
	recentPlayersCB.Size = eui.Point{X: worldColumnWidth, Y: settingsControlHeight}
	recentPlayersCB.Checked = gs.ShowRecentPlayers
	recentPlayersEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			gs.ShowRecentPlayers = ev.Checked
			playersDirty = true
			settingsDirty = true
		}
	}
	playersSection.AddItem(recentPlayersCB)

	clanPlayersCB, clanPlayersEvents := eui.NewCheckbox()
	clanPlayersCB.Text = "Group clan members together"
	clanPlayersCB.Size = eui.Point{X: worldColumnWidth, Y: settingsControlHeight}
	clanPlayersCB.Checked = gs.GroupClanMembers
	clanPlayersEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			gs.GroupClanMembers = ev.Checked
			playersDirty = true
			settingsDirty = true
		}
	}
	playersSection.AddItem(clanPlayersCB)

	shareIconsCB, shareIconsEvents := eui.NewCheckbox()
	shareIconsCB.Text = "Show sharing icons in Players list"
	shareIconsCB.Size = eui.Point{X: worldColumnWidth, Y: settingsControlHeight}
	shareIconsCB.Checked = gs.PlayerShareIcons
	shareIconsEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			gs.PlayerShareIcons = ev.Checked
			playersDirty = true
			settingsDirty = true
		}
	}
	playersSection.AddItem(shareIconsCB)

	consoleFontSlider, consoleFontEvents := eui.NewSlider()
	consoleFontSlider.Label = "Console Font Size"
	consoleFontSlider.MinValue = 4
	consoleFontSlider.MaxValue = 48
	consoleFontSlider.Value = float32(gs.ConsoleFontSize)
	consoleFontSlider.Size = eui.Point{X: 400, Y: settingsControlHeight}
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
	chatWindowFontSlider.Size = eui.Point{X: 400, Y: settingsControlHeight}
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
	chatFontSlider.Size = eui.Point{X: 400, Y: settingsControlHeight}
	chatFontEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventSliderChanged {
			gs.BubbleFontSize = float64(ev.Value)
			initFont()
			settingsDirty = true
		}
	}
	textSizeSection.AddItem(chatFontSlider)

	addDisplaySettings(windowSection, displayColumnWidth)
	addWindowSettings(windowsSection, displayColumnWidth)
	addControlSettings(controlsSection, panelWidth)
	addTextSettings(chatSection, inputSection, displayColumnWidth)
	addBubbleSettings(bubbleSection, panelWidth)
	addAudioSettings(audioSection, panelWidth)
	addTTSSettings(ttsSection, ttsTestSection)
	addFileSettings(filesSection, recordingSection, panelWidth)
	addToolSettings(diagnosticsSection, resetSection, panelWidth)
	addNetworkSettings(networkSection, panelWidth)

	outer.Tabs = []*eui.ItemData{displayPage, worldPage, textPage, bubblesPage, audioPage, ttsPage,
		controlsPage, performancePage, networkPage, filesPage, toolsPage}
	settingsWin.AddItem(outer)
	settingsWin.AddWindow(false)
}

var settingsWindowShadowsCB *eui.ItemData

var (
	settingsPlayersCB   *eui.ItemData
	settingsInventoryCB *eui.ItemData
	settingsChatCB      *eui.ItemData
	settingsConsoleCB   *eui.ItemData
	settingsPNACheckbox *eui.ItemData
)

func addWindowVisibilityCheckbox(section *eui.ItemData, label string, width float32, target func() *eui.WindowData) *eui.ItemData {
	checkbox, events := eui.NewCheckbox()
	checkbox.Text = label
	checkbox.Size = eui.Point{X: width, Y: settingsControlHeight}
	checkbox.Checked = target() != nil && target().IsOpen()
	checkbox.Disabled = gs.TiledWindows
	checkbox.SetTooltip("Show or hide this standalone window. Tiled mode manages window visibility automatically.")
	events.Handle = func(ev eui.UIEvent) {
		if ev.Type != eui.EventCheckboxChanged {
			return
		}
		if gs.TiledWindows {
			refreshWindowSettingsControls()
			return
		}
		if win := target(); win != nil {
			if ev.Checked {
				win.MarkOpenNear(ev.Item)
			} else {
				win.Close()
			}
		}
	}
	section.AddItem(checkbox)
	return checkbox
}

func addWindowSettings(section *eui.ItemData, width float32) {
	const gap float32 = 8
	checkboxWidth := (width - gap) / 2
	firstRow := eui.NewRow()
	secondRow := eui.NewRow()
	section.AddItem(firstRow)
	section.AddItem(secondRow)
	settingsPlayersCB = addWindowVisibilityCheckbox(firstRow, "Players", checkboxWidth, func() *eui.WindowData { return playersWin })
	settingsInventoryCB = addWindowVisibilityCheckbox(firstRow, "Inventory", checkboxWidth, func() *eui.WindowData { return inventoryWin })
	settingsChatCB = addWindowVisibilityCheckbox(secondRow, "Chat", checkboxWidth, func() *eui.WindowData { return chatWin })
	settingsConsoleCB = addWindowVisibilityCheckbox(secondRow, "Console", checkboxWidth, func() *eui.WindowData { return consoleWin })

	resetBtn, resetEvents := eui.NewButton()
	resetBtn.Text = "Reset Windows"
	setMaterialButtonIcon(resetBtn, "restart_alt")
	resetBtn.Size = eui.Point{X: width, Y: settingsControlHeight}
	resetBtn.SetTooltip("Restore the default window layout.")
	resetEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			confirmResetWindows()
		}
	}
	section.AddItem(resetBtn)
}

func refreshWindowSettingsControls() {
	for _, item := range []*eui.ItemData{tileKeepGameLargeCB, wizardKeepGameLargeCB} {
		if item != nil {
			item.Checked = gs.TiledKeepGameLarge
			item.Disabled = gs.TiledLayout == TiledLayoutSide
			item.Dirty = true
		}
	}
	refreshTiledMessageLayoutControls(tileMessageOrderDD, tileMessageSplitDD)
	for _, control := range []struct {
		checkbox *eui.ItemData
		window   *eui.WindowData
	}{
		{settingsPlayersCB, playersWin},
		{settingsInventoryCB, inventoryWin},
		{settingsChatCB, chatWin},
		{settingsConsoleCB, consoleWin},
	} {
		if control.checkbox != nil {
			control.checkbox.Checked = control.window != nil && control.window.IsOpen()
			control.checkbox.Disabled = gs.TiledWindows
			control.checkbox.Dirty = true
		}
	}
	for _, item := range []*eui.ItemData{settingsCombineMessagesCB, tileCombineMessagesCB} {
		if item != nil {
			item.Checked, item.Dirty = gs.MessagesToConsole, true
		}
	}
	for _, item := range []*eui.ItemData{settingsTiledModeCB, tileTiledModeCB} {
		if item != nil {
			item.Checked, item.Dirty = gs.TiledWindows, true
		}
	}
	if settingsWin != nil {
		settingsWin.Refresh()
	}
	if tileLayoutWin != nil {
		tileLayoutWin.Refresh()
	}
	if setupWizardWin != nil {
		setupWizardWin.Refresh()
	}
}

func newCombineMessagesCheckbox(width float32) *eui.ItemData {
	checkbox, events := eui.NewCheckbox()
	checkbox.Text = "Combine chat + console"
	checkbox.Size = eui.Point{X: width, Y: settingsControlHeight}
	checkbox.Checked = gs.MessagesToConsole
	checkbox.SetTooltip("Show chat and console output together in the Console window and hide Chat. Works with tiled or standalone windows.")
	events.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			gs.MessagesToConsole = ev.Checked
			applyTiledWorkspaceLayout()
		}
	}
	return checkbox
}

func newWindowShadowsCheckbox(width float32) *eui.ItemData {
	checkbox, events := eui.NewCheckbox()
	checkbox.Text = "Window Shadows"
	checkbox.SetTooltip("Show the shadow or glow defined by the theme around floating windows and menus.")
	checkbox.Size = eui.Point{X: width, Y: settingsControlHeight}
	checkbox.Checked = gs.WindowShadows
	events.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			SettingsLock.Lock()
			defer SettingsLock.Unlock()
			gs.WindowShadows = ev.Checked
			applyWindowShadowsSetting()
			settingsDirty = true
		}
	}
	return checkbox
}

func applyWindowShadowsSetting() {
	eui.SetWindowShadows(gs.WindowShadows)
	for _, checkbox := range []*eui.ItemData{windowShadowsCB, settingsWindowShadowsCB} {
		if checkbox != nil {
			checkbox.Checked = gs.WindowShadows
			checkbox.Dirty = true
		}
	}
}

func addDisplaySettings(windowSection *eui.ItemData, columnWidth float32) {
	showFPSCB, showFPSEvents := eui.NewCheckbox()
	showFPSCB.Text = "Show FPS"
	showFPSCB.Size = eui.Point{X: columnWidth, Y: settingsControlHeight}
	showFPSCB.Checked = gs.ShowFPS
	showFPSCB.SetTooltip("Show the current frame rate in the game view.")
	showFPSEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			SettingsLock.Lock()
			gs.ShowFPS = ev.Checked
			SettingsLock.Unlock()
			settingsDirty = true
		}
	}
	windowSection.AddItem(showFPSCB)

	alwaysTopCB, alwaysTopEvents := eui.NewCheckbox()
	alwaysTopCB.Text = "Always on top"
	alwaysTopCB.Size = eui.Point{X: columnWidth, Y: settingsControlHeight}
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
	windowSection.AddItem(alwaysTopCB)
	settingsWindowShadowsCB = newWindowShadowsCheckbox(columnWidth)
	windowSection.AddItem(settingsWindowShadowsCB)
}

func addControlSettings(controlsSection *eui.ItemData, columnWidth float32) {
	midMove, midMoveEvents := eui.NewCheckbox()
	midMove.Text = "Middle-click moves windows"
	midMove.Size = eui.Point{X: columnWidth, Y: settingsControlHeight}
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
	controlsSection.AddItem(midMove)

	keySpeedSlider, keySpeedEvents := eui.NewSlider()
	keySpeedSlider.Label = "Keyboard Walk Speed"
	keySpeedSlider.MinValue = 0.1
	keySpeedSlider.MaxValue = 1.0
	keySpeedSlider.Value = float32(gs.KBWalkSpeed)
	keySpeedSlider.Size = eui.Point{X: 400, Y: settingsControlHeight}
	keySpeedEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventSliderChanged {
			SettingsLock.Lock()
			defer SettingsLock.Unlock()
			gs.KBWalkSpeed = float64(ev.Value)
			settingsDirty = true
		}
	}
	controlsSection.AddItem(keySpeedSlider)

	joystickBtn, joystickEvents := eui.NewButton()
	joystickBtn.Text = "Gamepad"
	setMaterialButtonIcon(joystickBtn, "sports_esports")
	joystickBtn.Size = eui.Point{X: 140, Y: settingsControlHeight}
	joystickEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			SettingsLock.Lock()
			defer SettingsLock.Unlock()
			joystickWin.ToggleNear(ev.Item)
		}
	}
	controlsSection.AddItem(joystickBtn)
}

func addTextSettings(chatSection, inputSection *eui.ItemData, columnWidth float32) {
	autocomplete, autocompleteEvents := eui.NewCheckbox()
	autocomplete.Text = "Autocomplete"
	autocomplete.Size = eui.Point{X: columnWidth, Y: settingsControlHeight}
	autocomplete.Checked = gs.InputAutocomplete
	autocomplete.SetTooltip("Show input suggestions and let Tab complete them.")
	autocompleteEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			SettingsLock.Lock()
			gs.InputAutocomplete = ev.Checked
			SettingsLock.Unlock()
			settingsDirty = true
			updateMessageInputWindows()
		}
	}
	inputSection.AddItem(autocomplete)
	inputSection.AddItem(&eui.ItemData{ItemType: eui.ITEM_FLOW, Size: eui.Point{X: columnWidth, Y: 8}, Fixed: true})

	spellcheck, spellcheckEvents := eui.NewCheckbox()
	spellcheck.Text = "Spellcheck"
	spellcheck.Size = eui.Point{X: columnWidth, Y: settingsControlHeight}
	spellcheck.Checked = gs.InputSpellcheck
	spellcheck.SetTooltip("Underline misspelled words and offer corrections.")
	spellcheckEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			SettingsLock.Lock()
			gs.InputSpellcheck = ev.Checked
			SettingsLock.Unlock()
			spellDirty = true
			settingsDirty = true
			updateMessageInputWindows()
		}
	}
	inputSection.AddItem(spellcheck)

	tsFormatInput, tsFormatEvents := eui.NewInput()
	tsFormatInput.Label = "Timestamp format"
	tsFormatInput.Text = gs.TimestampFormat
	tsFormatInput.TextPtr = &gs.TimestampFormat
	tsFormatInput.Size = eui.Point{X: 320, Y: settingsControlHeight}
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
	chatSection.AddItem(tsFormatInput)
}

func addBubbleSettings(bubbleSection *eui.ItemData, columnWidth float32) {
	bubbleBtn, bubbleEvents := eui.NewButton()
	bubbleBtn.Text = "Message Bubbles"
	setMaterialButtonIcon(bubbleBtn, "chat")
	bubbleBtn.Size = eui.Point{X: 180, Y: settingsControlHeight}
	bubbleEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			SettingsLock.Lock()
			defer SettingsLock.Unlock()
			bubbleWin.ToggleNear(ev.Item)
		}
	}
	bubbleSection.AddItem(bubbleBtn)
}

func addAudioSettings(audioSection *eui.ItemData, columnWidth float32) {
	soundFontDD, soundFontEvents := eui.NewDropdown()
	soundFontDD.Label = "Music SoundFont"
	soundFontDD.Size = eui.Point{X: 400, Y: settingsControlHeight}
	soundFontDD.SetTooltip("Choose a valid .sf2 file from the Assets & Audio folder. The list refreshes when opened.")
	refreshSoundFonts := func() {
		fonts, err := listSoundFonts()
		if err != nil || len(fonts) == 0 {
			soundFontDD.Options = nil
			soundFontDD.Selected = 0
			return
		}
		soundFontDD.Options = fonts
		soundFontDD.Selected = 0
		for i, name := range fonts {
			if name == configuredSoundFontFile() {
				soundFontDD.Selected = i
				break
			}
		}
	}
	refreshSoundFonts()
	soundFontDD.Action = func() {
		if soundFontDD.Open {
			refreshSoundFonts()
		}
	}
	soundFontEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type != eui.EventDropdownSelected || ev.Index < 0 || ev.Index >= len(soundFontDD.Options) {
			return
		}
		if err := selectSoundFont(soundFontDD.Options[ev.Index]); err != nil {
			logError("select music soundfont: %v", err)
			refreshSoundFonts()
		}
	}
	soundFontRefreshBtn, soundFontRefreshEvents := eui.NewButton()
	setMaterialIconOnly(soundFontRefreshBtn, "refresh", "Refresh")
	soundFontRefreshBtn.Size = eui.Point{X: settingsControlHeight, Y: settingsControlHeight}
	soundFontRefreshEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			refreshSoundFonts()
			soundFontDD.Dirty = true
		}
	}
	soundFontRow := eui.NewRow()
	soundFontRow.AddItem(soundFontDD)
	soundFontRow.AddItem(soundFontRefreshBtn)
	audioSection.AddItem(soundFontRow)

	mixerBtn, mixerEvents := eui.NewButton()
	mixerBtn.Text = "Audio Mixer"
	setMaterialButtonIcon(mixerBtn, "volume_up")
	mixerBtn.Size = eui.Point{X: 180, Y: settingsControlHeight}
	mixerBtn.SetTooltip("Open the Mixer to adjust volume levels.")
	mixerEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			makeMixerWindow()
		}
	}
	audioSection.AddItem(mixerBtn)

	soundEnhanceSlider, soundEnhanceSliderEvents := eui.NewSlider()
	soundEnhanceSlider.Label = "Sound effect ambience"
	soundEnhanceSlider.MinValue = 0.1
	soundEnhanceSlider.MaxValue = 10
	soundEnhanceSlider.Value = float32(clampSoundEnhancementAmount(gs.SoundEnhancementAmount))
	soundEnhanceSlider.Size = eui.Point{X: 400, Y: settingsControlHeight}
	soundEnhanceSlider.Disabled = !gs.SoundEnhancement
	soundEnhanceSliderEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventSliderChanged {
			gs.SoundEnhancementAmount = clampSoundEnhancementAmount(math.Round(float64(ev.Value)*100) / 100)
			refreshMixerEnhancementControls()
			settingsDirty = true
		}
	}
	soundEnhanceCB, soundEnhanceEvents := eui.NewCheckbox()
	soundEnhanceCB.Text = "Enhance sound effects"
	soundEnhanceCB.Size = eui.Point{X: columnWidth, Y: settingsControlHeight}
	soundEnhanceCB.Checked = gs.SoundEnhancement
	soundEnhanceCB.SetTooltip("Add ambience to newly played game sound effects.")
	soundEnhanceEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			gs.SoundEnhancement = ev.Checked
			soundEnhanceSlider.Disabled = !ev.Checked
			soundEnhanceSlider.Dirty = true
			refreshMixerEnhancementControls()
			settingsDirty = true
		}
	}
	audioSection.AddItem(soundEnhanceCB)
	audioSection.AddItem(soundEnhanceSlider)

	musicEnhanceSlider, musicEnhanceSliderEvents := eui.NewSlider()
	musicEnhanceSlider.Label = "Bard music ambience"
	musicEnhanceSlider.MinValue = 0.1
	musicEnhanceSlider.MaxValue = 2
	musicEnhanceSlider.Value = float32(clampMusicEnhancementAmount(gs.MusicEnhancementAmount))
	musicEnhanceSlider.Size = eui.Point{X: 400, Y: settingsControlHeight}
	musicEnhanceSlider.Disabled = !gs.MusicEnhancement
	musicEnhanceSliderEvents.Handle = func(ev eui.UIEvent) {
		switch ev.Type {
		case eui.EventSliderChanged:
			gs.MusicEnhancementAmount = clampMusicEnhancementAmount(math.Round(float64(ev.Value)*100) / 100)
			refreshMixerEnhancementControls()
			settingsDirty = true
		case eui.EventSliderReleased:
			restartMusicWithCurrentSettings()
		}
	}
	musicEnhanceCB, musicEnhanceEvents := eui.NewCheckbox()
	musicEnhanceCB.Text = "Enhance bard music"
	musicEnhanceCB.Size = eui.Point{X: columnWidth, Y: settingsControlHeight}
	musicEnhanceCB.Checked = gs.MusicEnhancement
	musicEnhanceCB.SetTooltip("Add ambience to newly started bard music.")
	musicEnhanceEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			gs.MusicEnhancement = ev.Checked
			musicEnhanceSlider.Disabled = !ev.Checked
			musicEnhanceSlider.Dirty = true
			refreshMixerEnhancementControls()
			settingsDirty = true
		}
	}
	audioSection.AddItem(musicEnhanceCB)
	audioSection.AddItem(musicEnhanceSlider)

	musicBufferSlider, musicBufferEvents := eui.NewSlider()
	musicBufferSlider.Label = "Music Buffer (s)"
	musicBufferSlider.MinValue = 1
	musicBufferSlider.MaxValue = 10
	musicBufferSlider.IntOnly = true
	musicBufferSlider.Value = float32(clampMusicBufferSeconds(gs.MusicBufferSeconds))
	musicBufferSlider.Size = eui.Point{X: 400, Y: settingsControlHeight}
	musicBufferSlider.SetTooltip("Seconds of bard music prepared before playback. More buffering tolerates slower SoundFonts but takes longer to start.")
	musicBufferEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventSliderChanged {
			gs.MusicBufferSeconds = clampMusicBufferSeconds(int(ev.Value))
			settingsDirty = true
		}
	}
	audioSection.AddItem(musicBufferSlider)

	throttleCB, throttleEvents := eui.NewCheckbox()
	throttleSoundCB = throttleCB
	throttleSoundCB.Text = "Throttle Repeated Sounds"
	throttleSoundCB.Size = eui.Point{X: columnWidth, Y: settingsControlHeight}
	throttleSoundCB.Checked = gs.ThrottleSounds
	throttleSoundCB.SetTooltip("Suppress the same effect when it repeats in adjacent server updates.")
	throttleEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			gs.ThrottleSounds = ev.Checked
			clearCaches()
			settingsDirty = true
		}
	}
	audioSection.AddItem(throttleSoundCB)

	resampleCB, resampleEvents := eui.NewCheckbox()
	resampleAudioCB = resampleCB
	resampleCB.Text = "High quality resampling"
	resampleCB.Size = eui.Point{X: columnWidth, Y: settingsControlHeight}
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
	audioSection.AddItem(resampleCB)

}

func addTTSSettings(ttsSection, testSection *eui.ItemData) {
	addTTSEnablementControls(ttsSection, 240)

	ttsSpeedSlider, ttsSpeedEvents := eui.NewSlider()
	ttsSpeedSlider.Label = "TTS Speed"
	ttsSpeedSlider.MinValue = 0.5
	ttsSpeedSlider.MaxValue = 2.0
	ttsSpeedSlider.Value = float32(gs.ChatTTSSpeed)
	ttsSpeedSlider.Size = eui.Point{X: 400, Y: settingsControlHeight}
	ttsSpeedEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventSliderChanged {
			SettingsLock.Lock()
			gs.ChatTTSSpeed = float64(ev.Value)
			SettingsLock.Unlock()
			settingsDirty = true
		}
	}
	ttsSection.AddItem(ttsSpeedSlider)

	voiceDD, voiceEvents := eui.NewDropdown()
	voiceDD.Label = "TTS Voice"
	if voices, err := listPiperVoices(); err == nil && len(voices) > 0 {
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
		if voices, err := listPiperVoices(); err == nil && len(voices) > 0 {
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
	voiceDD.Size = eui.Point{X: 400, Y: settingsControlHeight}
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
	ttsSection.AddItem(voiceDD)

	ttsTestInput, ttsTestEvents := eui.NewInput()
	ttsTestInput.Label = "TTS test phrase"
	ttsTestInput.Text = ttsTestPhrase
	ttsTestInput.TextPtr = &ttsTestPhrase
	ttsTestInput.Size = eui.Point{X: 480, Y: settingsControlHeight}
	ttsTestEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventInputChanged {
			ttsTestPhrase = ev.Text
		}
	}
	testSection.AddItem(ttsTestInput)

	ttsTestBtn, ttsTestBtnEvents := eui.NewButton()
	ttsTestBtn.Text = "Test TTS"
	setMaterialButtonIcon(ttsTestBtn, "volume_up")
	ttsTestBtn.Size = eui.Point{X: 140, Y: settingsControlHeight}
	ttsTestBtnEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			if !setTTSEnabled(true) {
				return
			}
			go playChatTTS(chatTTSCtx, ttsTestPhrase)
		}
	}

	ttsEditBtn, ttsEditEvents := eui.NewButton()
	ttsEditBtn.Text = "Edit TTS corrections"
	setMaterialButtonIcon(ttsEditBtn, "edit")
	ttsEditBtn.Size = eui.Point{X: 200, Y: settingsControlHeight}
	ttsEditEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			open.Run(dataDirPath)
		}
	}
	ttsActions := eui.NewRow()
	ttsEditBtn.Position.X = 8
	ttsActions.AddItem(ttsTestBtn)
	ttsActions.AddItem(ttsEditBtn)
	testSection.AddItem(ttsActions)
}

func addFileSettings(filesSection, recordingSection *eui.ItemData, columnWidth float32) {
	dlBtn, dlEvents := eui.NewButton()
	dlBtn.Text = "Download Files"
	setMaterialButtonIcon(dlBtn, "download")
	dlBtn.Size = eui.Point{X: 180, Y: settingsControlHeight}
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
	filesSection.AddItem(dlBtn)

	dataFolderBtn, dataFolderEvents := eui.NewButton()
	dataFolderBtn.Text = "Open User Data Folder"
	setMaterialButtonIcon(dataFolderBtn, "folder_open")
	dataFolderBtn.Size = eui.Point{X: 260, Y: settingsControlHeight}
	dataFolderBtn.SetTooltip("Open the persistent folder containing settings, characters, scripts, recordings, themes, and downloaded assets.")
	dataFolderEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			if err := open.Run(dataDirPath); err != nil {
				consoleMessage("open user data folder: " + err.Error())
			}
		}
	}
	filesSection.AddItem(dataFolderBtn)

	diagnosticsFolderBtn, diagnosticsFolderEvents := eui.NewButton()
	diagnosticsFolderBtn.Text = "Open Diagnostics Folder"
	setMaterialButtonIcon(diagnosticsFolderBtn, "folder_open")
	diagnosticsFolderBtn.Size = eui.Point{X: 260, Y: settingsControlHeight}
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
	filesSection.AddItem(diagnosticsFolderBtn)
	autoRecCB, autoRecEvents := eui.NewCheckbox()
	autoRecCB.Text = "Auto-record sessions"
	autoRecCB.Size = eui.Point{X: columnWidth, Y: settingsControlHeight}
	autoRecCB.Checked = gs.AutoRecord
	autoRecEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			gs.AutoRecord = ev.Checked
			settingsDirty = true
		}
	}
	recordingSection.AddItem(autoRecCB)
}

func addToolSettings(diagnosticsSection, resetSection *eui.ItemData, columnWidth float32) {
	debugBtn, debugEvents := eui.NewButton()
	debugBtn.Text = "Debug Settings"
	setMaterialButtonIcon(debugBtn, "bug_report")
	debugBtn.Size = eui.Point{X: 180, Y: settingsControlHeight}
	debugEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			SettingsLock.Lock()
			defer SettingsLock.Unlock()
			debugWin.ToggleNear(ev.Item)
		}
	}
	diagnosticsSection.AddItem(debugBtn)
	resetBtn, resetEv := eui.NewButton()
	resetBtn.Text = "Reset All Settings"
	setMaterialButtonIcon(resetBtn, "restart_alt")
	resetBtn.Size = eui.Point{X: 200, Y: settingsControlHeight}
	resetBtn.Color = eui.ColorDarkRed
	resetBtn.HoverColor = eui.ColorRed
	resetEv.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			SettingsLock.Lock()
			defer SettingsLock.Unlock()
			confirmResetSettings()
		}
	}
	resetSection.AddItem(resetBtn)
}

func addNetworkSettings(networkSection *eui.ItemData, columnWidth float32) {
	pnaEnabledCheckbox, pnaEnabledEvents := eui.NewCheckbox()
	settingsPNACheckbox = pnaEnabledCheckbox
	settingsPNACheckbox.Text = "Enable NLSPT"
	settingsPNACheckbox.Size = eui.Point{X: columnWidth, Y: settingsControlHeight}
	settingsPNACheckbox.Checked = gs.AltNetMode
	settingsPNACheckbox.SetTooltip("Enable Network Latency & Server Phase Timing, which learns the server frame phase and sends fresh input shortly before its next processing window.")
	pnaEnabledEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			setPNAEnabled(ev.Checked)
		}
	}
	networkSection.AddItem(settingsPNACheckbox)

	pnaSafetySlider, pnaSafetyEvents := eui.NewSlider()
	pnaSafetySlider.Label = "NLSPT safety (%)"
	pnaSafetySlider.MinValue = 0
	pnaSafetySlider.MaxValue = 50
	pnaSafetySlider.Value = float32(networkAdjustmentSafetyPercent.Load())
	pnaSafetySlider.Size = eui.Point{X: 400, Y: settingsControlHeight}
	pnaSafetySlider.SetTooltip("Minimum lead as a percentage of one server frame, with frame jitter added separately. Session-only; resets to 10% when goThoom starts.")
	pnaSafetyEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventSliderChanged {
			networkAdjustmentSafetyPercent.Store(int64(ev.Value))
		}
	}
	networkSection.AddItem(pnaSafetySlider)

	serverListButton, serverListEvents := eui.NewButton()
	serverListButton.Text = "Edit Server List"
	serverListButton.Size = eui.Point{X: 180, Y: settingsControlHeight}
	serverListButton.SetTooltip("Add or remove server addresses used by the Login screen.")
	serverListEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			openServerListWindow()
		}
	}
	networkSection.AddItem(serverListButton)

}

func refreshTiledMessageLayoutControls(order, split *eui.ItemData) {
	if order != nil {
		vertical := gs.TiledLayout == TiledLayoutMessagesSplit || (tiledPairedMessages() && gs.TiledMessagesStacked)
		order.Label = "Console / Chat"
		order.Options = []string{"Console left, Chat right", "Chat left, Console right"}
		if vertical {
			order.Options = []string{"Console above, Chat below", "Chat above, Console below"}
		}
		if gs.MessagesToConsole {
			order.Label = "Combined chat + console"
			order.Options = []string{"Combined messages left", "Combined messages right"}
			if vertical {
				order.Options = []string{"Combined messages above", "Combined messages below"}
			}
		}
		order.Selected = 0
		if !gs.TiledConsoleLeft {
			order.Selected = 1
		}
		order.Disabled, order.Dirty = tiledMessageOrderDisabled(), true
	}
	if split != nil {
		split.Selected = 0
		if gs.TiledMessagesStacked {
			split.Selected = 1
		}
		split.Disabled, split.Dirty = gs.MessagesToConsole || !tiledPairedMessages(), true
	}
}
