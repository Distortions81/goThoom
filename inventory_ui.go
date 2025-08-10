//go:build !test

package main

import (
	"github.com/Distortions81/EUI/eui"
)

var inventoryWin *eui.WindowData
var inventoryList *eui.ItemData
var inventoryDirty bool

func updateInventoryWindow() {
	if inventoryList == nil {
		return
	}
	items := getInventory()
	changed := false
	for i, it := range items {
		text := it.Name
		if it.Equipped {
			text = "* " + text
		}
		if i < len(inventoryList.Contents) {
			if inventoryList.Contents[i].Text != text {
				inventoryList.Contents[i].Text = text
				changed = true
			}
		} else {
			t, err := eui.NewText(&eui.ItemData{Text: text, Size: eui.Point{X: 256, Y: 24}, FontSize: 10})
			if err != nil {
				logError("failed to create inventory text: %v", err)
				continue
			}
			inventoryList.AddItem(t)
			changed = true
		}
		logDebug("Ivn Name: %v, ID: %v", it.Name, it.ID)
	}
	if len(inventoryList.Contents) > len(items) {
		inventoryList.Contents = inventoryList.Contents[:len(items)]
		changed = true
	}
	if changed {
		inventoryDirty = true
	}
}
