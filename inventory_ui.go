//go:build !test

package main

import (
	"gothoom/eui"
)

var inventoryWin *eui.WindowData
var inventoryList *eui.ItemData
var inventoryDirty bool

func makeInventoryWindow() {
	if inventoryWin != nil {
		return
	}
	inventoryWin, inventoryList, _ = makeTextWindow("Inventory", eui.HZoneLeft, eui.VZoneTop, true)
	updateInventoryWindow()
}

func updateInventoryWindow() {
	invstr := []string{}

	items := getInventory()
	for _, item := range items {
		invstr = append(invstr, "* "+item.Name)
	}
	updateTextWindow(inventoryWin, inventoryList, nil, invstr, gs.ConsoleFontSize, "")
}
