//go:build script

package main

import "gt2"

const scriptName = "Server Event Scanner"
const scriptID = "server-event-scanner"
const scriptAuthor = "Examples"
const scriptCategory = "Tools"
const scriptAPIVersion = 2

func Init() {
	gt2.OnServerMessage(gt2.ServerMessageFilter{Type: "logon"}, rememberLogon)
	gt2.OnServerMessage(gt2.ServerMessageFilter{Type: "logoff"}, rememberLogoff)
	gt2.OnServerMessage(gt2.ServerMessageFilter{Type: "location"}, rememberLocation)
}

func rememberLogon(event gt2.ServerMessage) {
	gt2.Store("last-logon", event.Message)
}

func rememberLogoff(event gt2.ServerMessage) {
	gt2.Store("last-logoff", event.Message)
}

func rememberLocation(event gt2.ServerMessage) {
	gt2.Store("last-location", event.Message)
}
