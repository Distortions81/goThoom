//go:build !test

package main

import (
	"github.com/Distortions81/EUI/eui"
)

var inventoryWin *eui.WindowData
var inventoryList *eui.ItemData

func updateInventoryWindow() {
	if inventoryList == nil {
		return
	}
	items := getInventory()
	inventoryList.Contents = inventoryList.Contents[:0]
	for _, it := range items {
		text := it.Name
		if it.Equipped {
			text = "* " + text
		}
		t, _ := eui.NewText(&eui.ItemData{Text: text, Size: eui.Point{X: 256, Y: 24}, FontSize: 10})
		inventoryList.AddItem(t)
		logDebug("Ivn Name: %v, ID: %v", it.Name, it.ID)
	}
	if inventoryWin != nil {
		inventoryWin.Refresh()
	}
}
