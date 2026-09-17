package main

import "gothoom/eui"

type sourceEditorSettingsPanel struct {
	win       *eui.WindowData
	custom    *eui.ItemData
	swatches  []*eui.ItemData
	reset     *eui.ItemData
	colors    eui.SyntaxColors
	useCustom bool
}

var sourceEditorSettings *sourceEditorSettingsPanel

var sourceSyntaxColorOptions = []struct {
	label string
	color func(*eui.SyntaxColors) *eui.Color
}{
	{"Comments", func(c *eui.SyntaxColors) *eui.Color { return &c.Comments }},
	{"Strings and rune literals", func(c *eui.SyntaxColors) *eui.Color { return &c.Strings }},
	{"Keywords", func(c *eui.SyntaxColors) *eui.Color { return &c.Keywords }},
	{"Numbers", func(c *eui.SyntaxColors) *eui.Color { return &c.Numbers }},
	{"Macro variables", func(c *eui.SyntaxColors) *eui.Color { return &c.Variables }},
	{"Macro keybindings", func(c *eui.SyntaxColors) *eui.Color { return &c.Bindings }},
}

func openSourceEditorSettings() {
	if sourceEditorSettings != nil {
		sourceEditorSettings.win.MarkOpen()
		sourceEditorSettings.win.BringForward()
		refreshSourceEditorSettings()
		return
	}
	panel := &sourceEditorSettingsPanel{win: eui.NewWindow(), colors: sourceEditorColors(), useCustom: gs.EditorUseCustomColors}
	sourceEditorSettings = panel
	win := panel.win
	win.Title = "Editor Settings"
	win.Closable, win.Movable, win.AutoSize = true, true, true
	win.OnClose = func() {
		win.RemoveWindow()
		if sourceEditorSettings == panel {
			sourceEditorSettings = nil
		}
	}
	content := eui.NewColumn()
	content.AddItem(eui.NewLabel("Colors apply to all macro and Go script editors."))
	panel.custom, _ = eui.NewCheckbox()
	panel.custom.Text = "Use custom syntax colors"
	panel.custom.Size = eui.Point{X: 380, Y: 28}
	panel.custom.Checked = gs.EditorUseCustomColors
	panel.custom.SetTooltip("When unchecked, syntax colors follow the current color theme. Your custom colors are kept for later.")
	panel.custom.Handler.Handle = func(event eui.UIEvent) {
		if event.Type != eui.EventCheckboxChanged {
			return
		}
		if event.Checked && gs.EditorSyntaxColors == nil {
			colors := eui.CurrentSyntaxColors()
			gs.EditorSyntaxColors = &colors
		}
		gs.EditorUseCustomColors = event.Checked
		sourceEditorColorsChanged()
	}
	content.AddItem(panel.custom)
	for _, option := range sourceSyntaxColorOptions {
		row := eui.NewRow()
		label := eui.NewLabel(option.label)
		label.Size = eui.Point{X: 220, Y: 28}
		row.AddItem(label)
		swatch := newColorSwatch(option.label, *option.color(&panel.colors), func(color eui.Color) {
			colors := sourceEditorColors()
			if gs.EditorSyntaxColors != nil {
				colors = *gs.EditorSyntaxColors
			}
			*option.color(&colors) = color
			gs.EditorSyntaxColors = &colors
			sourceEditorColorsChanged()
		})
		swatch.Disabled = !gs.EditorUseCustomColors
		panel.swatches = append(panel.swatches, swatch)
		row.AddItem(swatch)
		content.AddItem(row)
	}
	panel.reset = eui.NewActionButton("Copy Theme Colors", func() {
		colors := eui.CurrentSyntaxColors()
		gs.EditorSyntaxColors = &colors
		sourceEditorColorsChanged()
	})
	panel.reset.Disabled = !gs.EditorUseCustomColors
	panel.reset.SetTooltip("Replace your custom colors with the current theme's syntax colors.")
	content.AddItem(panel.reset)
	win.AddItem(content)
	panel.refresh()
	win.AddWindow(false)
	win.MarkOpen()
}

func refreshSourceEditorSettings() {
	panel := sourceEditorSettings
	if panel == nil {
		return
	}
	colors := sourceEditorColors()
	if panel.colors == colors && panel.useCustom == gs.EditorUseCustomColors {
		return
	}
	panel.refresh()
}

func sourceEditorColorsChanged() {
	settingsDirty = true
	if sourceEditorSettings != nil {
		sourceEditorSettings.refresh()
	}
	for _, ed := range sourceEditors {
		ed.refreshHighlighting()
	}
}

func (panel *sourceEditorSettingsPanel) refresh() {
	colors := sourceEditorColors()
	panel.colors, panel.useCustom = colors, gs.EditorUseCustomColors
	panel.custom.Checked = gs.EditorUseCustomColors
	panel.reset.Disabled = !gs.EditorUseCustomColors
	for i, swatch := range panel.swatches {
		swatch.Disabled = !gs.EditorUseCustomColors
		if swatch.Disabled {
			swatch.SetTooltip("Enable custom syntax colors to change this color.")
		} else {
			swatch.SetTooltip("Choose " + sourceSyntaxColorOptions[i].label + ".")
		}
		eui.SetColorSwatch(swatch, *sourceSyntaxColorOptions[i].color(&colors))
	}
	panel.win.Refresh()
}
