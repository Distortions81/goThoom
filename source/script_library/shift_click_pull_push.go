//go:build script

package main

import "gt2"

const scriptName = "Shift-Click Pull and Push"
const scriptID = "shift-click-pull-push"
const scriptAuthor = "Examples"
const scriptCategory = "Quality Of Life"
const scriptAPIVersion = 2

func Init() {
	gt2.Bind("Shift-LeftClick", pullPlayer)
	gt2.Bind("Shift-RightClick", pushPlayer)
}

func pullPlayer(event gt2.InputEvent) {
	if event.PlayerName == "" {
		return
	}
	gt2.Send("/pull " + event.SimpleName)
	event.Consume()
}

func pushPlayer(event gt2.InputEvent) {
	if event.PlayerName == "" {
		return
	}
	gt2.Send("/push " + event.SimpleName)
	event.Consume()
}
