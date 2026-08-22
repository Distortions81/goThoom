package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"gothoom/eui"

	"github.com/dustin/go-humanize"
)

var (
	savedDataWin    *eui.WindowData
	savedDataList   *eui.ItemData
	dataEntriesWin  *eui.WindowData
	dataEntriesList *eui.ItemData
)

func makeSavedDataWindow() {
	if savedDataWin != nil {
		return
	}
	savedDataWin = eui.NewWindow()
	savedDataWin.Title = "Saved Data"
	savedDataWin.Size = eui.Point{X: 320, Y: 240}
	savedDataWin.Closable = true
	savedDataWin.Movable = true
	savedDataWin.Resizable = true
	savedDataWin.NoScroll = true
	savedDataWin.SetZone(eui.HZoneCenter, eui.VZoneMiddleTop)

	flow := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL, Fixed: true}
	savedDataWin.AddItem(flow)
	savedDataList = &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL, Scrollable: true, Fixed: true}
	flow.AddItem(savedDataList)

	savedDataWin.OnResize = func() {
		refreshSavedDataList()
		if savedDataWin != nil {
			savedDataWin.Refresh()
		}
	}
	savedDataWin.AddWindow(false)
	refreshSavedDataList()
}

func refreshSavedDataList() {
	if savedDataList == nil {
		return
	}
	savedDataList.Contents = savedDataList.Contents[:0]

	scriptMu.RLock()
	owners := make([]string, 0, len(scriptDisplayNames))
	for o := range scriptDisplayNames {
		owners = append(owners, o)
	}
	scriptMu.RUnlock()
	sort.Strings(owners)

	for _, o := range owners {
		path := scriptStoragePath(o)
		fi, err := os.Stat(path)
		if err != nil || fi.Size() == 0 {
			continue
		}
		ps := getscriptStore(o)
		ps.mu.Lock()
		count := len(ps.data)
		ps.mu.Unlock()
		if count == 0 {
			continue
		}
		disp := getscriptDisplayName(o)
		row := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_HORIZONTAL}
		txt, _ := eui.NewText()
		txt.Text = fmt.Sprintf("%s (%d entries, %s)", disp, count, humanize.Bytes(uint64(fi.Size())))
		txt.Size = eui.Point{X: 240, Y: 24}
		row.AddItem(txt)
		viewBtn, vh := eui.NewButton()
		viewBtn.Text = "View"
		viewBtn.Size = eui.Point{X: 64, Y: 24}
		owner := o
		vh.Handle = func(ev eui.UIEvent) {
			if ev.Type == eui.EventClick {
				showSavedDataEntries(owner)
			}
		}
		row.AddItem(viewBtn)
		savedDataList.AddItem(row)
	}
	if savedDataWin != nil {
		savedDataWin.Refresh()
	}
}

func showSavedDataEntries(owner string) {
	if dataEntriesWin == nil {
		dataEntriesWin = eui.NewWindow()
		dataEntriesWin.Size = eui.Point{X: 320, Y: 240}
		dataEntriesWin.Closable = true
		dataEntriesWin.Movable = true
		dataEntriesWin.Resizable = true
		dataEntriesWin.NoScroll = true
		dataEntriesWin.SetZone(eui.HZoneCenter, eui.VZoneMiddleTop)
		flow := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL, Fixed: true}
		dataEntriesWin.AddItem(flow)
		dataEntriesList = &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL, Scrollable: true, Fixed: true}
		flow.AddItem(dataEntriesList)
		dataEntriesWin.OnResize = func() {
			refreshSavedDataEntries(owner)
			if dataEntriesWin != nil {
				dataEntriesWin.Refresh()
			}
		}
		dataEntriesWin.AddWindow(false)
	}
	dataEntriesWin.Title = getscriptDisplayName(owner) + " Data"
	refreshSavedDataEntries(owner)
	dataEntriesWin.MarkOpen()
}

func refreshSavedDataEntries(owner string) {
	if dataEntriesList == nil {
		return
	}
	dataEntriesList.Contents = dataEntriesList.Contents[:0]
	ps := getscriptStore(owner)
	ps.mu.Lock()
	keys := make([]string, 0, len(ps.data))
	for k := range ps.data {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		v := ps.data[k]
		b, _ := json.Marshal(v)
		t, _ := eui.NewText()
		t.Text = fmt.Sprintf("%s: %s", k, strings.TrimSpace(string(b)))
		t.Size = eui.Point{X: 280, Y: 24}
		dataEntriesList.AddItem(t)
	}
	ps.mu.Unlock()
	if dataEntriesWin != nil {
		dataEntriesWin.Refresh()
	}
}
