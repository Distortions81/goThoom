//go:build plugin

package main

import (
	"time"

	"gt"
)

// Plugin metadata
const PluginName = "Healer Helper"
const PluginAuthor = "Examples"
const PluginCategory = "Profession"
const PluginAPIVersion = 1

// Init launches a tiny loop that watches for right clicks on ourselves.
func Init() {
	go func() {
		for {
			if gt.MouseJustPressed("right") {
				c := gt.LastClick()
				if c.OnMobile {
					if gt.IgnoreCase(c.Mobile.Name, gt.PlayerName()) {
						equipItem("moonstone")
						gt.Run("/use 10")
					} else {
						equipItem("asklepean")
						gt.Run("/use " + c.Mobile.Name)
					}
				}
			} else if gt.MouseJustPressed("middle") {
				c := gt.LastClick()
				if c.OnMobile {
					if gt.IgnoreCase(c.Mobile.Name, gt.PlayerName()) {
						equipItem("asklepean")
						gt.Run("/use")
					} else {
						equipItem("moonstone")
						gt.Run("/use 10")
					}
				}
			}
			time.Sleep(50 * time.Millisecond)
		}
	}()
}

// equipItem equips the moonstone if it isn't already in hand.
func equipItem(item string) {
	for _, it := range gt.Inventory() {
		if gt.IgnoreCase(it.Name, item) {
			if !it.Equipped {
				gt.Equip(it.ID)
			}
			return
		}
	}
}
