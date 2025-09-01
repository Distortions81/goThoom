//go:build plugin

package main

import (
	"gt"
	"time"
)

// Plugin metadata
const PluginName = "Chain Swap"
const PluginAuthor = "Examples"
const PluginCategory = "Equipment"
const PluginAPIVersion = 1

var savedID uint16
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

	var chainID uint16
	var equipped *gt.InventoryItem
	for _, it := range gt.Inventory() {
		if gt.IgnoreCase(it.Name, "chain") {
			chainID = it.ID
		}
		if it.Equipped && !gt.IgnoreCase(it.Name, "chain") {
			item := it // capture for pointer
			equipped = &item
		}
	}
	if chainID == 0 {
		// No chain? Nothing to do.
		return
	}
	if equipped != nil {
		// Remember what we unequipped so we can switch back later.
		savedID = equipped.ID
		gt.Equip(chainID)
	} else if savedID != 0 {
		// Chain already equipped, so swap back.
		gt.Equip(savedID)
	}
}
