//go:build script

package main

import (
	"time"

	"gt"
)

// script metadata
const scriptName = "Iron Armor Manager"
const scriptAuthor = "Examples"
const scriptCategory = "Equipment"
const scriptAPIVersion = 1

var armorCondition string

// Init wires up commands, hotkeys, and a chat watcher for examine results.
func Init() {
	gt.RegisterCommand("ironarmortoggle", ironArmorToggleCmd)
	gt.RegisterCommand("examinearmor", examineArmorCmd)
	gt.AddHotkeyFn("Ctrl-F10", ironArmorToggleHotkey)
	gt.AddHotkeyFn("Ctrl-F11", examineArmorHotkey)
	gt.RegisterChatHandler(armorChat)
}

func ironArmorToggleCmd(args string)         { ironArmorToggler() }
func examineArmorCmd(args string)            { examineArmor() }
func ironArmorToggleHotkey(e gt.HotkeyEvent) { ironArmorToggler() }
func examineArmorHotkey(e gt.HotkeyEvent)    { examineArmor() }
func armorChat(msg string)                   { armorCondition = msg }

func hasEquipped(name string) bool {
	for _, it := range gt.EquippedItems() {
		if gt.IgnoreCase(it.Name, name) {
			return true
		}
	}
	return false
}

func ironArmorToggler() {
	if hasEquipped("iron breastplate") && hasEquipped("iron helmet") && hasEquipped("iron shield") {
		gt.Run("/unequip ironbreastplate")
		gt.Run("/unequip ironhelmet")
		gt.Run("/unequip ironshield")
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
	gt.Run("/equip " + cmd)
	time.Sleep(100 * time.Millisecond)
	if !hasEquipped(name) {
		gt.Run("/unequip " + cmd)
		gt.Print("* " + display + " unequipped due to durability.")
	}
}

func examineArmor() {
	gt.Print("* Armor Examiner:")
	if gt.HasItem("iron breastplate") {
		gt.Run("/examine ironbreastplate")
		time.Sleep(100 * time.Millisecond)
		armorLabeler("5")
	}
	if gt.HasItem("iron helmet") {
		gt.Run("/examine ironhelmet")
		time.Sleep(100 * time.Millisecond)
		armorLabeler("4")
	}
	if gt.HasItem("iron shield") {
		gt.Run("/examine ironshield")
		time.Sleep(100 * time.Millisecond)
		armorLabeler("3")
	}
}

func armorLabeler(slot string) {
	lower := gt.Lower(armorCondition)
	switch {
	case gt.Includes(lower, "perfect"):
		gt.Run("/name " + slot + " (perfect)")
	case gt.Includes(lower, "good"):
		gt.Run("/name " + slot + " (good)")
	case gt.Includes(lower, "look"):
		gt.Run("/name " + slot + " (worn)")
	}
}
