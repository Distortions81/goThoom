//go:build !test

package main

import (
	"fmt"
	"gothoom/eui"
	"math"

	text "github.com/hajimehoshi/ebiten/v2/text/v2"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

var inventoryWin *eui.WindowData
var inventoryList *eui.ItemData
var inventoryDirty bool

var TitleCaser = cases.Title(language.AmericanEnglish)

func makeInventoryWindow() {
	if inventoryWin != nil {
		return
	}
	inventoryWin, inventoryList, _ = makeTextWindow("Inventory", eui.HZoneLeft, eui.VZoneMiddleTop, true)
	// Ensure layout updates immediately on resize to avoid gaps.
	inventoryWin.OnResize = func() { updateInventoryWindow() }
	updateInventoryWindow()
}

func updateInventoryWindow() {
	if inventoryWin == nil || inventoryList == nil {
		return
	}

	// Build a unique list of items by ID while counting duplicates and tracking
	// whether any instance of a given ID is equipped.
	items := getInventory()
	counts := make(map[uint16]int)
	first := make(map[uint16]inventoryItem)
	anyEquipped := make(map[uint16]bool)
	order := make([]uint16, 0, len(items))
	for _, it := range items {
		if _, seen := counts[it.ID]; !seen {
			order = append(order, it.ID)
			first[it.ID] = it
		}
		counts[it.ID]++
		if it.Equipped {
			anyEquipped[it.ID] = true
		}
	}

	// Clear prior contents and rebuild rows as [icon][name (xN)].
	inventoryList.Contents = nil

	// Compute row height from actual font metrics (ascent+descent) and add
	// a small cushion so descenders are never clipped regardless of scale.
	fontSize := gs.InventoryFontSize
	if fontSize <= 0 {
		fontSize = gs.ConsoleFontSize
	}
	uiScale := eui.UIScale()
	// Build a face at the scaled point size and measure
	facePx := float64(float32(fontSize) * uiScale)
	var goFace *text.GoTextFace
	if src := eui.FontSource(); src != nil {
		goFace = &text.GoTextFace{Source: src, Size: facePx}
	} else {
		goFace = &text.GoTextFace{Size: facePx}
	}
	metrics := goFace.Metrics()
	// Use ceil(ascent+descent) plus 2px cushion to protect descenders
	rowPx := float32(math.Ceil(metrics.HAscent + metrics.HDescent + 2))
	rowUnits := rowPx / uiScale
	iconSize := int(rowUnits + 0.5)
	for _, id := range order {
		it := first[id]
		qty := counts[id]

		// Row container for icon + text
		row := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_HORIZONTAL, Fixed: true}

		// Icon
		icon, _ := eui.NewImageItem(iconSize, iconSize)
		icon.Filled = false
		icon.Border = 0

		// Choose a pict ID for the item sprite and determine equipped location.
		var pict uint32
		loc := ""
		if clImages != nil {
			// Inventory list usually uses the worn pict for display.
			if p := clImages.ItemWornPict(uint32(id)); p != 0 {
				pict = p
			}
			// Location label derived from slot, displayed only if any instance
			// of this item ID is equipped.
			if anyEquipped[id] {
				switch clImages.ItemSlot(uint32(id)) {
				case 14: // kItemSlotRightHand
					loc = "right"
				case 15: // kItemSlotLeftHand
					loc = "left"
				default:
					loc = "worn"
				}
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
			label = fmt.Sprintf("(%v) %v", qty, label)
		}
		if loc != "" {
			label = fmt.Sprintf("%v [%v]", label, loc)
		}

		t, _ := eui.NewText()

		t.Text = TitleCaser.String(label)
		t.FontSize = float32(fontSize)
		// Constrain the text item height to match the computed row height (UI units).
		t.Size.Y = rowUnits
		t.Size.X = 1000
		row.AddItem(t)

		// Row height matches the icon/text height with minimal padding.
		row.Size.Y = rowUnits

		inventoryList.AddItem(row)
	}

	// Add a trailing spacer equal to one row height so the last item is never
	// clipped at the bottom when fully scrolled.
	spacer, _ := eui.NewText()
	spacer.Text = ""
	spacer.Size = eui.Point{X: 1, Y: rowUnits}
	spacer.FontSize = float32(fontSize)
	inventoryList.AddItem(spacer)

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
