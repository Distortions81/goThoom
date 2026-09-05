package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"gothoom/eui"

	"github.com/hajimehoshi/ebiten/v2"
	open "github.com/skratchdot/open-golang/open"
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
	section := newConfigurationSection(title, width)
	if len(page.Contents) == 0 {
		section.Contents = section.Contents[1:]
	}
	page.AddItem(section)
	return section
}

func addSettingsWindowButton(section *eui.ItemData, title, icon string, width float32, open func(*eui.ItemData)) {
	button, events := eui.NewButton()
	button.Text = title
	button.Size = eui.Point{X: width, Y: 24}
	setMaterialButtonIcon(button, icon)
	events.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			open(ev.Item)
		}
	}
	section.AddItem(button)
}

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

	const panelWidth float32 = 660
	outer := &eui.ItemData{
		ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL,
		ActiveOutline: true, TabColumns: 5,
	}
	displayPage := newSettingsPage("Display", panelWidth)
	worldPage := newSettingsPage("World", panelWidth)
	textPage := newSettingsPage("Text", panelWidth)
	bubblesPage := newSettingsPage("Bubbles", panelWidth)
	audioPage := newSettingsPage("Audio", panelWidth)
	controlsPage := newSettingsPage("Controls", panelWidth)
	performancePage := newSettingsPage("Performance", panelWidth)
	networkPage := newSettingsPage("Network", panelWidth)
	filesPage := newSettingsPage("Files", panelWidth)
	toolsPage := newSettingsPage("Tools", panelWidth)

	windowSection := addSettingsSection(displayPage, "Window & Display", panelWidth)
	appearanceSection := addSettingsSection(displayPage, "Appearance", panelWidth)
	textSizeSection := addSettingsSection(textPage, "Text Sizes", panelWidth)
	chatSection := addSettingsSection(textPage, "Chat & Messages", panelWidth)
	const worldColumnWidth float32 = (panelWidth - 20) / 2
	worldStatus := newSettingsPage("", worldColumnWidth)
	worldNames := newSettingsPage("", worldColumnWidth)
	worldColumns := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_HORIZONTAL}
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
	ttsSection := addSettingsSection(audioPage, "Text to Speech", panelWidth)
	notificationsSection := addSettingsSection(audioPage, "Notifications", panelWidth)
	controlsSection := addSettingsSection(controlsPage, "Movement & Input", panelWidth)
	automationSection := addSettingsSection(controlsPage, "Scripts", panelWidth)
	qualitySection := addSettingsSection(performancePage, "Graphics Quality", panelWidth)
	cacheSection := addSettingsSection(performancePage, "Artwork & Sprite Cache", panelWidth)
	powerSection := addSettingsSection(performancePage, "Power Saving", panelWidth)
	networkSection := addSettingsSection(networkPage, "Connection & Timing", panelWidth)
	filesSection := addSettingsSection(filesPage, "Files & Folders", panelWidth)
	recordingSection := addSettingsSection(filesPage, "Recordings", panelWidth)
	gettingStartedSection := addSettingsSection(toolsPage, "Setup", panelWidth)
	diagnosticsSection := addSettingsSection(toolsPage, "Diagnostics", panelWidth)
	resetSection := addSettingsSection(toolsPage, "Reset", panelWidth)

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
		radio.Size = eui.Point{X: worldColumnWidth, Y: 24}
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
	barColorCB.Size = eui.Point{X: worldColumnWidth, Y: 24}
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
	maxNightSlider.Size = eui.Point{X: worldColumnWidth - 10, Y: 24}
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
	nameBgSlider.Size = eui.Point{X: worldColumnWidth - 10, Y: 24}
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
	nameBorderCB.Size = eui.Point{X: worldColumnWidth - 10, Y: 24}
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
	healthBarStyleDD.Size = eui.Point{X: worldColumnWidth - 10, Y: 24}
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
	healthBarPositionDD.Size = eui.Point{X: worldColumnWidth - 10, Y: 24}
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
	healthBarThicknessSlider.Size = eui.Point{X: worldColumnWidth - 10, Y: 24}
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
	hideSelfNameCB.Size = eui.Point{X: worldColumnWidth - 10, Y: 24}
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
	nameHoverCB.Size = eui.Point{X: worldColumnWidth - 10, Y: 24}
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
	barOpacitySlider.Size = eui.Point{X: worldColumnWidth - 10, Y: 24}
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
	filePathsBtn.Size = eui.Point{X: panelWidth, Y: 24}
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

	windowSection.AddItem(resetWindowsBtn)

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
	recentPlayersCB.Size = eui.Point{X: worldColumnWidth, Y: 24}
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
	clanPlayersCB.Size = eui.Point{X: worldColumnWidth, Y: 24}
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
	shareIconsCB.Size = eui.Point{X: worldColumnWidth, Y: 24}
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

	addDisplaySettings(windowSection, panelWidth)
	addControlSettings(controlsSection, automationSection, panelWidth)
	addTextSettings(chatSection, panelWidth)
	addBubbleSettings(bubbleSection, panelWidth)
	addAudioSettings(ttsSection, audioSection, panelWidth)
	addFileSettings(filesSection, recordingSection, panelWidth)
	addToolSettings(diagnosticsSection, resetSection, panelWidth)
	addNetworkSettings(networkSection, panelWidth)
	addPerformanceSettings(qualitySection, cacheSection, powerSection, panelWidth)
	addSettingsWindowButton(audioSection, "Audio Mixer", "equalizer", panelWidth, func(item *eui.ItemData) { mixerWin.ToggleNear(item) })
	addSettingsWindowButton(controlsSection, "Keybindings", "keyboard", panelWidth, func(item *eui.ItemData) { refreshKeybindingsList(); keybindingsWin.ToggleNear(item) })
	addSettingsWindowButton(controlsSection, "Hotkeys", "keyboard", panelWidth, func(item *eui.ItemData) { hotkeysWin.ToggleNear(item) })

	outer.Tabs = []*eui.ItemData{displayPage, worldPage, textPage, bubblesPage, audioPage,
		controlsPage, performancePage, networkPage, filesPage, toolsPage}
	settingsWin.AddItem(outer)
	settingsWin.AddWindow(false)
}

func addDisplaySettings(windowSection *eui.ItemData, columnWidth float32) {
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
	windowSection.AddItem(alwaysTopCB)
}

func addControlSettings(controlsSection, automationSection *eui.ItemData, columnWidth float32) {
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
	controlsSection.AddItem(midMove)

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
	controlsSection.AddItem(keySpeedSlider)

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
	controlsSection.AddItem(joystickBtn)
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
	automationSection.AddItem(scriptKillCB)
}

func addTextSettings(chatSection *eui.ItemData, columnWidth float32) {
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
	chatSection.AddItem(tsFormatInput)
}

func addBubbleSettings(bubbleSection *eui.ItemData, columnWidth float32) {
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
	bubbleSection.AddItem(bubbleBtn)
}

func addAudioSettings(ttsSection, audioSection *eui.ItemData, columnWidth float32) {
	ttsEnabledCB, ttsEnabledEvents := eui.NewCheckbox()
	ttsEnabledCB.Text = "Enable chat TTS"
	ttsEnabledCB.Size = eui.Point{X: columnWidth, Y: 24}
	ttsEnabledCB.Checked = gs.ChatTTS
	ttsEnabledCB.Action = func() { ttsEnabledCB.Checked = gs.ChatTTS }
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
	ttsSection.AddItem(ttsEnabledCB)
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
	ttsSection.AddItem(voiceDD)

	ttsTestInput, ttsTestEvents := eui.NewInput()
	ttsTestInput.Label = "TTS test phrase"
	ttsTestInput.Text = ttsTestPhrase
	ttsTestInput.TextPtr = &ttsTestPhrase
	ttsTestInput.Size = eui.Point{X: columnWidth, Y: 24}
	ttsTestEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventInputChanged {
			ttsTestPhrase = ev.Text
		}
	}
	ttsSection.AddItem(ttsTestInput)

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
	ttsSection.AddItem(ttsTestBtn)

	ttsEditBtn, ttsEditEvents := eui.NewButton()
	ttsEditBtn.Text = "Edit TTS corrections"
	setMaterialButtonIcon(ttsEditBtn, "edit")
	ttsEditBtn.Size = eui.Point{X: columnWidth, Y: 24}
	ttsEditEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			open.Run(dataDirPath)
		}
	}
	ttsSection.AddItem(ttsEditBtn)
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
	audioSection.AddItem(throttleSoundCB)

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
	audioSection.AddItem(enhancementCB)

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
	audioSection.AddItem(resampleCB)

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
	audioSection.AddItem(musicEnhancementCB)
}

func addFileSettings(filesSection, recordingSection *eui.ItemData, columnWidth float32) {
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
	filesSection.AddItem(dlBtn)

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
	filesSection.AddItem(dataFolderBtn)

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
	filesSection.AddItem(diagnosticsFolderBtn)
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
	recordingSection.AddItem(autoRecCB)
}

func addToolSettings(diagnosticsSection, resetSection *eui.ItemData, columnWidth float32) {
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
	diagnosticsSection.AddItem(debugBtn)
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
	resetSection.AddItem(resetBtn)
}

func addNetworkSettings(networkSection *eui.ItemData, columnWidth float32) {
	altNetCB, altNetEvents := eui.NewCheckbox()
	settingsPNACheckbox = altNetCB
	altNetCB.Text = "Network Latency & Server Phase Timing (NLSPT)"
	altNetCB.Size = eui.Point{X: columnWidth, Y: 24}
	altNetCB.Checked = gs.AltNetMode
	altNetCB.SetTooltip("Learns the server frame phase and sends fresh input shortly before its next processing window. Packet loss temporarily restores original timing.")
	altNetEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			setPNAEnabled(ev.Checked)
		}
	}
	networkSection.AddItem(altNetCB)

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
	networkSection.AddItem(pnaSafetySlider)

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
	networkSection.AddItem(serverInput)

	timingLabel, _ := eui.NewText()
	timingLabel.Text = ""
	timingLabel.Size = eui.Point{X: columnWidth, Y: 24}
	timingLabel.FontSize = 10
	networkSection.AddItem(timingLabel)

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
				settingsWin.Refresh()
				return
			}
			reply, jitter := networkTimingSnapshot()
			if reply == 0 {
				timingLabel.Text = fmt.Sprintf("Cmd reply: waiting   Frame p95: %s", formatToolbarLatency(jitter))
			} else {
				timingLabel.Text = fmt.Sprintf("Cmd reply: %s   Frame p95: %s", formatToolbarLatency(reply), formatToolbarLatency(jitter))
			}
			timingLabel.Dirty = true
			settingsWin.Refresh()
		}
	}
	networkSection.AddItem(timingBtn)
}

func addPerformanceSettings(renderingSection, cacheSection, powerSection *eui.ItemData, columnWidth float32) {
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
	renderingSection.AddItem(coordsCB)
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
	cacheSection.AddItem(batchArtworkCB)

	spriteReserve, spriteReserveExplanation := newSpriteCacheControls(columnWidth - 10)
	cacheSection.AddItem(spriteReserve)
	cacheSection.AddItem(spriteReserveExplanation)

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
	powerSection.AddItem(psBGCB)

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
	powerSection.AddItem(psAlwaysCB)

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
	powerSection.AddItem(psFPSSlider)
}
