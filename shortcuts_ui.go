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
    shortcutsWin  *eui.WindowData
    shortcutsList *eui.ItemData
)

func makeShortcutsWindow() {
    if shortcutsWin != nil {
        return
    }
    shortcutsWin = eui.NewWindow()
    shortcutsWin.Title = "Shortcuts"
    shortcutsWin.Size = eui.Point{X: 300, Y: 200}
    shortcutsWin.Closable = true
    shortcutsWin.Movable = true
    shortcutsWin.Resizable = true
    shortcutsWin.NoScroll = true
    shortcutsWin.SetZone(eui.HZoneCenter, eui.VZoneMiddleTop)

    flow := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL, Fixed: true}
    shortcutsWin.AddItem(flow)

    shortcutsList = &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL, Scrollable: true, Fixed: true}
    flow.AddItem(shortcutsList)
    shortcutsWin.OnResize = func() { refreshShortcutsList(); if shortcutsWin != nil { shortcutsWin.Refresh() } }
    shortcutsWin.AddWindow(false)
    refreshShortcutsList()
}

func refreshShortcutsList() {
    if shortcutsList == nil {
        return
    }
    // Compute client area for sizing the flow and list.
    clientW := shortcutsWin.GetSize().X
    clientH := shortcutsWin.GetSize().Y - shortcutsWin.GetTitleSize()
    s := eui.UIScale()
    if shortcutsWin.NoScale { s = 1 }
    pad := (shortcutsWin.Padding + shortcutsWin.BorderPad) * s
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
    if shortcutsList.Parent != nil {
        shortcutsList.Parent.Size.X = clientWAvail
        shortcutsList.Parent.Size.Y = clientHAvail
    }
    shortcutsList.Size.X = clientWAvail
    shortcutsList.Size.Y = clientHAvail

    shortcutsList.Contents = shortcutsList.Contents[:0]
    shortcutMu.RLock()
    type pair struct{ short, full string }
    type entry struct {
        owner  string
        macros []pair
    }
    var plugins []entry
    for owner, m := range shortcutMaps {
        e := entry{owner: owner}
        for k, v := range m {
            e.macros = append(e.macros, pair{k, v})
        }
        plugins = append(plugins, e)
    }
    shortcutMu.RUnlock()
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
        shortcutsList.AddItem(ht)
        sort.Slice(p.macros, func(i, j int) bool { return p.macros[i].short < p.macros[j].short })
        for _, m := range p.macros {
            txt := fmt.Sprintf("  %s = %s", m.short, strings.TrimSpace(m.full))
            t, _ := eui.NewText()
            t.Text = txt
            t.FontSize = float32(fontSize)
            t.Size = eui.Point{X: clientWAvail, Y: rowUnits}
            shortcutsList.AddItem(t)
        }
    }
    if shortcutsWin != nil {
        shortcutsWin.Refresh()
    }
}
