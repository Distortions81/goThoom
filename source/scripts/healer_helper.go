//go:build script

package main

import (
	"gt"
)

// script metadata
const scriptName = "Healer Helper"
const scriptAuthor = "Examples"
const scriptCategory = "Profession"
const scriptAPIVersion = 1

// Init subscribes to mouse click hotkeys rather than polling.
func Init() {
	// RightClick: heal others, self-heal with moonstone
	gt.AddHotkeyFn("RightClick", func(e gt.HotkeyEvent) {
		c := gt.LastClick()
		if !c.OnMobile {
			return
		}
		if gt.IgnoreCase(c.Mobile.Name, gt.PlayerName()) {
			// Right-click self: use moonstone on self slot 10
			equipItem("moonstone")
			gt.Run("/use 10")
		} else {
			// Right-click other: use asklepean on target
			equipItem("asklepean")
			gt.Run("/use " + c.Mobile.Name)
		}
	})

	// MiddleClick: reverse behavior from RightClick
	gt.AddHotkeyFn("MiddleClick", func(e gt.HotkeyEvent) {
		c := gt.LastClick()
		if !c.OnMobile {
			return
		}
		if gt.IgnoreCase(c.Mobile.Name, gt.PlayerName()) {
			// Middle-click self: asklepean self-use
			equipItem("asklepean")
			gt.Run("/use")
		} else {
			// Middle-click other: moonstone to slot 10
			equipItem("moonstone")
			gt.Run("/use 10")
		}
	})
}

// equipItem equips the moonstone if it isn't already in hand.
func equipItem(name string) {
	for _, it := range gt.Inventory() {
		if gt.IgnoreCase(it.Name, name) {
			if !it.Equipped {
				gt.Equip(it.Name)
			}
			return
		}
	}
}
