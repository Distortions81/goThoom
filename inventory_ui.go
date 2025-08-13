//go:build !test

package main

import (
	"gothoom/eui"
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
			t, _ := eui.NewText()
			t.Text = text
			t.Size = eui.Point{X: 256, Y: 24}
			t.FontSize = 10
			inventoryList.AddItem(t)
			changed = true
		}
		logDebug("Ivn Name: %v, ID: %v", it.Name, it.ID)
	}
	if len(inventoryList.Contents) > len(items) {
		for i := len(items); i < len(inventoryList.Contents); i++ {
			inventoryList.Contents[i] = nil
		}
		inventoryList.Contents = inventoryList.Contents[:len(items)]
		changed = true
	}
	if changed {
		inventoryList.Dirty = true
	}
}
