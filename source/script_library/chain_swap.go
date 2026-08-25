//go:build script

package main

import (
	"gt2"
	"strings"
	"time"
)

// script metadata
const scriptName = "Chain Swap"
const scriptID = "chain-swap"
const scriptAuthor = "Examples"
const scriptCategory = "Equipment"
const scriptAPIVersion = 2

var savedName string
var lastSwap time.Time

// Init wires up our command and mouse-wheel hotkeys.
func Init() {
	gt2.Command("swapchain", swapChainCmd)
	// Bind wheel to a simple function handler.
	gt2.Bind("WheelUp", swapChainHotkey)
	gt2.Bind("WheelDown", swapChainHotkey)
}

func swapChainCmd(args string)       { swapChain() }
func swapChainHotkey(gt2.InputEvent) { swapChain() }

// swapChain toggles between a chain weapon and whatever was equipped before.
func swapChain() {
	// Tiny debounce to avoid duplicate toggles on the same wheel action.
	if time.Since(lastSwap) < 40*time.Millisecond {
		return
	}
	lastSwap = time.Now()

	var chainName string
	var equippedName string
	for _, it := range gt2.Inventory() {
		if strings.EqualFold(it.Name, "chain") {
			chainName = it.Name
		}
		if it.Equipped && !strings.EqualFold(it.Name, "chain") {
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
		gt2.Equip(chainName)
	} else if savedName != "" {
		// Chain already equipped, so swap back.
		gt2.Equip(savedName)
	}
}
