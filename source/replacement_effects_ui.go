package main

import (
	"github.com/hajimehoshi/ebiten/v2"
	"gothoom/eui"
)

var (
	replacementEffectsPreviewWin     *eui.WindowData
	replacementEffectsPreviewRoot    *eui.ItemData
	replacementEffectsPreviewList    *eui.ItemData
	replacementEffectsPreviewCards   []replacementEffectPreviewCard
	replacementEffectsPreviewColumns int
	replacementEffectsPreviewZoomUI  *eui.ItemData
)

const (
	replacementEffectsPreviewWindowWidth  float32 = 660
	replacementEffectsPreviewWindowHeight float32 = 700
	replacementEffectsPreviewCardWidth    float32 = 292
	replacementEffectsPreviewCardGap      float32 = 8
	replacementEffectsPreviewImageHeight  float32 = 184
	replacementEffectsPreviewFontSize     float32 = 12
	previewGalleryZoomMinPercent          float32 = 25
	previewGalleryZoomMaxPercent          float32 = 200
	previewGalleryZoomDefaultPercent      float32 = 100
)

type replacementEffectPreviewCard struct {
	group    replacementEffectPreviewGroup
	checkbox *eui.ItemData
	image    *ebiten.Image
}

func sizeReplacementEffectsPreviewControl(item *eui.ItemData, width float32) {
	item.Size = eui.Point{X: width, Y: settingsControlHeight}
	item.FontSize = replacementEffectsPreviewFontSize
}

func addExperimentalSettings(effectsSection, spritesSection *eui.ItemData, width float32) {
	replacementCB, replacementEvents := eui.NewCheckbox()
	replacementEffectsCB = replacementCB
	replacementCB.Text = "Replacement Effects"
	replacementCB.Size = eui.Point{X: width, Y: settingsControlHeight}
	replacementCB.Checked = gs.ReplacementEffects
	replacementCB.SetTooltip("Use the enabled procedural replacements instead of their original effect sprites.")
	replacementEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			gs.ReplacementEffects = ev.Checked
			settingsDirty = true
		}
	}
	effectsSection.AddItem(replacementCB)

	mobileConeCB, mobileConeEvents := eui.NewCheckbox()
	mobileLightConeShadowsCB = mobileConeCB
	mobileConeCB.Text = "Mobile light-cone shadows"
	mobileConeCB.Size = eui.Point{X: width, Y: settingsControlHeight}
	mobileConeCB.Checked = gs.MobileLightConeShadows
	mobileConeCB.Disabled = !gs.ShaderLighting
	mobileConeCB.SetTooltip("Let mobiles cast experimental soft cone shadows from nearby lights.")
	mobileConeEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			gs.MobileLightConeShadows = ev.Checked
			settingsDirty = true
		}
	}
	effectsSection.AddItem(mobileConeCB)

	viewBtn, viewEvents := eui.NewButton()
	viewBtn.Text = "View Experimental Effects"
	setMaterialButtonIcon(viewBtn, "visibility")
	viewBtn.Size = eui.Point{X: width, Y: settingsControlHeight}
	viewBtn.SetTooltip("Open the full effect gallery, comparison controls, and per-effect switches.")
	viewEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			openReplacementEffectsPreview()
		}
	}
	effectsSection.AddItem(viewBtn)

	spritePackCB, spritePackEvents := eui.NewCheckbox()
	hdSpritePackCB = spritePackCB
	spritePackCB.Text = "Use sprite pack files"
	spritePackCB.Size = eui.Point{X: width, Y: settingsControlHeight}
	spritePackCB.Checked = gs.UseSpritePackFiles
	spritePackCB.SetTooltip("Replace compatible single-frame sprites from PNGs and ZIPs in game or user-data hdimg folders.")
	spritePackEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged && gs.UseSpritePackFiles != ev.Checked {
			gs.UseSpritePackFiles = ev.Checked
			if hdPicturePreviewOpenBtn != nil {
				hdPicturePreviewOpenBtn.Disabled = !ev.Checked
				hdPicturePreviewOpenBtn.Dirty = true
			}
			if !ev.Checked && hdPicturePreviewWin != nil && hdPicturePreviewWin.IsOpen() {
				hdPicturePreviewWin.Close()
			}
			reloadHDPictures()
			settingsDirty = true
			if gameWin != nil {
				gameWin.Refresh()
			}
		}
	}
	spritesSection.AddItem(spritePackCB)

	previewBtn, previewEvents := eui.NewButton()
	hdPicturePreviewOpenBtn = previewBtn
	previewBtn.Text = "View HD Sprite Replacements"
	setMaterialButtonIcon(previewBtn, "visibility")
	previewBtn.Size = eui.Point{X: width, Y: settingsControlHeight}
	previewBtn.Disabled = !gs.UseSpritePackFiles
	previewBtn.SetTooltip("Enable Use sprite pack files first, then browse, compare, and select the installed HD replacements.")
	previewEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			openHDPicturePreview()
		}
	}
	spritesSection.AddItem(previewBtn)

	reloadBtn, reloadEvents := eui.NewButton()
	reloadBtn.Text = "Reload HD Sprites"
	setMaterialButtonIcon(reloadBtn, "restart_alt")
	reloadBtn.Size = eui.Point{X: width, Y: settingsControlHeight}
	reloadBtn.SetTooltip("Read PNGs and ZIPs in the game or user-data hdimg folders again.")
	reloadEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			reloadHDPictureDebug()
		}
	}
	spritesSection.AddItem(reloadBtn)
}

func makeReplacementEffectsPreviewWindow() {
	if replacementEffectsPreviewWin != nil {
		return
	}

	replacementEffectsPreviewWin = eui.NewWindow()
	replacementEffectsPreviewWin.Title = "Effects Preview"
	replacementEffectsPreviewWin.Closable = true
	replacementEffectsPreviewWin.Resizable = true
	replacementEffectsPreviewWin.AutoSize = false
	replacementEffectsPreviewWin.Movable = true
	replacementEffectsPreviewWin.NoScroll = true
	replacementEffectsPreviewWin.NoCache = true
	replacementEffectsPreviewWin.Size = eui.Point{X: replacementEffectsPreviewWindowWidth, Y: replacementEffectsPreviewWindowHeight}
	replacementEffectsPreviewWin.ShowTooltipIndicators = true
	replacementEffectsPreviewWin.SetZone(eui.HZoneCenter, eui.VZoneMiddleTop)

	replacementEffectsPreviewRoot = eui.NewColumn()
	pickerRow := eui.NewRow()

	modePicker, modeEvents := eui.NewDropdown()
	modePicker.Label = "Display"
	sizeReplacementEffectsPreviewControl(modePicker, 230)
	modePicker.Options = []string{
		replacementEffectPreviewLabel(replacementEffectPreviewNew),
		replacementEffectPreviewLabel(replacementEffectPreviewOriginal),
		replacementEffectPreviewLabel(replacementEffectPreviewBoth),
	}
	modePicker.Selected = int(replacementEffectsPreviewMode)
	modePicker.SetTooltip("Compare the original sprite animation with the procedural replacement. This affects the preview only.")
	modeEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventDropdownSelected && ev.Index >= 0 && ev.Index < len(modePicker.Options) {
			replacementEffectsPreviewMode = replacementEffectPreviewMode(ev.Index)
			replacementEffectsPreview = true
		}
	}
	pickerRow.AddItem(modePicker)

	ratePicker, rateEvents := eui.NewDropdown()
	ratePicker.Label = "Animation rate"
	sizeReplacementEffectsPreviewControl(ratePicker, 140)
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
	pickerRow.AddItem(ratePicker)
	replacementEffectsPreviewRoot.AddItem(pickerRow)

	actionRow := eui.NewRow()

	reloadBtn, reloadEvents := eui.NewButton()
	reloadBtn.Text = "Reload Shaders"
	setMaterialButtonIcon(reloadBtn, "restart_alt")
	sizeReplacementEffectsPreviewControl(reloadBtn, 160)
	reloadBtn.SetTooltip("Recompile lighting and replacement shaders. Replacement effects read this checked-out source tree.")
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
	actionRow.AddItem(reloadBtn)

	zoomSlider, zoomEvents := eui.NewSlider()
	replacementEffectsPreviewZoomUI = zoomSlider
	zoomSlider.Label = "Zoom (%)"
	zoomSlider.MinValue = previewGalleryZoomMinPercent
	zoomSlider.MaxValue = previewGalleryZoomMaxPercent
	zoomSlider.Value = replacementEffectsPreviewZoom * 100
	zoomSlider.IntOnly = true
	sizeReplacementEffectsPreviewControl(zoomSlider, 180)
	zoomSlider.SetTooltip("Resize the preview cards. Smaller cards make more columns; the largest size fills the gallery width.")
	zoomEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventSliderChanged {
			replacementEffectsPreviewZoom = ev.Value / 100
			replacementEffectsPreviewColumns = 0
			layoutReplacementEffectsPreviewWindow()
		}
	}
	actionRow.AddItem(zoomSlider)

	replacementEffectsPreviewRoot.AddItem(actionRow)

	replacementEffectsPreviewList = &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL, Fixed: true, Scrollable: true}
	replacementEffectsPreviewRoot.AddItem(replacementEffectsPreviewList)
	replacementEffectsPreviewWin.OnResize = layoutReplacementEffectsPreviewWindow
	replacementEffectsPreviewWin.OnClose = func() { replacementEffectsPreview = false }
	replacementEffectsPreviewWin.AddItem(replacementEffectsPreviewRoot)
	replacementEffectsPreviewWin.AddWindow(false)
	layoutReplacementEffectsPreviewWindow()
	if replacementEffectsPreview {
		closeLoginForReplacementEffectsPreview()
		replacementEffectsPreviewWin.MarkOpen()
	}
}

func layoutReplacementEffectsPreviewWindow() {
	if replacementEffectsPreviewWin == nil || replacementEffectsPreviewRoot == nil || replacementEffectsPreviewList == nil {
		return
	}
	eui.LayoutWindowBody(replacementEffectsPreviewWin, replacementEffectsPreviewRoot, replacementEffectsPreviewList)
	columns := replacementEffectsPreviewColumnCount(replacementEffectsPreviewList.Size.X)
	if columns != replacementEffectsPreviewColumns || len(replacementEffectsPreviewCards) == 0 {
		rebuildReplacementEffectsPreviewCards(columns)
		eui.LayoutWindowBody(replacementEffectsPreviewWin, replacementEffectsPreviewRoot, replacementEffectsPreviewList)
	}
	replacementEffectsPreviewWin.Refresh()
}

func replacementEffectsPreviewColumnCount(width float32) int {
	cardWidth, _ := replacementEffectsPreviewCardDimensions(width)
	return max(1, int((width+replacementEffectsPreviewCardGap)/(cardWidth+replacementEffectsPreviewCardGap)))
}

func replacementEffectsPreviewCardDimensions(availableWidth float32) (width, imageHeight float32) {
	zoom := replacementEffectsPreviewZoom
	if zoom <= 0 {
		zoom = 1
	}
	if zoom >= previewGalleryZoomMaxPercent/100 && availableWidth > 0 {
		usableWidth := max(float32(1), availableWidth-eui.ScrollbarWidth()/eui.UIScale())
		zoom = usableWidth / replacementEffectsPreviewCardWidth
	}
	return replacementEffectsPreviewCardWidth * zoom, replacementEffectsPreviewImageHeight * zoom
}

func rebuildReplacementEffectsPreviewCards(columns int) {
	if replacementEffectsPreviewList == nil {
		return
	}
	if columns < 1 {
		columns = 1
	}
	scroll := replacementEffectsPreviewList.Scroll
	for _, card := range replacementEffectsPreviewCards {
		if card.image != nil {
			card.image.Deallocate()
		}
	}
	replacementEffectsPreviewCards = nil
	replacementEffectsPreviewList.Contents = nil
	replacementEffectsPreviewColumns = columns
	cardWidth, imageHeight := replacementEffectsPreviewCardDimensions(replacementEffectsPreviewList.Size.X)

	groups := replacementEffectPreviewGalleryGroups()
	for start := 0; start < len(groups); start += columns {
		row := eui.NewRow()
		for index := start; index < min(start+columns, len(groups)); index++ {
			group := groups[index]
			card := eui.NewColumn()
			card.Fixed = true
			card.Filled = true
			card.Color = eui.SubtleAlternateRowColor()
			card.Size = eui.Point{X: cardWidth, Y: imageHeight + 30}

			checkbox, events := eui.NewCheckbox()
			checkbox.Text = group.label
			checkbox.Checked = group.enabled()
			checkbox.FontSize = replacementEffectsPreviewFontSize
			checkbox.Size = eui.Point{X: cardWidth, Y: 26}
			events.Handle = func(ev eui.UIEvent) {
				if ev.Type == eui.EventCheckboxChanged {
					group.setEnabled(ev.Checked)
					settingsDirty = true
				}
			}
			card.AddItem(checkbox)

			imageItem, backing := eui.NewImageFastItem(max(1, roundToInt(float64(cardWidth))), max(1, roundToInt(float64(imageHeight))))
			imageItem.Size = eui.Point{X: cardWidth, Y: imageHeight}
			card.AddItem(imageItem)
			row.AddItem(card)
			replacementEffectsPreviewCards = append(replacementEffectsPreviewCards, replacementEffectPreviewCard{
				group: group, checkbox: checkbox, image: backing,
			})
		}
		replacementEffectsPreviewList.AddItem(row)
	}
	replacementEffectsPreviewList.Scroll = scroll
}

func openReplacementEffectsPreview() {
	makeReplacementEffectsPreviewWindow()
	replacementEffectsPreview = true
	closeLoginForReplacementEffectsPreview()
	replacementEffectsPreviewWin.MarkOpen()
	layoutReplacementEffectsPreviewWindow()
}

func closeLoginForReplacementEffectsPreview() {
	if loginWin != nil && loginWin.IsOpen() {
		loginWin.Close()
	}
}
