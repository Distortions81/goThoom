//go:build !test

package main

import (
	"fmt"
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
	if inventoryWin == nil || inventoryList == nil {
		return
	}

	// Build a unique list of items by ID while counting duplicates.
	items := getInventory()
	counts := make(map[uint16]int)
	first := make(map[uint16]inventoryItem)
	order := make([]uint16, 0, len(items))
	for _, it := range items {
		if _, seen := counts[it.ID]; !seen {
			order = append(order, it.ID)
			first[it.ID] = it
		}
		counts[it.ID]++
	}

	// Clear prior contents and rebuild rows as [icon][name (xN)].
	inventoryList.Contents = nil

	// Auto-scale row height to approximately the text height.
	// Use console font size + a small buffer for clarity.
	iconSize := int(gs.ConsoleFontSize + 2)
	for _, id := range order {
		it := first[id]
		qty := counts[id]

		// Row container for icon + text
		row := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_HORIZONTAL, Fixed: true}

		// Icon
		icon, _ := eui.NewImageItem(iconSize, iconSize)
		icon.Filled = false
		icon.Border = 0

		// Choose a pict ID for the item sprite.
		var pict uint32
		loc := ""
		if clImages != nil {
			if p := clImages.ItemWornPict(uint32(id)); p != 0 {
				pict = p
				loc = "worn"
			} else if p := clImages.ItemRightHandPict(uint32(id)); p != 0 {
				pict = p
				loc = "right"
			} else if p := clImages.ItemLeftHandPict(uint32(id)); p != 0 {
				pict = p
				loc = "left"
			}
		}
		if pict != 0 {
			if img := loadImage(uint16(pict)); img != nil {
				icon.Image = img
				icon.ImageName = fmt.Sprintf("item:%d", id)
			}
		}
		// Add a small right margin after the icon
		icon.Margin = 4
		row.AddItem(icon)

		// Text label with quantity suffix when >1
		label := it.Name
		if label == "" && clImages != nil {
			label = clImages.ItemName(uint32(id))
		}
		if label == "" {
			label = fmt.Sprintf("Item %d", id)
		}
		if qty > 1 {
			label = fmt.Sprintf("%s (x%d)", label, qty)
		}
		if loc != "" {
			label = fmt.Sprintf("%s [%s]", label, loc)
		}

		t, _ := eui.NewText()
		t.Text = label
		t.FontSize = float32(gs.ConsoleFontSize)
		// Constrain the text item height to match icon/text height.
		t.Size.Y = float32(iconSize)
		t.Size.X = 1000
		row.AddItem(t)

		// Row height matches the icon/text height with minimal padding.
		row.Size.Y = float32(iconSize)

		inventoryList.AddItem(row)
	}

	// Size the list and refresh window similar to updateTextWindow behavior.
	if inventoryWin != nil {
		clientW := inventoryWin.GetSize().X
		clientH := inventoryWin.GetSize().Y - inventoryWin.GetTitleSize()
		if inventoryList.Parent != nil {
			inventoryList.Parent.Size.X = clientW
			inventoryList.Parent.Size.Y = clientH
		}
		inventoryList.Size.X = clientW
		inventoryList.Size.Y = clientH
		inventoryWin.Refresh()
	}
}
