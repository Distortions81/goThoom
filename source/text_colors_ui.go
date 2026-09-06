package main

import "gothoom/eui"

const (
	colorSwatchHeight      float32 = 28
	colorSwatchColumnWidth float32 = 160
	textColorLabelWidth    float32 = 110
	textColorsContentWidth         = textColorLabelWidth + 2*colorSwatchColumnWidth
)

var (
	textColorsWin            *eui.WindowData
	textColorSwatchesDark    map[string]*eui.ItemData
	textColorSwatchesLight   map[string]*eui.ItemData
	themeTextColorOverrideCB *eui.ItemData
	classicMessageColorsCB   *eui.ItemData
)

func refreshTextColorControlStates() {
	for messageType, swatch := range textColorSwatchesDark {
		swatch.Disabled = gs.ClassicMessageColors || (messageType == messageTextTypeSystem && !gs.OverrideThemeTextColor)
	}
	for messageType, swatch := range textColorSwatchesLight {
		swatch.Disabled = gs.ClassicMessageColors || (messageType == messageTextTypeSystem && !gs.OverrideThemeTextColor)
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

	content := eui.NewColumn()
	classicMessageColorsCB, _ = eui.NewCheckbox()
	classicMessageColorsCB.Text = "Use classic client text colors"
	classicMessageColorsCB.Size = eui.Point{X: textColorsContentWidth, Y: 24}
	classicMessageColorsCB.Checked = gs.ClassicMessageColors
	classicMessageColorsCB.SetTooltip("Use the classic client's black speech, green labeled-friend speech, purple self speech, and green private think-to text verbatim.")
	classicMessageColorsCB.Handler.Handle = func(event eui.UIEvent) {
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

	header := eui.NewRow()
	header.AddItem(textColorColumnHeading("", textColorLabelWidth))
	header.AddItem(textColorColumnHeading("Dark theme", colorSwatchColumnWidth))
	header.AddItem(textColorColumnHeading("Light theme", colorSwatchColumnWidth))
	content.AddItem(header)
	paletteList := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL, Fixed: true, Scrollable: true}
	paletteList.Size = eui.Point{X: textColorsContentWidth + 32, Y: float32(min(420, len(messageTextColorOptions)*36))}
	content.AddItem(paletteList)

	textColorSwatchesDark = make(map[string]*eui.ItemData, len(messageTextColorOptions))
	textColorSwatchesLight = make(map[string]*eui.ItemData, len(messageTextColorOptions))
	for _, option := range messageTextColorOptions {
		option := option
		row := eui.NewRow()
		label, _ := eui.NewText()
		label.Text = option.Label
		label.FontSize = 12
		label.Size = eui.Point{X: textColorLabelWidth, Y: colorSwatchHeight}
		row.AddItem(label)

		for _, light := range []bool{false, true} {
			light := light
			paletteName := "Dark"
			if light {
				paletteName = "Light"
			}
			swatch := newColorSwatch(option.Label+" — "+paletteName, messageTextColorForPalette(option.Type, light), func(col eui.Color) {
				SettingsLock.Lock()
				messageTextPalette(light)[option.Type] = col
				SettingsLock.Unlock()
				settingsDirty = true
				refreshMessageTextWindows()
			})
			swatch.Size = eui.Point{X: colorSwatchColumnWidth - 12, Y: colorSwatchHeight}
			swatch.Position.X = 6
			swatch.Disabled = option.Type == messageTextTypeSystem && !gs.OverrideThemeTextColor

			if light {
				textColorSwatchesLight[option.Type] = swatch
			} else {
				textColorSwatchesDark[option.Type] = swatch
			}
			row.AddItem(swatch)
		}
		paletteList.AddItem(row)
	}

	themeTextColorOverrideCB, _ = eui.NewCheckbox()
	themeTextColorOverrideCB.Text = "Override theme default text color"
	themeTextColorOverrideCB.Size = eui.Point{X: textColorsContentWidth, Y: 24}
	themeTextColorOverrideCB.Checked = gs.OverrideThemeTextColor
	themeTextColorOverrideCB.SetTooltip("When off, ordinary console and interface text follows the active theme. Highlighted message types still use the palette above.")
	themeTextColorOverrideCB.Handler.Handle = func(event eui.UIEvent) {
		if event.Type != eui.EventCheckboxChanged {
			return
		}
		gs.OverrideThemeTextColor = event.Checked
		textColorSwatchesDark[messageTextTypeSystem].Disabled = !event.Checked
		textColorSwatchesLight[messageTextTypeSystem].Disabled = !event.Checked
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
		for messageType, swatch := range textColorSwatchesDark {
			eui.SetColorSwatch(swatch, messageTextColorForPalette(messageType, false))
			swatch.Disabled = messageType == messageTextTypeSystem
			swatch.Dirty = true
		}
		for messageType, swatch := range textColorSwatchesLight {
			eui.SetColorSwatch(swatch, messageTextColorForPalette(messageType, true))
			swatch.Disabled = messageType == messageTextTypeSystem
			swatch.Dirty = true
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
