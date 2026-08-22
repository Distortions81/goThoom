package main

import (
	"math"
	"sort"

	"gothoom/eui"

	text "github.com/hajimehoshi/ebiten/v2/text/v2"
)

var (
	triggersWin  *eui.WindowData
	triggersList *eui.ItemData
)

func makeTriggersWindow() {
	if triggersWin != nil {
		return
	}
	triggersWin = eui.NewWindow()
	triggersWin.Title = "Triggers"
	triggersWin.Size = eui.Point{X: 500, Y: 500}
	triggersWin.Closable = true
	triggersWin.Movable = true
	triggersWin.Resizable = true
	triggersWin.NoScroll = true
	triggersWin.SetZone(eui.HZoneCenter, eui.VZoneMiddleTop)

	flow := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL, Fixed: true}
	triggersWin.AddItem(flow)

	triggersList = &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL, Scrollable: true, Fixed: true}
	flow.AddItem(triggersList)
	triggersWin.OnResize = func() {
		refreshTriggersList()
		if triggersWin != nil {
			triggersWin.Refresh()
		}
	}
	triggersWin.AddWindow(false)
	refreshTriggersList()
}

func refreshTriggersList() {
	if triggersList == nil {
		return
	}
	// Compute client area.
	clientW := triggersWin.GetSize().X
	clientH := triggersWin.GetSize().Y - triggersWin.GetTitleSize()
	s := eui.UIScale()
	if triggersWin.NoScale {
		s = 1
	}
	pad := (triggersWin.Padding + triggersWin.BorderPad) * s
	clientWAvail := clientW - 2*pad
	if clientWAvail < 0 {
		clientWAvail = 0
	}
	clientHAvail := clientH - 2*pad
	if clientHAvail < 0 {
		clientHAvail = 0
	}

	// Row height from font metrics.
	fontSize := gs.ConsoleFontSize
	ui := eui.UIScale()
	facePx := float64(float32(fontSize) * ui)
	var goFace *text.GoTextFace
	if src := eui.FontSource(); src != nil {
		goFace = &text.GoTextFace{Source: src, Size: facePx}
	} else {
		goFace = &text.GoTextFace{Size: facePx}
	}
	metrics := goFace.Metrics()
	linePx := math.Ceil(metrics.HAscent + metrics.HDescent + 2)
	rowUnits := float32(linePx) / ui

	// Size containers to client area.
	if triggersList.Parent != nil {
		triggersList.Parent.Size.X = clientWAvail
		triggersList.Parent.Size.Y = clientHAvail
	}
	triggersList.Size.X = clientWAvail
	triggersList.Size.Y = clientHAvail

	triggersList.Contents = triggersList.Contents[:0]
	triggerHandlersMu.RLock()
	type entry struct {
		owner    string
		triggers []string
	}
	byOwner := map[string]*entry{}
	for phrase, hs := range scriptTriggers {
		for _, h := range hs {
			e := byOwner[h.owner]
			if e == nil {
				e = &entry{owner: h.owner}
				byOwner[h.owner] = e
			}
			e.triggers = append(e.triggers, phrase)
		}
	}
	triggerHandlersMu.RUnlock()
	var entries []entry
	for _, e := range byOwner {
		entries = append(entries, *e)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].owner < entries[j].owner })
	for _, e := range entries {
		disp := getscriptDisplayName(e.owner)
		ht, _ := eui.NewText()
		ht.Text = disp + ":"
		ht.FontSize = float32(fontSize)
		ht.Size = eui.Point{X: clientWAvail, Y: rowUnits}
		triggersList.AddItem(ht)
		sort.Strings(e.triggers)
		for _, t := range e.triggers {
			tt, _ := eui.NewText()
			tt.Text = "  " + t
			tt.FontSize = float32(fontSize)
			tt.Size = eui.Point{X: clientWAvail, Y: rowUnits}
			triggersList.AddItem(tt)
		}
	}
	if triggersWin != nil {
		// Mark the window dirty so tests observing Dirty can detect refresh.
		triggersWin.Dirty = true
	}
}
