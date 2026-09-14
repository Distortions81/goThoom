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
	savedDataWin     *eui.WindowData
	savedDataRoot    *eui.ItemData
	savedDataList    *eui.ItemData
	dataEntriesWin   *eui.WindowData
	dataEntriesList  *eui.ItemData
	dataEntriesOwner string
	dataEntriesWrap  eui.TextWindowWrapCache
)

func makeSavedDataWindow() {
	if savedDataWin != nil {
		return
	}
	savedDataWin = eui.NewWindow()
	savedDataWin.Title = "Saved Data"
	savedDataWin.Size = eui.Point{X: 520, Y: 360}
	savedDataWin.Closable = true
	savedDataWin.Movable = true
	savedDataWin.Resizable = true
	savedDataWin.NoScroll = true
	savedDataWin.SetZone(eui.HZoneCenter, eui.VZoneMiddleTop)

	savedDataRoot = eui.NewColumn()
	savedDataRoot.Fixed = true
	savedDataWin.AddItem(savedDataRoot)
	intro := eui.NewWrappedLabel("Data saved by scripts is stored locally. Select View to inspect a script's keys and values.", 480)
	intro.Position.Y = 2
	savedDataRoot.AddItem(intro)
	savedDataList = &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL, Scrollable: true, Fixed: true}
	savedDataList.Position.Y = 4
	savedDataRoot.AddItem(savedDataList)

	savedDataWin.OnResize = func() {
		refreshSavedDataList()
	}
	savedDataWin.AddWindow(false)
	refreshSavedDataList()
}

func refreshSavedDataList() {
	if savedDataList == nil {
		return
	}
	eui.LayoutWindowBody(savedDataWin, savedDataRoot, savedDataList)
	savedDataList.Contents = savedDataList.Contents[:0]

	scriptMu.RLock()
	owners := make([]string, 0, len(scriptDisplayNames))
	for o := range scriptDisplayNames {
		owners = append(owners, o)
	}
	scriptMu.RUnlock()
	sort.Strings(owners)

	visible := 0
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
		row := newSavedDataRow(disp, count, fi.Size(), savedDataList.Size.X, o)
		row.Filled = visible%2 == 1
		row.Color = eui.SubtleAlternateRowColor()
		savedDataList.AddItem(row)
		visible++
	}
	if visible == 0 {
		empty := eui.NewWrappedLabel("No scripts have saved data yet.", savedDataContentWidth(savedDataList.Size.X))
		empty.Size.Y = 32
		savedDataList.AddItem(empty)
	}
	if savedDataWin != nil {
		savedDataWin.Refresh()
	}
}

func savedDataContentWidth(listWidth float32) float32 {
	scale := eui.UIScale()
	if scale <= 0 {
		scale = 1
	}
	return max(float32(1), listWidth-eui.ScrollbarWidth()/scale-8)
}

func newSavedDataRow(displayName string, count int, bytes int64, listWidth float32, owner string) *eui.ItemData {
	const (
		viewWidth  = float32(92)
		rowPadding = float32(8)
	)
	contentWidth := savedDataContentWidth(listWidth)
	textWidth := max(float32(1), contentWidth-viewWidth-rowPadding*3)

	name := eui.NewWrappedLabel(displayName, textWidth)
	name.FontSize = 13
	name.SetWrappedText(displayName)
	name.SetTooltip(displayName)
	scale := eui.UIScale()
	if scale <= 0 {
		scale = 1
	}
	nameHeight := name.GetSize().Y / scale

	details := eui.NewLabel(fmt.Sprintf("%d entries  |  %s", count, humanize.Bytes(uint64(bytes))))
	details.FontSize = 11
	details.Position = eui.Point{X: rowPadding, Y: rowPadding + nameHeight + 2}
	details.Size = eui.Point{X: textWidth, Y: 20}
	name.Position = eui.Point{X: rowPadding, Y: rowPadding}

	rowHeight := max(float32(52), details.Position.Y+details.Size.Y+rowPadding)
	viewBtn, viewEvents := eui.NewButton()
	viewBtn.Text = "View"
	setMaterialButtonIcon(viewBtn, "visibility")
	viewBtn.Size = eui.Point{X: viewWidth, Y: 30}
	viewBtn.ConstrainToSize = true
	viewBtn.Position = eui.Point{X: contentWidth - viewWidth - rowPadding, Y: (rowHeight - 30) / 2}
	viewBtn.SetTooltip("View saved keys and values.")
	viewEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			showSavedDataEntries(owner)
		}
	}

	row := eui.NewOverlay(name, details, viewBtn)
	row.Fixed = true
	row.ConstrainToSize = true
	row.Size = eui.Point{X: contentWidth, Y: rowHeight}
	return row
}

func showSavedDataEntries(owner string) {
	if dataEntriesWin == nil {
		dataEntriesWin, dataEntriesList, _ = eui.NewTextWindow("Saved Data", eui.HZoneCenter, eui.VZoneMiddleTop, false)
		dataEntriesWin.Size = eui.Point{X: 560, Y: 420}
		dataEntriesWin.OnResize = func() {
			refreshSavedDataEntries(dataEntriesOwner)
		}
	}
	dataEntriesOwner = owner
	dataEntriesWin.Title = "Saved Data"
	dataEntriesList.Scroll = eui.Point{}
	refreshSavedDataEntries(owner)
	dataEntriesWin.MarkOpen()
}

func refreshSavedDataEntries(owner string) {
	if dataEntriesList == nil {
		return
	}
	ps := getscriptStore(owner)
	ps.mu.Lock()
	keys := make([]string, 0, len(ps.data))
	for k := range ps.data {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	messages := make([]string, 0, len(keys)+1)
	messages = append(messages, "Script\n"+getscriptDisplayName(owner))
	for _, k := range keys {
		v := ps.data[k]
		b, err := json.MarshalIndent(v, "", "  ")
		if err != nil {
			b = []byte(fmt.Sprint(v))
		}
		messages = append(messages, fmt.Sprintf("%s\n%s", k, strings.TrimSpace(string(b))))
	}
	ps.mu.Unlock()
	if len(keys) == 0 {
		messages = append(messages, "No saved keys.")
	}
	eui.UpdateTextWindow(dataEntriesWin, dataEntriesList, nil, messages, eui.TextWindowOptions{
		FontSize: 12, AlternateRows: true,
	}, &dataEntriesWrap)
	if dataEntriesWin != nil {
		dataEntriesWin.Refresh()
	}
}
