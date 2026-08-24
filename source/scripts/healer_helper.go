//go:build script

package main

import (
	"gt2"
	"strings"
)

// script metadata
const scriptName = "Healer Helper"
const scriptID = "healer-helper"
const scriptAuthor = "Examples"
const scriptCategory = "Profession"
const scriptAPIVersion = 2

// Init subscribes to mouse click hotkeys rather than polling.
func Init() {
	// RightClick: heal others, self-heal with moonstone
	gt2.Bind("RightClick", func(e gt2.InputEvent) {
		if !e.OnMobile {
			return
		}
		if strings.EqualFold(e.Mobile.Name, gt2.Self().Name) {
			// Right-click self: use moonstone on self slot 10
			equipItem("moonstone")
			gt2.Send("/use 10")
		} else {
			// Right-click other: use asklepean on target
			equipItem("asklepean")
			gt2.Send("/use " + e.Mobile.Name)
		}
	})

	// MiddleClick: reverse behavior from RightClick
	gt2.Bind("MiddleClick", func(e gt2.InputEvent) {
		if !e.OnMobile {
			return
		}
		if strings.EqualFold(e.Mobile.Name, gt2.Self().Name) {
			// Middle-click self: asklepean self-use
			equipItem("asklepean")
			gt2.Send("/use")
		} else {
			// Middle-click other: moonstone to slot 10
			equipItem("moonstone")
			gt2.Send("/use 10")
		}
	})
}

// equipItem equips the moonstone if it isn't already in hand.
func equipItem(name string) {
	for _, it := range gt2.Inventory() {
		if strings.EqualFold(it.Name, name) {
			if !it.Equipped {
				gt2.Equip(it.Name)
			}
			return
		}
	}
}
