//go:build script

package main

import (
	"fmt"
	"math/rand"
	"strconv"

	"gt"
)

const scriptAuthor = "Examples"
const scriptCategory = "Fun"
const scriptAPIVersion = 1
const scriptName = "Dice Roller"

// Init registers the /roll command.
func Init() {
	gt.RegisterCommand("roll", roll)
}

func roll(args string) {
	args = gt.Trim(gt.Lower(args))
	if args == "" {
		gt.Print("usage: /roll NdM, e.g. /roll 2d6")
		return
	}
	parts := gt.Split(args, "d")
	if len(parts) != 2 {
		gt.Print("usage: /roll NdM, e.g. /roll 2d6")
		return
	}
	n := 1
	if parts[0] != "" {
		n, _ = strconv.Atoi(parts[0])
	}
	sides, _ := strconv.Atoi(parts[1])
	if n <= 0 || sides <= 0 {
		gt.Print("invalid dice")
		return
	}

	// Try to equip a dice item if present so others see it.
	for _, it := range gt.Inventory() {
		if gt.Includes(gt.Lower(it.Name), "dice") {
			if !it.Equipped {
				gt.Equip(it.Name)
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
