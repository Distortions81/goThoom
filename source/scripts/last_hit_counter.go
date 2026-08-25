//go:build script

package main

import (
	"fmt"

	"gt2"
)

const scriptName = "Last-Hit Counter"
const scriptID = "last-hit-counter"
const scriptAuthor = "Examples"
const scriptCategory = "Quality Of Life"
const scriptAPIVersion = 2

var enabled bool
var announceEvery int

func Init() {
	enabled = gt2.Bool(gt2.BoolOption{
		Key: "enabled", Label: "Count last hits", Scope: gt2.ScopeCharacter, Default: true,
		OnChange: func(value bool) { enabled = value },
	})
	announceEvery = gt2.Integer(gt2.IntegerOption{
		Key: "announce-every", Label: "Show total every", Help: "0 disables automatic messages.",
		Scope: gt2.ScopeCharacter, Default: 10, Min: 0, Max: 100, Step: 1,
		OnChange: func(value int) { announceEvery = value },
	})
	gt2.OnServerMessage(gt2.ServerMessageFilter{Type: "youkilled"}, countLastHit)
	gt2.Command("hits", showLastHits)
	gt2.Command("resethits", resetLastHits)
}

func countLastHit(event gt2.ServerMessage) {
	if !enabled {
		return
	}
	count := gt2.LoadInteger(lastHitKey(), 0) + 1
	gt2.Store(lastHitKey(), count)
	if announceEvery > 0 && count%announceEvery == 0 {
		gt2.Print(fmt.Sprintf("Last hits: %d", count))
	}
}

func showLastHits(args string) {
	gt2.Print(fmt.Sprintf("Last hits: %d", gt2.LoadInteger(lastHitKey(), 0)))
}

func resetLastHits(args string) {
	gt2.Store(lastHitKey(), 0)
	gt2.Print("Last-hit counter reset.")
}

func lastHitKey() string {
	return "last-hits:" + gt2.Self().Name
}
