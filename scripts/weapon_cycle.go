//go:build plugin

package main

import "gt"

// Plugin metadata
const PluginName = "Weapon Cycle"
const PluginAuthor = "Examples"
const PluginCategory = "Equipment"
const PluginAPIVersion = 1

var cycleItems = []string{"Axe", "Short Sword", "Dagger", "Chocolate"}

// Init binds F3 to cycle through weapons.
func Init() {
    gt.RegisterCommand("cycleweapon", cycleWeaponCmd)
    gt.AddHotkeyFn("F3", cycleWeaponHotkey)
}

func cycleWeaponCmd(args string) { cycleWeapon() }
func cycleWeaponHotkey(e gt.HotkeyEvent) { cycleWeapon() }

// cycleWeapon equips the next item in cycleItems.
func cycleWeapon() {
	inv := gt.Inventory()
	current := ""
	for _, it := range inv {
		if it.Equipped {
			current = it.Name
			break
		}
	}
	next := cycleItems[0]
	for i, name := range cycleItems {
		if gt.IgnoreCase(current, name) {
			next = cycleItems[(i+1)%len(cycleItems)]
			break
		}
	}
	for _, it := range inv {
		if gt.IgnoreCase(it.Name, next) {
			gt.Equip(it.ID)
			return
		}
	}
}
