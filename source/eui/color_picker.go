package eui

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// SetColorSwatch updates a swatch and refreshes its window.
func SetColorSwatch(item *ItemData, col Color) {
	item.WheelColor = col
	item.Text = ""
	item.Dirty = true
	if item.ParentWindow != nil {
		item.ParentWindow.Refresh()
	}
}

// NewColorSwatch creates a button that edits a color on click. A nil choose
// callback opens an independent EUI picker; applications can supply their own
// picker lifecycle. apply is called only after confirmation and may be nil.
func NewColorSwatch(title string, initial Color, choose func(string, Color, func(Color)), apply func(Color)) *ItemData {
	item, events := NewButton()
	item.ColorSwatch = true
	item.Filled = true
	item.Size = Point{X: 128, Y: 28}
	item.FontSize = 12
	SetColorSwatch(item, initial)
	item.SetTooltip("Choose " + strings.ToLower(title))
	events.Handle = func(ev UIEvent) {
		if ev.Type != EventClick {
			return
		}
		if choose == nil {
			choose = func(title string, initial Color, apply func(Color)) { ShowColorPicker(title, initial, apply) }
		}
		choose(title, item.WheelColor, func(col Color) {
			SetColorSwatch(item, col)
			if apply != nil {
				apply(col)
			}
		})
	}
	return item
}

// ShowColorPicker opens an independent HSV/opacity editor. Changes remain local
// until Apply; closing or cancelling discards them. The window removes itself
// on close. Initialize the EUI font source before opening a picker.
func ShowColorPicker(title string, initial Color, apply func(Color)) *WindowData {
	win := NewWindow()
	win.Title = title
	win.Closable, win.Movable, win.AutoSize = true, true, true
	win.Resizable = false
	win.Padding = 12
	win.OnClose = func() {
		win.RemoveWindow()
	}
	root := &ItemData{ItemType: ITEM_FLOW, FlowType: FLOW_VERTICAL, Size: Point{X: 480, Y: 1}}
	columns := NewRow()
	left := &ItemData{ItemType: ITEM_FLOW, FlowType: FLOW_VERTICAL, Size: Point{X: 200, Y: 1}}
	right := &ItemData{ItemType: ITEM_FLOW, FlowType: FLOW_VERTICAL, Size: Point{X: 264, Y: 1}, Position: Point{X: 8}}
	columns.AddItem(left)
	columns.AddItem(right)
	root.AddItem(columns)
	wheel, _ := NewColorWheel()
	wheel.Size = Point{X: 192, Y: 192}
	wheel.SetTooltip("Choose hue and brightness; saturation is adjustable on the right.")
	left.AddItem(wheel)
	preview, _ := NewButton()
	preview.ColorSwatch, preview.Filled = true, true
	preview.Size = Point{X: 192, Y: 28}
	preview.FontSize = 12
	left.AddItem(preview)
	status, _ := NewText()
	status.FontSize = 11
	status.Size = Point{X: 480, Y: 20}
	root.AddItem(status)
	h, s, v, a := initial.HSVA()
	values := [4]float64{h, s * 100, v * 100, a * 100}
	pending := initial
	var sliders, inputs [4]*ItemData
	var syncControls func(bool)
	applyButton, applyEvents := NewButton()
	applyButton.Text = "Apply"
	applyButton.Size = Point{X: 112, Y: 32}
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
		caption, _ := NewText()
		caption.Text = label
		caption.FontSize = 12
		caption.Size = Point{X: 264, Y: 20}
		right.AddItem(caption)
		row := NewRow()
		slider, events := NewSlider()
		sliders[i] = slider
		slider.HideValue = true
		slider.MinValue = 0
		slider.MaxValue = 100
		if i == 0 {
			slider.MaxValue = 360
		}
		slider.Size = Point{X: 176, Y: 28}
		input, inputEvents := NewInput()
		inputs[i] = input
		input.Size = Point{X: 72, Y: 28}
		input.FontSize = 12
		input.Position.X = 8
		row.AddItem(slider)
		row.AddItem(input)
		right.AddItem(row)
		events.Handle = func(ev UIEvent) {
			if ev.Type == EventSliderChanged {
				values[i] = float64(ev.Value)
				valid[i] = true
				syncControls(true)
			}
		}
		inputEvents.Handle = func(ev UIEvent) {
			if ev.Type != EventInputChanged {
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
		pending = NewColorHSV(values[0], values[1]/100, values[2]/100, values[3]/100)
		wheel.WheelColor = pending
		wheel.Dirty = true
		SetColorSwatch(preview, pending)
		for i := range sliders {
			sliders[i].Value = float32(values[i])
			if updateInputs {
				inputs[i].Text = strconv.FormatFloat(values[i], 'f', 1, 64)
				valid[i] = true
			}
		}
		refreshValidity("")
	}
	wheel.OnColorChange = func(col Color) { h, _, v, _ := col.HSVA(); values[0], values[2] = h, v*100; syncControls(true) }
	footer := NewRow()
	cancel, cancelEvents := NewButton()
	cancel.Text = "Cancel"
	cancel.Size = Point{X: 112, Y: 32}
	cancelEvents.Handle = func(ev UIEvent) {
		if ev.Type == EventClick {
			win.Close()
		}
	}
	applyEvents.Handle = func(ev UIEvent) {
		if ev.Type == EventClick && !applyButton.Disabled {
			if apply != nil {
				apply(pending)
			}
			win.Close()
		}
	}
	footer.AddItem(cancel)
	footer.AddItem(applyButton)
	root.AddItem(footer)
	syncControls(true)
	// Opening a picker and immediately applying should preserve the exact color.
	pending = initial
	SetColorSwatch(preview, initial)
	wheel.WheelColor = initial
	win.AddItem(root)
	win.AddWindow(false)
	win.MarkOpen()
	win.DefaultButton = applyButton
	_ = win.SetPos(Point{X: 80, Y: 80})
	return win
}
