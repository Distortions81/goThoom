//go:build script

package main

import (
	"strings"
	"time"

	"gt2"
)

// script metadata
const scriptName = "Iron Armor Manager"
const scriptID = "iron-armor"
const scriptAuthor = "Examples"
const scriptCategory = "Equipment"
const scriptAPIVersion = 2

var armorCondition string

// Init wires up commands, hotkeys, and a chat watcher for examine results.
func Init() {
	gt2.Command("ironarmortoggle", ironArmorToggleCmd)
	gt2.Command("examinearmor", examineArmorCmd)
	gt2.Bind("Ctrl-F10", ironArmorToggleHotkey)
	gt2.Bind("Ctrl-F11", examineArmorHotkey)
	gt2.OnChat(gt2.ChatFilter{}, armorChat)
}

func ironArmorToggleCmd(args string)       { ironArmorToggler() }
func examineArmorCmd(args string)          { examineArmor() }
func ironArmorToggleHotkey(gt2.InputEvent) { ironArmorToggler() }
func examineArmorHotkey(gt2.InputEvent)    { examineArmor() }
func armorChat(event gt2.ChatEvent)        { armorCondition = event.Raw }

func hasEquipped(name string) bool {
	for _, it := range gt2.EquippedItems() {
		if strings.EqualFold(it.Name, name) {
			return true
		}
	}
	return false
}

func ironArmorToggler() {
	if hasEquipped("iron breastplate") && hasEquipped("iron helmet") && hasEquipped("iron shield") {
		gt2.Send("/unequip ironbreastplate")
		gt2.Send("/unequip ironhelmet")
		gt2.Send("/unequip ironshield")
		return
	}
	equipIronArmor()
}

func equipIronArmor() {
	equipItem("iron breastplate", "ironbreastplate", "Iron Breastplate")
	equipItem("iron helmet", "ironhelmet", "Iron Helmet")
	equipItem("iron shield", "ironshield", "Iron Shield")
}

func equipItem(name, cmd, display string) {
	if hasEquipped(name) {
		return
	}
	gt2.Send("/equip " + cmd)
	gt2.Wait(100 * time.Millisecond)
	if !hasEquipped(name) {
		gt2.Send("/unequip " + cmd)
		gt2.Print("* " + display + " unequipped due to durability.")
	}
}

func examineArmor() {
	gt2.Print("* Armor Examiner:")
	if gt2.HasItem("iron breastplate") {
		gt2.Send("/examine ironbreastplate")
		gt2.Wait(100 * time.Millisecond)
		armorLabeler("5")
	}
	if gt2.HasItem("iron helmet") {
		gt2.Send("/examine ironhelmet")
		gt2.Wait(100 * time.Millisecond)
		armorLabeler("4")
	}
	if gt2.HasItem("iron shield") {
		gt2.Send("/examine ironshield")
		gt2.Wait(100 * time.Millisecond)
		armorLabeler("3")
	}
}

func armorLabeler(slot string) {
	lower := strings.ToLower(armorCondition)
	switch {
	case strings.Contains(lower, "perfect"):
		gt2.Send("/name " + slot + " (perfect)")
	case strings.Contains(lower, "good"):
		gt2.Send("/name " + slot + " (good)")
	case strings.Contains(lower, "look"):
		gt2.Send("/name " + slot + " (worn)")
	}
}
