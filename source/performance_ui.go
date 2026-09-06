package main

import "gothoom/eui"

func addPerformanceSettings(cacheSection, powerSection *eui.ItemData, columnWidth float32) {
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

func addPerformancePages(outer *eui.ItemData) {
	const width = settingsPanelWidth
	cachePage := newSettingsPage("Caching", width)
	powerPage := newSettingsPage("Power Saving", width)
	cacheSection := addSettingsSection(cachePage, "Artwork & Sound Loading", width)
	powerSection := addSettingsSection(powerPage, "Frame Rate & Power", width)
	addPerformanceSettings(cacheSection, powerSection, width)
	psCB, precacheSoundEvents := eui.NewCheckbox()
	precacheSoundCB = psCB
	precacheSoundCB.Text = "Precache Sounds"
	precacheSoundCB.Size = eui.Point{X: width, Y: 24}
	precacheSoundCB.Checked = gs.PrecacheSounds
	precacheSoundCB.SetTooltip("Uses roughly 300 MB more RAM to avoid first-play sound decoding delays.")
	precacheSoundEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			gs.PrecacheSounds = ev.Checked
			if ev.Checked {
				if noCacheCB != nil {
					noCacheCB.Checked = false
				}
				go precacheSounds()
			}
			settingsDirty = true
			if settingsWin != nil {
				settingsWin.Refresh()
			}
			if graphicsWin != nil {
				graphicsWin.Refresh()
			}
			if debugWin != nil {
				debugWin.Refresh()
			}
		}
	}
	cacheSection.AddItem(precacheSoundCB)

	activityIndicatorsCB, activityIndicatorEvents := eui.NewCheckbox()
	activityIndicatorsCB.Text = "Show asset activity dots"
	activityIndicatorsCB.Size = eui.Point{X: width, Y: 24}
	activityIndicatorsCB.Checked = gs.AssetActivityIndicators
	activityIndicatorsCB.SetTooltip("Show activity dots in the game view's lower-right: green for artwork decoding and processing, amber for audio decoding, and red for GPU artwork uploads.")
	activityIndicatorEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			gs.AssetActivityIndicators = ev.Checked
			setClientActivityIndicatorsEnabled(ev.Checked)
			settingsDirty = true
		}
	}
	cacheSection.AddItem(activityIndicatorsCB)

	pcCB, potatoEvents := eui.NewCheckbox()
	potatoCB = pcCB
	potatoCB.Text = "Potato GPU (4096px Limit)"
	potatoCB.SetTooltip("Use standalone textures instead of a large shared atlas for GPUs limited to 4096x4096 textures. Artwork resolution and upscaling are unchanged.")
	potatoCB.Size = eui.Point{X: width, Y: 24}
	potatoCB.Checked = gs.PotatoGPU
	potatoEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			gs.PotatoGPU = ev.Checked
			applySettings()
			clearCaches()
			settingsDirty = true
		}
	}
	cacheSection.AddItem(potatoCB)

	vsyncCB, vsyncEvents := eui.NewCheckbox()
	vsyncCB.Text = "VSync - Limit FPS"
	vsyncCB.Size = eui.Point{X: width, Y: 24}
	vsyncCB.Checked = gs.VSync
	vsyncCB.SetTooltip("Sync frame rate to the display.")
	vsyncEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			gs.VSync = ev.Checked
			applyVSyncSetting()
			settingsDirty = true
		}
	}
	powerSection.AddItem(vsyncCB)

	outer.Tabs = append(outer.Tabs, cachePage, powerPage)
}
