package main

import "gothoom/eui"

const (
	colorPickerDiameter    float32 = 128
	colorPickerWidth       float32 = 160
	textColorLabelWidth    float32 = 110
	textColorsContentWidth         = textColorLabelWidth + 2*colorPickerWidth
)

var (
	textColorsWin            *eui.WindowData
	textColorWheelsDark      map[string]*eui.ItemData
	textColorWheelsLight     map[string]*eui.ItemData
	themeTextColorOverrideCB *eui.ItemData
	classicMessageColorsCB   *eui.ItemData
)

func refreshTextColorControlStates() {
	for messageType, wheel := range textColorWheelsDark {
		wheel.Disabled = gs.ClassicMessageColors || (messageType == messageTextTypeSystem && !gs.OverrideThemeTextColor)
	}
	for messageType, wheel := range textColorWheelsLight {
		wheel.Disabled = gs.ClassicMessageColors || (messageType == messageTextTypeSystem && !gs.OverrideThemeTextColor)
	}
	if themeTextColorOverrideCB != nil {
		themeTextColorOverrideCB.Disabled = gs.ClassicMessageColors
	}
}

func textColorColumnHeading(text string, width float32) *eui.ItemData {
	label, _ := eui.NewText()
	label.Text = text
	label.FontSize = 11
	label.Size = eui.Point{X: width, Y: 22}
	return label
}

func makeTextColorsWindow() {
	if textColorsWin != nil {
		return
	}
	ensureMessageTextColors()
	textColorsWin = eui.NewWindow()
	textColorsWin.ShowTooltipIndicators = true
	textColorsWin.Title = "Text Colors"
	textColorsWin.Closable = true
	textColorsWin.Movable = true
	textColorsWin.AutoSize = true

	content := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL}
	classicMessageColorsCB, classicEvents := eui.NewCheckbox()
	classicMessageColorsCB.Text = "Use classic client text colors"
	classicMessageColorsCB.Size = eui.Point{X: textColorsContentWidth, Y: 24}
	classicMessageColorsCB.Checked = gs.ClassicMessageColors
	classicMessageColorsCB.SetTooltip("Use the classic client's black speech, green labeled-friend speech, purple self speech, and green private think-to text verbatim.")
	classicEvents.Handle = func(event eui.UIEvent) {
		if event.Type != eui.EventCheckboxChanged {
			return
		}
		gs.ClassicMessageColors = event.Checked
		if event.Checked {
			gs.OverrideThemeTextColor = false
			themeTextColorOverrideCB.Checked = false
		}
		refreshTextColorControlStates()
		settingsDirty = true
		refreshMessageTextWindows()
		textColorsWin.Refresh()
	}
	content.AddItem(classicMessageColorsCB)

	header := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_HORIZONTAL}
	header.AddItem(textColorColumnHeading("", textColorLabelWidth))
	header.AddItem(textColorColumnHeading("Dark theme", colorPickerWidth))
	header.AddItem(textColorColumnHeading("Light theme", colorPickerWidth))
	content.AddItem(header)
	paletteList := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL, Fixed: true, Scrollable: true}
	paletteList.Size = eui.Point{X: textColorsContentWidth + 32, Y: 420}
	content.AddItem(paletteList)

	textColorWheelsDark = make(map[string]*eui.ItemData, len(messageTextColorOptions))
	textColorWheelsLight = make(map[string]*eui.ItemData, len(messageTextColorOptions))
	for _, option := range messageTextColorOptions {
		option := option
		row := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_HORIZONTAL}
		label, _ := eui.NewText()
		label.Text = option.Label
		label.FontSize = 12
		label.Size = eui.Point{X: textColorLabelWidth, Y: colorPickerDiameter}
		row.AddItem(label)

		for _, light := range []bool{false, true} {
			light := light
			wheel, events := eui.NewColorWheel()
			wheel.Size = eui.Point{X: colorPickerWidth, Y: colorPickerDiameter}
			wheel.WheelColor = messageTextColorForPalette(option.Type, light)
			wheel.Disabled = option.Type == messageTextTypeSystem && !gs.OverrideThemeTextColor
			events.Handle = func(event eui.UIEvent) {
				if event.Type != eui.EventColorChanged {
					return
				}
				SettingsLock.Lock()
				messageTextPalette(light)[option.Type] = event.Color
				SettingsLock.Unlock()
				settingsDirty = true
				refreshMessageTextWindows()
			}
			if light {
				textColorWheelsLight[option.Type] = wheel
			} else {
				textColorWheelsDark[option.Type] = wheel
			}
			row.AddItem(wheel)
		}
		paletteList.AddItem(row)
	}

	themeTextColorOverrideCB, overrideEvents := eui.NewCheckbox()
	themeTextColorOverrideCB.Text = "Override theme default text color"
	themeTextColorOverrideCB.Size = eui.Point{X: textColorsContentWidth, Y: 24}
	themeTextColorOverrideCB.Checked = gs.OverrideThemeTextColor
	themeTextColorOverrideCB.SetTooltip("When off, ordinary console and interface text follows the active theme. Highlighted message types still use the palette above.")
	overrideEvents.Handle = func(event eui.UIEvent) {
		if event.Type != eui.EventCheckboxChanged {
			return
		}
		gs.OverrideThemeTextColor = event.Checked
		textColorWheelsDark[messageTextTypeSystem].Disabled = !event.Checked
		textColorWheelsLight[messageTextTypeSystem].Disabled = !event.Checked
		settingsDirty = true
		refreshMessageTextWindows()
		textColorsWin.Refresh()
	}
	content.AddItem(themeTextColorOverrideCB)
	refreshTextColorControlStates()

	reset, resetEvents := eui.NewButton()
	reset.Text = "Reset Colors"
	setMaterialButtonIcon(reset, "restart_alt")
	reset.Size = eui.Point{X: 130, Y: 24}
	resetEvents.Handle = func(event eui.UIEvent) {
		if event.Type != eui.EventClick {
			return
		}
		SettingsLock.Lock()
		gs.MessageTextColors = defaultMessageTextColors()
		gs.MessageTextColorsLight = defaultLightMessageTextColors()
		gs.OverrideThemeTextColor = false
		gs.ClassicMessageColors = false
		SettingsLock.Unlock()
		for messageType, wheel := range textColorWheelsDark {
			wheel.WheelColor = messageTextColorForPalette(messageType, false)
			wheel.Disabled = messageType == messageTextTypeSystem
			wheel.Dirty = true
		}
		for messageType, wheel := range textColorWheelsLight {
			wheel.WheelColor = messageTextColorForPalette(messageType, true)
			wheel.Disabled = messageType == messageTextTypeSystem
			wheel.Dirty = true
		}
		themeTextColorOverrideCB.Checked = false
		classicMessageColorsCB.Checked = false
		refreshTextColorControlStates()
		settingsDirty = true
		refreshMessageTextWindows()
		textColorsWin.Refresh()
	}
	content.AddItem(reset)
	textColorsWin.AddItem(content)
	textColorsWin.AddWindow(false)
}
