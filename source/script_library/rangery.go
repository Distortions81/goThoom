//go:build script

package main

import "gt2"

const scriptName = "Rangery"
const scriptID = "rangery"
const scriptAuthor = "Examples"
const scriptCategory = "Profession"
const scriptAPIVersion = 2

func Init() {
	gt2.Bind("F3", useShieldstone)
	gt2.Bind("Shift-F3", removeShieldstone)
	gt2.Bind("F4", useHeartwood)
	gt2.Bind("WheelUp", useHeartwood)
	gt2.Bind("WheelDown", slowWithHeartwood)
}

func useShieldstone(event gt2.InputEvent) {
	if !gt2.IsEquipped("Shieldstone") {
		gt2.Equip("Shieldstone")
	}
	gt2.Send("/useitem Shieldstone")
	event.Consume()
}

func removeShieldstone(event gt2.InputEvent) {
	if !gt2.IsEquipped("Shieldstone") {
		return
	}
	gt2.Unequip("Shieldstone")
	event.Consume()
}

func useHeartwood(event gt2.InputEvent) {
	if gt2.Self().Spirit <= 0 {
		gt2.Print("Not enough spirit to use Heartwood.")
		return
	}
	if !gt2.IsEquipped("Heartwood Charm") {
		gt2.Equip("Heartwood Charm")
	}
	gt2.Send("/useitem Heartwood Charm")
	event.Consume()
}

func slowWithHeartwood(event gt2.InputEvent) {
	if gt2.Self().Spirit <= 0 {
		return
	}
	gt2.Send("/useitem Heartwood Charm /slow")
	event.Consume()
}
