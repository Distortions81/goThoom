//go:build script

package main

import (
	"strings"

	"gt2"
)

// script metadata
const scriptName = "Weapon Cycle"
const scriptID = "weapon-cycle"
const scriptAuthor = "Examples"
const scriptCategory = "Equipment"
const scriptAPIVersion = 2

var cycleItems = []string{"Axe", "Short Sword", "Dagger", "Chocolate"}

// Init binds F3 to cycle through weapons.
func Init() {
	gt2.Command("cycleweapon", cycleWeaponCmd)
	gt2.Bind("F3", cycleWeaponHotkey)
}

func cycleWeaponCmd(args string)       { cycleWeapon() }
func cycleWeaponHotkey(gt2.InputEvent) { cycleWeapon() }

// cycleWeapon equips the next item in cycleItems.
func cycleWeapon() {
	inv := gt2.Inventory()
	current := ""
	for _, it := range inv {
		if it.Equipped {
			current = it.Name
			break
		}
	}
	next := cycleItems[0]
	for i, name := range cycleItems {
		if strings.EqualFold(current, name) {
			next = cycleItems[(i+1)%len(cycleItems)]
			break
		}
	}
	// Equip by name using the simplified API
	gt2.Equip(next)
}
