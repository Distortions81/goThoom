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
	shortcutsWin      *eui.WindowData
	shortcutsList     *eui.ItemData
	shortcutEditWin   *eui.WindowData
	shortcutShortInp  *eui.ItemData
	shortcutFullInp   *eui.ItemData
	shortcutEditOwner string
)

func makeShortcutsWindow() {
	if shortcutsWin != nil {
		return
	}
	shortcutsWin = eui.NewWindow()
	shortcutsWin.Title = "Shortcuts"
	shortcutsWin.Size = eui.Point{X: 500, Y: 500}
	shortcutsWin.Closable = true
	shortcutsWin.Movable = true
	shortcutsWin.Resizable = true
	shortcutsWin.NoScroll = true
	shortcutsWin.SetZone(eui.HZoneCenter, eui.VZoneMiddleTop)

	flow := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL, Fixed: true}
	shortcutsWin.AddItem(flow)

	btnRow := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_HORIZONTAL, Fixed: true}
	addUserBtn, addUserEvents := eui.NewButton()
	addUserBtn.Text = "Add for character"
	addUserBtn.Size = eui.Point{X: 160, Y: 24}
	addUserEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			openShortcutEditor("user")
		}
	}
	btnRow.AddItem(addUserBtn)
	addGlobalBtn, addGlobalEvents := eui.NewButton()
	addGlobalBtn.Text = "Add For All"
	addGlobalBtn.Size = eui.Point{X: 160, Y: 24}
	addGlobalEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			openShortcutEditor("global")
		}
	}
	btnRow.AddItem(addGlobalBtn)
	flow.AddItem(btnRow)

	shortcutsList = &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL, Scrollable: true, Fixed: true}
	flow.AddItem(shortcutsList)
	shortcutsWin.OnResize = func() {
		refreshShortcutsList()
		if shortcutsWin != nil {
			shortcutsWin.Refresh()
		}
	}
	shortcutsWin.AddWindow(false)
	refreshShortcutsList()
}

func openShortcutEditor(owner string) {
	if shortcutEditWin != nil {
		return
	}
	shortcutEditOwner = owner
	shortcutEditWin = eui.NewWindow()
	shortcutEditWin.OnClose = func() { shortcutEditWin = nil }
	if owner == "global" {
		shortcutEditWin.Title = "Global Shortcut"
	} else {
		shortcutEditWin.Title = "User Shortcut"
	}
	shortcutEditWin.Size = eui.Point{X: 280, Y: 120}
	shortcutEditWin.AutoSize = true
	shortcutEditWin.Closable = true
	shortcutEditWin.Movable = true
	shortcutEditWin.Resizable = false
	shortcutEditWin.NoScroll = true
	shortcutEditWin.SetZone(eui.HZoneCenter, eui.VZoneMiddleTop)

	flow := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL, Fixed: true}
	shortcutEditWin.AddItem(flow)

	shortRow := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_HORIZONTAL, Fixed: true}
	shortLbl, _ := eui.NewText()
	shortLbl.Text = "In:"
	shortLbl.Size = eui.Point{X: 80, Y: 20}
	shortLbl.FontSize = 12
	shortRow.AddItem(shortLbl)
	shortcutShortInp, _ = eui.NewInput()
	shortcutShortInp.Size = eui.Point{X: 400, Y: 20}
	shortcutShortInp.FontSize = 12
	shortRow.AddItem(shortcutShortInp)
	flow.AddItem(shortRow)

	fullRow := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_HORIZONTAL, Fixed: true}
	fullLbl, _ := eui.NewText()
	fullLbl.Text = "Out:"
	fullLbl.Size = eui.Point{X: 80, Y: 20}
	fullLbl.FontSize = 12
	fullRow.AddItem(fullLbl)
	shortcutFullInp, _ = eui.NewInput()
	shortcutFullInp.Size = eui.Point{X: 400, Y: 20}
	shortcutFullInp.FontSize = 12
	fullRow.AddItem(shortcutFullInp)
	flow.AddItem(fullRow)

	btnRow := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_HORIZONTAL, Fixed: true}
	okBtn, okEvents := eui.NewButton()
	okBtn.Text = "OK"
	okBtn.Size = eui.Point{X: 80, Y: 20}
	okBtn.FontSize = 12
	okEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			finishShortcutEdit(true)
		}
	}
	btnRow.AddItem(okBtn)
	cancelBtn, cancelEvents := eui.NewButton()
	cancelBtn.Text = "Cancel"
	cancelBtn.Size = eui.Point{X: 80, Y: 20}
	cancelBtn.FontSize = 12
	cancelEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			finishShortcutEdit(false)
		}
	}
	btnRow.AddItem(cancelBtn)
	flow.AddItem(btnRow)

	shortcutEditWin.AddWindow(true)
	shortcutEditWin.MarkOpen()
}

func finishShortcutEdit(ok bool) {
	if shortcutEditWin == nil {
		return
	}
	if ok {
		short := strings.TrimSpace(shortcutShortInp.Text)
		full := strings.TrimSpace(shortcutFullInp.Text)
		if short != "" && full != "" {
			if shortcutEditOwner == "global" {
				addGlobalShortcut(short, full)
			} else {
				addUserShortcut(short, full)
			}
		}
	}
	shortcutEditWin.Close()
	shortcutEditWin = nil
}

func refreshShortcutsList() {
	if shortcutsList == nil {
		return
	}
	// Compute client area for sizing the flow and list.
	clientW := shortcutsWin.GetSize().X
	clientH := shortcutsWin.GetSize().Y - shortcutsWin.GetTitleSize()
	s := eui.UIScale()
	if shortcutsWin.NoScale {
		s = 1
	}
	pad := (shortcutsWin.Padding + shortcutsWin.BorderPad) * s
	clientWAvail := clientW - 2*pad
	if clientWAvail < 0 {
		clientWAvail = 0
	}
	clientHAvail := clientH - 2*pad
	if clientHAvail < 0 {
		clientHAvail = 0
	}

	// Determine row height from font metrics.
	fontSize := gs.ConsoleFontSize
	ui := eui.UIScale()
	// Match EUI's render-time size (fontSize*ui + 2) so rows aren't under-sized.
	facePx := float64(float32(fontSize)*ui) + 2
	var goFace *text.GoTextFace
	if src := eui.FontSource(); src != nil {
		goFace = &text.GoTextFace{Source: src, Size: facePx}
	} else {
		goFace = &text.GoTextFace{Size: facePx}
	}
	metrics := goFace.Metrics()
	linePx := math.Ceil(metrics.HAscent + metrics.HDescent + 2)
	rowUnits := float32(linePx) / ui

	shortcutsList.Size.X = clientWAvail
	shortcutsList.Size.Y = clientHAvail - (rowUnits * 2)

	shortcutsList.Contents = shortcutsList.Contents[:0]
	shortcutMu.RLock()
	type pair struct{ short, full string }
	type entry struct {
		owner  string
		macros []pair
	}
	var scripts []entry
	for owner, m := range shortcutMaps {
		e := entry{owner: owner}
		for k, v := range m {
			e.macros = append(e.macros, pair{k, v})
		}
		scripts = append(scripts, e)
	}
	shortcutMu.RUnlock()
	sort.Slice(scripts, func(i, j int) bool { return scripts[i].owner < scripts[j].owner })
	for _, p := range scripts {
		// Header label: for user, show the character name; for scripts, add (script)
		var label string
		if p.owner == "user" {
			if name := effectiveCharacterName(); name != "" {
				label = name
			} else {
				label = "user"
			}
		} else {
			label = getscriptDisplayName(p.owner)
			if p.owner != "global" {
				label += " (script)"
			}
		}
		ht, _ := eui.NewText()
		ht.Text = label + ":"
		ht.FontSize = float32(fontSize)
		ht.Size = eui.Point{X: clientWAvail, Y: rowUnits}
		shortcutsList.AddItem(ht)
		sort.Slice(p.macros, func(i, j int) bool { return p.macros[i].short < p.macros[j].short })
		if p.owner == "user" || p.owner == "global" {
			for _, m := range p.macros {
				row := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_HORIZONTAL, Fixed: true}
				width := clientWAvail - rowUnits*1
				if p.owner == "user" || p.owner == "global" {
					width -= rowUnits
				}
				txt := fmt.Sprintf("  %s = %s", m.short, strings.TrimSpace(m.full))
				t, _ := eui.NewText()
				t.Text = txt
				t.FontSize = float32(fontSize)
				t.Size = eui.Point{X: width, Y: rowUnits}
				row.AddItem(t)
				delBtn, delEvents := eui.NewButton()
				delBtn.Text = "X"
				delBtn.Size = eui.Point{X: rowUnits, Y: rowUnits}
				delBtn.FontSize = float32(fontSize)
				owner := p.owner
				short := m.short
				delEvents.Handle = func(ev eui.UIEvent) {
					if ev.Type == eui.EventClick {
						removeShortcut(owner, short)
					}
				}
				row.AddItem(delBtn)
				shortcutsList.AddItem(row)
			}
		} else {
			// Script-provided shortcuts: show only a summary with a View button.
			count := len(p.macros)
			row := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_HORIZONTAL, Fixed: true}
			width := clientWAvail - (rowUnits * 4)
			if width < rowUnits*4 {
				width = clientWAvail
			}
			t, _ := eui.NewText()
			if count == 1 {
				t.Text = "  1 shortcut"
			} else {
				t.Text = fmt.Sprintf("  %d shortcuts", count)
			}
			t.FontSize = float32(fontSize)
			t.Size = eui.Point{X: width, Y: rowUnits}
			row.AddItem(t)
			viewBtn, vh := eui.NewButton()
			viewBtn.Text = "View"
			viewBtn.Size = eui.Point{X: rowUnits * 3, Y: rowUnits}
			viewBtn.FontSize = float32(fontSize)
			owner := p.owner
			vh.Handle = func(ev eui.UIEvent) {
				if ev.Type == eui.EventClick {
					makescriptsWindow()
					selectscript(owner)
					if scriptsWin != nil {
						scriptsWin.MarkOpen()
					}
				}
			}
			row.AddItem(viewBtn)
			shortcutsList.AddItem(row)
		}
	}
	if shortcutsWin != nil {
		shortcutsWin.Refresh()
	}
}
