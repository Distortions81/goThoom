//go:build !test

package main

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"gothoom/eui"
)

func promptCustomGroupName(title, initial string, accept func(string)) {
	input, _ := eui.NewInput()
	input.Label = "Group name"
	input.Text = initial
	input.Size = eui.Point{X: 300, Y: 28}
	showPopup(title, "", []popupButton{
		{Text: "Cancel"},
		{Text: "OK", Action: func() {
			if name := strings.TrimSpace(input.Text); name != "" {
				accept(name)
			}
		}},
	}, input)
}

func showCustomGroupPicker(groups *customGroups, entry, noun string, pos eui.Point, changed func()) {
	groups.normalize()
	options := []string{fmt.Sprintf("%s group", noun), "Ungrouped"}
	options = append(options, groups.Names...)
	options = append(options, "New group…")
	menu := eui.ShowContextMenu(options, pos.X, pos.Y, func(i int) {
		switch {
		case i == 1:
			groups.assign(entry, "")
			settingsDirty = true
			changed()
		case i >= 2 && i < 2+len(groups.Names):
			groups.assign(entry, groups.Names[i-2])
			settingsDirty = true
			changed()
		case i == 2+len(groups.Names):
			promptCustomGroupName("New Group", "", func(name string) {
				name = groups.add(name)
				groups.assign(entry, name)
				settingsDirty = true
				changed()
			})
		}
	})
	if menu != nil {
		menu.HeaderCount = 1
	}
}

func showEditPlayerGroupWindow(group string) {
	showDualGroupEditor("Edit Players — "+group, "Online players", "In "+group,
		func() (available, members []groupEditorEntry) {
			players := getPlayers()
			sort.Slice(players, func(i, j int) bool { return strings.ToLower(players[i].Name) < strings.ToLower(players[j].Name) })
			for _, player := range players {
				if player.Name == "" || player.IsNPC {
					continue
				}
				entry := groupEditorEntry{key: playerCustomGroupKey(player.Name), label: player.Name}
				assigned := gs.PlayerGroups.group(entry.key)
				if strings.EqualFold(assigned, group) {
					members = append(members, entry)
				} else if assigned == "" && !player.Offline {
					available = append(available, entry)
				}
			}
			return
		},
		func(key string, add bool) {
			if add {
				gs.PlayerGroups.assign(key, group)
			} else {
				gs.PlayerGroups.assign(key, "")
			}
			settingsDirty = true
			playersDirty = true
		},
		func() {
			gs.PlayerGroups.remove(group)
			settingsDirty = true
			playersDirty = true
		},
		"Add Visible Players", func() {
			for _, key := range nearbyVisiblePlayerGroupKeys() {
				gs.PlayerGroups.assign(key, group)
			}
			settingsDirty = true
			playersDirty = true
		})
}

func inventoryCustomGroupKey(id uint16) string { return strconv.Itoa(int(id)) }

type groupEditorEntry struct {
	key   string
	label string
}

func sortGroupEditorEntries(entries []groupEditorEntry) {
	sort.Slice(entries, func(i, j int) bool {
		return strings.ToLower(entries[i].label) < strings.ToLower(entries[j].label)
	})
}

func showDualGroupEditor(title, availableTitle, memberTitle string, entries func() (available, members []groupEditorEntry), move func(string, bool), deleteGroup func(), quickLabel string, quickAdd func()) {
	win := eui.NewWindow()
	win.Title = title
	win.Size = eui.Point{X: 680, Y: 500}
	win.Closable = true
	win.Resizable = false
	win.Movable = true
	win.NoScroll = true
	win.SetZone(eui.HZoneCenter, eui.VZoneMiddleTop)

	outer := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL, Fixed: true}
	headings := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_HORIZONTAL, Fixed: true}
	leftTitle, _ := eui.NewText()
	leftTitle.Text = availableTitle
	leftTitle.Face = mainFontBold
	leftTitle.Size = eui.Point{X: 320, Y: 28}
	headings.AddItem(leftTitle)
	rightTitle, _ := eui.NewText()
	rightTitle.Text = memberTitle
	rightTitle.Face = mainFontBold
	rightTitle.Size = eui.Point{X: 320, Y: 28}
	headings.AddItem(rightTitle)
	outer.AddItem(headings)

	lists := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_HORIZONTAL, Fixed: true}
	left := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL, Fixed: true, Scrollable: true, Size: eui.Point{X: 320, Y: 390}}
	right := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL, Fixed: true, Scrollable: true, Size: eui.Point{X: 320, Y: 390}}
	lists.AddItem(left)
	lists.AddItem(right)
	outer.AddItem(lists)

	var rebuild func()
	rebuild = func() {
		available, members := entries()
		sortGroupEditorEntries(available)
		sortGroupEditorEntries(members)
		left.Contents = nil
		right.Contents = nil
		for _, entry := range available {
			button, events := eui.NewButton()
			button.Text = entry.label
			setMaterialButtonIcon(button, "add")
			button.Size = eui.Point{X: 300, Y: 24}
			key := entry.key
			events.Handle = func(ev eui.UIEvent) {
				if ev.Type == eui.EventClick {
					move(key, true)
					rebuild()
				}
			}
			left.AddItem(button)
		}
		for _, entry := range members {
			button, events := eui.NewButton()
			button.Text = entry.label
			setMaterialButtonIcon(button, "remove")
			button.Size = eui.Point{X: 300, Y: 24}
			key := entry.key
			events.Handle = func(ev eui.UIEvent) {
				if ev.Type == eui.EventClick {
					move(key, false)
					rebuild()
				}
			}
			right.AddItem(button)
		}
		win.Refresh()
	}

	buttons := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_HORIZONTAL, Fixed: true}
	if quickAdd != nil && quickLabel != "" {
		quickButton, quickEvents := eui.NewButton()
		quickButton.Text = quickLabel
		setMaterialButtonIcon(quickButton, "add")
		quickButton.Size = eui.Point{X: 180, Y: 26}
		quickButton.SetTooltip("Add nearby players.")
		quickEvents.Handle = func(ev eui.UIEvent) {
			if ev.Type == eui.EventClick {
				quickAdd()
				rebuild()
			}
		}
		buttons.AddItem(quickButton)
	}
	deleteButton, deleteEvents := eui.NewButton()
	deleteButton.Text = "Delete Group"
	setMaterialButtonIcon(deleteButton, "delete")
	deleteButton.Color = eui.ColorDarkRed
	deleteButton.HoverColor = eui.ColorRed
	deleteButton.Size = eui.Point{X: 140, Y: 26}
	deleteEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			deleteGroup()
			win.Close()
		}
	}
	buttons.AddItem(deleteButton)
	closeButton, closeEvents := eui.NewButton()
	closeButton.Text = "Close"
	setMaterialButtonIcon(closeButton, "close")
	closeButton.Size = eui.Point{X: 110, Y: 26}
	closeEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			win.Close()
		}
	}
	buttons.AddItem(closeButton)
	outer.AddItem(buttons)

	win.AddItem(outer)
	win.AddWindow(false)
	rebuild()
	win.MarkOpen()
}

func showEditInventoryGroupWindow(group string) {
	showDualGroupEditor("Edit Items — "+group, "Available items", "In "+group,
		func() (available, members []groupEditorEntry) {
			byID := make(map[uint16]string)
			for _, item := range getInventory() {
				name := item.Name
				if name == "" && clImages != nil {
					name = clImages.ItemName(uint32(item.ID))
				}
				if name == "" {
					name = fmt.Sprintf("Item %d", item.ID)
				}
				if _, exists := byID[item.ID]; !exists {
					byID[item.ID] = name
				}
			}
			for id, name := range byID {
				entry := groupEditorEntry{key: inventoryCustomGroupKey(id), label: name}
				assigned := gs.InventoryGroups.group(entry.key)
				if strings.EqualFold(assigned, group) {
					members = append(members, entry)
				} else if assigned == "" {
					available = append(available, entry)
				}
			}
			sortGroupEditorEntries(available)
			sortGroupEditorEntries(members)
			return
		},
		func(key string, add bool) {
			if add {
				gs.InventoryGroups.assign(key, group)
			} else {
				gs.InventoryGroups.assign(key, "")
			}
			settingsDirty = true
			inventoryDirty = true
			updateInventoryWindow()
		},
		func() {
			gs.InventoryGroups.remove(group)
			settingsDirty = true
			inventoryDirty = true
			updateInventoryWindow()
		}, "", nil)
}
