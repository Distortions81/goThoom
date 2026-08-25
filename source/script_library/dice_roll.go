//go:build script

package main

import (
	"fmt"
	"math/rand"
	"strconv"
	"strings"

	"gt2"
)

const scriptAuthor = "Examples"
const scriptID = "dice-roll"
const scriptCategory = "Fun"
const scriptAPIVersion = 2
const scriptName = "Dice Roller"

// Init registers the /roll command.
func Init() {
	gt2.Command("roll", roll)
}

func roll(args string) {
	args = strings.TrimSpace(strings.ToLower(args))
	if args == "" {
		gt2.Print("usage: /roll NdM, e.g. /roll 2d6")
		return
	}
	parts := strings.Split(args, "d")
	if len(parts) != 2 {
		gt2.Print("usage: /roll NdM, e.g. /roll 2d6")
		return
	}
	n := 1
	if parts[0] != "" {
		n, _ = strconv.Atoi(parts[0])
	}
	sides, _ := strconv.Atoi(parts[1])
	if n <= 0 || sides <= 0 {
		gt2.Print("invalid dice")
		return
	}

	rolls := make([]string, n)
	total := 0
	for i := 0; i < n; i++ {
		r := rand.Intn(sides) + 1
		rolls[i] = strconv.Itoa(r)
		total += r
	}
	message := fmt.Sprintf("/me rolls %s: %s (total %d)", args, strings.Join(rolls, " "), total)
	send := func() {
		// Give the equipped dice one game tick to become visible before rolling.
		gt2.WaitTicks(1)
		gt2.Send(message)
	}
	if dice := gt2.SearchItems("dice"); len(dice) > 0 {
		gt2.WithEquipment(dice[0].Name, send)
		return
	}
	send()
}
