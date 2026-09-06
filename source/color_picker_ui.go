package main

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"gothoom/eui"
)

var colorPickerWin *eui.WindowData

func setColorSwatch(item *eui.ItemData, col eui.Color) {
	item.WheelColor = col
	item.Text = ""
	item.Dirty = true
	if item.ParentWindow != nil {
		item.ParentWindow.Refresh()
	}
}

func newColorSwatch(title string, initial eui.Color, apply func(eui.Color)) *eui.ItemData {
	item, events := eui.NewButton()
	item.ColorSwatch = true
	item.Filled = true
	item.Size = eui.Point{X: 128, Y: 28}
	item.FontSize = 12
	setColorSwatch(item, initial)
	item.SetTooltip("Choose " + strings.ToLower(title))
	events.Handle = func(ev eui.UIEvent) {
		if ev.Type != eui.EventClick {
			return
		}
		openColorPicker(title, item.WheelColor, func(col eui.Color) {
			setColorSwatch(item, col)
			apply(col)
		})
	}
	return item
}

func openColorPicker(title string, initial eui.Color, apply func(eui.Color)) {
	if colorPickerWin != nil {
		colorPickerWin.Close()
	}
	win := eui.NewWindow()
	colorPickerWin = win
	win.Title = title
	win.Closable, win.Movable, win.AutoSize = true, true, true
	win.Resizable = false
	win.Padding = 12
	win.OnClose = func() {
		win.RemoveWindow()
		if colorPickerWin == win {
			colorPickerWin = nil
		}
	}
	root := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL, Size: eui.Point{X: 480, Y: 1}}
	columns := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_HORIZONTAL}
	left := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL, Size: eui.Point{X: 200, Y: 1}}
	right := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL, Size: eui.Point{X: 264, Y: 1}, Position: eui.Point{X: 8}}
	columns.AddItem(left)
	columns.AddItem(right)
	root.AddItem(columns)
	wheel, _ := eui.NewColorWheel()
	wheel.Size = eui.Point{X: 192, Y: 192}
	wheel.SetTooltip("Choose hue and brightness; saturation is adjustable on the right.")
	left.AddItem(wheel)
	preview, _ := eui.NewButton()
	preview.ColorSwatch, preview.Filled = true, true
	preview.Size = eui.Point{X: 192, Y: 28}
	preview.FontSize = 12
	left.AddItem(preview)
	status, _ := eui.NewText()
	status.FontSize = 11
	status.Size = eui.Point{X: 480, Y: 20}
	root.AddItem(status)
	h, s, v, a := initial.HSVA()
	values := [4]float64{h, s * 100, v * 100, a * 100}
	pending := initial
	var sliders, inputs [4]*eui.ItemData
	var syncControls func(bool)
	applyButton, applyEvents := eui.NewButton()
	applyButton.Text = "Apply"
	applyButton.Size = eui.Point{X: 112, Y: 32}
	valid := [4]bool{true, true, true, true}
	refreshValidity := func(message string) {
		applyButton.Disabled = false
		for _, ok := range valid {
			if !ok {
				applyButton.Disabled = true
			}
		}
		status.Text = message
		if applyButton.Disabled && message == "" {
			status.Text = "Correct the invalid value before applying."
		}
		win.Refresh()
	}
	for i, label := range []string{"Hue (°)", "Saturation (%)", "Brightness (%)", "Opacity (%)"} {
		i := i
		caption, _ := eui.NewText()
		caption.Text = label
		caption.FontSize = 12
		caption.Size = eui.Point{X: 264, Y: 20}
		right.AddItem(caption)
		row := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_HORIZONTAL}
		slider, events := eui.NewSlider()
		sliders[i] = slider
		slider.HideValue = true
		slider.MinValue = 0
		slider.MaxValue = 100
		if i == 0 {
			slider.MaxValue = 360
		}
		slider.Size = eui.Point{X: 176, Y: 28}
		input, inputEvents := eui.NewInput()
		inputs[i] = input
		input.Size = eui.Point{X: 72, Y: 28}
		input.FontSize = 12
		input.Position.X = 8
		row.AddItem(slider)
		row.AddItem(input)
		right.AddItem(row)
		events.Handle = func(ev eui.UIEvent) {
			if ev.Type == eui.EventSliderChanged {
				values[i] = float64(ev.Value)
				valid[i] = true
				syncControls(true)
			}
		}
		inputEvents.Handle = func(ev eui.UIEvent) {
			if ev.Type != eui.EventInputChanged {
				return
			}
			n, err := strconv.ParseFloat(strings.TrimSpace(ev.Text), 64)
			if err != nil || math.IsNaN(n) || math.IsInf(n, 0) || n < 0 || n > float64(slider.MaxValue) {
				valid[i] = false
				refreshValidity(fmt.Sprintf("%s must be between 0 and %.0f.", label, slider.MaxValue))
				return
			}
			values[i] = n
			valid[i] = true
			syncControls(false)
		}
	}
	syncControls = func(updateInputs bool) {
		pending = eui.NewColorHSV(values[0], values[1]/100, values[2]/100, values[3]/100)
		wheel.WheelColor = pending
		wheel.Dirty = true
		setColorSwatch(preview, pending)
		for i := range sliders {
			sliders[i].Value = float32(values[i])
			if updateInputs {
				inputs[i].Text = strconv.FormatFloat(values[i], 'f', 1, 64)
				valid[i] = true
			}
		}
		refreshValidity("")
	}
	wheel.OnColorChange = func(col eui.Color) { h, _, v, _ := col.HSVA(); values[0], values[2] = h, v*100; syncControls(true) }
	footer := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_HORIZONTAL}
	cancel, cancelEvents := eui.NewButton()
	cancel.Text = "Cancel"
	cancel.Size = eui.Point{X: 112, Y: 32}
	cancelEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			win.Close()
		}
	}
	applyEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick && !applyButton.Disabled {
			apply(pending)
			win.Close()
		}
	}
	footer.AddItem(cancel)
	footer.AddItem(applyButton)
	root.AddItem(footer)
	syncControls(true)
	// Opening a picker and immediately applying should preserve the exact color.
	pending = initial
	setColorSwatch(preview, initial)
	wheel.WheelColor = initial
	win.AddItem(root)
	win.AddWindow(false)
	win.MarkOpen()
	win.DefaultButton = applyButton
	_ = win.SetPos(eui.Point{X: 80, Y: 80})
}
