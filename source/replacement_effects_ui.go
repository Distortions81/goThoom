package main

import "gothoom/eui"

var replacementEffectsPreviewWin *eui.WindowData

func makeReplacementEffectsPreviewWindow() {
	if replacementEffectsPreviewWin != nil {
		return
	}

	const width float32 = 270
	replacementEffectsPreviewWin = eui.NewWindow()
	replacementEffectsPreviewWin.Title = "Effects Preview"
	replacementEffectsPreviewWin.Closable = false
	replacementEffectsPreviewWin.Resizable = false
	replacementEffectsPreviewWin.AutoSize = true
	replacementEffectsPreviewWin.Movable = true
	replacementEffectsPreviewWin.ShowTooltipIndicators = true
	replacementEffectsPreviewWin.SetZone(eui.HZoneRight, eui.VZoneTop)

	flow := eui.NewColumn()
	intro, _ := eui.NewText()
	intro.Text = "Preview a replacement effect in the game view."
	intro.Size = eui.Point{X: width, Y: 28}
	flow.AddItem(intro)

	effectPicker, effectEvents := eui.NewDropdown()
	effectPicker.Label = "Effect"
	effectPicker.Size = eui.Point{X: width, Y: settingsControlHeight}
	effectPicker.Options = []string{"All effects"}
	for _, preview := range replacementEffectsPreviews {
		effectPicker.Options = append(effectPicker.Options, preview.label)
	}
	effectPicker.Selected = replacementEffectsPreviewSelection + 1
	if effectPicker.Selected < 0 || effectPicker.Selected >= len(effectPicker.Options) {
		effectPicker.Selected = 0
	}
	effectPicker.SetTooltip("Show every effect in a gallery, or one effect at a larger scale.")
	effectEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventDropdownSelected {
			replacementEffectsPreviewSelection = ev.Index - 1
			replacementEffectsPreview = true
		}
	}
	flow.AddItem(effectPicker)

	modePicker, modeEvents := eui.NewDropdown()
	modePicker.Label = "Display"
	modePicker.Size = eui.Point{X: width, Y: settingsControlHeight}
	modePicker.Options = []string{"New effect only", "Original animation", "Original + new effect"}
	modePicker.Selected = int(replacementEffectsPreviewMode)
	modePicker.SetTooltip("Compare the original sprite animation with the procedural replacement. This affects the preview only.")
	modeEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventDropdownSelected && ev.Index >= 0 && ev.Index < len(modePicker.Options) {
			replacementEffectsPreviewMode = replacementEffectPreviewMode(ev.Index)
			replacementEffectsPreview = true
		}
	}
	flow.AddItem(modePicker)

	scalePicker, scaleEvents := eui.NewDropdown()
	scalePicker.Label = "Scale"
	scalePicker.Size = eui.Point{X: width, Y: settingsControlHeight}
	scalePicker.Options = []string{"At real size", "200% size", "Full window size"}
	scalePicker.Selected = int(replacementEffectsPreviewScale)
	scalePicker.SetTooltip("Set the size of one selected effect. The all-effects gallery always fits its tiles to the game view.")
	scaleEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventDropdownSelected && ev.Index >= 0 && ev.Index < len(scalePicker.Options) {
			replacementEffectsPreviewScale = replacementEffectPreviewScale(ev.Index)
			replacementEffectsPreview = true
		}
	}
	flow.AddItem(scalePicker)

	ratePicker, rateEvents := eui.NewDropdown()
	ratePicker.Label = "Animation rate"
	ratePicker.Size = eui.Point{X: width, Y: settingsControlHeight}
	ratePicker.Options = []string{"1 UPS", "2 UPS", "5 UPS", "10 UPS", "20 UPS", "30 UPS"}
	rateValues := []int{1, 2, 5, 10, 20, 30}
	ratePicker.Selected = 2 // 5 UPS is the default Clan Lord movie update rate.
	for index, value := range rateValues {
		if value == replacementEffectsPreviewUPS {
			ratePicker.Selected = index
			break
		}
	}
	ratePicker.SetTooltip("Set the original sprite animation's updates per second. The procedural effect stays smoothly animated between updates.")
	rateEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventDropdownSelected && ev.Index >= 0 && ev.Index < len(rateValues) {
			replacementEffectsPreviewUPS = rateValues[ev.Index]
			replacementEffectsPreview = true
		}
	}
	flow.AddItem(ratePicker)

	reloadBtn, reloadEvents := eui.NewButton()
	reloadBtn.Text = "Reload Shaders"
	setMaterialButtonIcon(reloadBtn, "restart_alt")
	reloadBtn.Size = eui.Point{X: width, Y: settingsControlHeight}
	reloadBtn.SetTooltip("Reload lighting and replacement shaders from this checked-out source tree.")
	reloadEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type != eui.EventClick {
			return
		}
		if err := ReloadLightingShader(); err != nil {
			consoleMessage("Shader reload failed: " + err.Error())
		} else if err := ReloadReplacementEffectsShader(); err != nil {
			consoleMessage("Shader reload failed: " + err.Error())
		} else {
			consoleMessage("Shaders reloaded.")
		}
	}
	flow.AddItem(reloadBtn)

	closeBtn, closeEvents := eui.NewButton()
	closeBtn.Text = "Close Preview"
	setMaterialButtonIcon(closeBtn, "close")
	closeBtn.Size = eui.Point{X: width, Y: settingsControlHeight}
	closeBtn.SetTooltip("Return the game view to its normal scene.")
	closeEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			replacementEffectsPreview = false
			replacementEffectsPreviewWin.Close()
		}
	}
	flow.AddItem(closeBtn)

	replacementEffectsPreviewWin.AddItem(flow)
	replacementEffectsPreviewWin.AddWindow(false)
	if replacementEffectsPreview {
		closeLoginForReplacementEffectsPreview()
		replacementEffectsPreviewWin.MarkOpen()
	}
}

func openReplacementEffectsPreview() {
	makeReplacementEffectsPreviewWindow()
	replacementEffectsPreview = true
	closeLoginForReplacementEffectsPreview()
	replacementEffectsPreviewWin.MarkOpen()
}

func closeLoginForReplacementEffectsPreview() {
	if loginWin != nil && loginWin.IsOpen() {
		loginWin.Close()
	}
}
