//go:build script

package main

import (
	"fmt"
	"gt"
	"strconv"
	"time"
)

// script metadata
const scriptName = "Coin Lord"
const scriptAuthor = "Examples"
const scriptCategory = "Quality Of Life"
const scriptAPIVersion = 1

var (
	clRunning bool
	clTotal   int
	clStart   time.Time
)

func Init() {
	// Load saved totals.
	if s := gt.Load("total"); s != "" {
		if n, err := strconv.Atoi(s); err == nil {
			clTotal = n
		}
	}
	if s := gt.Load("start"); s != "" {
		if n, err := strconv.ParseInt(s, 10, 64); err == nil {
			clStart = time.Unix(n, 0)
		}
	}
	if clStart.IsZero() {
		clStart = time.Now()
	}

	// Toggle counting with /cw.
	gt.RegisterCommand("cw", cwCmd)

	// Reset the totals with /cwnew.
	gt.RegisterCommand("cwnew", cwNewCmd)

	// Clear the total with /cwclear.
	gt.RegisterCommand("cwclear", cwClearCmd)

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
		gt.Save("start", strconv.FormatInt(clStart.Unix(), 10))
		gt.Save("total", "0")
		gt.Print("Coin Lord started")
	} else {
		gt.Save("start", strconv.FormatInt(clStart.Unix(), 10))
		gt.Save("total", strconv.Itoa(clTotal))
		gt.Print("Coin Lord stopped")
	}
}

func cwNewCmd(args string) {
	clStart = time.Now()
	clTotal = 0
	gt.Save("start", strconv.FormatInt(clStart.Unix(), 10))
	gt.Save("total", "0")
	gt.Print("Coin data reset")
}

func cwClearCmd(args string) {
	clTotal = 0
	gt.Save("total", "0")
	gt.Print("Coin total cleared")
}

func cwDataCmd(args string) {
	hours := time.Since(clStart).Hours()
	rate := 0.0
	if hours > 0 {
		rate = float64(clTotal) / hours
	}
	gt.Print(fmt.Sprintf("Coins: %d (%.0f/hr)", clTotal, rate))
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
	gt.Save("total", strconv.Itoa(clTotal))
}
