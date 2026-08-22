//go:build script

package main

import "gt"

// script metadata
const scriptName = "Weapon Cycle"
const scriptAuthor = "Examples"
const scriptCategory = "Equipment"
const scriptAPIVersion = 1

var cycleItems = []string{"Axe", "Short Sword", "Dagger", "Chocolate"}

// Init binds F3 to cycle through weapons.
func Init() {
	gt.RegisterCommand("cycleweapon", cycleWeaponCmd)
	gt.Key("F3", cycleWeapon)
}

func cycleWeaponCmd(args string) { cycleWeapon() }

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
	// Equip by name using the simplified API
	gt.Equip(next)
}
