//go:build plugin

package main

import (
	"fmt"
	"math/rand"
	"regexp"
	"strconv"

	"gt"
)

const PluginAuthor = "Examples"
const PluginCategory = "Fun"
const PluginAPIVersion = 1
const PluginName = "Dice Roller"

var diceRE = regexp.MustCompile(`(?i)^([0-9]*)d([0-9]+)$`)

// Init registers the /roll command.
func Init() {
	gt.RegisterCommand("roll", roll)
}

func roll(args string) {
	args = gt.Trim(args)
	if args == "" {
		gt.Console("usage: /roll NdM, e.g. /roll 2d6")
		return
	}
	m := diceRE.FindStringSubmatch(args)
	if m == nil {
		gt.Console("usage: /roll NdM, e.g. /roll 2d6")
		return
	}
	n := 1
	if m[1] != "" {
		n, _ = strconv.Atoi(m[1])
	}
	sides, _ := strconv.Atoi(m[2])
	if n <= 0 || sides <= 0 {
		gt.Console("invalid dice")
		return
	}

	// Try to equip a dice item if present so others see it.
	for _, it := range gt.Inventory() {
		if gt.Includes(gt.Lower(it.Name), "dice") {
			if !it.Equipped {
				gt.Equip(it.ID)
			}
			break
		}
	}

	rolls := make([]string, n)
	total := 0
	for i := 0; i < n; i++ {
		r := rand.Intn(sides) + 1
		rolls[i] = strconv.Itoa(r)
		total += r
	}
	gt.Run(fmt.Sprintf("/me rolls %s: %s (total %d)", args, gt.Join(rolls, " "), total))
}
