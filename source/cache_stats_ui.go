package main

import "gothoom/eui"

var cacheStatsWin *eui.WindowData

func makeCacheStatsWindow() {
	if cacheStatsWin != nil {
		return
	}
	const width float32 = 420
	cacheStatsWin = eui.NewWindow()
	cacheStatsWin.Title = "Cache Statistics"
	cacheStatsWin.Closable = true
	cacheStatsWin.AutoSize = true
	cacheStatsWin.Movable = true
	cacheStatsWin.SetZone(eui.HZoneCenterLeft, eui.VZoneMiddleTop)
	cacheSection := eui.NewColumn()
	cacheLabel, _ := eui.NewText()
	cacheLabel.Text = "Caches:"
	cacheLabel.Size = eui.Point{X: width, Y: 24}
	cacheLabel.FontSize = 10
	cacheSection.AddItem(cacheLabel)

	sheetCacheLabel, _ = eui.NewText()
	sheetCacheLabel.Text = ""
	sheetCacheLabel.Size = eui.Point{X: width, Y: 24}
	sheetCacheLabel.FontSize = 10
	cacheSection.AddItem(sheetCacheLabel)

	frameCacheLabel, _ = eui.NewText()
	frameCacheLabel.Text = ""
	frameCacheLabel.Size = eui.Point{X: width, Y: 24}
	frameCacheLabel.FontSize = 10
	cacheSection.AddItem(frameCacheLabel)

	scaledFrameCacheLabel, _ = eui.NewText()
	scaledFrameCacheLabel.Text = ""
	scaledFrameCacheLabel.Size = eui.Point{X: width, Y: 24}
	scaledFrameCacheLabel.FontSize = 10
	cacheSection.AddItem(scaledFrameCacheLabel)

	mobileCacheLabel, _ = eui.NewText()
	mobileCacheLabel.Text = ""
	mobileCacheLabel.Size = eui.Point{X: width, Y: 24}
	mobileCacheLabel.FontSize = 10
	cacheSection.AddItem(mobileCacheLabel)

	scaledMobileCacheLabel, _ = eui.NewText()
	scaledMobileCacheLabel.Text = ""
	scaledMobileCacheLabel.Size = eui.Point{X: width, Y: 24}
	scaledMobileCacheLabel.FontSize = 10
	cacheSection.AddItem(scaledMobileCacheLabel)
	spriteSlotCacheLabel, _ = eui.NewText()
	spriteSlotCacheLabel.Size = eui.Point{X: width, Y: 80}
	spriteSlotCacheLabel.FontSize = 10
	spriteSlotCacheLabel.SetTooltip("Live slot area determines cache pressure; spare allocations do not. First IDs and reloads count sprite residencies, not individual poses. A reload means a previously evicted ID returned. Counters reset when artwork caches are cleared.")
	cacheSection.AddItem(spriteSlotCacheLabel)
	renderPoolCacheLabel, _ = eui.NewText()
	renderPoolCacheLabel.Size = eui.Point{X: width, Y: 48}
	renderPoolCacheLabel.FontSize = 10
	renderPoolCacheLabel.SetTooltip("Active and spare managed render allocations, including slot padding. Atlas packing overhead is additional.")
	cacheSection.AddItem(renderPoolCacheLabel)

	soundCacheLabel, _ = eui.NewText()
	soundCacheLabel.Text = ""
	soundCacheLabel.Size = eui.Point{X: width, Y: 24}
	soundCacheLabel.FontSize = 10
	cacheSection.AddItem(soundCacheLabel)

	clearCacheBtn, clearCacheEvents := eui.NewButton()
	clearCacheBtn.Text = "Clear All Caches"
	setMaterialButtonIcon(clearCacheBtn, "delete_sweep")
	clearCacheBtn.Size = eui.Point{X: width, Y: 24}
	clearCacheEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			clearCaches()
			updateDebugStats()
		}
	}
	cacheSection.AddItem(clearCacheBtn)
	totalCacheLabel, _ = eui.NewText()
	totalCacheLabel.Text = ""
	totalCacheLabel.Size = eui.Point{X: width, Y: 24}
	totalCacheLabel.FontSize = 10
	cacheSection.AddItem(totalCacheLabel)

	cacheStatsWin.AddItem(cacheSection)
	cacheStatsWin.AddWindow(false)
}
