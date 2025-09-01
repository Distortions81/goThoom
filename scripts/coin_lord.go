//go:build plugin

package main

import (
	"fmt"
	"gt"
	"strconv"
	"time"
)

// Plugin metadata
const PluginName = "Coin Lord"
const PluginAuthor = "Examples"
const PluginCategory = "Quality Of Life"
const PluginAPIVersion = 1

var (
	clRunning bool
	clTotal   int
	clStart   time.Time
)

func Init() {
	// Toggle counting with /cw.
	gt.RegisterCommand("cw", cwCmd)

	// Reset the totals with /cwnew.
	gt.RegisterCommand("cwnew", cwNewCmd)

	// Show current totals with /cwdata or Shift+C.
	gt.RegisterCommand("cwdata", cwDataCmd)

	// Tally coin messages like "You get 3 coins"
	gt.Chat("You get ", clHandle)
	// Use a function hotkey instead of running a slash command.
	gt.Key("Shift-C", cwDataHotkey)
}

func cwCmd(args string) {
	clRunning = !clRunning
	if clRunning {
		clStart = time.Now()
		clTotal = 0
		gt.Console("Coin Lord started")
	} else {
		gt.Console("Coin Lord stopped")
	}
}

func cwNewCmd(args string) {
	clStart = time.Now()
	clTotal = 0
	gt.Console("Coin data reset")
}

func cwDataCmd(args string) {
	hours := time.Since(clStart).Hours()
	rate := 0.0
	if hours > 0 {
		rate = float64(clTotal) / hours
	}
	gt.Console(fmt.Sprintf("Coins: %d (%.0f/hr)", clTotal, rate))
}

func cwDataHotkey() { cwDataCmd("") }

// clHandle watches chat for messages like "You get 3 coins" and tallies them.
func clHandle(msg string) {
	if !clRunning {
		return
	}
	if !gt.StartsWith(msg, "You get ") || !gt.Includes(msg, " coin") {
		return
	}
	fields := gt.Words(msg)
	if len(fields) < 3 {
		return
	}
	n, err := strconv.Atoi(fields[2])
	if err != nil {
		return
	}
	clTotal += n
}
