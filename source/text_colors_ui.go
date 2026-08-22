package main

import "gothoom/eui"

var (
	textColorsWin   *eui.WindowData
	textColorWheels map[string]*eui.ItemData
)

func makeTextColorsWindow() {
	if textColorsWin != nil {
		return
	}
	ensureMessageTextColors()
	textColorsWin = eui.NewWindow()
	textColorsWin.Title = "Text Colors"
	textColorsWin.Closable = true
	textColorsWin.Movable = true
	textColorsWin.AutoSize = true

	content := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL}
	textColorWheels = make(map[string]*eui.ItemData, len(messageTextColorOptions))
	for _, option := range messageTextColorOptions {
		option := option
		row := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_HORIZONTAL}
		label, _ := eui.NewText()
		label.Text = option.Label
		label.FontSize = 12
		label.Size = eui.Point{X: 90, Y: 40}
		row.AddItem(label)
		wheel, events := eui.NewColorWheel()
		wheel.Size = eui.Point{X: 40, Y: 40}
		wheel.WheelColor = messageTextColor(option.Type)
		events.Handle = func(event eui.UIEvent) {
			if event.Type != eui.EventColorChanged {
				return
			}
			SettingsLock.Lock()
			gs.MessageTextColors[option.Type] = event.Color
			SettingsLock.Unlock()
			settingsDirty = true
			refreshMessageTextWindows()
		}
		textColorWheels[option.Type] = wheel
		row.AddItem(wheel)
		content.AddItem(row)
	}
	reset, resetEvents := eui.NewButton()
	reset.Text = "Reset Colors"
	reset.Size = eui.Point{X: 130, Y: 24}
	resetEvents.Handle = func(event eui.UIEvent) {
		if event.Type != eui.EventClick {
			return
		}
		SettingsLock.Lock()
		gs.MessageTextColors = defaultMessageTextColors()
		SettingsLock.Unlock()
		for messageType, wheel := range textColorWheels {
			wheel.WheelColor = messageTextColor(messageType)
			wheel.Dirty = true
		}
		settingsDirty = true
		refreshMessageTextWindows()
		textColorsWin.Refresh()
	}
	content.AddItem(reset)
	textColorsWin.AddItem(content)
	textColorsWin.AddWindow(false)
}
