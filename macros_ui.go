package main

import (
    "fmt"
    "math"
    "sort"
    "strings"

    "gothoom/eui"
    text "github.com/hajimehoshi/ebiten/v2/text/v2"
)

var (
	macrosWin  *eui.WindowData
	macrosList *eui.ItemData
)

func makeMacrosWindow() {
	if macrosWin != nil {
		return
	}
	macrosWin = eui.NewWindow()
	macrosWin.Title = "Macros"
	macrosWin.Size = eui.Point{X: 300, Y: 200}
	macrosWin.Closable = true
	macrosWin.Movable = true
	macrosWin.Resizable = true
	macrosWin.NoScroll = true
	macrosWin.SetZone(eui.HZoneCenter, eui.VZoneMiddleTop)

    flow := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL, Fixed: true}
    macrosWin.AddItem(flow)

    macrosList = &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL, Scrollable: true, Fixed: true}
    flow.AddItem(macrosList)
    macrosWin.OnResize = func() { refreshMacrosList(); if macrosWin != nil { macrosWin.Refresh() } }
    macrosWin.AddWindow(false)
    refreshMacrosList()
}

func refreshMacrosList() {
    if macrosList == nil {
        return
    }
    // Compute client area for sizing the flow and list.
    clientW := macrosWin.GetSize().X
    clientH := macrosWin.GetSize().Y - macrosWin.GetTitleSize()
    s := eui.UIScale()
    if macrosWin.NoScale { s = 1 }
    pad := (macrosWin.Padding + macrosWin.BorderPad) * s
    clientWAvail := clientW - 2*pad
    if clientWAvail < 0 { clientWAvail = 0 }
    clientHAvail := clientH - 2*pad
    if clientHAvail < 0 { clientHAvail = 0 }

    // Determine row height from font metrics.
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

    // Size the outer flow and list to the client area.
    if macrosList.Parent != nil {
        macrosList.Parent.Size.X = clientWAvail
        macrosList.Parent.Size.Y = clientHAvail
    }
    macrosList.Size.X = clientWAvail
    macrosList.Size.Y = clientHAvail

    macrosList.Contents = macrosList.Contents[:0]
	macroMu.RLock()
	type pair struct{ short, full string }
	type entry struct {
		owner  string
		macros []pair
	}
	var plugins []entry
	for owner, m := range macroMaps {
		e := entry{owner: owner}
		for k, v := range m {
			e.macros = append(e.macros, pair{k, v})
		}
		plugins = append(plugins, e)
	}
	macroMu.RUnlock()
	sort.Slice(plugins, func(i, j int) bool { return plugins[i].owner < plugins[j].owner })
	for _, p := range plugins {
		disp := pluginDisplayNames[p.owner]
		if disp == "" {
			disp = p.owner
		}
        ht, _ := eui.NewText()
        ht.Text = disp + ":"
        ht.FontSize = float32(fontSize)
        ht.Size = eui.Point{X: clientWAvail, Y: rowUnits}
        macrosList.AddItem(ht)
		sort.Slice(p.macros, func(i, j int) bool { return p.macros[i].short < p.macros[j].short })
        for _, m := range p.macros {
            txt := fmt.Sprintf("  %s = %s", m.short, strings.TrimSpace(m.full))
            t, _ := eui.NewText()
            t.Text = txt
            t.FontSize = float32(fontSize)
            t.Size = eui.Point{X: clientWAvail, Y: rowUnits}
            macrosList.AddItem(t)
        }
    }
    if macrosWin != nil {
        macrosWin.Refresh()
    }
}
