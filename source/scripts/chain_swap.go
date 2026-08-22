//go:build script

package main

import (
	"gt"
	"time"
)

// script metadata
const scriptName = "Chain Swap"
const scriptAuthor = "Examples"
const scriptCategory = "Equipment"
const scriptAPIVersion = 1

var savedName string
var lastSwap time.Time

// Init wires up our command and mouse-wheel hotkeys.
func Init() {
	gt.RegisterCommand("swapchain", swapChainCmd)
	// Bind wheel to a simple function handler.
	gt.Key("WheelUp", swapChain)
	gt.Key("WheelDown", swapChain)
}

func swapChainCmd(args string) { swapChain() }

// swapChain toggles between a chain weapon and whatever was equipped before.
func swapChain() {
	// Tiny debounce to avoid duplicate toggles on the same wheel action.
	if time.Since(lastSwap) < 40*time.Millisecond {
		return
	}
	lastSwap = time.Now()

	var chainName string
	var equippedName string
	for _, it := range gt.Inventory() {
		if gt.IgnoreCase(it.Name, "chain") {
			chainName = it.Name
		}
		if it.Equipped && !gt.IgnoreCase(it.Name, "chain") {
			equippedName = it.Name
		}
	}
	if chainName == "" {
		// No chain found.
		return
	}
	if equippedName != "" {
		// Remember what we unequipped so we can switch back later.
		savedName = equippedName
		gt.Equip(chainName)
	} else if savedName != "" {
		// Chain already equipped, so swap back.
		gt.Equip(savedName)
	}
}
